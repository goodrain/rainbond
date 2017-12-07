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

package controller

import (
	conf "github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/node/core/job"
	"github.com/goodrain/rainbond/pkg/node/core/k8s"
	"github.com/goodrain/rainbond/pkg/node/core/store"

	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	api "github.com/goodrain/rainbond/pkg/util/http"

	"github.com/Sirupsen/logrus"
	"github.com/go-chi/chi"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//"k8s.io/client-go/pkg/api/v1"
	"github.com/coreos/etcd/client"
	//"k8s.io/apimachinery/pkg/types"

	"strconv"

	"github.com/goodrain/rainbond/pkg/node/api/model"

	"github.com/coreos/etcd/clientv3"

	"bytes"

	"github.com/goodrain/rainbond/pkg/util"
)

func LoginCompute(w http.ResponseWriter, r *http.Request) {
	loginInfo := new(model.Login)

	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	err := decoder.Decode(loginInfo)

	_, err = job.UnifiedLogin(loginInfo)
	if err != nil {
		logrus.Errorf("login remote host failed,details %s", err.Error())
		api.ReturnError(r, w, http.StatusBadRequest, err.Error())
		return
	}

	//check instation

	nodeIP := strings.Split(loginInfo.HostPort, ":")[0]
	logrus.Infof("target hostport is %s,node ip is %s", loginInfo.HostPort, nodeIP)

	mayExist, err := k8s.GetSource(conf.Config.K8SNode + nodeIP)
	if err == nil || mayExist != nil {
		//if err != nil {
		//	logrus.Warnf("error wile test node exist,details %s",err.Error())
		//}
		logrus.Infof("already installed")
		api.ReturnError(r, w, 400, "already installed")
		return
	}
	cli2, err := job.UnifiedLogin(loginInfo)
	if err != nil {
		logrus.Errorf("login remote host failed,details %s", err.Error())
		api.ReturnError(r, w, http.StatusBadRequest, err.Error())
		return
	}
	sess, err := cli2.NewSession()
	if err != nil {
		logrus.Errorf("get remote host ssh session failed,details %s", err.Error())
		api.ReturnError(r, w, http.StatusBadRequest, err.Error())
		return
	}
	defer sess.Close()
	buf := bytes.NewBuffer(nil)
	sess.Stdout = buf
	err = sess.Run("cat " + conf.Config.InstalledMarker)
	if err == nil {
		logrus.Infof("already installed,checked by installed marker file,details %s", err.Error())
		api.ReturnError(r, w, 400, "already installed")
		return
	}
	installedType := buf.String()
	if strings.Contains(installedType, "\n") {
		installedType = strings.Replace(installedType, "\n", "", -1)
	}
	if installedType == loginInfo.HostType {
		logrus.Infof("already installed,checked by installed marker file,details %s", err.Error())
		api.ReturnError(r, w, 400, "already installed")
		return
	} else {
		//可以安装
		logrus.Infof("installing new role to a node,whose installed role is %s,instaling %s", installedType, loginInfo.HostType)
	}

	_, err = newComputeNodeToInstall(nodeIP)
	if err != nil {
		logrus.Warnf("reg node %s to build-in jobs failed,details: %s", nodeIP, err.Error())
	}
	//todo 在这里给全局channel<-
	logrus.Infof("prepare add item to channel canRunJob")
	//core.CanRunJob<-nodeIP
	store.DefalutClient.NewRunnable("/acp_node/runnable/"+nodeIP, nodeIP)
	logrus.Infof("add runnable to node ip %s", nodeIP)

	result := new(model.LoginResult)
	result.HostPort = loginInfo.HostPort
	result.LoginType = loginInfo.LoginType
	result.Result = "success"
	//添加一条记录，保存信息

	//sess.Run()

	cnode := &model.HostNode{
		ID:              nodeIP,
		HostName:        nodeIP,
		InternalIP:      nodeIP,
		ExternalIP:      nodeIP,
		Role:            []string{loginInfo.HostType},
		Status:          "installing",
		Unschedulable:   false,
		Labels:          nil,
		AvailableCPU:    0,
		AvailableMemory: 0,
	}
	err = k8s.AddSource(conf.Config.K8SNode+nodeIP, cnode)
	if err != nil {
		logrus.Errorf("error add source ,details %s", err.Error())
		api.ReturnError(r, w, 500, err.Error())
		return
	}
	//k8s.AddSource(conf.Config.K8SNode+node.UUID, node)
	b, _ := json.Marshal(loginInfo)
	store.DefalutClient.Put(conf.Config.ConfigStoragePath+"login/"+strings.Split(loginInfo.HostPort, ":")[0], string(b))

	api.ReturnSuccess(r, w, result)
}
func newComputeNodeToInstall(node string) (*job.JobList, error) {

	//这里改成所有
	// jobs, err := job.GetBuildinJobs() //状态为未安装
	// if err != nil {
	// 	return nil, err
	// }
	// logrus.Infof("added new node %s to jobs", node)
	// err = job.AddNewNodeToJobs(jobs, node)
	// if err != nil {
	// 	return nil, err
	// }
	return nil, nil
}
func NodeInit(w http.ResponseWriter, r *http.Request) {
	// nodeIP := strings.TrimSpace(chi.URLParam(r, "ip"))
	// logrus.Infof("init node whose ip is %s", nodeIP)
	// loginInfo := new(model.Login)
	// resp, err := store.DefalutClient.Get(conf.Config.ConfigPath + "login/" + nodeIP)
	// if err != nil {
	// 	logrus.Errorf("prepare stage  failed,get login info failed,details %s", err.Error())
	// 	api.ReturnError(r, w, http.StatusBadRequest, err.Error())
	// 	return
	// }
	// if resp.Count > 0 {
	// 	err := json.Unmarshal(resp.Kvs[0].Value, loginInfo)
	// 	if err != nil {
	// 		logrus.Errorf("decode request failed,details %s", err.Error())
	// 		api.ReturnError(r, w, http.StatusBadRequest, err.Error())
	// 		return
	// 	}
	// } else {
	// 	logrus.Errorf("prepare stage failed,get login info failed,details %s", err.Error())
	// 	api.ReturnError(r, w, http.StatusBadRequest, err.Error())
	// 	return
	// }
	// logrus.Infof("starting new goruntine to init")
	// go asyncInit(loginInfo, nodeIP)

	api.ReturnSuccess(r, w, nil)
}
func CheckInitStatus(w http.ResponseWriter, r *http.Request) {
	nodeIP := strings.TrimSpace(chi.URLParam(r, "ip"))
	var result InitStatus
	logrus.Infof("geting init status by key %s", conf.Config.InitStatus+nodeIP)
	resp, err := store.DefalutClient.Get(conf.Config.InitStatus+nodeIP, clientv3.WithPrefix())
	if err != nil {
		logrus.Warnf("error getting resp from etcd with given key %s,details %s", conf.Config.InitStatus+nodeIP, err.Error())
		api.ReturnError(r, w, 500, err.Error())
		return
	}
	if resp.Count > 0 {

		status := string(resp.Kvs[0].Value)
		result.Status = status
		if strings.HasPrefix(status, "failed") {
			result.Status = "failed"
			logrus.Infof("init failed")
			errmsg := strings.Split(status, "|")[1]
			result.Msg = errmsg
		}
	} else {
		logrus.Infof("get nothing from etcd")
		result.Status = "uninit"
	}

	api.ReturnSuccess(r, w, &result)
}

