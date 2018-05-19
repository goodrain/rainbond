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
	"io/ioutil"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/bitly/go-simplejson"
	"github.com/goodrain/rainbond/node/api/model"
	"github.com/pquerna/ffjson/ffjson"
	//"github.com/goodrain/rainbond/grctl/cmd"
	"errors"

	"github.com/goodrain/rainbond/api/util"
	utilhttp "github.com/goodrain/rainbond/util/http"
)

var nodeclient *RNodeClient

//NewNode new node client
func NewNode(nodeAPI string) {
	if nodeclient == nil {
		nodeclient = &RNodeClient{
			NodeAPI: nodeAPI,
		}
	}
}
func GetNode() *RNodeClient {
	return nodeclient
}

type RNodeClient struct {
	NodeAPI string
}

func (r *RNodeClient) Tasks() TaskInterface {
	return &task{client: r, prefix: "/tasks"}
}
func (r *RNodeClient) Nodes() NodeInterface {
	return &node{client: r, prefix: "/nodes"}
}
func (r *RNodeClient) Configs() ConfigsInterface {
	return &configs{client: r, prefix: "/configs"}
}

type task struct {
	client *RNodeClient
	prefix string
}
type node struct {
	client *RNodeClient
	prefix string
}
type TaskInterface interface {
	Get(name string) (*model.Task, *util.APIHandleError)
	Status(name string) (*TaskStatus, error)
	HandleTaskStatus(task string) (*map[string]*model.TaskStatus, *util.APIHandleError)
	Add(task *model.Task) *util.APIHandleError
	AddGroup(group *model.TaskGroup) *util.APIHandleError
	Exec(name string, nodes []string) *util.APIHandleError
	List() ([]*model.Task, *util.APIHandleError)
	Refresh() *util.APIHandleError
}
type NodeInterface interface {
	Rule(rule string) ([]*model.HostNode, *util.APIHandleError)
	Get(node string) (*model.HostNode, *util.APIHandleError)
	List() ([]*model.HostNode, *util.APIHandleError)
	Add(node *model.APIHostNode) *util.APIHandleError
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
type configs struct {
	client *RNodeClient
	prefix string
}

func (c *configs) Get() (*model.GlobalConfig, *util.APIHandleError) {
	body, code, err := c.client.Request(c.prefix+"/datacenter", "GET", nil)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("Get database center configs code %d", code))
	}
	var res utilhttp.ResponseBody
	var gc = model.GlobalConfig{
		Configs: make(map[string]*model.ConfigUnit),
	}
	res.Bean = &gc
	if err := ffjson.Unmarshal(body, &res); err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if gc, ok := res.Bean.(*model.GlobalConfig); ok {
		return gc, nil
	}
	return nil, nil
}

func (c *configs) Put(gc *model.GlobalConfig) *util.APIHandleError {
	rebody, err := ffjson.Marshal(gc)
	if err != nil {
		return util.CreateAPIHandleError(400, err)
	}
	_, code, err := c.client.Request(c.prefix+"/datacenter", "PUT", rebody)
	if err != nil {
		return util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return util.CreateAPIHandleError(code, fmt.Errorf("Put database center configs code %d", code))
	}
	return nil
}

