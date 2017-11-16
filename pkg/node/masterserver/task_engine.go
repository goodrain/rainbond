// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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

package masterserver

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/goodrain/rainbond/pkg/node/core/config"
	"github.com/goodrain/rainbond/pkg/node/core/store"

	"github.com/pquerna/ffjson/ffjson"

	"github.com/Sirupsen/logrus"

	client "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/node/api/model"
	"github.com/goodrain/rainbond/pkg/node/core/job"
)

//TaskEngine 任务引擎
// 处理任务的执行，结果处理，任务自动调度
// TODO:执行记录清理工作
type TaskEngine struct {
	ctx              context.Context
	cancel           context.CancelFunc
	config           *option.Conf
	tasks            map[string]*model.Task
	tasksLock        sync.Mutex
	dataCenterConfig *config.DataCenterConfig
	nodeCluster      *NodeCluster
}

//CreateTaskEngine 创建task管理引擎
func CreateTaskEngine(nodeCluster *NodeCluster) *TaskEngine {
	ctx, cancel := context.WithCancel(context.Background())
	task := &TaskEngine{
		ctx:              ctx,
		cancel:           cancel,
		tasks:            make(map[string]*model.Task),
		config:           option.Config,
		dataCenterConfig: config.GetDataCenterConfig(),
		nodeCluster:      nodeCluster,
	}
	task.loadTask()
	task.LoadStaticTask()
	return task
}

//Start 启动
func (t *TaskEngine) Start() {
	logrus.Info("task engine start")
	go t.HandleJobRecord()
	go t.watchTasks()
}

//Stop 启动
func (t *TaskEngine) Stop() {
	t.cancel()
}

//loadTask 加载所有task
func (t *TaskEngine) loadTask() error {
	//加载节点信息
	res, err := store.DefalutClient.Get("/store/tasks/", client.WithPrefix())
	if err != nil {
		return fmt.Errorf("load tasks error:%s", err.Error())
	}
	for _, kv := range res.Kvs {
		if task := t.getTaskFromKV(kv); task != nil {
			t.CacheTask(task)
		}
	}
	return nil
}