type InitStatus struct {
	Status string `json:"status"`
	Msg    string `json:"msg"`
}

func asyncInit(login *model.Login, nodeIp string) {
	//save initing to etcd
	// store.DefalutClient.Put(conf.Config.InitStatus+nodeIp, "initing")
	// logrus.Infof("changing init stauts to initing ")
	// _, err := job.PrepareState(login)
	// if err != nil {
	// 	logrus.Errorf("async prepare stage failed,details %s", err.Error())
	// 	//save error to etcd
	// 	store.DefalutClient.Put(conf.Config.InitStatus+nodeIp, "failed|"+err.Error())

	// 	//api.ReturnError(r,w,http.StatusBadRequest,err.Error())
	// 	return
	// }
	// //save init success to etcd
	// logrus.Infof("changing init stauts to success ")
	store.DefalutClient.Put(conf.Config.InitStatus+nodeIp, "success")
}
func CheckJobGetStatus(w http.ResponseWriter, r *http.Request) {
	nodeIP := strings.TrimSpace(chi.URLParam(r, "ip"))
	jl, err := job.GetJobStatusByNodeIP(nodeIP)
	if err != nil {
		logrus.Warnf("get job status failed")
		api.ReturnError(r, w, http.StatusInternalServerError, err.Error())
	}
	api.ReturnSuccess(r, w, jl)
}

