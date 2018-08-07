// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package task

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/goodrain/rainbond/util/watch"

	"github.com/goodrain/rainbond/util"

	"github.com/goodrain/rainbond/util/etcd/etcdlock"

	"github.com/Sirupsen/logrus"
	client "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/api/model"
	"github.com/goodrain/rainbond/node/core/config"
	"github.com/goodrain/rainbond/node/core/job"
	"github.com/goodrain/rainbond/node/core/store"
	"github.com/goodrain/rainbond/node/masterserver/node"
	nodeclient "github.com/goodrain/rainbond/node/nodem/client"
	"github.com/goodrain/rainbond/node/nodem/taskrun"
	"github.com/pquerna/ffjson/ffjson"
)

// TaskEngine 任务引擎
// 处理任务的执行，结果处理，任务自动调度
// TODO:执行记录清理工作
type TaskEngine struct {
	ctx                 context.Context
	cancel              context.CancelFunc
	config              *option.Conf
	tasks               map[string]*model.Task
	jobs                taskrun.Jobs
	tasksLock, jobsLock sync.Mutex
	dataCenterConfig    *config.DataCenterConfig
	nodeCluster         *node.Cluster
	currentNode         *nodeclient.HostNode
	down                chan struct{}
	masterID            client.LeaseID
	scheduler           *Scheduler
	masterRun           bool
}

//CreateTaskEngine 创建task管理引擎
func CreateTaskEngine(nodeCluster *node.Cluster, node *nodeclient.HostNode) *TaskEngine {
	ctx, cancel := context.WithCancel(context.Background())
	task := &TaskEngine{
		ctx:              ctx,
		cancel:           cancel,
		tasks:            make(map[string]*model.Task),
		jobs:             make(taskrun.Jobs),
		config:           option.Config,
		dataCenterConfig: config.GetDataCenterConfig(),
		nodeCluster:      nodeCluster,
		currentNode:      node,
		down:             make(chan struct{}),
	}
	scheduler := createScheduler(task)
	task.scheduler = scheduler
	return task
}

//Start start
func (t *TaskEngine) Start(errchan chan error) error {
	//load all task and wath change event
	go t.loadAndWatchTasks(errchan)
	t.LoadStaticTask()
	go util.Exec(t.ctx, func() error {
		t.start(errchan)
		return nil
	}, 1)
	return nil
}
func (t *TaskEngine) start(errchan chan error) {
	master, err := etcdlock.CreateMasterLock(t.config.Etcd.Endpoints, "/rainbond/nodetaskscheduler", t.currentNode.HostName, 10)
	if err != nil {
		errchan <- err
		return
	}
	master.Start()
	defer master.Stop()
	for {
		select {
		case event := <-master.EventsChan():
			if event.Type == etcdlock.MasterAdded {
				logrus.Infof("Current node(%s) have task scheduler authority", t.currentNode.HostName)
				go t.startScheduler()
				go t.startHandleJobRecord()
				t.masterRun = true
			}
			if event.Type == etcdlock.MasterDeleted {
				if t.masterRun {
					errchan <- fmt.Errorf("master node delete")
				}
				return
			}
			if event.Type == etcdlock.MasterError {
				if event.Error.Error() == "elect: session expired" {
					//TODO:if etcd error. worker restart
				}
				//if this is master node, exit
				if t.masterRun {
					errchan <- event.Error
				}
				return
			}
		}
	}
}

//Stop task engine stop
func (t *TaskEngine) Stop() {
	t.cancel()
}

//watchTasks watchTasks
func (t *TaskEngine) loadAndWatchTasks(errChan chan error) {
	watcher := watch.New(store.DefalutClient.Client, "")
	taskwatchChan, err := watcher.WatchList(t.ctx, "/rainbond/store/tasks/", "")
	if err != nil {
		errChan <- err
	}
	defer taskwatchChan.Stop()
	for ev := range taskwatchChan.ResultChan() {
		switch ev.Type {
		case watch.Added, watch.Modified:
			task := new(model.Task)
			if err := task.Decode(ev.GetValue()); err != nil {
				logrus.Errorf("decode task info error :%s", err)
				continue
			}
			t.CacheTask(task)
		case watch.Deleted:
			task := new(model.Task)
			if err := task.Decode(ev.GetPreValue()); err != nil {
				logrus.Errorf("decode task info error :%s", err)
				continue
			}
			t.RemoveTask(task)
		case watch.Error:
			errChan <- ev.Error
		}
	}
}

