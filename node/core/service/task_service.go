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

package service

import (
	"fmt"
	"time"

	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/api/model"
	"github.com/goodrain/rainbond/node/core/store"
	"github.com/goodrain/rainbond/node/masterserver"
	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/goodrain/rainbond/node/utils"

	"github.com/Sirupsen/logrus"
	"github.com/pquerna/ffjson/ffjson"

	"github.com/coreos/etcd/clientv3"

	"github.com/twinj/uuid"
)

//TaskService task services
type TaskService struct {
	SavePath string
	conf     *option.Conf
	ms       *masterserver.MasterServer
}

var taskService *TaskService

//CreateTaskService create task service
func CreateTaskService(c *option.Conf, ms *masterserver.MasterServer) *TaskService {
	if taskService == nil {
		taskService = &TaskService{
			SavePath: "/rainbond/store/tasks",
			conf:     c,
			ms:       ms,
		}
	}
	return taskService
}
func (ts *TaskService) getTasksByCheck(checkTasks []string, nodeID string) ([]*model.Task, *utils.APIHandleError) {
	var result []*model.Task
	var nextTask []string
	for _, v := range checkTasks {
		checkTask, err := taskService.GetTask(v)
		if err != nil {
			return nil, err
		}
		for _, out := range checkTask.OutPut {
			if out.NodeID == nodeID {
				for _, status := range out.Status {
					for _, v := range status.NextTask {
						nextTask = append(nextTask, v)
					}
				}
			}
		}

	}
	//tids:=[]string{"do_rbd_images","install_acp_plugins","install_base_plugins","install_db",
	//	"install_docker","install_k8s","install_manage_ready","install_network","install_plugins","install_storage","install_webcli","update_dns","update_entrance_services","create_host_id_list"}
	for _, v := range nextTask {
		task, err := taskService.GetTask(v)
		if err != nil {
			return nil, err
		}
		result = append(result, task)
	}
	return result, nil
}

//GetTasksByNode get tasks by node
func (ts *TaskService) GetTasksByNode(n *client.HostNode) ([]*model.Task, *utils.APIHandleError) {
	if n.Role.HasRule("compute") && len(n.Role) == 1 {
		checkTask := []string{"check_compute_services"}
		//tids:=[]string{"install_compute_ready","update_dns_compute","install_storage_client","install_network_compute","install_plugins_compute","install_docker_compute","install_kubelet"}
		result, err := ts.getTasksByCheck(checkTask, n.ID)
		if err != nil {
			return nil, err
		}
		return result, nil
	} else if n.Role.HasRule("manage") && len(n.Role) == 1 {
		//checkTask:=[]string{"check_manage_base_services","check_manage_services"}
		checkTask := []string{"check_manage_services"}
		result, err := ts.getTasksByCheck(checkTask, n.ID)
		if err != nil {
			return nil, err
		}
		return result, nil
	} else {
		//checkTask:=[]string{"check_manage_base_services","check_manage_services","check_compute_services"}
		checkTask := []string{"check_manage_services", "check_compute_services"}
		//tids:=[]string{"do_rbd_images","install_acp_plugins","install_base_plugins","install_db","install_docker","install_k8s","install_manage_ready","install_network","install_plugins","install_storage","install_webcli","update_dns","update_entrance_services","create_host_id_list","install_kubelet_manage","install_compute_ready_manage"}
		result, err := ts.getTasksByCheck(checkTask, n.ID)
		if err != nil {
			return nil, err
		}
		return result, nil
	}
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

	err := ts.ms.TaskEngine.AddTask(t)
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
	var result []*model.Task
	for _, kv := range res.Kvs {
		var t model.Task
		if err = ffjson.Unmarshal(kv.Value, &t); err != nil {
			logrus.Errorf("unmarshal task info from etcd value error:%s", err.Error())
			continue
		}
		tasks = append(tasks, &t)
	}

	for _, v := range tasks {
		task := ts.ms.TaskEngine.GetTask(v.ID)
		result = append(result, task)
	}
	if len(result) < 1 {
		return nil, utils.CreateAPIHandleError(500, err)
	}
	return result, nil
}