func StartBuildInJobs(w http.ResponseWriter, r *http.Request) {
	nodeIP := strings.TrimSpace(chi.URLParam(r, "ip"))
	logrus.Infof("node start install %s", nodeIP)
	done := make(chan *job.JobList)
	doneOne := make(chan *job.BuildInJob)
	store.DefalutClient.NewRunnable("/acp_node/runnable/"+nodeIP, nodeIP)
	logrus.Infof("adding install runnable to node ip %s", nodeIP)
	jl, err := job.GetJobStatusByNodeIP(nodeIP)
	if err != nil {
		logrus.Warnf("get job status failed")
		api.ReturnError(r, w, http.StatusInternalServerError, err.Error())
	}
	jl.SEQ = util.NewUUID()
	for _, v := range jl.List {
		v.JobSEQ = jl.SEQ
	}
	go writer(jl.SEQ, nodeIP, done, doneOne)
	for _, v := range jl.List {
		v.JobSEQ = jl.SEQ
	}
	job.UpdateNodeJobStatus(nodeIP, jl.List)
	//core.CanRunJob<-nodeIP
	go job.RunBuildJobs(nodeIP, done, doneOne)
	api.ReturnSuccess(r, w, jl)
}

func GetNodeDetails(w http.ResponseWriter, r *http.Request) {
	nodeUID := strings.TrimSpace(chi.URLParam(r, "node"))
	hostNode, err := k8s.GetSource(conf.Config.K8SNode + nodeUID)
	if err != nil {
		logrus.Infof("getting resource of node uid %s", nodeUID)
		outRespDetails(w, 404, "error get resource "+nodeUID+" from etcd "+err.Error(), "找不到指定节点", nil, nil)
		return
	}
	node, err := k8s.GetNodeByName(hostNode.HostName)
	if err != nil {
		outRespDetails(w, 404, "error get node "+nodeUID+" from core "+err.Error(), "找不到指定节点", nil, nil)
		return
	}
	logrus.Debugf("geting node %s 's details from core", nodeUID)

	d := &model.NodeDetails{}
	d.Status = hostNode.Status
	d.Name = node.Name
	d.CreationTimestamp = node.CreationTimestamp.String()
	d.Role = hostNode.Role
	d.Labels = node.Labels
	d.Annotations = node.Annotations
	d.Conditions = nil
	addMap := make(map[string]string)
	for _, v := range node.Status.Addresses {
		addMap[string(v.Type)] = v.Address
	}
	d.Addresses = addMap
	d.ExternalID = node.Spec.ExternalID
	d.Conditions = node.Status.Conditions
	ca := make(map[string]string)

	ca["cpu"] = string(node.Status.Capacity.Cpu().String())
	ca["memory"] = strconv.Itoa(convertMemoryToMBInt(node.Status.Capacity.Memory().String(), false)) + " M"
	ca["pods"] = string(node.Status.Capacity.Pods().String())
	d.Capacity = ca
	ac := make(map[string]string)
	ac["cpu"] = string(node.Status.Allocatable.Cpu().String())
	am := node.Status.Allocatable.Memory().String()
	ac["memory"] = strconv.Itoa(convertMemoryToMBInt(am, false)) + " M"
	ac["pods"] = string(node.Status.Allocatable.Pods().String())

	d.Allocatable = ac

	d.SystemInfo = node.Status.NodeInfo
	ps, err := k8s.GetPodsByNodeName(hostNode.HostName)
	if err != nil {

	}
	rs := make(map[string]string)
	cpuR := 0
	cpuRR := 0
	cpuL := 0
	cpuLR := 0
	memR := 0
	memRR := 0
	memL := 0
	memLR := 0
	pods := []*model.Pods{}
	for _, v := range ps {
		pod := &model.Pods{}
		pod.Namespace = v.Namespace
		serviceId := v.Labels["name"]
		if serviceId == "" {
			continue
		}
		pod.Name = v.Name
		pod.Id = serviceId

		lc := v.Spec.Containers[0].Resources.Limits.Cpu().String()
		cpuL += getCpuInt(lc)
		lm := v.Spec.Containers[0].Resources.Limits.Memory().String()

		memL += convertMemoryToMBInt(lm, true)
		rc := v.Spec.Containers[0].Resources.Requests.Cpu().String()
		cpuR += getCpuInt(rc)
		rm := v.Spec.Containers[0].Resources.Requests.Memory().String()

		memR += convertMemoryToMBInt(rm, true)

		logrus.Infof("namespace %s,podid %s :limit cpu %s,requests cpu %s,limit mem %s,request mem %s", pod.Namespace, pod.Id, lc, rc, lm, rm)
		pod.CPURequests = rc
		pod.CPURequestsR = getFinalRate(true, rc, ca["cpu"], ca["memory"])
		crr, _ := strconv.Atoi(pod.CPURequestsR)
		cpuRR += crr
		pod.CPULimits = lc
		pod.CPULimitsR = getFinalRate(true, lc, ca["cpu"], ca["memory"])
		clr, _ := strconv.Atoi(pod.CPULimitsR)
		cpuLR += clr
		pod.MemoryRequests = strconv.Itoa(convertMemoryToMBInt(rm, true)) + " M"
		pod.MemoryRequestsR = getFinalRate(false, strconv.Itoa(convertMemoryToMBInt(rm, true)), ca["cpu"], ca["memory"])
		mrr, _ := strconv.Atoi(pod.MemoryRequestsR)
		memRR += mrr

		pod.MemoryLimits = strconv.Itoa(convertMemoryToMBInt(lm, true)) + " M"
		pod.MemoryLimitsR = getFinalRate(false, strconv.Itoa(convertMemoryToMBInt(lm, true)), ca["cpu"], ca["memory"])
		mlr, _ := strconv.Atoi(pod.MemoryLimitsR)
		memLR += mlr
		pods = append(pods, pod)
	}
	d.NonterminatedPods = pods
	rs["CPURequests"] = strconv.Itoa(cpuR)
	rs["CPURequestsR"] = strconv.Itoa(cpuRR)
	rs["CPULimits"] = strconv.Itoa(cpuL)
	rs["CPULimitsR"] = strconv.Itoa(cpuLR)
	logrus.Infof("memoryRequest %s,memoryLimits %s", memR, memL)
	rs["MemoryRequests"] = strconv.Itoa(memR)
	rs["MemoryRequestsR"] = strconv.Itoa(memRR)
	rs["MemoryLimits"] = strconv.Itoa(memL)
	rs["MemoryLimitsR"] = strconv.Itoa(memLR)
	d.AllocatedResources = rs
	outRespSuccess(w, d, nil)

}
func convertByteToMB(a string) int {

	i, _ := strconv.Atoi(a)
	logrus.Infof("converting byte %s to mb %s", a, i/1024/1024)
	return i / 1024 / 1024
}
func convertMemoryToMBInt(mem string, pod bool) (v int) {
	if pod {
		if len(mem) != 1 {

			if strings.Contains(mem, "G") || strings.Contains(mem, "g") {
				mem = mem[0 : len(mem)-2]
				v, _ = strconv.Atoi(mem)
				v = v * 1024

			} else if strings.Contains(mem, "M") || strings.Contains(mem, "m") {
				mem = mem[0 : len(mem)-2]
				v, _ = strconv.Atoi(mem)

			} else if strings.Contains(mem, "K") || strings.Contains(mem, "k") {
				mem = mem[0 : len(mem)-2]
				vi, _ := strconv.Atoi(mem)
				v = vi / 1024

			} else {
				vi, _ := strconv.Atoi(mem)
				v = vi / 1024 / 1024

			}

		} else {
			v, _ = strconv.Atoi(mem)

		}
		logrus.Infof("input mem is %s,output mem is %s", mem, v)
	} else {
		if len(mem) != 1 {

			if strings.Contains(mem, "G") {
				mem = mem[0 : len(mem)-2]
				v, _ = strconv.Atoi(mem)
				v = v * 1024

			} else if strings.Contains(mem, "M") {
				mem = mem[0 : len(mem)-2]
				v, _ = strconv.Atoi(mem)

			} else if strings.Contains(mem, "K") {
				mem = mem[0 : len(mem)-2]
				vi, _ := strconv.Atoi(mem)
				v = vi / 1024

			} else {
				vi, _ := strconv.Atoi(mem)
				v = vi / 1024

			}

		} else {
			v, _ = strconv.Atoi(mem)

		}
	}
	return
}