func (n *node) Get(node string) (*model.HostNode, *util.APIHandleError) {
	body, code, err := n.client.Request(n.prefix+"/"+node, "GET", nil)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("Get database center configs code %d", code))
	}
	var res utilhttp.ResponseBody
	var gc model.HostNode
	res.Bean = &gc
	if err := ffjson.Unmarshal(body, &res); err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if gc, ok := res.Bean.(*model.HostNode); ok {
		return gc, nil
	}
	return nil, nil
}
func (n *node) Rule(rule string) ([]*model.HostNode, *util.APIHandleError) {
	body, code, err := n.client.Request(n.prefix+"/rule/"+rule, "GET", nil)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("Get database center configs code %d", code))
	}
	var res utilhttp.ResponseBody
	var gc []*model.HostNode
	res.List = &gc
	if err := ffjson.Unmarshal(body, &res); err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if gc, ok := res.List.(*[]*model.HostNode); ok {
		return *gc, nil
	}
	return nil, nil
}
func (n *node) List() ([]*model.HostNode, *util.APIHandleError) {
	body, code, err := n.client.Request(n.prefix, "GET", nil)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("Get database center configs code %d", code))
	}
	var res utilhttp.ResponseBody
	var gc []*model.HostNode
	res.List = &gc
	if err := ffjson.Unmarshal(body, &res); err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if gc, ok := res.List.(*[]*model.HostNode); ok {
		return *gc, nil
	}
	return nil, nil
}

// put/post 类

func (n *node) Add(node *model.APIHostNode) *util.APIHandleError {
	body, err := json.Marshal(node)
	if err != nil {
		return util.CreateAPIHandleError(400, err)
	}
	resp, code, err := n.client.Request(n.prefix, "POST", body)
	return n.client.handleErrAndCodeWithMsg(resp, err, code)
}
func (n *node) Label(nid string, label map[string]string) *util.APIHandleError {
	body, err := json.Marshal(label)
	if err != nil {
		return util.CreateAPIHandleError(400, err)
	}
	resp, code, err := n.client.Request(n.prefix+"/"+nid+"/labels", "PUT", body)
	return n.client.handleErrAndCodeWithMsg(resp, err, code)
}

func (n *node) Delete(nid string) *util.APIHandleError {
	resp, code, err := n.client.Request(n.prefix+"/"+nid, "DELETE", nil)
	return n.client.handleErrAndCodeWithMsg(resp, err, code)
}
func (n *node) Up(nid string) *util.APIHandleError {
	resp, code, err := n.client.Request(n.prefix+"/"+nid+"/up", "POST", nil)
	return n.client.handleErrAndCodeWithMsg(resp, err, code)
}
func (n *node) Down(nid string) *util.APIHandleError {
	resp, code, err := n.client.Request(n.prefix+"/"+nid+"/down", "POST", nil)
	return n.client.handleErrAndCodeWithMsg(resp, err, code)
}
func (n *node) UnSchedulable(nid string) *util.APIHandleError {
	resp, code, err := n.client.Request(n.prefix+"/"+nid+"/unschedulable", "PUT", nil)
	return n.client.handleErrAndCodeWithMsg(resp, err, code)
}
func (n *node) ReSchedulable(nid string) *util.APIHandleError {
	resp, code, err := n.client.Request(n.prefix+"/"+nid+"/reschedulable", "PUT", nil)
	return n.client.handleErrAndCodeWithMsg(resp, err, code)
}

func (t *task) Get(id string) (*model.Task, *util.APIHandleError) {
	url := t.prefix + "/" + id
	body, code, err := nodeclient.Request(url, "GET", nil)

	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("get task with code %d", code))
	}
	var res utilhttp.ResponseBody
	var gc model.Task
	res.Bean = &gc
	if err := ffjson.Unmarshal(body, &res); err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if gc, ok := res.Bean.(*model.Task); ok {
		return gc, nil
	}
	return nil, nil

}

//List list all task
func (t *task) List() ([]*model.Task, *util.APIHandleError) {
	url := t.prefix
	body, code, err := nodeclient.Request(url, "GET", nil)

	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("get task with code %d", code))
	}
	var res utilhttp.ResponseBody
	var gc []*model.Task
	res.List = &gc
	if err := ffjson.Unmarshal(body, &res); err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if gc, ok := res.List.(*[]*model.Task); ok {
		return *gc, nil
	}
	return nil, nil
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
	resp, code, err := nodeclient.Request(url, "POST", body)
	return t.client.handleErrAndCodeWithMsg(resp, err, code)
}
func (t *task) Add(task *model.Task) *util.APIHandleError {

	body, _ := json.Marshal(task)
	url := t.prefix
	resp, code, err := nodeclient.Request(url, "POST", body)
	return t.client.handleErrAndCodeWithMsg(resp, err, code)
}
func (t *task) AddGroup(group *model.TaskGroup) *util.APIHandleError {
	body, _ := json.Marshal(group)
	url := "/taskgroups"
	resp, code, err := nodeclient.Request(url, "POST", body)
	return t.client.handleErrAndCodeWithMsg(resp, err, code)
}