//GetTask get task by taskID
func (ts *TaskService) GetTask(taskID string) (*model.Task, *utils.APIHandleError) {
	var task *model.Task
	task = ts.ms.TaskEngine.GetTask(taskID)
	if task == nil {
		return nil, utils.CreateAPIHandleError(404, fmt.Errorf("task not found"))
	}
	return task, nil
}

//DeleteTask delete task by taskID
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
	_, er = store.DefalutClient.Delete("/store/tasks_part_status"+"/"+taskID, clientv3.WithPrefix())
	if er != nil {
		return utils.CreateAPIHandleErrorFromDBError("delete task", er)
	}
	_, er = store.DefalutClient.Delete("/store/tasks_part_output"+"/"+taskID, clientv3.WithPrefix())
	if er != nil {
		return utils.CreateAPIHandleErrorFromDBError("delete task", er)
	}
	_, er = store.DefalutClient.Delete("/store/tasks_part_scheduler"+"/"+taskID, clientv3.WithPrefix())
	if er != nil {
		return utils.CreateAPIHandleErrorFromDBError("delete task", er)
	}
	return nil
}

//ExecTask exec a task in nodes
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
		return utils.CreateAPIHandleError(400, fmt.Errorf("exec node can not be empty"))
	}
	for _, node := range nodes {
		if n := ts.ms.Cluster.GetNode(node); n == nil {
			return utils.CreateAPIHandleError(400, fmt.Errorf(" exec node  %s not found", node))
		}
	}
	var er error
	for _, node := range nodes {
		er = ts.ms.TaskEngine.PutSchedul(taskID, node)
		if er != nil {
			logrus.Error("create task scheduler info error,", er.Error())
		}
	}
	if er != nil {
		return utils.CreateAPIHandleError(400, fmt.Errorf("exec task encounters an error"))
	}
	return nil
}

//ReloadStaticTasks reload task
func (ts *TaskService) ReloadStaticTasks() {
	ts.ms.TaskEngine.LoadStaticTask()
}

//TaskTempService task temp service
type TaskTempService struct {
	SavePath string
	conf     *option.Conf
}

var taskTempService *TaskTempService

//CreateTaskTempService create task temp service
func CreateTaskTempService(c *option.Conf) *TaskTempService {
	if taskTempService == nil {
		taskTempService = &TaskTempService{
			SavePath: "/store/tasks",
			conf:     c,
		}
	}
	return taskTempService
}

//SaveTaskTemp add task temp
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

//GetTaskTemp get task temp
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

//DeleteTaskTemp delete task temp
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

//TaskGroupService task group
type TaskGroupService struct {
	SavePath string
	conf     *option.Conf
	ms       *masterserver.MasterServer
}

var taskGroupService *TaskGroupService

//CreateTaskGroupService create Task group service
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

//GetTaskGroup get Task group
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

//DeleteTaskGroup delete TaskGroup
//delete group but do not delete task in this group
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

//ExecTaskGroup exec group task
func (ts *TaskGroupService) ExecTaskGroup(taskGroupID string, nodes []string) *utils.APIHandleError {
	t, err := ts.GetTaskGroup(taskGroupID)
	if err != nil {
		return err
	}
	if nodes == nil || len(nodes) == 0 {
		return utils.CreateAPIHandleError(400, fmt.Errorf("exec node can not be empty"))
	}
	for _, node := range nodes {
		if n := ts.ms.Cluster.GetNode(node); n == nil {
			return utils.CreateAPIHandleError(400, fmt.Errorf(" exec node  %s not found", node))
		}
	}
	var er error
	for _, node := range nodes {
		er = ts.ms.TaskEngine.ScheduleGroup(t, node)
		if er != nil {
			logrus.Error("create task scheduler info error,", err.Error())
		}
	}
	if er != nil {
		return utils.CreateAPIHandleError(400, fmt.Errorf("exec task encounters an error"))
	}
	return nil
}