func getCpuInt(v string) int {
	if len(v) != 1 {
		v = v[0 : len(v)-1]
		vi, _ := strconv.Atoi(v)
		return vi
	} else {
		v, _ := strconv.Atoi(v)
		return v
	}
}

func getFinalRate(cpu bool, value string, capCpu, capMemMB string) (result string) {
	defer func() { // 必须要先声明defer，否则不能捕获到panic异常
		if err := recover(); err != nil {
			logrus.Warnf("get resource rate failed,details :%s", err)
			result = "0"
		}
	}()

	if cpu {
		if len(value) != 1 {
			value = value[0 : len(value)-1]
		}
		i, _ := strconv.Atoi(value)
		capCpuInt, _ := strconv.Atoi(capCpu)
		capCpuInt *= 1000
		result = strconv.Itoa(i * 100 / capCpuInt)
	} else {
		capMemMB = capMemMB[0 : len(capMemMB)-2]
		capMemMBInt, _ := strconv.Atoi(capMemMB)
		if len(value) != 1 {

			u, err := strconv.Atoi(value)
			if err != nil {
				logrus.Infof("err occured details:%s", err.Error())
			}

			result = strconv.Itoa(u * 100 / (capMemMBInt))
		} else {
			result = value
		}

	}
	return
}

func GetNodeBasic(w http.ResponseWriter, r *http.Request) {
	nodeUID := strings.TrimSpace(chi.URLParam(r, "node"))
	hostnode, err := k8s.GetSource(conf.Config.K8SNode + nodeUID)
	if err != nil {
		logrus.Infof("getting resource of node uid %s", nodeUID)
		outRespDetails(w, 404, "error get resource "+nodeUID+" from etcd "+err.Error(), "找不到指定节点", nil, nil)
		return
	}
	outRespSuccess(w, hostnode, nil)
}

