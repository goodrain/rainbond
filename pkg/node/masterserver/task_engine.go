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
	ctx                context.Context
	cancel             context.CancelFunc
	config             *option.Conf
	tasks              map[string]*model.Task
	tasksLock          sync.Mutex
	dataCenterConfig   *config.DataCenterConfig
	nodeCluster        *NodeCluster
	currentNode        *model.HostNode
	schedulerCache     map[string]bool
	schedulerCacheLock sync.Mutex
}

//TaskSchedulerInfo 请求调度信息
//指定任务到指定节点执行
//执行完成后该数据从集群中删除
//存储key: taskid+nodeid
type TaskSchedulerInfo struct {
	TaskID              string    `json:"taskID"`
	Node                string    `json:"node"`
	JobID               string    `json:"jobID"`
	CreateTime          time.Time `json:"create_time"`
	SchedulerMasterNode string    `json:"create_master_node"`
	Status              model.SchedulerStatus
}

//NewTaskSchedulerInfo 创建请求调度信息
func NewTaskSchedulerInfo(taskID, nodeID string) *TaskSchedulerInfo {
	return &TaskSchedulerInfo{
		TaskID:     taskID,
		Node:       nodeID,
		CreateTime: time.Now(),
	}
}
func getTaskSchedulerInfoFromKV(kv *mvccpb.KeyValue) *TaskSchedulerInfo {
	var taskinfo TaskSchedulerInfo
	if err := ffjson.Unmarshal(kv.Value, &taskinfo); err != nil {
		logrus.Error("parse task scheduler info error:", err.Error())
		return nil
	}
	return &taskinfo
}

//Post 发布
func (t TaskSchedulerInfo) Post() {
	body, err := ffjson.Marshal(t)
	if err == nil {
		store.DefalutClient.Post("/rainbond-node/scheduler/taskshcedulers/"+t.TaskID+"/"+t.Node, string(body))
		logrus.Infof("put a scheduler info %s:%s", t.TaskID, t.Node)
	}
}

//Update 更新数据
func (t TaskSchedulerInfo) Update() {
	body, err := ffjson.Marshal(t)
	if err == nil {
		store.DefalutClient.Put("/rainbond-node/scheduler/taskshcedulers/"+t.TaskID+"/"+t.Node, string(body))
	}
}

//Delete 删除数据
func (t TaskSchedulerInfo) Delete() {
	store.DefalutClient.Delete("/rainbond-node/scheduler/taskshcedulers/" + t.TaskID + "/" + t.Node)
}

//CreateTaskEngine 创建task管理引擎
func CreateTaskEngine(nodeCluster *NodeCluster, node *model.HostNode) *TaskEngine {
	ctx, cancel := context.WithCancel(context.Background())
	task := &TaskEngine{
		ctx:              ctx,
		cancel:           cancel,
		tasks:            make(map[string]*model.Task),
		config:           option.Config,
		dataCenterConfig: config.GetDataCenterConfig(),
		nodeCluster:      nodeCluster,
		currentNode:      node,
		schedulerCache:   make(map[string]bool),
	}
	return task
}