func (t *TaskEngine) getTaskFromKey(key string) *model.Task {
	index := strings.LastIndex(key, "/")
	if index < 0 {
		return nil
	}
	id := key[index+1:]
	return t.GetTask(id)
}

//RemoveTask 从缓存移除task
func (t *TaskEngine) RemoveTask(task *model.Task) {
	t.tasksLock.Lock()
	defer t.tasksLock.Unlock()
	if _, ok := t.tasks[task.ID]; ok {
		delete(t.tasks, task.ID)
	}
}

func (t *TaskEngine) getTaskFromKV(kv *mvccpb.KeyValue) *model.Task {
	var task model.Task
	if err := ffjson.Unmarshal(kv.Value, &task); err != nil {
		logrus.Error("parse task info error:", err.Error())
		return nil
	}
	return &task
}

//LoadStaticTask 从文件加载task
//TODO:动态加载
func (t *TaskEngine) LoadStaticTask() {
	logrus.Infof("start load static task form path:%s", t.config.StaticTaskPath)
	file, err := os.Stat(t.config.StaticTaskPath)
	if err != nil {
		logrus.Errorf("load static task error %s", err.Error())
		return
	}
	if file.IsDir() {
		files, err := ioutil.ReadDir(t.config.StaticTaskPath)
		if err != nil {
			logrus.Errorf("load static task error %s", err.Error())
			return
		}
		for _, file := range files {
			if file.IsDir() {
				continue
			} else if strings.HasSuffix(file.Name(), ".json") {
				t.loadFile(path.Join(t.config.StaticTaskPath, file.Name()))
			}
		}
	} else {
		t.loadFile(t.config.StaticTaskPath)
	}
}

func (t *TaskEngine) loadFile(path string) {
	taskBody, err := ioutil.ReadFile(path)
	if err != nil {
		logrus.Errorf("read static task file %s error.%s", path, err.Error())
		return
	}
	var filename string
	index := strings.LastIndex(path, "/")
	if index < 0 {
		filename = path
	}
	filename = path[index+1:]
	if strings.Contains(filename, "group") {
		var group model.TaskGroup
		if err := ffjson.Unmarshal(taskBody, &group); err != nil {
			logrus.Errorf("unmarshal static task file %s error.%s", path, err.Error())
			return
		}
		if group.Name == "" {
			logrus.Errorf("task group name can not be empty. file %s", path)
			return
		}
		if group.ID == "" {
			group.ID = group.Name
		}
		if group.Tasks == nil {
			logrus.Errorf("task group tasks can not be empty. file %s", path)
			return
		}
		for _, task := range group.Tasks {
			task.GroupID = group.ID
			if task.Name == "" {
				logrus.Errorf("task name can not be empty. file %s", path)
				return
			}
			if task.ID == "" {
				task.ID = task.Name
			}
			if task.Temp == nil {
				logrus.Errorf("task [%s] temp can not be empty.", task.Name)
				return
			}
			if task.Temp.ID == "" {
				task.Temp.ID = task.Temp.Name
			}
			t.AddTask(task)
		}
		t.UpdateGroup(&group)
		logrus.Infof("Load a static group %s.", group.Name)
	}
	if strings.Contains(filename, "task") {
		var task model.Task
		if err := ffjson.Unmarshal(taskBody, &task); err != nil {
			logrus.Errorf("unmarshal static task file %s error.%s", path, err.Error())
			return
		}
		if task.Name == "" {
			logrus.Errorf("task name can not be empty. file %s", path)
			return
		}
		if task.ID == "" {
			task.ID = task.Name
		}
		if task.Temp == nil {
			logrus.Errorf("task [%s] temp can not be empty.", task.Name)
			return
		}
		if task.Temp.ID == "" {
			task.Temp.ID = task.Temp.Name
		}
		t.AddTask(&task)
		logrus.Infof("Load a static task %s.", task.Name)
	}
}