//Refresh 刷新静态配置
func (t *task) Refresh() *util.APIHandleError {
	url := t.prefix + "/taskreload"
	resp, code, err := nodeclient.Request(url, "PUT", nil)
	return t.client.handleErrAndCodeWithMsg(resp, err, code)
}

type TaskStatus struct {
	Status map[string]model.TaskStatus `json:"status,omitempty"`
}

func (t *task) Status(name string) (*TaskStatus, error) {
	taskId := name

	return HandleTaskStatus(taskId)
}
func (t *task) HandleTaskStatus(task string) (*map[string]*model.TaskStatus, *util.APIHandleError) {
	body, code, err := nodeclient.Request("/tasks/"+task+"/status", "GET", nil)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("get task with code %d", code))
	}
	var res utilhttp.ResponseBody
	var gc map[string]*model.TaskStatus
	res.Bean = &gc
	if err := ffjson.Unmarshal(body, &res); err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if gc, ok := res.Bean.(*map[string]*model.TaskStatus); ok {
		return gc, nil
	}
	return nil, nil
}
func HandleTaskStatus(task string) (*TaskStatus, error) {
	resp, code, err := nodeclient.Request("/tasks/"+task+"/status", "GET", nil)
	if err != nil {
		logrus.Errorf("error execute status Request,details %s", err.Error())
		return nil, err
	}

	if code == 200 {
		j, _ := simplejson.NewJson(resp)
		bean := j.Get("bean")
		beanB, _ := json.Marshal(bean)
		var status TaskStatus
		statusMap := make(map[string]model.TaskStatus)

		json, _ := simplejson.NewJson(beanB)

		second := json.Interface()

		if second == nil {
			return nil, errors.New("get status failed")
		}
		m := second.(map[string]interface{})

		for k, _ := range m {
			var taskStatus model.TaskStatus
			taskStatus.CompleStatus = m[k].(map[string]interface{})["comple_status"].(string)
			taskStatus.Status = m[k].(map[string]interface{})["status"].(string)
			taskStatus.JobID = k
			statusMap[k] = taskStatus
		}
		status.Status = statusMap
		return &status, nil
	}
	return nil, errors.New(fmt.Sprintf("response status is %s", code))
}

//Request Request
func (r *RNodeClient) Request(url, method string, body []byte) ([]byte, int, error) {
	//logrus.Infof("requesting url: %s by method :%s,and body is %s",r.NodeAPI+url,method,string(body))
	request, err := http.NewRequest(method, "http://127.0.0.1:6100/v2"+url, bytes.NewBuffer(body))
	if err != nil {
		return nil, 500, err
	}
	request.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, 500, err
	}
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, 500, err
	}
	defer res.Body.Close()
	//logrus.Infof("response is %s,response code is %d",string(data),res.StatusCode)
	return data, res.StatusCode, nil
}
func (r *RNodeClient) handleErrAndCode(err error, code int) *util.APIHandleError {
	if err != nil {
		return util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return util.CreateAPIHandleError(code, fmt.Errorf("error with code %d", code))
	}
	return nil
}
func (r *RNodeClient) handleErrAndCodeWithMsg(resp []byte, err error, code int) *util.APIHandleError {
	if err != nil {
		return util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return util.CreateAPIHandleError(code, fmt.Errorf("error with code %d , %s", code, string(resp)))
	}
	return nil
}
