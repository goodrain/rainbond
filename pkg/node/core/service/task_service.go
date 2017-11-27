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

package service

import (
	"fmt"
	"time"

	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/node/api/model"
	"github.com/goodrain/rainbond/pkg/node/core/store"
	"github.com/goodrain/rainbond/pkg/node/masterserver"
	"github.com/goodrain/rainbond/pkg/node/utils"

	"github.com/Sirupsen/logrus"
	"github.com/pquerna/ffjson/ffjson"

	"github.com/coreos/etcd/clientv3"

	"github.com/twinj/uuid"
)

//TaskService 处理taskAPI
type TaskService struct {
	SavePath string
	conf     *option.Conf
	ms       *masterserver.MasterServer
}

var taskService *TaskService

//CreateTaskService 创建Task service
func CreateTaskService(c *option.Conf, ms *masterserver.MasterServer) *TaskService {
	if taskService == nil {
		taskService = &TaskService{
			SavePath: "/store/tasks",
			conf:     c,
			ms:       ms,
		}
	}
	return taskService
}

//AddTask add task
func (ts *TaskService) AddTask(t *model.Task) *utils.APIHandleError {
	if t.ID == "" {
		t.ID = uuid.NewV4().String()
	}
	if len(t.Nodes) < 1 {
		return utils.CreateAPIHandleError(400, fmt.Errorf("task exec nodes can not be empty"))
	}
	if t.TempID == "" && t.Temp == nil {
		return utils.CreateAPIHandleError(400, fmt.Errorf("task temp can not be empty"))
	}
	if t.TempID != "" {
		//TODO:确定是否应该在执行时获取最新TEMP
		temp, err := taskTempService.GetTaskTemp(t.TempID)
		if err != nil {
			return err
		}
		t.Temp = temp
	}
	if t.Temp == nil {
		return utils.CreateAPIHandleError(400, fmt.Errorf("task temp can not be empty"))
	}
	t.Status = map[string]model.TaskStatus{}
	for _, n := range t.Nodes {
		t.Status[n] = model.TaskStatus{
			Status: "create",
		}
	}
	t.CreateTime = time.Now()
	_, err := store.DefalutClient.Put(ts.SavePath+"/"+t.ID, t.String())
	if err != nil {
		return utils.CreateAPIHandleErrorFromDBError("save task", err)
	}
	return nil
}

//GetTasks get tasks
func (ts *TaskService) GetTasks() ([]*model.Task, *utils.APIHandleError) {
	res, err := store.DefalutClient.Get(ts.SavePath, clientv3.WithPrefix())
	if err != nil {
		return nil, utils.CreateAPIHandleErrorFromDBError("save task", err)
	}
	if res.Count == 0 {
		return nil, nil
	}
	var tasks []*model.Task
	for _, kv := range res.Kvs {
		var t model.Task
		if err = ffjson.Unmarshal(kv.Value, &t); err != nil {
			logrus.Errorf("unmarshal task info from etcd value error:%s", err.Error())
			continue
		}
		tasks = append(tasks, &t)
	}
	if len(tasks) < 1 {
		return nil, utils.CreateAPIHandleError(500, err)
	}
	return tasks, nil
}

//GetTask 获取Task
func (ts *TaskService) GetTask(taskID string) (*model.Task, *utils.APIHandleError) {
	res, err := store.DefalutClient.Get(ts.SavePath + "/" + taskID)
	if err != nil {
		return nil, utils.CreateAPIHandleErrorFromDBError("save task", err)
	}
	if res.Count == 0 {
		return nil, utils.CreateAPIHandleError(404, fmt.Errorf("task not found"))
	}
	var task model.Task
	for _, kv := range res.Kvs {
		if err = ffjson.Unmarshal(kv.Value, &task); err != nil {
			logrus.Errorf("unmarshal task info from etcd value error:%s", err.Error())
			return nil, utils.CreateAPIHandleError(500, fmt.Errorf("unmarshal task value error %s", err.Error()))
		}
		break
	}
	return &task, nil
}