func Resources(w http.ResponseWriter, r *http.Request) {
	nodeList, err := k8s.K8S.Core().Nodes().List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("error get nodes from k8s ,details %s", err.Error())
		api.ReturnError(r, w, 500, "failed,details "+err.Error())
		return
	}
	result := new(model.Resource)
	cpuR := 0
	memR := 0
	for _, v := range nodeList.Items {

		ps, _ := k8s.GetPodsByNodeName(v.Name)
		for _, pv := range ps {
			rc := pv.Spec.Containers[0].Resources.Requests.Cpu().String()
			rm := pv.Spec.Containers[0].Resources.Requests.Memory().String()
			cpuR += getCpuInt(rc)
			memR += convertMemoryToMBInt(rm, true)
		}
	}
	result.CpuR = cpuR
	result.MemR = memR
	logrus.Infof("get cpu %v and mem %v", cpuR, memR)
	api.ReturnSuccess(r, w, result)
}
func CapRes(w http.ResponseWriter, r *http.Request) {
	nodes, err := nodeService.GetAllNode()
	if err != nil {
		err.Handle(r, w)
		return
	}
	var capCpu int64
	var capMem int64
	for _,v:=range nodes{
		if v.NodeStatus != nil {
			capCpu+=v.NodeStatus.Capacity.Cpu().Value()
			capMem+=v.NodeStatus.Capacity.Memory().Value()
		}
	}

	result := new(model.Resource)
	result.CpuR=int(capCpu)
	result.MemR=int(capMem)
	logrus.Infof("get cpu %v and mem %v", capCpu, capMem)
	api.ReturnSuccess(r, w, result)
}

