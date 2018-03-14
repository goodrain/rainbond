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
	"github.com/goodrain/rainbond/pkg/node/api/model"
	"github.com/goodrain/rainbond/pkg/node/masterserver"
	"github.com/goodrain/rainbond/pkg/node/utils"

	"github.com/twinj/uuid"
)

//TaskService 处理taskAPI
type PrometheusService struct {
	prometheusAPI *model.PrometheusAPI
	conf     *option.Conf
	ms       *masterserver.MasterServer
}

var prometheusService *PrometheusService

//CreateTaskService 创建Task service
func CreatePrometheusService(c *option.Conf, ms *masterserver.MasterServer) *PrometheusService {
	if prometheusService == nil {
		prometheusService = &PrometheusService{
			prometheusAPI: &model.PrometheusAPI{API:c.PrometheusAPI},
			conf:     c,
			ms:       ms,
		}
	}
	return prometheusService
}
func (ts *PrometheusService)getTasksByCheck(checkTasks []string,nodeID string) ([]*model.Task, *utils.APIHandleError) {
	var result []*model.Task
	var nextTask []string
	for _,v:=range checkTasks{
		checkTask,err:=taskService.GetTask(v)
		if err != nil {
			return nil,err
		}
		for _,out:=range checkTask.OutPut{
			if out.NodeID == nodeID {
				for _,status:=range out.Status{
					for _,v:=range status.NextTask{
						nextTask=append(nextTask,v)
					}
				}
			}
		}

	}
	//tids:=[]string{"do_rbd_images","install_acp_plugins","install_base_plugins","install_db",
	//	"install_docker","install_k8s","install_manage_ready","install_network","install_plugins","install_storage","install_webcli","update_dns","update_entrance_services","create_host_id_list"}
	for _,v:=range nextTask{
		task,err:=taskService.GetTask(v)
		if err != nil {
			return nil,err
		}
		result=append(result, task)
	}
	return result,nil
}
func (ts *PrometheusService)GetTasksByNode(n *model.HostNode)([]*model.Task,*utils.APIHandleError)  {
	if n.Role.HasRule("compute") &&len(n.Role)==1{
		checkTask:=[]string{"check_compute_services"}
		//tids:=[]string{"install_compute_ready","update_dns_compute","install_storage_client","install_network_compute","install_plugins_compute","install_docker_compute","install_kubelet"}
		result,err:=ts.getTasksByCheck(checkTask,n.ID)
		if err != nil {
			return nil,err
		}
		return result,nil
	}else if n.Role.HasRule("manage") &&len(n.Role)==1{
		//checkTask:=[]string{"check_manage_base_services","check_manage_services"}
		checkTask:=[]string{"check_manage_services"}
		result,err:=ts.getTasksByCheck(checkTask,n.ID)
		if err != nil {
			return nil,err
		}
		return result,nil
	}else {
		//checkTask:=[]string{"check_manage_base_services","check_manage_services","check_compute_services"}
		checkTask:=[]string{"check_manage_services","check_compute_services"}
		//tids:=[]string{"do_rbd_images","install_acp_plugins","install_base_plugins","install_db","install_docker","install_k8s","install_manage_ready","install_network","install_plugins","install_storage","install_webcli","update_dns","update_entrance_services","create_host_id_list","install_kubelet_manage","install_compute_ready_manage"}
		result,err:=ts.getTasksByCheck(checkTask,n.ID)
		if err != nil {
			return nil,err
		}
		return result,nil
	}
}
//AddTask add task
func (ts *PrometheusService) AddTask(t *model.Task) *utils.APIHandleError {
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

//AddTask add task
func (ts *PrometheusService) Exec(expr string) (*model.Prome,*utils.APIHandleError) {
	resp, err := ts.prometheusAPI.Query(expr)
	if err != nil {
		return nil,err
	}
	return resp,nil
}
func (ts *PrometheusService) ExecRange(expr,start,end,step string) (*model.Prome,*utils.APIHandleError) {
	resp, err := ts.prometheusAPI.QueryRange(expr,start,end,step)
	if err != nil {
		return nil,err
	}
	return resp,nil
}