//GetTask gettask
func (t *TaskEngine) GetTask(taskID string) *model.Task {
	t.tasksLock.Lock()
	defer t.tasksLock.Unlock()
	if task, ok := t.tasks[taskID]; ok {
		return task
	}
	res, err := store.DefalutClient.Get("/rainbond/store/tasks/" + taskID)
	if err != nil {
		return nil
	}
	if res.Count < 1 {
		return nil
	}
	var task model.Task
	if err := ffjson.Unmarshal(res.Kvs[0].Value, &task); err != nil {
		return nil
	}
	if task.Status == nil {
		task.Status = map[string]model.TaskStatus{}
	}
	return &task
}

//AddTask 添加task
//新添加task
func (t *TaskEngine) AddTask(task *model.Task) error {
	oldTask := t.GetTask(task.ID)
	if oldTask != nil {
		task.Status = oldTask.Status
		task.OutPut = oldTask.OutPut
		task.EventID = oldTask.EventID
		task.CreateTime = oldTask.CreateTime
		task.ExecCount = oldTask.ExecCount
		task.StartTime = oldTask.StartTime
		if oldTask.Scheduler.Status != nil {
			task.Scheduler.Status = oldTask.Scheduler.Status
		}
	}
	if task.Temp == nil {
		return fmt.Errorf("task temp can not be nil")
	}
	if task.EventID == "" {
		task.EventID = task.ID
	}
	if len(task.Nodes) == 0 {
		task.Nodes = t.nodeCluster.GetLabelsNode(task.Temp.Labels)
	}
	if task.Status == nil {
		task.Status = map[string]model.TaskStatus{}
	}
	if task.CreateTime.IsZero() {
		task.CreateTime = time.Now()
	}
	if task.Temp.CreateTime.IsZero() {
		task.Temp.CreateTime = time.Now()
	}
	if task.Scheduler.Mode == "" {
		task.Scheduler.Mode = "Passive"
	}
	if task.RunMode == "" {
		task.RunMode = string(job.OnlyOnce)
	}
	t.CacheTask(task)
	_, err := store.DefalutClient.Put("/rainbond/store/tasks/"+task.ID, task.String())
	if err != nil {
		return err
	}
	if task.Scheduler.Mode == "Intime" {
		for _, node := range t.nodeCluster.GetLabelsNode(task.Temp.Labels) {
			t.PutSchedul(task.ID, node)
		}
	}
	return nil
}

//UpdateTask 更新task
func (t *TaskEngine) UpdateTask(task *model.Task) {
	t.tasksLock.Lock()
	defer t.tasksLock.Unlock()
	t.tasks[task.ID] = task
	_, err := store.DefalutClient.Put("/rainbond/store/tasks/"+task.ID, task.String())
	if err != nil {
		logrus.Errorf("update task error,%s", err.Error())
	}
}

//UpdateGroup 更新taskgroup
func (t *TaskEngine) UpdateGroup(group *model.TaskGroup) {
	group.Tasks = nil
	_, err := store.DefalutClient.Put("/rainbond/store/taskgroups/"+group.ID, group.String())
	if err != nil {
		logrus.Errorf("update taskgroup error,%s", err.Error())
	}
}

//GetTaskGroup 获取taskgroup
func (t *TaskEngine) GetTaskGroup(taskGroupID string) *model.TaskGroup {
	res, err := store.DefalutClient.Get("/rainbond/store/taskgroups/" + taskGroupID)
	if err != nil {
		return nil
	}
	if res.Count < 1 {
		return nil
	}
	var group model.TaskGroup
	if err := ffjson.Unmarshal(res.Kvs[0].Value, &group); err != nil {
		return nil
	}
	return &group
}

//CacheTask 缓存task
func (t *TaskEngine) CacheTask(task *model.Task) {
	t.tasksLock.Lock()
	defer t.tasksLock.Unlock()
	if task.Status == nil {
		task.Status = map[string]model.TaskStatus{}
	}
	t.tasks[task.ID] = task
}

//AddGroupConfig 添加组会话配置
func (t *TaskEngine) AddGroupConfig(groupID string, configs map[string]string) {
	ctx := t.dataCenterConfig.GetGroupConfig(groupID)
	for k, v := range configs {
		ctx.Add(k, v)
	}
}