//DeleteTask 删除Task
func (ts *TaskService) DeleteTask(taskID string) *utils.APIHandleError {
	task, err := ts.GetTask(taskID)
	if err != nil {
		return err
	}
	if ok := task.CanBeDelete(); !ok {
		return utils.CreateAPIHandleError(400, fmt.Errorf("this task is not be deleted"))
	}
	_, er := store.DefalutClient.Delete(ts.SavePath+"/"+taskID, clientv3.WithPrefix())
	if er != nil {
		return utils.CreateAPIHandleErrorFromDBError("delete task", er)
	}
	return nil
}

//ExecTask 执行任务API处理
func (ts *TaskService) ExecTask(taskID string, nodes []string) *utils.APIHandleError {
	t, err := ts.GetTask(taskID)
	if err != nil {
		return err
	}
	//var depTasks []*model.Task
	if len(t.Temp.Depends) > 0 {
		for _, dep := range t.Temp.Depends {
			_, err := ts.GetTask(dep.DependTaskID)
			if err != nil || t == nil {
				return utils.CreateAPIHandleError(400, fmt.Errorf("depend task %s not found", dep))
			}
		}
	}
	if nodes == nil || len(nodes) == 0 {
		ts.ms.TaskEngine.PutSchedul(taskID, "")
	} else {
		for _, node := range nodes {
			if n := ts.ms.Cluster.GetNode(node); n == nil {
				return utils.CreateAPIHandleError(400, fmt.Errorf(" exec node  %s not found", node))
			}
		}
		for _, node := range nodes {
			ts.ms.TaskEngine.PutSchedul(taskID, node)
		}
	}
	return nil
}

//ReloadStaticTasks reload task
func (ts *TaskService) ReloadStaticTasks() {
	ts.ms.TaskEngine.LoadStaticTask()
}

//TaskTempService 任务模版
type TaskTempService struct {
	SavePath string
	conf     *option.Conf
}

var taskTempService *TaskTempService

//CreateTaskTempService 创建Task service
func CreateTaskTempService(c *option.Conf) *TaskTempService {
	if taskTempService == nil {
		taskTempService = &TaskTempService{
			SavePath: "/store/tasks",
			conf:     c,
		}
	}
	return taskTempService
}

//SaveTaskTemp add task
func (ts *TaskTempService) SaveTaskTemp(t *model.TaskTemp) *utils.APIHandleError {
	if t.ID == "" {
		t.ID = uuid.NewV4().String()
	}
	if t.CreateTime.IsZero() {
		t.CreateTime = time.Now()
	}
	_, err := store.DefalutClient.Put(ts.SavePath+"/"+t.ID, t.String())
	if err != nil {
		return utils.CreateAPIHandleErrorFromDBError("save tasktemp", err)
	}
	return nil
}

//GetTaskTemp add task
func (ts *TaskTempService) GetTaskTemp(tempID string) (*model.TaskTemp, *utils.APIHandleError) {
	res, err := store.DefalutClient.Get(ts.SavePath + "/" + tempID)
	if err != nil {
		return nil, utils.CreateAPIHandleErrorFromDBError("save task", err)
	}
	if res.Count == 0 {
		return nil, utils.CreateAPIHandleError(404, fmt.Errorf("task temp not found"))
	}
	var task model.TaskTemp
	for _, kv := range res.Kvs {
		if err = ffjson.Unmarshal(kv.Value, &task); err != nil {
			logrus.Errorf("unmarshal task info from etcd value error:%s", err.Error())
			return nil, utils.CreateAPIHandleError(500, fmt.Errorf("unmarshal task value error %s", err.Error()))
		}
		break
	}
	return &task, nil
}

//DeleteTaskTemp 删除任务模版
func (ts *TaskTempService) DeleteTaskTemp(tempID string) *utils.APIHandleError {
	_, err := ts.GetTaskTemp(tempID)
	if err != nil {
		return err
	}
	_, er := store.DefalutClient.Delete(ts.SavePath+"/"+tempID, clientv3.WithPrefix())
	if er != nil {
		return utils.CreateAPIHandleErrorFromDBError("delete task", err)
	}
	return nil
}

//TaskGroupService 任务组
type TaskGroupService struct {
	SavePath string
	conf     *option.Conf
	ms       *masterserver.MasterServer
}

