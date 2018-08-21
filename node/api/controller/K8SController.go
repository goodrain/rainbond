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
	conf "github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/core/k8s"

	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	api "github.com/goodrain/rainbond/util/http"

	"github.com/Sirupsen/logrus"
	"github.com/go-chi/chi"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//"k8s.io/client-go/pkg/api/v1"
	"github.com/coreos/etcd/client"
	//"k8s.io/apimachinery/pkg/types"

	"strconv"

	"github.com/goodrain/rainbond/node/api/model"
	"syscall"
)

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

//Resources specified node scheduler resources info
func Resources(w http.ResponseWriter, r *http.Request) {
	result := new(model.Resource)
	cpuR := 0
	memR := 0
	ps, _ := k8s.GetAllPods()
	for _, pv := range ps {
		for _, c := range pv.Spec.Containers {
			rc := c.Resources.Requests.Cpu().String()
			rm := c.Resources.Requests.Memory().String()
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
	for _, v := range nodes {
		if v.NodeStatus != nil {
			capCpu += v.NodeStatus.Capacity.Cpu().Value()
			capMem += v.NodeStatus.Capacity.Memory().Value()
		}
	}

	result := new(model.Resource)
	result.CpuR = int(capCpu)
	result.MemR = int(capMem)
	logrus.Infof("get cpu %v and mem %v", capCpu, capMem)
	api.ReturnSuccess(r, w, result)
}

type DiskStatus struct {
	All  uint64 `json:"all"`
	Used uint64 `json:"used"`
	Free uint64 `json:"free"`
}

// disk usage of path/disk
func DiskUsage(path string) (disk DiskStatus) {
	fs := syscall.Statfs_t{}
	err := syscall.Statfs(path, &fs)
	if err != nil {
		return
	}
	disk.All = fs.Blocks * uint64(fs.Bsize)
	disk.Free = fs.Bfree * uint64(fs.Bsize)
	disk.Used = disk.All - disk.Free
	return
}

func RegionRes(w http.ResponseWriter, r *http.Request) {
	nodes, err := nodeService.GetAllNode()
	if err != nil {
		err.Handle(r, w)
		return
	}
	var capCpu int64
	var capMem int64
	for _, v := range nodes {
		if v.NodeStatus != nil && v.Unschedulable == false {
			capCpu += v.NodeStatus.Capacity.Cpu().Value()
			capMem += v.NodeStatus.Capacity.Memory().Value()
		}
	}
	ps, _ := k8s.GetAllPods()
	var cpuR int64 = 0
	var memR int64 = 0
	for _, pv := range ps {
		for _, c := range pv.Spec.Containers {
			rc := c.Resources.Requests.Cpu().MilliValue()
			rm := c.Resources.Requests.Memory().Value()
			cpuR += rc
			memR += rm
		}
	}
	disk := DiskUsage("/grdata")
	podMemRequestMB := memR / 1024 / 1024
	result := new(model.ClusterResource)
	result.CapCpu = int(capCpu)
	result.CapMem = int(capMem) / 1024 / 1024
	result.ReqCpu = float32(cpuR) / 1000
	result.ReqMem = int(podMemRequestMB)
	result.Node = len(nodes)
	result.Tenant = 0
	result.CapDisk = disk.All
	result.ReqDisk = disk.Used
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
