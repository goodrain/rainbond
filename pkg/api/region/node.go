package region

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/bitly/go-simplejson"
	"github.com/goodrain/rainbond/pkg/node/api/model"
	"github.com/pquerna/ffjson/ffjson"
	//"github.com/goodrain/rainbond/pkg/grctl/cmd"
	"errors"

	utilhttp "github.com/goodrain/rainbond/pkg/util/http"
)

var nodeServer *RNodeServer

func NewNode(nodeAPI string) {
	if nodeServer == nil {
		nodeServer = &RNodeServer{
			NodeAPI: nodeAPI,
		}
	}
}
func GetNode() *RNodeServer {
	return nodeServer
}

type RNodeServer struct {
	NodeAPI string
}

func (r *RNodeServer) Tasks() TaskInterface {
	return &Task{}
}
func (r *RNodeServer) Nodes() NodeInterface {
	return &Node{}
}

type Task struct {
	TaskID string `json:"task_id"`
	Task   *model.Task
}
type Node struct {
	Id   string
	Node *model.HostNode `json:"node"`
}
type TaskInterface interface {
	Get(name string) (*Task, error)
	Add(task *model.Task) error
	AddGroup(group *model.TaskGroup) error
	Exec(name string, nodes []string) error
	List() ([]*model.Task, error)
	Refresh() error
}
type NodeInterface interface {
	Add(node *model.APIHostNode)
	Delete()
	Rule(rule string) []*model.HostNode
	Get(node string) *Node
	List() []*model.HostNode
	Up()
	Down()
	UnSchedulable()
	ReSchedulable()
	Label(label map[string]string)
}

func (t *Node) Label(label map[string]string) {
	body, _ := json.Marshal(label)
	_, _, err := nodeServer.Request("/nodes/"+t.Id+"/labels", "PUT", body)
	if err != nil {
		logrus.Errorf("error details %s", err.Error())
	}
}

func (t *Node) Add(node *model.APIHostNode) {
	body, _ := json.Marshal(node)
	_, _, err := nodeServer.Request("/nodes", "POST", body)
	if err != nil {
		logrus.Errorf("error details %s", err.Error())
	}
}
func (t *Node) Delete() {
	_, _, err := nodeServer.Request("/nodes/"+t.Id, "DELETE", nil)
	if err != nil {
		logrus.Errorf("error details %s", err.Error())
	}
}
func (t *Node) Up() {
	nodeServer.Request("/nodes/"+t.Id+"/up", "POST", nil)
}
func (t *Node) Down() {
	nodeServer.Request("/nodes/"+t.Id+"/down", "POST", nil)
}
func (t *Node) UnSchedulable() {
	nodeServer.Request("/nodes/"+t.Id+"/unschedulable", "PUT", nil)
}
func (t *Node) ReSchedulable() {
	nodeServer.Request("/nodes/"+t.Id+"/reschedulable", "PUT", nil)
}

func (t *Node) Get(node string) *Node {
	body, _, err := nodeServer.Request("/nodes/"+node, "GET", nil)
	if err != nil {
		logrus.Errorf("error get node %s,details %s", node, err.Error())
		return nil
	}
	t.Id = node
	var stored model.HostNode
	j, err := simplejson.NewJson(body)
	if err != nil {
		logrus.Errorf("error get node %s 's json,details %s", node, err.Error())
		return nil
	}
	bean := j.Get("bean")
	n, err := json.Marshal(bean)
	if err != nil {
		logrus.Errorf("error get bean from response,details %s", err.Error())
		return nil
	}
	err = json.Unmarshal([]byte(n), &stored)
	if err != nil {
		logrus.Errorf("error unmarshal node %s,details %s", node, err.Error())
		return nil
	}
	t.Node = &stored
	return t
}

