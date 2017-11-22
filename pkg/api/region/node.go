package region

import (
	"github.com/goodrain/rainbond/pkg/node/api/model"
	"bytes"
	"net/http"
	"io/ioutil"
	"encoding/json"
	"github.com/bitly/go-simplejson"
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

}
type TaskInterface interface {
	Get(name string) (*Task)
	Exec(nodes []string ) error
}
type NodeInterface interface {
	Add(node *model.APIHostNode)
}

func (t *Node)Add(node *model.APIHostNode) {
	body,_:=json.Marshal(node)
	Request("/nodes/","POST",body)
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

	_,_,err:=Request("/tasks/"+taskId+"/exec","POST",body)
	return err
}
type TaskStatus struct {
	Status map[string]model.TaskStatus `json:"status,omitempty"`
}
func (t *Task)Status() (*TaskStatus,error) {
	taskId:=t.taskID
	resp,_,_:=Request("/tasks/"+taskId+"/status","GET",nil)
	j,_:=simplejson.NewJson(resp)
	bean,_:=j.Get("bean").Bytes()


	var status TaskStatus
	err:=json.Unmarshal(bean,&status)
	if err != nil {
		return nil,err
	}
	return &status,nil
}
func Request(url ,method string, body []byte) ([]byte,int,error) {
	request, err := http.NewRequest(method, "http://127.0.0.1:6100/v2"+url, bytes.NewBuffer(body))
	if err != nil {
		return nil,500,err
	}
	request.Header.Set("Content-Type", "application/json")
	if region.token != "" {
		request.Header.Set("Authorization", "Token "+region.token)
	}

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, 500,err
	}

	data, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	return data,res.StatusCode,err
}