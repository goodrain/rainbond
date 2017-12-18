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

package task

import (
	"crypto/sha1"
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	client "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/goodrain/rainbond/pkg/node/api/model"
	"github.com/goodrain/rainbond/pkg/node/core/job"
	"github.com/goodrain/rainbond/pkg/node/core/store"
	"github.com/pquerna/ffjson/ffjson"
)

//StartScheduler 开始调度
func (t *TaskEngine) startScheduler() {

}

func (t *TaskEngine) stopScheduler() {

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

//PutSchedul 发布调度需求，即定义task的某个执行节点
//taskID+nodeID = 一个调度单位,保证不重复
//node不能为空
func (t *TaskEngine) PutSchedul(taskID string, nodeID string) (err error) {
	if taskID == "" || nodeID == "" {
		return fmt.Errorf("taskid or nodeid can not be empty")
	}
	task := t.GetTask(taskID)
	if task == nil {
		return fmt.Errorf("task %s not found", taskID)
	}
	node := t.nodeCluster.GetNode(nodeID)
	if node == nil {
		return fmt.Errorf("node %s not found", nodeID)
	}
	hash := getHash(taskID, nodeID)
	logrus.Infof("scheduler hash %s", hash)
	var jb *job.Job
	if task.GroupID == "" {
		jb, err = job.CreateJobFromTask(task, nil)
		if err != nil {
			return fmt.Errorf("create job error,%s", err.Error())
		}
	} else {

	}

	return nil
}

func getHash(source ...string) string {
	h := sha1.New()
	for _, s := range source {
		h.Write([]byte(s))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
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
									taskSchedulerInfo.Status.SchedulerTime = time.Now()
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
							taskSchedulerInfo.Status.SchedulerTime = time.Now()
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
							taskSchedulerInfo.Status.SchedulerTime = time.Now()
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
					taskSchedulerInfo.Status.SchedulerTime = time.Now()
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