//Start 启动
func (t *TaskEngine) Start() {
	logrus.Info("task engine start")
	t.loadTask()
	go t.watchTasks()
	go t.HandleJobRecord()
	go t.watcheScheduler()
	t.LoadStaticTask()
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
func (t *TaskEngine) watcheScheduler() {
	load, _ := store.DefalutClient.Get("/rainbond-node/scheduler/taskshcedulers/", client.WithPrefix())
	if load != nil && load.Count > 0 {
		for _, kv := range load.Kvs {
			logrus.Debugf("watch a scheduler task %s", kv.Key)
			if taskinfo := getTaskSchedulerInfoFromKV(kv); taskinfo != nil {
				t.prepareScheduleTask(taskinfo)
			}
		}
	}
	ch := store.DefalutClient.Watch("/rainbond-node/scheduler/taskshcedulers/", client.WithPrefix())
	for {
		select {
		case <-t.ctx.Done():
			return
		case event := <-ch:
			for _, ev := range event.Events {
				switch {
				case ev.IsCreate(), ev.IsModify():
					logrus.Debugf("watch a scheduler task %s", ev.Kv.Key)
					if taskinfo := getTaskSchedulerInfoFromKV(ev.Kv); taskinfo != nil {
						t.prepareScheduleTask(taskinfo)
					}
				}
			}
		}
	}
}

//PutSchedul 发布请求调度信息
//同样请求将被拒绝，在上一次请求完成之前
//目前单节点调度，本地保证不重复调度
func (t *TaskEngine) PutSchedul(taskID string, node string) error {
	if node == "" {
		//执行节点确定
		task := t.GetTask(taskID)
		if task == nil {
			return fmt.Errorf("task (%s) can not be found", taskID)
		}
		if task.Temp == nil {
			return fmt.Errorf("task (%s) temp can not be found", taskID)
		}
		//task定义了执行节点
		if task.Nodes != nil && len(task.Nodes) > 0 {
			for _, node := range task.Nodes {
				if n := t.nodeCluster.GetNode(node); n != nil {
					info := NewTaskSchedulerInfo(taskID, node)
					info.Post()
				}

			}
		} else { //从lables决定执行节点
			nodes := t.nodeCluster.GetLabelsNode(task.Temp.Labels)
			for _, node := range nodes {
				if n := t.nodeCluster.GetNode(node); n != nil {
					info := NewTaskSchedulerInfo(taskID, node)
					info.Post()
				}
			}
		}
	} else {
		//保证同时只调度一次
		t.schedulerCacheLock.Lock()
		defer t.schedulerCacheLock.Unlock()
		if _, ok := t.schedulerCache[taskID+node]; ok {
			return nil
		}
		if n := t.nodeCluster.GetNode(node); n != nil {
			info := NewTaskSchedulerInfo(taskID, node)
			info.Post()
		}
		t.schedulerCache[taskID+node] = true
	}
	return nil
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
func (t *TaskEngine) StopTask(task *model.Task, node string) {
	if status, ok := task.Status[node]; ok {
		if status.JobID != "" {
			if task.IsOnce {
				_, err := store.DefalutClient.Delete(t.config.Once + "/" + status.JobID)
				if err != nil {
					logrus.Errorf("stop task %s error.%s", task.Name, err.Error())
				}
			} else {
				_, err := store.DefalutClient.Delete(t.config.Cmd + "/" + status.JobID)
				if err != nil {
					logrus.Errorf("stop task %s error.%s", task.Name, err.Error())
				}
			}
			_, err := store.DefalutClient.Delete(t.config.ExecutionRecordPath+"/"+status.JobID, client.WithPrefix())
			if err != nil {
				logrus.Errorf("delete execution record for task %s error.%s", task.Name, err.Error())
			}
		}
	}
	store.DefalutClient.Delete("/rainbond-node/scheduler/taskshcedulers/" + task.ID + "/" + node)
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
		task.Status = oldTask.Status
		task.OutPut = oldTask.OutPut
		task.EventID = oldTask.EventID
		task.CreateTime = oldTask.CreateTime
		task.ExecCount = oldTask.ExecCount
		task.StartTime = oldTask.StartTime
		if task.Scheduler.Status != nil {
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
		for _, n := range task.Nodes {
			task.Status[n] = model.TaskStatus{
				Status: "create",
			}
		}
	}
	if task.CreateTime.IsZero() {
		task.CreateTime = time.Now()
	}
	if task.Scheduler.Mode == "" {
		task.Scheduler.Mode = "Passive"
	}
	t.CacheTask(task)
	_, err := store.DefalutClient.Put("/store/tasks/"+task.ID, task.String())
	if err != nil {
		return err
	}
	if task.Scheduler.Mode == "Intime" {
		t.PutSchedul(task.ID, "")
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
		JobID:        er.JobID,
		StartTime:    er.BeginTime,
		EndTime:      er.EndTime,
		CompleStatus: "Failure",
	}
	if status, ok := task.Status[er.Node]; ok {
		taskStatus = status
		taskStatus.EndTime = time.Now()
		taskStatus.Status = "complete"
	}
	if er.Output != "" {
		index := strings.Index(er.Output, "{")
		jsonOutPut := er.Output
		if index > -1 {
			jsonOutPut = er.Output[index:]
		}
		output, err := model.ParseTaskOutPut(jsonOutPut)
		output.JobID = er.JobID
		if err != nil {
			taskStatus.Status = "Parse task output error"
			taskStatus.CompleStatus = "Unknow"
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
				if status.ConditionType != "" && status.ConditionStatus != "" {
					t.nodeCluster.UpdateNodeCondition(er.Node, status.ConditionType, status.ConditionStatus)
				}
				if status.NextTask != nil && len(status.NextTask) > 0 {
					for _, taskID := range status.NextTask {
						//由哪个节点发起的执行请求，当前task只在此节点执行
						t.PutSchedul(taskID, output.NodeID)
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
			taskStatus.CompleStatus = output.ExecStatus
		}
		task.UpdataOutPut(output)
	}
	task.ExecCount++
	if task.Status == nil {
		task.Status = make(map[string]model.TaskStatus)
	}
	task.Status[er.Node] = taskStatus
	//如果是is_once的任务，处理完成后删除job
	if task.IsOnce {
		task.CompleteTime = time.Now()
		t.StopTask(task, er.Node)
	} else { //如果是一次性任务，执行记录已经被删除，无需更新
		er.CompleteHandle()
	}
	t.schedulerCacheLock.Lock()
	defer t.schedulerCacheLock.Unlock()
	delete(t.schedulerCache, task.ID+er.Node)
}

//waitScheduleTask 等待调度条件成熟
func (t *TaskEngine) waitScheduleTask(taskSchedulerInfo *TaskSchedulerInfo, task *model.Task) {
	//continueScheduler 是否继续调度，如果调度条件无法满足，停止调度
	var continueScheduler = true
	canRun := func() bool {
		defer t.UpdateTask(task)
		if task.Temp.Depends != nil && len(task.Temp.Depends) > 0 {
			var result = make([]bool, len(task.Temp.Depends))
			for i, dep := range task.Temp.Depends {
				if depTask := t.GetTask(dep.DependTaskID); depTask != nil {
					//判断依赖任务调度情况
					if depTask.Scheduler.Mode == "Passive" {
						var needScheduler bool
						if depTask.Scheduler.Status == nil {
							needScheduler = true
						}
						//当前节点未调度且依赖策略为当前节点必须执行，则调度
						if _, ok := depTask.Scheduler.Status[taskSchedulerInfo.Node]; !ok && dep.DetermineStrategy == model.SameNodeStrategy {
							needScheduler = true
						}
						if needScheduler {
							//依赖任务i未就绪
							result[i] = false
							//发出依赖任务的调度请求
							if dep.DetermineStrategy == model.SameNodeStrategy {
								t.PutSchedul(depTask.ID, taskSchedulerInfo.Node)
							} else if dep.DetermineStrategy == model.AtLeastOnceStrategy {
								nodes := t.nodeCluster.GetLabelsNode(depTask.Temp.Labels)
								if len(nodes) > 0 {
									t.PutSchedul(depTask.ID, nodes[0])
								} else {
									taskSchedulerInfo.Status.Message = fmt.Sprintf("depend task %s can not found exec node", depTask.ID)
									taskSchedulerInfo.Status.Status = "Failure"
									task.Scheduler.Status[taskSchedulerInfo.Node] = taskSchedulerInfo.Status
									continueScheduler = false
									continue
								}
							}
							taskSchedulerInfo.Status.Message = fmt.Sprintf("depend task %s is not complete", depTask.ID)
							taskSchedulerInfo.Status.Status = "Waiting"
							task.Scheduler.Status[taskSchedulerInfo.Node] = taskSchedulerInfo.Status
							continue
						}
					}
					//判断依赖任务的执行情况
					//依赖策略为任务全局只要执行一次
					if dep.DetermineStrategy == model.AtLeastOnceStrategy {
						if depTask.Status == nil || len(depTask.Status) < 1 {
							taskSchedulerInfo.Status.Message = fmt.Sprintf("depend task %s is not complete", depTask.ID)
							taskSchedulerInfo.Status.Status = "Waiting"
							task.Scheduler.Status[taskSchedulerInfo.Node] = taskSchedulerInfo.Status
							return false
						}
						var access bool
						var faiiureSize int
						if len(depTask.Status) > 0 {
							for _, status := range depTask.Status {
								if status.CompleStatus == "Success" {
									logrus.Debugf("dep task %s ready", depTask.ID)
									access = true
								} else {
									faiiureSize++
								}
							}
						}
						//如果依赖的某个服务全部执行记录失败，条件不可能满足，本次调度结束
						if faiiureSize != 0 && faiiureSize >= len(depTask.Scheduler.Status) {
							taskSchedulerInfo.Status.Message = fmt.Sprintf("depend task %s Condition cannot be satisfied", depTask.ID)
							taskSchedulerInfo.Status.Status = "Failure"
							task.Scheduler.Status[taskSchedulerInfo.Node] = taskSchedulerInfo.Status
							continueScheduler = false
							return false
						}
						result[i] = access
					}
					//依赖任务相同节点执行成功
					if dep.DetermineStrategy == model.SameNodeStrategy {
						if depTask.Status == nil || len(depTask.Status) < 1 {
							taskSchedulerInfo.Status.Message = fmt.Sprintf("depend task %s is not complete", depTask.ID)
							taskSchedulerInfo.Status.Status = "Waiting"
							task.Scheduler.Status[taskSchedulerInfo.Node] = taskSchedulerInfo.Status
							return false
						}
						if nodestatus, ok := depTask.Status[taskSchedulerInfo.Node]; ok && nodestatus.CompleStatus == "Success" {
							result[i] = true
							continue
						} else if ok && nodestatus.CompleStatus != "" {
							taskSchedulerInfo.Status.Message = fmt.Sprintf("depend task %s(%s) Condition cannot be satisfied", depTask.ID, nodestatus.CompleStatus)
							taskSchedulerInfo.Status.Status = "Failure"
							task.Scheduler.Status[taskSchedulerInfo.Node] = taskSchedulerInfo.Status
							continueScheduler = false
							return false
						} else {
							taskSchedulerInfo.Status.Message = fmt.Sprintf("depend task %s is not complete", depTask.ID)
							taskSchedulerInfo.Status.Status = "Waiting"
							task.Scheduler.Status[taskSchedulerInfo.Node] = taskSchedulerInfo.Status
							return false
						}
					}
				} else {
					taskSchedulerInfo.Status.Message = fmt.Sprintf("depend task %s is not found", dep.DependTaskID)
					taskSchedulerInfo.Status.Status = "Failure"
					task.Scheduler.Status[taskSchedulerInfo.Node] = taskSchedulerInfo.Status
					result[i] = false
					continueScheduler = false
					return false
				}
			}
			for _, ok := range result {
				if !ok {
					return false
				}
			}
		}
		return true
	}
	for continueScheduler {
		if canRun() {
			t.scheduler(taskSchedulerInfo, task)
			return
		}
		logrus.Infof("task %s can not be run .waiting depend tasks complete", task.Name)
		time.Sleep(2 * time.Second)
	}
	//调度失败，删除任务
	taskSchedulerInfo.Delete()
}

//ScheduleTask 调度执行指定task
//单节点或不确定节点
func (t *TaskEngine) prepareScheduleTask(taskSchedulerInfo *TaskSchedulerInfo) {
	if task := t.GetTask(taskSchedulerInfo.TaskID); task != nil {
		if task == nil {
			return
		}
		//已经调度且没有完成
		if status, ok := task.Status[taskSchedulerInfo.Node]; ok && status.Status == "start" {
			logrus.Warningf("prepare scheduler task(%s) error,it already scheduler", taskSchedulerInfo.TaskID)
			return
		}
		if task.Temp == nil {
			logrus.Warningf("prepare scheduler task(%s) temp can not be nil", taskSchedulerInfo.TaskID)
			return
		}
		if task.Scheduler.Status == nil {
			task.Scheduler.Status = make(map[string]model.SchedulerStatus)
		}
		if task.Temp.Depends != nil && len(task.Temp.Depends) > 0 {
			go t.waitScheduleTask(taskSchedulerInfo, task)
		} else {
			//真正调度
			t.scheduler(taskSchedulerInfo, task)
		}
		t.UpdateTask(task)
	}
}

//scheduler 调度一个Task到一个节点执行
func (t *TaskEngine) scheduler(taskSchedulerInfo *TaskSchedulerInfo, task *model.Task) {
	j, err := job.CreateJobFromTask(task, nil)
	if err != nil {
		taskSchedulerInfo.Status.Status = "Failure"
		taskSchedulerInfo.Status.Message = err.Error()
		//更新调度状态
		task.Scheduler.Status[taskSchedulerInfo.Node] = taskSchedulerInfo.Status
		t.UpdateTask(task)
		logrus.Errorf("run task %s error.%s", task.Name, err.Error())
		return
	}
	//如果指定nodes
	if taskSchedulerInfo.Node != "" {
		for _, rule := range j.Rules {
			rule.NodeIDs = []string{taskSchedulerInfo.Node}
		}
	}
	if j.IsOnce {
		if err := job.PutOnce(j); err != nil {
			taskSchedulerInfo.Status.Status = "Failure"
			taskSchedulerInfo.Status.Message = err.Error()
			//更新调度状态
			task.Scheduler.Status[taskSchedulerInfo.Node] = taskSchedulerInfo.Status
			t.UpdateTask(task)
			logrus.Errorf("run task %s error.%s", task.Name, err.Error())
			return
		}
	} else {
		if err := job.AddJob(j); err != nil {
			taskSchedulerInfo.Status.Status = "Failure"
			taskSchedulerInfo.Status.Message = err.Error()
			//更新调度状态
			task.Scheduler.Status[taskSchedulerInfo.Node] = taskSchedulerInfo.Status
			t.UpdateTask(task)
			logrus.Errorf("run task %s error.%s", task.Name, err.Error())
			return
		}
	}
	task.StartTime = time.Now()
	taskSchedulerInfo.Status.Status = "Success"
	taskSchedulerInfo.Status.Message = "scheduler success"
	taskSchedulerInfo.Status.SchedulerTime = time.Now()
	taskSchedulerInfo.Status.SchedulerMaster = t.currentNode.ID
	//更新调度状态
	task.Scheduler.Status[taskSchedulerInfo.Node] = taskSchedulerInfo.Status
	if task.Status == nil {
		task.Status = make(map[string]model.TaskStatus)
	}
	task.Status[t.currentNode.ID] = model.TaskStatus{
		JobID:     j.ID,
		StartTime: time.Now(),
		Status:    "Start",
	}
	t.UpdateTask(task)
	logrus.Infof("success scheduler a task %s to node %s", task.Name, taskSchedulerInfo.Node)
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
