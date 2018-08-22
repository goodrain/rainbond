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
	"fmt"
	"net/http"
	"strings"

	api "github.com/goodrain/rainbond/util/http"

	"github.com/Sirupsen/logrus"
	"github.com/go-chi/chi"
	//"k8s.io/client-go/pkg/api/v1"

	//"k8s.io/apimachinery/pkg/types"

	"strconv"

	"syscall"

	"github.com/goodrain/rainbond/node/api/model"
	httputil "github.com/goodrain/rainbond/util/http"
)

//GetNodeDetails GetNodeDetails
func GetNodeDetails(w http.ResponseWriter, r *http.Request) {
	nodeUID := strings.TrimSpace(chi.URLParam(r, "node"))
	hostNode, err := nodeService.GetNode(nodeUID)
	if err != nil {
		logrus.Infof("getting node by  uid %s", nodeUID)
		outRespDetails(w, 404, "error get resource "+nodeUID+" from etcd "+err.Error(), "找不到指定节点", nil, nil)
		return
	}
	node, erre := kubecli.GetNodeByName(hostNode.HostName)
	if erre != nil {
		outRespDetails(w, 404, "error get node "+nodeUID+" from core "+err.Error(), "找不到指定节点", nil, nil)
		return
	}
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
	ps, erre := kubecli.GetPodsByNodes(node.Name)
	if erre != nil {
		logrus.Errorf("error get node pod %s", hostNode.ID)
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
		serviceID := v.Labels["name"]
		if serviceID == "" {
			continue
		}
		pod.Name = v.Name
		pod.Id = serviceID

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

//GetNodeBasic GetNodeBasic
func GetNodeBasic(w http.ResponseWriter, r *http.Request) {
	nodeUID := strings.TrimSpace(chi.URLParam(r, "node"))
	hostnode, err := nodeService.GetNode(nodeUID)
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
	ps, _ := kubecli.GetAllPods()
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

//CapRes CapRes
func CapRes(w http.ResponseWriter, r *http.Request) {
	nodes, err := kubecli.GetNodes()
	if err != nil {
		api.ReturnError(r, w, 500, err.Error())
		return
	}
	var capCPU int64
	var capMem int64
	for _, v := range nodes {
		capCPU += v.Status.Capacity.Cpu().Value()
		capMem += v.Status.Capacity.Memory().Value()
	}

	result := new(model.Resource)
	result.CpuR = int(capCPU)
	result.MemR = int(capMem)
	logrus.Infof("get cpu %v and mem %v", capCPU, capMem)
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

//ClusterInfo ClusterInfo
func ClusterInfo(w http.ResponseWriter, r *http.Request) {
	usedNodeList := make([]string, 0, 10)
	nodes, err := kubecli.GetNodes()
	if err != nil {
		api.ReturnError(r, w, 500, err.Error())
		return
	}
	var capCPU int64
	var capMem int64
	for _, v := range nodes {
		if v.Spec.Unschedulable == false {
			capCPU += v.Status.Capacity.Cpu().Value()
			capMem += v.Status.Capacity.Memory().Value()
			usedNodeList = append(usedNodeList, v.Name)
		}
	}
	var cpuR int64
	var memR int64
	for _, node := range usedNodeList {
		pods, _ := kubecli.GetPodsByNodes(node)
		for _, pod := range pods {
			for _, c := range pod.Spec.Containers {
				rc := c.Resources.Requests.Cpu().MilliValue()
				rm := c.Resources.Requests.Memory().Value()
				cpuR += rc
				memR += rm
			}
		}
	}
	disk := DiskUsage("/grdata")
	podMemRequestMB := memR / 1024 / 1024
	result := &model.ClusterResource{
		CapCPU:      int(capCPU),
		CapMem:      int(capMem) / 1024 / 1024,
		ReqCPU:      float32(cpuR) / 1000,
		ReqMem:      int(podMemRequestMB),
		ComputeNode: len(nodes),
		CapDisk:     disk.All,
		ReqDisk:     disk.Used,
	}
	allnodes, _ := nodeService.GetAllNode()
	result.AllNode = len(allnodes)
	for _, n := range allnodes {
		if n.Status != "running" {
			result.NotReadyNode++
		}
	}
	api.ReturnSuccess(r, w, result)
}

func outSuccess(w http.ResponseWriter) {
	s := `{"ok":true}`
	w.WriteHeader(200)
	fmt.Fprint(w, s)
}

func GetServicesHealthy(w http.ResponseWriter, r *http.Request) {
	healthMap, err := nodeService.GetServicesHealthy()
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, healthMap)
}

//GetNodeResource
func GetNodeResource(w http.ResponseWriter, r *http.Request) {
	nodeUID := strings.TrimSpace(chi.URLParam(r, "node_id"))
	hostNode, apierr := nodeService.GetNode(nodeUID)
	if apierr != nil {
		api.ReturnError(r, w, 500, apierr.Error())
		return
	}
	node, err := kubecli.GetNodeByName(hostNode.ID)
	if err != nil {
		api.ReturnError(r, w, 500, err.Error())
		return
	}
	var capCPU int64
	var capMem int64

	capCPU = node.Status.Capacity.Cpu().Value()
	capMem = node.Status.Capacity.Memory().Value()

	var cpuR int64
	var memR int64

	pods, _ := kubecli.GetPodsByNodes(hostNode.ID)
	for _, pod := range pods {
		for _, c := range pod.Spec.Containers {
			rc := c.Resources.Requests.Cpu().MilliValue()
			rm := c.Resources.Requests.Memory().Value()
			cpuR += rc
			memR += rm
		}
	}

	podMemRequestMB := memR / 1024 / 1024
	result := &model.NodeResource{
		CapCPU: int(capCPU),
		CapMem: int(capMem) / 1024 / 1024,
		ReqCPU: float32(cpuR) / 1000,
		ReqMem: int(podMemRequestMB),
	}

	api.ReturnSuccess(r, w, result)
}
