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

package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/node/api/model"
	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/goodrain/rainbond/node/utils"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/version"
	"github.com/sirupsen/logrus"
)

func init() {
	prometheus.MustRegister(version.NewCollector("node_exporter"))
}

//InstallNode install a node
func InstallNode(w http.ResponseWriter, r *http.Request) {
	nodeUID := strings.TrimSpace(chi.URLParam(r, "node_id"))
	node, err := nodeService.GetNode(nodeUID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	if err := nodeService.InstallNode(node); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, node)
}

//AddNode add a node
func AddNode(w http.ResponseWriter, r *http.Request) {
	isInstall := r.FormValue("is_install")
	var node client.APIHostNode
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &node, nil); !ok {
		return
	}
	rnode, err := nodeService.AddNode(&node)
	if err != nil {
		err.Handle(r, w)
		return
	}
	if isInstall == "true" || node.AutoInstall {
		if err := nodeService.InstallNode(rnode); err != nil {
			err.Handle(r, w)
			return
		}
	}
	httputil.ReturnSuccess(r, w, rnode)
}

//GetNodes 获取全部节点
func GetNodes(w http.ResponseWriter, r *http.Request) {
	searchNodeList := make([]client.HostNode, 0)
	searchKey := r.FormValue("search_key")
	nodes, err := nodeService.GetAllNode()
	if err != nil {
		err.Handle(r, w)
		return
	}
	for _, node := range nodes {
		var nodevalue = *node
		//list value need simplify
		nodevalue.NodeStatus.Conditions = nil
		nodevalue.NodeStatus.KubeNode = nil
		if searchKey != "" {
			if strings.Contains(node.HostName, searchKey) || strings.Contains(node.InternalIP, searchKey) || strings.Contains(node.ExternalIP, searchKey) {
				searchNodeList = append(searchNodeList, nodevalue)
			}
		} else {
			searchNodeList = append(searchNodeList, nodevalue)
		}
	}
	httputil.ReturnSuccess(r, w, searchNodeList)
}

//GetNode 获取一个节点详情
func GetNode(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "node_id")
	node, err := nodeService.GetNode(nodeID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	node.DeleteCondition("OutOfDisk", "MemoryPressure", "DiskPressure", "PIDPressure")
	httputil.ReturnSuccess(r, w, node)
}

func getKubeletMessage(v *client.HostNode) string {

	for _, condiction := range v.NodeStatus.Conditions {
		if condiction.Type == "kubelet" {
			return condiction.Message
		}
	}
	return ""
}

//GetRuleNodes 获取分角色节点
func GetRuleNodes(w http.ResponseWriter, r *http.Request) {
	rule := chi.URLParam(r, "rule")
	allowRule := map[string]struct{}{
		"compute": struct{}{},
		"manage":  struct{}{},
		"storage": struct{}{},
		"gateway": struct{}{},
	}
	if _, ok := allowRule[rule]; !ok {
		httputil.ReturnError(r, w, 400, rule+" rule is not define")
		return
	}
	nodes, err := nodeService.GetAllNode()
	if err != nil {
		err.Handle(r, w)
		return
	}
	var masternodes []*client.HostNode
	for _, node := range nodes {
		if node.Role.HasRule(rule) {
			masternodes = append(masternodes, node)
		}
	}
	httputil.ReturnSuccess(r, w, masternodes)
}

//Resource -
func Resource(w http.ResponseWriter, r *http.Request) {
	nodeUID := strings.TrimSpace(chi.URLParam(r, "node_id"))
	if len(nodeUID) == 0 {
		err := utils.APIHandleError{
			Code: 404,
			Err:  fmt.Errorf("can't find node by node_id %s", nodeUID),
		}
		err.Handle(r, w)
		return
	}
	res, err := nodeService.GetNodeResource(nodeUID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, res)
}