func RegionRes(w http.ResponseWriter, r *http.Request) {
	nodes, err := nodeService.GetAllNode()
	if err != nil {
		err.Handle(r, w)
		return
	}


	var capCpu int64
	var capMem int64
	for _,v:=range nodes{
		if v.NodeStatus != nil {
			capCpu+=v.NodeStatus.Capacity.Cpu().Value()
			capMem+=v.NodeStatus.Capacity.Memory().Value()
		}
	}
	//
	//tenants, error := db.GetManager().TenantDao().GetALLTenants()
	//if error != nil {
	//	logrus.Errorf("error get tenants ,details %s",error.Error())
	//}
	//s:=len(tenants)
	nodeList, error := k8s.K8S.Core().Nodes().List(metav1.ListOptions{})
	if error != nil {
		logrus.Errorf("error get nodes from k8s ,details %s", error.Error())
		api.ReturnError(r, w, 500, "failed,details "+error.Error())
		return
	}

	cpuR := 0
	memR := 0
	for _, v := range nodeList.Items {

		ps, _ := k8s.GetPodsByNodeName(v.Name)
		for _, pv := range ps {
			rc := pv.Spec.Containers[0].Resources.Requests.Cpu().String()
			rm := pv.Spec.Containers[0].Resources.Requests.Memory().String()
			cpuR += getCpuInt(rc)
			memR += convertMemoryToMBInt(rm, true)
		}
	}


	result := new(model.ClusterResource)
	result.CapCpu=int(capCpu)
	result.CapMem=int(capMem)/1024/1024
	result.ReqCpu = float32(cpuR)/1000
	result.ReqMem = memR
	result.Node=len(nodes)
	result.Tenant=0
	logrus.Infof("get cpu %v and mem %v", capCpu, capMem)
	api.ReturnSuccess(r, w, result)
}
func UpdateNode(w http.ResponseWriter, r *http.Request) {

	nodeUID := strings.TrimSpace(chi.URLParam(r, "node"))
	node := new(model.HostNode)
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	err := decoder.Decode(node)

	if err != nil {
		outRespDetails(w, 400, "bad request", "更新失败，参数错误", nil, nil)
		return
	}
	updatedK8SNode, err := k8s.CreateK8sNode(node)

	_, err = k8s.K8S.Core().Nodes().Update(updatedK8SNode)
	if err != nil {
		outRespDetails(w, 500, "patch core node failed :"+err.Error(), "更新k8s node 失败", nil, nil)
		return
	}
	err = k8s.AddSource(conf.Config.K8SNode+nodeUID, node)
	data, _ := json.Marshal(node)
	logrus.Debugf("updating node %s to %s", nodeUID, string(data))
	if err != nil {
		if err := k8s.K8S.Core().Nodes().Delete(node.HostName, nil); err != nil {
			logrus.Errorf("Unable to register node %q to etcd: error deleting old node: %v", node.HostName, err)
		} else {
			logrus.Errorf("Deleted old node object %q", node.HostName)
		}
		if cerr, ok := err.(client.Error); ok {
			if cerr.Code == client.ErrorCodeNodeExist {
				outRespDetails(w, 400, "node exist", "节点已存在", nil, nil)
				return
			}
		}
		outRespDetails(w, 500, "error saving node ", "存储节点信息失败", nil, nil)

		return
	}
	result := []interface{}{}
	result = append(result, node)
	outRespSuccess(w, nil, result)
}