var taskGroupService *TaskGroupService

//CreateTaskGroupService 创建Task group service
func CreateTaskGroupService(c *option.Conf, ms *masterserver.MasterServer) *TaskGroupService {
	if taskGroupService == nil {
		taskGroupService = &TaskGroupService{
			SavePath: "/store/taskgroups",
			conf:     c,
			ms:       ms,
		}
	}
	return taskGroupService
}

//AddTaskGroup add task group
func (ts *TaskGroupService) AddTaskGroup(t *model.TaskGroup) *utils.APIHandleError {
	if t.ID == "" {
		t.ID = uuid.NewV4().String()
	}
	t.CreateTime = time.Now()
	if t.Tasks == nil || len(t.Tasks) < 1 {
		return utils.CreateAPIHandleError(400, fmt.Errorf("group must have at least one task"))
	}
	for _, task := range t.Tasks {
		if err := taskService.AddTask(task); err != nil {
			return err
		}
	}
	_, err := store.DefalutClient.Put(ts.SavePath+"/"+t.ID, t.String())
	if err != nil {
		return utils.CreateAPIHandleErrorFromDBError("save task", err)
	}
	return nil
}

//GetTaskGroups get tasks
func (ts *TaskGroupService) GetTaskGroups() ([]*model.TaskGroup, *utils.APIHandleError) {
	res, err := store.DefalutClient.Get(ts.SavePath, clientv3.WithPrefix())
	if err != nil {
		return nil, utils.CreateAPIHandleErrorFromDBError("save task", err)
	}
	if res.Count == 0 {
		return nil, nil
	}
	var tasks []*model.TaskGroup
	for _, kv := range res.Kvs {
		var t model.TaskGroup
		if err = ffjson.Unmarshal(kv.Value, &t); err != nil {
			logrus.Errorf("unmarshal task info from etcd value error:%s", err.Error())
			continue
		}
		tasks = append(tasks, &t)
	}
	if len(tasks) < 1 {
		return nil, utils.CreateAPIHandleError(500, err)
	}
	return tasks, nil
}

//GetTaskGroup 获取Task
func (ts *TaskGroupService) GetTaskGroup(taskGroupID string) (*model.TaskGroup, *utils.APIHandleError) {
	res, err := store.DefalutClient.Get(ts.SavePath + "/" + taskGroupID)
	if err != nil {
		return nil, utils.CreateAPIHandleErrorFromDBError("save task", err)
	}
	if res.Count == 0 {
		return nil, utils.CreateAPIHandleError(404, fmt.Errorf("task not found"))
	}
	var task model.TaskGroup
	for _, kv := range res.Kvs {
		if err = ffjson.Unmarshal(kv.Value, &task); err != nil {
			logrus.Errorf("unmarshal task info from etcd value error:%s", err.Error())
			return nil, utils.CreateAPIHandleError(500, fmt.Errorf("unmarshal task value error %s", err.Error()))
		}
		break
	}
	return &task, nil
}

//DeleteTaskGroup 删除TaskGroup
//删除Group不删除包含的Task
func (ts *TaskGroupService) DeleteTaskGroup(taskGroupID string) *utils.APIHandleError {
	taskGroup, err := ts.GetTaskGroup(taskGroupID)
	if err != nil {
		return err
	}
	if ok := taskGroup.CanBeDelete(); !ok {
		return utils.CreateAPIHandleError(400, fmt.Errorf("this task is not be deleted"))
	}
	_, er := store.DefalutClient.Delete(ts.SavePath+"/"+taskGroupID, clientv3.WithPrefix())
	if er != nil {
		return utils.CreateAPIHandleErrorFromDBError("delete task", er)
	}
	return nil
}

//ExecTaskGroup 执行组任务API处理
func (ts *TaskGroupService) ExecTaskGroup(taskGroupID string) *utils.APIHandleError {
	t, err := ts.GetTaskGroup(taskGroupID)
	if err != nil {
		return err
	}
	//TODO:增加执行判断
	ts.ms.TaskEngine.ScheduleGroup(nil, t)
	return nil
}