//CheckNode -
func CheckNode(w http.ResponseWriter, r *http.Request) {
	nodeUID := strings.TrimSpace(chi.URLParam(r, "node_id"))
	if len(nodeUID) == 0 {
		err := utils.APIHandleError{
			Code: 404,
			Err:  fmt.Errorf("can't find node by node_id %s", nodeUID),
		}
		err.Handle(r, w)
		return
	}
	final, err := nodeService.CheckNode(nodeUID)
	if err != nil {
		err.Handle(r, w)
		return
	}

	httputil.ReturnSuccess(r, w, &final)
}
func dealSeq(tasks []*model.ExecedTask) {
	var firsts []*model.ExecedTask
	var keymap map[string]*model.ExecedTask
	for _, v := range tasks {
		keymap[v.ID] = v
		if len(v.Depends) == 0 {
			v.Seq = 0
			firsts = append(firsts, v)
		}
	}
	for _, v := range firsts {
		dealLoopSeq(v, keymap)
	}
}
func dealLoopSeq(task *model.ExecedTask, keymap map[string]*model.ExecedTask) {
	for _, next := range task.Next {
		keymap[next].Seq = task.Seq + 1
		dealLoopSeq(keymap[next], keymap)
	}
}

//DeleteRainbondNode 节点删除
func DeleteRainbondNode(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "node_id")
	err := nodeService.DeleteNode(nodeID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, "delete success")
}

//Cordon 不可调度
func Cordon(w http.ResponseWriter, r *http.Request) {
	nodeUID := strings.TrimSpace(chi.URLParam(r, "node_id"))
	if err := nodeService.CordonNode(nodeUID, true); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//UnCordon 可调度
func UnCordon(w http.ResponseWriter, r *http.Request) {
	nodeUID := strings.TrimSpace(chi.URLParam(r, "node_id"))
	if err := nodeService.CordonNode(nodeUID, false); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//PutLabel 更新节点标签
func PutLabel(w http.ResponseWriter, r *http.Request) {
	nodeUID := strings.TrimSpace(chi.URLParam(r, "node_id"))
	var label = make(map[string]string)
	in, error := ioutil.ReadAll(r.Body)
	if error != nil {
		logrus.Errorf("error read from request ,details %s", error.Error())
		return
	}
	error = json.Unmarshal(in, &label)
	if error != nil {
		logrus.Errorf("error unmarshal labels  ,details %s", error.Error())
		return
	}
	labels, err := nodeService.PutNodeLabel(nodeUID, label)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, labels)
}

//DeleteLabel delete node label
func DeleteLabel(w http.ResponseWriter, r *http.Request) {
	nodeUID := strings.TrimSpace(chi.URLParam(r, "node_id"))
	var label = make(map[string]string)
	in, error := ioutil.ReadAll(r.Body)
	if error != nil {
		logrus.Errorf("error read from request ,details %s", error.Error())
		return
	}
	error = json.Unmarshal(in, &label)
	if error != nil {
		logrus.Errorf("error unmarshal labels  ,details %s", error.Error())
		return
	}
	labels, err := nodeService.DeleteNodeLabel(nodeUID, label)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, labels)
}

//GetLabel get node label
func GetLabel(w http.ResponseWriter, r *http.Request) {
	nodeUID := strings.TrimSpace(chi.URLParam(r, "node_id"))
	labels, err := nodeService.GetNodeLabels(nodeUID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, labels)
}

//ListNodeCondition list node condition
func ListNodeCondition(w http.ResponseWriter, r *http.Request) {
	nodeUID := strings.TrimSpace(chi.URLParam(r, "node_id"))
	node, err := nodeService.GetNode(nodeUID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, node.NodeStatus.Conditions)
}