func AddNode(w http.ResponseWriter, r *http.Request) {

	// swagger:operation PUT /v2/node/{node} v2 AddNode
	//
	// 重新上线计算节点
	//
	// add node
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: name
	//   in: path
	//   description: nodeuid
	//   required: true
	//   type: string
	//   format: string
	//
	// Responses:
	//   '200':
	//    description: '{"ok":true}'

	nodeUID := strings.TrimSpace(chi.URLParam(r, "node"))
	//k8snode,err:=core.GetNodeByName(nodeName) //maybe bug fixed

	node, err := k8s.GetSource(conf.Config.K8SNode + nodeUID)
	if err != nil {
		outRespDetails(w, http.StatusBadRequest, "error get node from etcd ", "etcd获取节点信息失败", nil, nil)
		return
	}
	if node.Status == "offline" && node.Role.HasRule("tree") {
		_, err := k8s.K8S.Core().Nodes().Get(node.HostName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Info("create node to kubernetes")
				newk8sNode, err := k8s.CreateK8sNode(node)
				if err != nil {
					outRespDetails(w, 500, "create node failed "+err.Error(), "解析创建node失败", nil, nil)
					return
				}
				realK8SNode, err := k8s.K8S.Core().Nodes().Create(newk8sNode)
				logrus.Infof("重新上线后node uid为 %s ,下线之前node uid 为 %s ", string(realK8SNode.UID), nodeUID)
				if err != nil {
					if !apierrors.IsAlreadyExists(err) {
						node.Status = "running"
					}
					outRespDetails(w, 500, "create node failed "+err.Error(), "创建k8s节点失败", nil, nil)
					return
				}
				logrus.Debugf("reup node %s (old),creating core node ", nodeUID)
				hostNode, err := k8s.GetSource(conf.Config.K8SNode + string(nodeUID))
				if err != nil {
					outRespDetails(w, 500, "get node resource failed "+err.Error(), "etcd获取node资源失败", nil, nil)
					return
				}
				hostNode.ID = string(realK8SNode.UID)
				hostNode.Status = "running"
				//更改状态
				data, _ := json.Marshal(hostNode)
				logrus.Infof("adding node :%s online ,updated to %s ", string(realK8SNode.UID), string(data))
				err = k8s.AddSource(conf.Config.K8SNode+hostNode.ID, hostNode)
				if err != nil {
					outRespDetails(w, 500, "add new node failed "+err.Error(), "添加新node信息失败", nil, nil)
					return
				}
				err = k8s.DeleteSource(conf.Config.K8SNode + nodeUID)
				if err != nil {
					outRespDetails(w, 500, "delete old node failed "+err.Error(), "删除老node信息失败", nil, nil)
					return
				}

				logrus.Infof("adding node :%s online ,updated to %s ", string(realK8SNode.UID), string(data))
			}
		}
	}

	outRespSuccess(w, nil, nil)
}

func outSuccess(w http.ResponseWriter) {
	s := `{"ok":true}`
	w.WriteHeader(200)
	fmt.Fprint(w, s)
}

type Success struct {
	ok bool `json:"ok"`
}