func (t *Node) Rule(rule string) []*model.HostNode {
	body, _, err := nodeServer.Request("/nodes/rule/"+rule, "GET", nil)
	if err != nil {
		logrus.Errorf("error get rule %s ,details %s", rule, err.Error())
		return nil
	}
	j, err := simplejson.NewJson(body)
	if err != nil {
		logrus.Errorf("error get json ,details %s", err.Error())
		return nil
	}
	nodeArr, err := j.Get("list").Array()
	if err != nil {
		logrus.Infof("error occurd,details %s", err.Error())
		return nil
	}
	jsonA, _ := json.Marshal(nodeArr)
	nodes := []*model.HostNode{}
	err = json.Unmarshal(jsonA, &nodes)
	if err != nil {
		logrus.Infof("error occurd,details %s", err.Error())
		return nil
	}
	return nodes
}
func (t *Node) List() []*model.HostNode {
	body, _, err := nodeServer.Request("/nodes", "GET", nil)
	if err != nil {
		logrus.Errorf("error get nodes ,details %s", err.Error())
		return nil
	}
	j, err := simplejson.NewJson(body)
	if err != nil {
		logrus.Errorf("error get json ,details %s", err.Error())
		return nil
	}
	nodeArr, err := j.Get("list").Array()
	if err != nil {
		logrus.Infof("error occurd,details %s", err.Error())
		return nil
	}
	jsonA, _ := json.Marshal(nodeArr)
	nodes := []*model.HostNode{}
	err = json.Unmarshal(jsonA, &nodes)
	if err != nil {
		logrus.Infof("error occurd,details %s", err.Error())
		return nil
	}
	return nodes
}
func (t *Task) Get(id string) (*Task, error) {
	t.TaskID = id
	url := "/tasks/" + id
	resp, code, err := nodeServer.Request(url, "GET", nil)
	if err != nil {
		logrus.Errorf("error request url %s,details %s", url, err.Error())
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("get task failure," + string(resp))
	}
	jsonTop, err := simplejson.NewJson(resp)
	if err != nil {
		logrus.Errorf("error get json from url %s", err.Error())
		return nil, err
	}
	var task model.Task
	beanJ := jsonTop.Get("bean")
	taskB, err := json.Marshal(beanJ)
	if err != nil {
		logrus.Errorf("error marshal task %s", err.Error())
		return nil, err
	}
	err = json.Unmarshal(taskB, &task)
	if err != nil {
		logrus.Errorf("error unmarshal task %s", err.Error())
		return nil, err
	}
	t.Task = &task
	return t, nil
}

//List list all task
func (t *Task) List() ([]*model.Task, error) {
	url := "/tasks"
	resp, _, err := nodeServer.Request(url, "GET", nil)
	if err != nil {
		logrus.Errorf("error request url %s,details %s", url, err.Error())
		return nil, err
	}
	var rb utilhttp.ResponseBody
	var tasks = new([]*model.Task)
	rb.List = tasks
	if err := ffjson.Unmarshal(resp, &rb); err != nil {
		return nil, err
	}
	if rb.List == nil {
		return nil, nil
	}
	list, ok := rb.List.(*[]*model.Task)
	if ok {
		return *list, nil
	}
	return nil, fmt.Errorf("unmarshal tasks data error")
}

//Exec 执行任务
func (t *Task) Exec(taskID string, nodes []string) error {
	var nodesBody struct {
		Nodes []string `json:"nodes"`
	}
	nodesBody.Nodes = nodes
	body, _ := json.Marshal(nodesBody)
	url := "/tasks/" + taskID + "/exec"
	resp, code, err := nodeServer.Request(url, "POST", body)
	if code != 200 {
		return fmt.Errorf("exec failure," + string(resp))
	}
	if err != nil {
		return err
	}
	return err
}
func (t *Task) Add(task *model.Task) error {

	body, _ := json.Marshal(task)
	url := "/tasks"
	resp, code, err := nodeServer.Request(url, "POST", body)
	if code != 200 {
		return fmt.Errorf("add task failure," + string(resp))
	}
	if err != nil {
		return err
	}
	return nil
}
func (t *Task) AddGroup(group *model.TaskGroup) error {
	body, _ := json.Marshal(group)
	url := "/taskgroups"
	resp, code, err := nodeServer.Request(url, "POST", body)
	if code != 200 {
		return fmt.Errorf("add taskgroup failure," + string(resp))
	}
	if err != nil {
		return err
	}
	return nil
}

//Refresh 刷新静态配置
func (t *Task) Refresh() error {
	url := "/tasks/taskreload"
	_, code, err := nodeServer.Request(url, "PUT", nil)
	if code != 200 {
		return fmt.Errorf("refresh error code,%d", code)
	}
	if err != nil {
		return err
	}
	return nil
}

type TaskStatus struct {
	Status map[string]model.TaskStatus `json:"status,omitempty"`
}

func (t *Task) Status() (*TaskStatus, error) {
	taskId := t.TaskID

	return HandleTaskStatus(taskId)
}
func HandleTaskStatus(task string) (*TaskStatus, error) {
	resp, code, err := nodeServer.Request("/tasks/"+task+"/status", "GET", nil)
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
func (r *RNodeServer) Request(url, method string, body []byte) ([]byte, int, error) {
	//logrus.Infof("requesting url: %s by method :%s,and body is ",r.NodeAPI+url,method,string(body))
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
	defer res.Body.Close()
	//logrus.Infof("response is %s,response code is %d",string(data),res.StatusCode)
	return data, res.StatusCode, err
}