//watchTasks watchTasks
func (t *TaskEngine) watchTasks() {
	ch := store.DefalutClient.Watch("/store/tasks/", client.WithPrefix())
	for {
		select {
		case <-t.ctx.Done():
			return
		case event := <-ch:
			for _, ev := range event.Events {
				switch {
				case ev.IsCreate(), ev.IsModify():
					if task := t.getTaskFromKV(ev.Kv); task != nil {
						t.CacheTask(task)
					}
				case ev.Type == client.EventTypeDelete:
					if task := t.getTaskFromKey(string(ev.Kv.Key)); task != nil {
						t.RemoveTask(task)
					}
				}
			}
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
		if group.ID == "" {
			group.ID = group.Name
		}
		if group.Name == "" {
			logrus.Errorf("task group name can not be empty. file %s", path)
			return
		}
		if group.Tasks == nil {
			logrus.Errorf("task group tasks can not be empty. file %s", path)
			return
		}
		t.ScheduleGroup(nil, &group)
		logrus.Infof("Load a static group %s.", group.Name)
	}
	if strings.Contains(filename, "task") {
		var task model.Task
		if err := ffjson.Unmarshal(taskBody, &task); err != nil {
			logrus.Errorf("unmarshal static task file %s error.%s", path, err.Error())
			return
		}
		if task.ID == "" {
			task.ID = task.Name
		}
		if task.Name == "" {
			logrus.Errorf("task name can not be empty. file %s", path)
			return
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
	res, err := store.DefalutClient.Get("/store/tasks/" + taskID)
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
	return &task
}

//StopTask 停止任务，即删除任务对应的JOB
func (t *TaskEngine) StopTask(task *model.Task) {
	if task.JobID != "" {
		if task.IsOnce {
			_, err := store.DefalutClient.Delete(t.config.Once + "/" + task.JobID)
			if err != nil {
				logrus.Errorf("stop task %s error.%s", task.Name, err.Error())
			}
		} else {
			_, err := store.DefalutClient.Delete(t.config.Cmd + "/" + task.JobID)
			if err != nil {
				logrus.Errorf("stop task %s error.%s", task.Name, err.Error())
			}
		}
		_, err := store.DefalutClient.Delete(t.config.ExecutionRecordPath+"/"+task.JobID, client.WithPrefix())
		if err != nil {
			logrus.Errorf("delete execution record for task %s error.%s", task.Name, err.Error())
		}
	}
}

//CacheTask 缓存task
func (t *TaskEngine) CacheTask(task *model.Task) {
	t.tasksLock.Lock()
	defer t.tasksLock.Unlock()
	t.tasks[task.ID] = task
}

//AddTask 添加task
func (t *TaskEngine) AddTask(task *model.Task) error {
	oldTask := t.GetTask(task.ID)
	if oldTask != nil {
		if oldTask.JobID != "" {
			task.JobID = oldTask.JobID
			task.Status = oldTask.Status
			task.OutPut = oldTask.OutPut
			task.EventID = oldTask.EventID
		}
	}
	if task.EventID == "" {
		task.EventID = task.ID
	}
	task.Status = map[string]model.TaskStatus{}
	for _, n := range task.Nodes {
		task.Status[n] = model.TaskStatus{
			Status: "create",
		}
	}
	task.CreateTime = time.Now()
	if task.Scheduler.Mode == "" {
		task.Scheduler.Mode = "Passive"
	}
	t.CacheTask(task)
	_, err := store.DefalutClient.Put("/store/tasks/"+task.ID, task.String())
	if err != nil {
		return err
	}
	if task.Scheduler.Mode == "Intime" {
		t.ScheduleTask(nil, task)
	}
	return nil
}

//UpdateTask 更新task
func (t *TaskEngine) UpdateTask(task *model.Task) {
	t.tasksLock.Lock()
	defer t.tasksLock.Unlock()
	t.tasks[task.ID] = task
	_, err := store.DefalutClient.Put("/store/tasks/"+task.ID, task.String())
	if err != nil {
		logrus.Errorf("update task error,%s", err.Error())
	}
}

//UpdateGroup 更新taskgroup
func (t *TaskEngine) UpdateGroup(group *model.TaskGroup) {
	_, err := store.DefalutClient.Put("/store/taskgroups/"+group.ID, group.String())
	if err != nil {
		logrus.Errorf("update taskgroup error,%s", err.Error())
	}
}

//GetTaskGroup 获取taskgroup
func (t *TaskEngine) GetTaskGroup(taskGroupID string) *model.TaskGroup {
	res, err := store.DefalutClient.Get("/store/taskgroups/" + taskGroupID)
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

//handleJobRecord 处理
func (t *TaskEngine) handleJobRecord(er *job.ExecutionRecord) {
	task := t.GetTask(er.TaskID)
	if task == nil {
		return
	}
	//更新task信息
	defer t.UpdateTask(task)
	taskStatus := model.TaskStatus{
		StartTime:    er.BeginTime,
		EndTime:      er.EndTime,
		CompleStatus: "",
	}
	if er.Output != "" {
		output, err := model.ParseTaskOutPut(er.Output)
		if err != nil {
			taskStatus.Status = "Parse task output error"
			logrus.Warning("parse task output error:", err.Error())
			output.NodeID = er.Node

		} else {
			output.NodeID = er.Node
			if output.Global != nil && len(output.Global) > 0 {
				for k, v := range output.Global {
					err := t.dataCenterConfig.PutConfig(&model.ConfigUnit{
						Name:           strings.ToUpper(k),
						Value:          v,
						ValueType:      "string",
						IsConfigurable: false,
					})
					if err != nil {
						logrus.Errorf("save datacenter config %s=%s error.%s", k, v, err.Error())
					}
				}
			}
			//groupID不为空，处理group连环操作
			if output.Inner != nil && len(output.Inner) > 0 && task.GroupID != "" {
				t.AddGroupConfig(task.GroupID, output.Inner)
			}
			for _, status := range output.Status {
				//install or check类型结果写入节点
				if output.Type == "install" || output.Type == "check" {
					if status.ConditionType != "" && status.ConditionStatus != "" {
						t.nodeCluster.UpdateNodeCondition(er.Node, status.ConditionType, status.ConditionStatus)
					}
					if status.NextTask != nil && len(status.NextTask) > 0 {
						for _, taskID := range status.NextTask {
							task := t.GetTask(taskID)
							if task == nil {
								continue
							}
							//由哪个节点发起的执行请求，当前task只在此节点执行
							t.ScheduleTask([]string{output.NodeID}, task)
						}
					}
					if status.NextGroups != nil && len(status.NextGroups) > 0 {
						for _, groupID := range status.NextGroups {
							group := t.GetTaskGroup(groupID)
							if group == nil {
								continue
							}
							t.ScheduleGroup([]string{output.NodeID}, group)
						}
					}
				}
			}
		}
		task.UpdataOutPut(output)
	}
	if er.Success {
		taskStatus.CompleStatus = "Success"
	} else {
		taskStatus.CompleStatus = "Failure"
	}
	if task.Status == nil {
		task.Status = make(map[string]model.TaskStatus)
	}
	task.Status[er.Node] = taskStatus
	//如果是is_once的任务，处理完成后删除job
	if task.IsOnce {
		task.CompleteTime = time.Now()
		t.StopTask(task)
	} else { //如果是一次性任务，执行记录已经被删除，无需更新
		er.CompleteHandle()
	}
}
func (t *TaskEngine) waitScheduleTask(nodes []string, task *model.Task) {
	canRun := func() bool {
		defer t.UpdateTask(task)
		if task.Temp.Depends != nil && len(task.Temp.Depends) > 0 {
			var result = true
			for _, dep := range task.Temp.Depends {
				if depTask := t.GetTask(dep.DependTaskID); depTask != nil {
					if depTask.Scheduler.Mode == "Passive" && depTask.Scheduler.Status == "" {
						t.ScheduleTask(nodes, depTask)
					}
					if dep.DetermineStrategy == model.AtLeastOnceStrategy {
						if len(depTask.Status) > 0 {
							result = result && true
						}
					}
					if dep.DetermineStrategy == model.SameNodeStrategy {
						if depTask.Status == nil || len(depTask.Status) < 1 {
							result = result && false
							task.Scheduler.Message = fmt.Sprintf("depend task %s is not complete", depTask.ID)
							task.Scheduler.Status = "Waiting"
							return false
						}
						if nodes != nil {
							for _, node := range nodes {
								if nodestatus, ok := depTask.Status[node]; !ok || nodestatus.EndTime.IsZero() {
									result = result && false
									task.Scheduler.Message = fmt.Sprintf("depend task %s is not complete in node %s", depTask.ID, node)
									task.Scheduler.Status = "Waiting"
									return false
								}
							}
						} else {
							return true
						}
					}
				} else {
					task.Scheduler.Message = fmt.Sprintf("depend task %s is not found", depTask.ID)
					task.Scheduler.Status = "Failure"
					result = result && false
					return false
				}
			}
			return result
		}
		return true
	}
	for {
		logrus.Infof("task %s can not be run .waiting depend tasks complete", task.Name)
		if canRun() {
			j, err := job.CreateJobFromTask(task, nil)
			if err != nil {
				task.Scheduler.Status = "Failure"
				task.Scheduler.Message = err.Error()
				t.UpdateTask(task)
				logrus.Errorf("run task %s error.%s", task.Name, err.Error())
				return
			}
			//如果指定nodes
			if nodes != nil {
				for _, rule := range j.Rules {
					rule.NodeIDs = nodes
				}
			}
			if j.IsOnce {
				if err := job.PutOnce(j); err != nil {
					task.Scheduler.Status = "Failure"
					task.Scheduler.Message = err.Error()
					t.UpdateTask(task)
					logrus.Errorf("run task %s error.%s", task.Name, err.Error())
					return
				}
			} else {
				if err := job.AddJob(j); err != nil {
					task.Scheduler.Status = "Failure"
					task.Scheduler.Message = err.Error()
					t.UpdateTask(task)
					logrus.Errorf("run task %s error.%s", task.Name, err.Error())
					return
				}
			}
			task.JobID = j.ID
			task.StartTime = time.Now()
			task.Scheduler.Status = "Success"
			task.Scheduler.Message = "scheduler success"
			t.UpdateTask(task)
			return
		}
		time.Sleep(2 * time.Second)
	}
}

//ScheduleTask 调度执行指定task
func (t *TaskEngine) ScheduleTask(nodes []string, nextTask ...*model.Task) {
	for _, task := range nextTask {
		if task == nil {
			continue
		}
		if task.JobID != "" {
			t.StopTask(task)
		}
		if task.Temp == nil {
			continue
		}
		if nodes == nil {
			nodes = task.Nodes
		}
		if task.Temp.Depends != nil && len(task.Temp.Depends) > 0 {
			go t.waitScheduleTask(nodes, task)
			task.Scheduler.Status = "Waiting"
		} else {
			logrus.Infof("scheduler a task %s", task.Name)
			j, err := job.CreateJobFromTask(task, nil)
			if err != nil {
				task.Scheduler.Status = "Failure"
				task.Scheduler.Message = err.Error()
				t.UpdateTask(task)
				logrus.Errorf("run task %s error.%s", task.Name, err.Error())
				return
			}
			//如果指定nodes
			if nodes != nil {
				for _, rule := range j.Rules {
					rule.NodeIDs = nodes
				}
			}
			if j.IsOnce {
				if err := job.PutOnce(j); err != nil {
					task.Scheduler.Status = "Failure"
					task.Scheduler.Message = err.Error()
					t.UpdateTask(task)
					logrus.Errorf("run task %s error.%s", task.Name, err.Error())
					return
				}
			} else {
				if err := job.AddJob(j); err != nil {
					task.Scheduler.Status = "Failure"
					task.Scheduler.Message = err.Error()
					t.UpdateTask(task)
					logrus.Errorf("run task %s error.%s", task.Name, err.Error())
					return
				}
			}
			task.JobID = j.ID
			task.StartTime = time.Now()
			task.Scheduler.Status = "Success"
			task.Scheduler.Message = "scheduler success"
		}
		t.UpdateTask(task)
	}
}

//ScheduleGroup 调度执行指定task
func (t *TaskEngine) ScheduleGroup(nodes []string, nextGroups ...*model.TaskGroup) {
	for _, group := range nextGroups {
		if group.Tasks == nil || len(group.Tasks) < 1 {
			group.Status = &model.TaskGroupStatus{
				StartTime: time.Now(),
				EndTime:   time.Now(),
				Status:    "NotDefineTask",
			}
			t.UpdateGroup(group)
		}
		for _, task := range group.Tasks {
			task.GroupID = group.ID
			t.AddTask(task)
		}
		group.Status = &model.TaskGroupStatus{
			StartTime: time.Now(),
			Status:    "Start",
		}
		t.UpdateGroup(group)
	}
}

//AddGroupConfig 添加组会话配置
func (t *TaskEngine) AddGroupConfig(groupID string, configs map[string]string) {
	ctx := config.NewGroupContext(groupID)
	for k, v := range configs {
		ctx.Add(k, v)
	}
}

//HandleJobRecord 处理task执行记录
func (t *TaskEngine) HandleJobRecord() {
	jobRecord := t.loadJobRecord()
	if jobRecord != nil {
		for _, er := range jobRecord {
			if !er.IsHandle {
				t.handleJobRecord(er)
			}
		}
	}
	ch := store.DefalutClient.Watch(t.config.ExecutionRecordPath, client.WithPrefix())
	for {
		select {
		case <-t.ctx.Done():
			return
		case event := <-ch:
			for _, ev := range event.Events {
				switch {
				case ev.IsCreate():
					var er job.ExecutionRecord
					if err := ffjson.Unmarshal(ev.Kv.Value, &er); err == nil {
						if !er.IsHandle {
							t.handleJobRecord(&er)
						}
					}
				}
			}
		}
	}
}
func (t *TaskEngine) loadJobRecord() (ers []*job.ExecutionRecord) {
	res, err := store.DefalutClient.Get(t.config.ExecutionRecordPath, client.WithPrefix())
	if err != nil {
		logrus.Error("load job execution record error.", err.Error())
		return nil
	}
	for _, re := range res.Kvs {
		var er job.ExecutionRecord
		if err := ffjson.Unmarshal(re.Value, &er); err == nil {
			ers = append(ers, &er)
		}
	}
	return
}
