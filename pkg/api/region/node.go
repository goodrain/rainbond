package region

import (
	"github.com/goodrain/rainbond/pkg/node/api/model"
	"bytes"
	"net/http"
	"io/ioutil"
	"encoding/json"
	"github.com/bitly/go-simplejson"
	"github.com/Sirupsen/logrus"
	"fmt"
)
var nodeServer *RNodeServer

func NewNode(nodeAPI string)  {
	if nodeServer==nil {
		nodeServer=&RNodeServer{
			nodeAPI:nodeAPI,
		}
	}
}
func GetNode() *RNodeServer {
	return nodeServer
}
type RNodeServer struct {
	nodeAPI string
}

func (r *RNodeServer)Tasks() TaskInterface {
	return &Task{}
}
func (r *RNodeServer)Nodes() NodeInterface {
	return &Node{}
}
type Task struct {
	taskID string
}
type Node struct {
	id string
	node *model.HostNode
}
type TaskInterface interface {
	Get(name string) (*Task)
	Exec(nodes []string ) error
}
type NodeInterface interface {
	Add(node *model.APIHostNode)
	Get(node string) *Node
	List() []*model.HostNode
	Up()
	Down()
	UnSchedulable()
	ReSchedulable()
}

func (t *Node)Add(node *model.APIHostNode) {
	body,_:=json.Marshal(node)
	Request("/nodes/","POST",body)
}
func (t *Node)Up() {
	Request("/nodes/"+t.id+"/up","POST",nil)
}
func (t *Node)Down() {
	Request("/nodes/"+t.id+"/down","POST",nil)
}
func (t *Node)UnSchedulable() {
	Request("/nodes/"+t.id+"/unschedulable","PUT",nil)
}
func (t *Node)ReSchedulable() {
	Request("/nodes/"+t.id+"/reschedulable","PUT",nil)
}
func (t *Node)Get(node string) *Node {
	body,_,err:=Request("/nodes/"+node,"GET",nil)
	if err != nil {
		return nil
	}
	t.id=node
	var stored model.HostNode
	err=json.Unmarshal(body,&node)
	if err != nil {
		return nil
	}
	t.node=&stored
	return t
}
func (t *Node)List() []*model.HostNode {
	body,_,_:=Request("/nodes","GET",nil)
	j,_:=simplejson.NewJson(body)
	nodeArr,err:=j.Get("list").Array()
	if err != nil {
		logrus.Infof("error occurd,details %s",err.Error())
		return nil
	}
	jsonA, _ := json.Marshal(nodeArr)
	nodes := []*model.HostNode{}
	err=json.Unmarshal(jsonA, &nodes)
	if err != nil {
		logrus.Infof("error occurd,details %s",err.Error())
		return nil
	}
	return nodes
}
func (t *Task)Get(id string) (*Task) {
	return &Task{
		taskID:id,
	}
}
func (t *Task)Exec(nodes []string ) error {
	taskId:=t.taskID
	var nodesBody struct {
		Nodes []string `json:"nodes"`
	}
	nodesBody.Nodes=nodes
	body,_:=json.Marshal(nodesBody)
	url:="/tasks/"+taskId+"/exec"
	resp,code,err:=Request(url,"POST",body)
	if code != 200 {
		fmt.Println("executing failed:"+string(resp))
	}
	if err!=nil {
		return err
	}
	return err
}
type TaskStatus struct {
	Status map[string]model.TaskStatus `json:"status,omitempty"`
}
func (t *Task)Status() (*TaskStatus,error) {
	taskId:=t.taskID
	resp,code,err:=Request("/tasks/"+taskId+"/status","GET",nil)
	if err != nil {
		logrus.Errorf("error execute status request,details %s",err.Error())
		return nil,err
	}
	if code == 200 {
		j,_:=simplejson.NewJson(resp)
		bean,_:=j.Get("bean").Bytes()
		var status TaskStatus
		err=json.Unmarshal(bean,&status)
		if err != nil {
			logrus.Errorf("error unmarshal response,details %s",err.Error())
			return nil,err
		}
		return &status,nil
	}
	return nil,nil
}
func Request(url ,method string, body []byte) ([]byte,int,error) {
	request, err := http.NewRequest(method, "http://127.0.0.1:6100/v2"+url, bytes.NewBuffer(body))
	if err != nil {
		return nil,500,err
	}
	request.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, 500,err
	}

	data, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	return data,res.StatusCode,err
}