//DeleteNodeCondition delete node condition
func DeleteNodeCondition(w http.ResponseWriter, r *http.Request) {
	nodeUID := strings.TrimSpace(chi.URLParam(r, "node_id"))
	conditionType := strings.TrimSpace(chi.URLParam(r, "condition"))
	node, err := nodeService.GetNode(nodeUID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	for _, condition := range node.NodeStatus.Conditions {
		if string(condition.Type) == conditionType {
			node, err := nodeService.DeleteNodeCondition(nodeUID, condition.Type)
			if err != nil {
				err.Handle(r, w)
				return
			}
			httputil.ReturnSuccess(r, w, node.NodeStatus.Conditions)
			return
		}
	}
	httputil.ReturnError(r, w, 404, "condition not exist")
}

//DownNode 节点下线，计算节点操作
func DownNode(w http.ResponseWriter, r *http.Request) {
	nodeUID := strings.TrimSpace(chi.URLParam(r, "node_id"))
	n, err := nodeService.GetNode(nodeUID)
	if err != nil {
		err := utils.APIHandleError{
			Code: 402,
			Err:  errors.New(fmt.Sprint("Can not get node by nodeID")),
		}
		err.Handle(r, w)
		return
	}
	if n.Role.HasRule("manage") {
		nodes, _ := nodeService.GetAllNode()
		if nodes != nil && len(nodes) > 0 {
			count := 0
			for _, node := range nodes {
				if node.Role.HasRule("manage") {
					count++
				}
			}
			if count < 2 {
				err := utils.APIHandleError{
					Code: 403,
					Err:  errors.New(fmt.Sprint("manage node less two, can not down it.")),
				}
				err.Handle(r, w)
				return
			}
		}
	}
	logrus.Info("Node down by node api controller: ", nodeUID)
	node, err := nodeService.DownNode(nodeUID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, node)
}

//UpNode 节点上线，计算节点操作
func UpNode(w http.ResponseWriter, r *http.Request) {
	nodeUID := strings.TrimSpace(chi.URLParam(r, "node_id"))
	logrus.Info("Node up by node api controller: ", nodeUID)
	node, err := nodeService.UpNode(nodeUID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, node)
}

//Instances get node service instances
func Instances(w http.ResponseWriter, r *http.Request) {
	nodeUID := strings.TrimSpace(chi.URLParam(r, "node_id"))
	node, err := nodeService.GetNode(nodeUID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	ps, error := kubecli.GetPodsByNodes(nodeUID)
	if error != nil {
		httputil.ReturnError(r, w, 404, error.Error())
		return
	}

	pods := []*model.Pods{}
	var cpuR int64
	var cpuL int64
	var memR int64
	var memL int64
	capCPU := node.AvailableCPU
	capMEM := node.AvailableMemory
	k8snode, _ := kubecli.GetNode(nodeUID)
	if k8snode != nil {
		capCPU = k8snode.Status.Allocatable.Cpu().Value()
		capMEM = k8snode.Status.Allocatable.Memory().Value()
	}
	for _, v := range ps {
		pod := &model.Pods{}
		pod.Namespace = v.Namespace
		serviceID := v.Labels["name"]
		if serviceID == "" {
			continue
		}
		pod.Name = v.Name
		pod.Id = serviceID

		ConditionsStatuss := v.Status.Conditions
		for _, val := range ConditionsStatuss {
			if val.Type == "Ready" {
				pod.Status = string(val.Status)
			}
		}

		//lc := v.Spec.Containers[0].Resources.Limits.Cpu().MilliValue()
		lc := v.Spec.Containers
		for _, v := range lc {
			cpuL += v.Resources.Limits.Cpu().MilliValue()
		}

		//lm := v.Spec.Containers[0].Resources.Limits.Memory().Value()
		lm := v.Spec.Containers
		for _, v := range lm {
			memL += v.Resources.Limits.Memory().Value()
		}

		//rc := v.Spec.Containers[0].Resources.Requests.Cpu().MilliValue()
		rc := v.Spec.Containers
		for _, v := range rc {
			cpuR += v.Resources.Requests.Cpu().MilliValue()
		}

		//rm := v.Spec.Containers[0].Resources.Requests.Memory().Value()
		rm := v.Spec.Containers
		for _, v := range rm {
			memR += v.Resources.Requests.Memory().Value()
		}

		pod.CPURequests = strconv.FormatFloat(float64(cpuR)/float64(1000), 'f', 2, 64)

		pod.CPURequestsR = strconv.FormatFloat(float64(cpuR/10)/float64(capCPU), 'f', 1, 64)

		pod.CPULimits = strconv.FormatFloat(float64(cpuL)/float64(1000), 'f', 2, 64)
		pod.CPULimitsR = strconv.FormatFloat(float64(cpuL/10)/float64(capCPU), 'f', 1, 64)

		pod.MemoryRequests = strconv.Itoa(int(memR))
		pod.MemoryRequestsR = strconv.FormatFloat(float64(memR*100)/float64(capMEM), 'f', 1, 64)
		pod.TenantName = v.Labels["tenant_name"]
		pod.MemoryLimits = strconv.Itoa(int(memL))
		pod.MemoryLimitsR = strconv.FormatFloat(float64(memL*100)/float64(capMEM), 'f', 1, 64)

		pods = append(pods, pod)
	}
	httputil.ReturnSuccess(r, w, pods)
}

//临时存在
func outJSON(w http.ResponseWriter, data interface{}) {
	outJSONWithCode(w, http.StatusOK, data)
}
func outRespSuccess(w http.ResponseWriter, bean interface{}, data []interface{}) {
	outRespDetails(w, 200, "success", "成功", bean, data)
	//m:=model.ResponseBody{}
	//m.Code=200
	//m.Msg="success"
	//m.MsgCN="成功"
	//m.Body.List=data
}
func outRespDetails(w http.ResponseWriter, code int, msg, msgcn string, bean interface{}, data []interface{}) {
	w.Header().Set("Content-Type", "application/json")
	m := model.ResponseBody{}
	m.Code = code
	m.Msg = msg
	m.MsgCN = msgcn
	m.Body.List = data
	m.Body.Bean = bean

	s := ""
	b, err := json.Marshal(m)

	if err != nil {
		s = `{"error":"json.Marshal error"}`
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		s = string(b)
		w.WriteHeader(code)
	}
	fmt.Fprint(w, s)
}
func outJSONWithCode(w http.ResponseWriter, httpCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	s := ""
	b, err := json.Marshal(data)
	fmt.Println(string(b))
	if err != nil {
		s = `{"error":"json.Marshal error"}`
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		s = string(b)
		w.WriteHeader(httpCode)
	}
	fmt.Fprint(w, s)
}

//GetAllNodeHealth get all node health
func GetAllNodeHealth(w http.ResponseWriter, r *http.Request) {
	nodes, err := nodeService.GetAllNode()
	if err != nil {
		err.Handle(r, w)
		return
	}
	StatusMap := make(map[string][]map[string]string, 30)
	roleList := make([]map[string]string, 0, 10)
	for _, n := range nodes {
		for _, v := range n.NodeStatus.Conditions {
			status, ok := StatusMap[string(v.Type)]
			if !ok {
				StatusMap[string(v.Type)] = []map[string]string{
					map[string]string{
						"type":     string(v.Type),
						"status":   string(v.Status),
						"message":  string(v.Message),
						"hostname": n.HostName,
					},
				}
			} else {
				list := status
				list = append(list, map[string]string{
					"type":     string(v.Type),
					"status":   string(v.Status),
					"message":  string(v.Message),
					"hostname": n.HostName,
				},
				)
				StatusMap[string(v.Type)] = list
			}
		}
		roleList = append(roleList, map[string]string{"role": n.Role.String(), "status": n.NodeStatus.Status})
	}
	StatusMap["Role"] = roleList
	httputil.ReturnSuccess(r, w, StatusMap)
}

//UpdateNodeStatus update node status
//`{"status":"installing"}`
func UpdateNodeStatus(w http.ResponseWriter, r *http.Request) {
	var req = make(map[string]string)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.ReturnError(r, w, 400, err.Error())
		return
	}
	nodeUID := strings.TrimSpace(chi.URLParam(r, "node_id"))
	if err := nodeService.UpdateNodeStatus(nodeUID, req["status"]); err != nil {
		err.Handle(r, w)
		return
	}
	node, err := nodeService.GetNode(nodeUID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, node)
}
