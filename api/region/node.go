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

package region

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/goodrain/rainbond/node/api/model"
	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/pquerna/ffjson/ffjson"
	//"github.com/goodrain/rainbond/grctl/cmd"

	"github.com/goodrain/rainbond/api/util"
	utilhttp "github.com/goodrain/rainbond/util/http"
)

func (r *regionImpl) Tasks() TaskInterface {
	return &task{regionImpl: *r, prefix: "/v2/tasks"}
}
func (r *regionImpl) Nodes() NodeInterface {
	return &node{regionImpl: *r, prefix: "/v2/nodes"}
}
func (r *regionImpl) Configs() ConfigsInterface {
	return &configs{regionImpl: *r, prefix: "/v2/configs"}
}

type task struct {
	regionImpl
	prefix string
}
type node struct {
	regionImpl
	prefix string
}

func (n *node) Get(node string) (*client.HostNode, *util.APIHandleError) {
	var res utilhttp.ResponseBody
	var gc client.HostNode
	res.Bean = &gc
	code, err := n.DoRequest(n.prefix+"/"+node, "GET", nil, &res)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("Get database center configs code %d", code))
	}
	return &gc, nil
}

func (n *node) GetNodeResource(node string) (*client.NodePodResource, *util.APIHandleError) {
	var res utilhttp.ResponseBody
	var gc client.NodePodResource
	res.Bean = &gc
	code, err := n.DoRequest(n.prefix+"/"+node+"/resource", "GET", nil, &res)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("Get database center configs code %d", code))
	}
	return &gc, nil
}

func (n *node) GetNodeByRule(rule string) ([]*client.HostNode, *util.APIHandleError) {
	var res utilhttp.ResponseBody
	var gc []*client.HostNode
	res.List = &gc
	code, err := n.DoRequest(n.prefix+"/rule/"+rule, "GET", nil, &res)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("Get database center configs code %d", code))
	}
	return gc, nil
}
func (n *node) List() ([]*client.HostNode, *util.APIHandleError) {
	var res utilhttp.ResponseBody
	var gc []*client.HostNode
	res.List = &gc
	code, err := n.DoRequest(n.prefix, "GET", nil, &res)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("Get database center configs code %d", code))
	}
	return gc, nil
}

func (n *node) Add(node *client.APIHostNode) *util.APIHandleError {
	body, err := json.Marshal(node)
	if err != nil {
		return util.CreateAPIHandleError(400, err)
	}
	code, err := n.DoRequest(n.prefix, "POST", bytes.NewBuffer(body), nil)
	if err != nil {
		return util.CreateAPIHandleError(code, err)
	}
	return nil
}
func (n *node) Label(nid string, label map[string]string) *util.APIHandleError {
	body, err := json.Marshal(label)
	if err != nil {
		return util.CreateAPIHandleError(400, err)
	}
	code, err := n.DoRequest(n.prefix+"/"+nid+"/labels", "PUT", bytes.NewBuffer(body), nil)
	if err != nil {
		return util.CreateAPIHandleError(code, err)
	}
	return nil
}

func (n *node) Delete(nid string) *util.APIHandleError {
	code, err := n.DoRequest(n.prefix+"/"+nid, "DELETE", nil, nil)
	if err != nil {
		return util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return util.CreateAPIHandleError(code, fmt.Errorf("delete node error"))
	}
	return nil
}

func (n *node) Up(nid string) *util.APIHandleError {
	code, err := n.DoRequest(n.prefix+"/"+nid+"/up", "POST", nil, nil)
	if err != nil {
		return util.CreateAPIHandleError(code, err)
	}
	return nil
}
func (n *node) Down(nid string) *util.APIHandleError {
	code, err := n.DoRequest(n.prefix+"/"+nid+"/down", "POST", nil, nil)
	if err != nil {
		return util.CreateAPIHandleError(code, err)
	}
	return nil
}
func (n *node) UnSchedulable(nid string) *util.APIHandleError {
	code, err := n.DoRequest(n.prefix+"/"+nid+"/unschedulable", "PUT", nil, nil)
	if err != nil {
		return util.CreateAPIHandleError(code, err)
	}
	return nil
}
func (n *node) ReSchedulable(nid string) *util.APIHandleError {
	code, err := n.DoRequest(n.prefix+"/"+nid+"/reschedulable", "PUT", nil, nil)
	if err != nil {
		return util.CreateAPIHandleError(code, err)
	}
	return nil
}

type configs struct {
	regionImpl
	prefix string
}

func (c *configs) Get() (*model.GlobalConfig, *util.APIHandleError) {
	var res utilhttp.ResponseBody
	var gc = model.GlobalConfig{
		Configs: make(map[string]*model.ConfigUnit),
	}
	res.Bean = &gc
	code, err := c.DoRequest(c.prefix+"/datacenter", "GET", nil, &res)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("Get database center configs code %d", code))
	}
	return &gc, nil
}

func (c *configs) Put(gc *model.GlobalConfig) *util.APIHandleError {
	rebody, err := ffjson.Marshal(gc)
	if err != nil {
		return util.CreateAPIHandleError(400, err)
	}
	code, err := c.DoRequest(c.prefix+"/datacenter", "PUT", bytes.NewBuffer(rebody), nil)
	if err != nil {
		return util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return util.CreateAPIHandleError(code, fmt.Errorf("Put database center configs code %d", code))
	}
	return nil
}

//TaskInterface task api
type TaskInterface interface {
	Get(name string) (*model.Task, *util.APIHandleError)
	GetTaskStatus(task string) (map[string]*model.TaskStatus, *util.APIHandleError)
	Add(task *model.Task) *util.APIHandleError
	AddGroup(group *model.TaskGroup) *util.APIHandleError
	Exec(name string, nodes []string) *util.APIHandleError
	List() ([]*model.Task, *util.APIHandleError)
}

//NodeInterface node api
type NodeInterface interface {
	GetNodeByRule(rule string) ([]*client.HostNode, *util.APIHandleError)
	Get(node string) (*client.HostNode, *util.APIHandleError)
	GetNodeResource(node string) (*client.NodePodResource, *util.APIHandleError)
	List() ([]*client.HostNode, *util.APIHandleError)
	Add(node *client.APIHostNode) *util.APIHandleError
	Up(nid string) *util.APIHandleError
	Down(nid string) *util.APIHandleError
	UnSchedulable(nid string) *util.APIHandleError
	ReSchedulable(nid string) *util.APIHandleError
	Delete(nid string) *util.APIHandleError
	Label(nid string, label map[string]string) *util.APIHandleError
}

//ConfigsInterface 数据中心配置API
type ConfigsInterface interface {
	Get() (*model.GlobalConfig, *util.APIHandleError)
	Put(*model.GlobalConfig) *util.APIHandleError
}

func (t *task) Get(id string) (*model.Task, *util.APIHandleError) {
	var res utilhttp.ResponseBody
	var gc model.Task
	res.Bean = &gc
	code, err := t.DoRequest(t.prefix+"/"+id, "GET", nil, &res)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("get task with code %d", code))
	}
	return &gc, nil
}

//List list all task
func (t *task) List() ([]*model.Task, *util.APIHandleError) {
	var res utilhttp.ResponseBody
	var gc []*model.Task
	res.List = &gc
	code, err := t.DoRequest(t.prefix, "GET", nil, &res)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("get task with code %d", code))
	}
	return gc, nil
}

//Exec 执行任务
func (t *task) Exec(taskID string, nodes []string) *util.APIHandleError {
	var nodesBody struct {
		Nodes []string `json:"nodes"`
	}
	nodesBody.Nodes = nodes
	body, err := json.Marshal(nodesBody)
	if err != nil {
		return util.CreateAPIHandleError(400, err)
	}
	url := t.prefix + "/" + taskID + "/exec"
	code, err := t.DoRequest(url, "POST", bytes.NewBuffer(body), nil)
	if err != nil {
		return util.CreateAPIHandleError(code, err)
	}
	return nil
}
func (t *task) Add(task *model.Task) *util.APIHandleError {

	body, _ := json.Marshal(task)
	url := t.prefix
	code, err := t.DoRequest(url, "POST", bytes.NewBuffer(body), nil)
	if err != nil {
		return util.CreateAPIHandleError(code, err)
	}
	return nil
}

func (t *task) AddGroup(group *model.TaskGroup) *util.APIHandleError {
	body, _ := json.Marshal(group)
	url := "/v2/taskgroups"
	code, err := t.DoRequest(url, "POST", bytes.NewBuffer(body), nil)
	if err != nil {
		return util.CreateAPIHandleError(code, err)
	}
	return nil
}
func (t *task) GetTaskStatus(task string) (map[string]*model.TaskStatus, *util.APIHandleError) {
	var res utilhttp.ResponseBody
	var gc = make(map[string]*model.TaskStatus)
	res.Bean = &gc
	code, err := t.DoRequest("/tasks/"+task+"/status", "GET", nil, &res)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("get task with code %d", code))
	}
	return gc, nil
}
