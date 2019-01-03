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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"time"

	"github.com/goodrain/rainbond/util"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/api/model"
	"github.com/goodrain/rainbond/node/kubecache"
	"github.com/goodrain/rainbond/node/masterserver/node"
	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/goodrain/rainbond/node/utils"
	"github.com/twinj/uuid"
)

//NodeService node service
type NodeService struct {
	c           *option.Conf
	nodecluster *node.Cluster
	kubecli     kubecache.KubeClient
}

//CreateNodeService create
func CreateNodeService(c *option.Conf, nodecluster *node.Cluster, kubecli kubecache.KubeClient) *NodeService {
	return &NodeService{
		c:           c,
		nodecluster: nodecluster,
		kubecli:     kubecli,
	}
}

//AddNode add node
func (n *NodeService) AddNode(node *client.APIHostNode) (*client.HostNode, *utils.APIHandleError) {
	if n.nodecluster == nil {
		return nil, utils.CreateAPIHandleError(400, fmt.Errorf("this node can not support this api"))
	}
	if node.Role == "" {
		return nil, utils.CreateAPIHandleError(400, fmt.Errorf("node role must not null"))
	}
	if node.Role != "manage" && node.Role != "compute" {
		return nil, utils.CreateAPIHandleError(400, fmt.Errorf("node role %s not support", node.Role))
	}
	if node.ID == "" {
		node.ID = uuid.NewV4().String()
	}
	if node.InternalIP == "" {
		return nil, utils.CreateAPIHandleError(400, fmt.Errorf("node internal ip can not be empty"))
	}
	if node.HostName == "" {
		return nil, utils.CreateAPIHandleError(400, fmt.Errorf("node hostname can not be empty"))
	}
	if node.RootPass != "" && node.Privatekey != "" {
		return nil, utils.CreateAPIHandleError(400, fmt.Errorf("options private-key and root-pass are conflicting"))
	}
	existNode := n.nodecluster.GetAllNode()
	for _, en := range existNode {
		if node.InternalIP == en.InternalIP {
			return nil, utils.CreateAPIHandleError(400, fmt.Errorf("node internal ip %s is exist", node.InternalIP))
		}
	}
	rbnode := node.Clone()
	rbnode.CreateTime = time.Now()
	n.nodecluster.UpdateNode(rbnode)
	return rbnode, nil
}

//InstallNode install node
func (n *NodeService) InstallNode(node *client.HostNode) *utils.APIHandleError {
	node.Status = client.Installing
	node.NodeStatus.Status = client.Installing
	n.nodecluster.UpdateNode(node)
	go n.AsynchronousInstall(node)
	return nil
}

//UpdateNodeStatus update node status
func (n *NodeService) UpdateNodeStatus(nodeID, status string) *utils.APIHandleError {
	node := n.nodecluster.GetNode(nodeID)
	if node == nil {
		return utils.CreateAPIHandleError(400, errors.New("node can not be found"))
	}
	if status != client.Installing && status != client.InstallFailed && status != client.InstallSuccess {
		return utils.CreateAPIHandleError(400, fmt.Errorf("node can not set status is %s", status))
	}
	node.Status = status
	node.NodeStatus.Status = status
	n.nodecluster.UpdateNode(node)
	return nil
}

//AsynchronousInstall AsynchronousInstall
func (n *NodeService) AsynchronousInstall(node *client.HostNode) {
	linkModel := "pass"
	if node.KeyPath != "" {
		linkModel = "key"
	}
	// start add node script
	logrus.Infof("Begin install node %s", node.ID)
	line := fmt.Sprintf("./node.sh %s %s %s %s %s %s %s", node.Role[0], node.HostName,
		node.InternalIP, linkModel, node.RootPass, node.KeyPath, node.ID)
	fileName := node.HostName + ".log"
	cmd := exec.Command("bash", "-c", line)
	util.CheckAndCreateDir("/grdata/downloads/log/")
	f, err := os.OpenFile("/grdata/downloads/log/"+fileName, os.O_WRONLY|os.O_CREATE|os.O_SYNC, 0755)
	if err != nil {
		logrus.Errorf("open log file %s failure %s", "/grdata/downloads/log/"+fileName, err.Error())
		node.Status = client.InstallFailed
		node.NodeStatus.Status = client.InstallFailed
		n.nodecluster.UpdateNode(node)
		return
	}
	defer f.Close()
	cmd.Stdout = f
	cmd.Dir = "/opt/rainbond/rainbond-ansible/scripts"
	cmd.Stderr = f
	err = cmd.Run()
	if err != nil {
		f.Write([]byte(err.Error()))
		logrus.Errorf("Error executing shell script,View log file：/grdata/downloads/log/" + fileName)
		node.Status = client.InstallFailed
		node.NodeStatus.Status = client.InstallFailed
		n.nodecluster.UpdateNode(node)
		return
	}
	logrus.Infof("Install node %s successful", node.ID)
	node.Status = client.InstallSuccess
	node.NodeStatus.Status = client.InstallSuccess
	n.nodecluster.UpdateNode(node)
}

//DeleteNode delete node
//only node status is offline and node can be deleted
func (n *NodeService) DeleteNode(nodeID string) *utils.APIHandleError {
	node := n.nodecluster.GetNode(nodeID)
	if node == nil {
		return utils.CreateAPIHandleError(404, fmt.Errorf("node is not found"))
	}
	if node.Status == "running" {
		return utils.CreateAPIHandleError(401, fmt.Errorf("node is running, you must closed node process in node %s", nodeID))
	}
	n.nodecluster.RemoveNode(node.ID)
	_, err := node.DeleteNode()
	if err != nil {
		return utils.CreateAPIHandleErrorFromDBError("delete node", err)
	}
	return nil
}

//GetNode get node info
func (n *NodeService) GetNode(nodeID string) (*client.HostNode, *utils.APIHandleError) {
	node := n.nodecluster.GetNode(nodeID)
	if node == nil {
		return nil, utils.CreateAPIHandleError(404, fmt.Errorf("node no found"))
	}
	return node, nil
}

//GetAllNode get all node
func (n *NodeService) GetAllNode() ([]*client.HostNode, *utils.APIHandleError) {
	if n.nodecluster == nil {
		return nil, utils.CreateAPIHandleError(400, fmt.Errorf("this node can not support this api"))
	}
	nodes := n.nodecluster.GetAllNode()
	sort.Sort(client.NodeList(nodes))
	return nodes, nil
}

//GetServicesHealthy get service health
func (n *NodeService) GetServicesHealthy() (map[string][]map[string]string, *utils.APIHandleError) {
	if n.nodecluster == nil {
		return nil, utils.CreateAPIHandleError(400, fmt.Errorf("this node can not support this api"))
	}
	StatusMap := make(map[string][]map[string]string, 30)
	nodes := n.nodecluster.GetAllNode()
	for _, n := range nodes {
		for _, v := range n.NodeStatus.Conditions {
			status, ok := StatusMap[string(v.Type)]
			if !ok {
				StatusMap[string(v.Type)] = []map[string]string{map[string]string{"type": string(v.Type), "status": string(v.Status), "message": string(v.Message), "hostname": n.HostName}}
			} else {
				list := status
				list = append(list, map[string]string{"type": string(v.Type), "status": string(v.Status), "message": string(v.Message), "hostname": n.HostName})
				StatusMap[string(v.Type)] = list
			}

		}
	}
	return StatusMap, nil
}

//CordonNode set node is unscheduler
func (n *NodeService) CordonNode(nodeID string, unschedulable bool) *utils.APIHandleError {
	hostNode, apierr := n.GetNode(nodeID)
	if apierr != nil {
		return apierr
	}
	if !hostNode.Role.HasRule(client.ComputeNode) {
		return utils.CreateAPIHandleError(400, fmt.Errorf("this node can not support this api"))
	}
	k8snode, err := n.kubecli.GetNode(hostNode.ID)
	if err != nil {
		logrus.Errorf("get k8s node(%s) error %s", hostNode.ID, err.Error())
		return utils.CreateAPIHandleError(500, fmt.Errorf("get k8s node(%s) error %s", hostNode.ID, err.Error()))
	}
	hostNode.Unschedulable = unschedulable
	//update k8s node unshcedulable status
	if k8snode != nil {
		node, err := n.kubecli.CordonOrUnCordon(hostNode.ID, unschedulable)
		if err != nil {
			return utils.CreateAPIHandleError(500, fmt.Errorf("set node schedulable info error,%s", err.Error()))
		}
		hostNode.UpdateK8sNodeStatus(*node)
	}
	n.nodecluster.UpdateNode(hostNode)
	return nil
}

//PutNodeLabel update node label
func (n *NodeService) PutNodeLabel(nodeID string, labels map[string]string) *utils.APIHandleError {
	hostNode, apierr := n.GetNode(nodeID)
	if apierr != nil {
		return apierr
	}
	if hostNode.Role.HasRule(client.ComputeNode) {
		node, err := n.kubecli.UpdateLabels(nodeID, labels)
		if err != nil {
			return utils.CreateAPIHandleError(500, fmt.Errorf("update k8s node labels error,%s", err.Error()))
		}
		hostNode.UpdateK8sNodeStatus(*node)
	}
	hostNode.Labels = labels
	n.nodecluster.UpdateNode(hostNode)
	return nil
}

//DownNode down node
func (n *NodeService) DownNode(nodeID string) (*client.HostNode, *utils.APIHandleError) {
	hostNode, apierr := n.GetNode(nodeID)
	if apierr != nil {
		return nil, apierr
	}
	// add the node from k8s if type is compute
	if hostNode.Role.HasRule(client.ComputeNode) {
		err := n.kubecli.DownK8sNode(hostNode.ID)
		if err != nil {
			logrus.Error("Failed to down node: ", err)
			return nil, utils.CreateAPIHandleError(500, fmt.Errorf("k8s node down error,%s", err.Error()))
		}
	}
	hostNode.Status = client.Offline
	hostNode.NodeStatus.Status = client.Offline
	n.nodecluster.UpdateNode(hostNode)
	return hostNode, nil
}

//UpNode up node
func (n *NodeService) UpNode(nodeID string) (*client.HostNode, *utils.APIHandleError) {
	hostNode, apierr := n.GetNode(nodeID)
	if apierr != nil {
		return nil, apierr
	}
	hostNode.Unschedulable = false
	// add the node to k8s if type is compute
	if hostNode.Role.HasRule(client.ComputeNode) {
		if k8snode, _ := n.kubecli.GetNode(hostNode.ID); k8snode == nil {
			node, err := n.kubecli.UpK8sNode(hostNode)
			if err != nil {
				return nil, utils.CreateAPIHandleError(500, fmt.Errorf("k8s node up error,%s", err.Error()))
			}
			hostNode.UpdateK8sNodeStatus(*node)
		}
	}
	hostNode.Status = client.Running
	hostNode.NodeStatus.Status = client.Running
	n.nodecluster.UpdateNode(hostNode)
	return hostNode, nil
}

//InitStatus node init status
func (n *NodeService) InitStatus(nodeIP string) (*model.InitStatus, *utils.APIHandleError) {
	var hostnode client.HostNode
	gotNode := false
	var status model.InitStatus
	i := 0
	for !gotNode && i < 3 {
		list, err := n.GetAllNode()
		if err != nil {
			return nil, err
		}
		for _, v := range list {
			if nodeIP == v.InternalIP {
				hostnode = *v
				gotNode = true
				i = 9
				break
			}
		}
		if i > 0 {
			time.Sleep(time.Second)
		}
		i++
	}
	if i != 10 {
		status.Status = 3
		status.StatusCN = "等待节点加入集群中"
		return &status, nil
	}
	nodeUID := hostnode.ID
	node, err := n.GetNode(nodeUID)
	if err != nil {
		return nil, err
	}
	for _, val := range node.NodeStatus.Conditions {
		if node.NodeStatus.Status == "running" || (val.Type == client.NodeInit && val.Status == client.ConditionTrue) {
			status.Status = 0
			status.StatusCN = "初始化成功"
			status.HostID = node.ID
		} else if val.Type == client.NodeInit && val.Status == client.ConditionFalse {
			status.Status = 1
			status.StatusCN = fmt.Sprintf("初始化失败,%s", val.Message)
		} else {
			status.Status = 2
			status.StatusCN = "初始化中"
		}
	}
	if len(node.NodeStatus.Conditions) == 0 {
		status.Status = 2
		status.StatusCN = "初始化中"
	}
	return &status, nil
}

//GetNodeResource get node resource
func (n *NodeService) GetNodeResource(nodeUID string) (*model.NodePodResource, *utils.APIHandleError) {
	node, err := n.GetNode(nodeUID)
	if err != nil {
		return nil, err
	}
	if !node.Role.HasRule("compute") {
		return nil, utils.CreateAPIHandleError(401, fmt.Errorf("node is not compute node"))
	}
	ps, error := n.kubecli.GetPodsByNodes(nodeUID)
	if error != nil {
		return nil, utils.CreateAPIHandleError(404, err)
	}
	var cpuTotal = node.AvailableCPU
	var memTotal = node.AvailableMemory
	var cpuLimit int64
	var cpuRequest int64
	var memLimit int64
	var memRequest int64
	for _, v := range ps {
		for _, v := range v.Spec.Containers {
			lc := v.Resources.Limits.Cpu().MilliValue()
			cpuLimit += lc
		}
		for _, v := range v.Spec.Containers {
			lm := v.Resources.Limits.Memory().Value()
			memLimit += lm
		}
		for _, v := range v.Spec.Containers {
			rc := v.Resources.Requests.Cpu().MilliValue()
			cpuRequest += rc
		}
		for _, v := range v.Spec.Containers {
			rm := v.Resources.Requests.Memory().Value()
			memRequest += rm
		}
		//lc := v.Spec.Containers[0].Resources.Limits.Cpu().MilliValue()
		//cpuLimit += lc
		//lm := v.Spec.Containers[0].Resources.Limits.Memory().Value()
		//memLimit += lm
		////logrus.Infof("pod %s limit cpu is %s",v.Name,v.Spec.Containers[0].Resources.Limits.Cpu().MilliValue())
		//rc := v.Spec.Containers[0].Resources.Requests.Cpu().MilliValue()
		//cpuRequest += rc
		//rm := v.Spec.Containers[0].Resources.Requests.Memory().Value()
		//memRequest += rm
	}
	var res model.NodePodResource
	res.CPULimits = cpuLimit
	//logrus.Infof("node %s cpu limit is %v",cpuLimit)
	res.CPURequests = cpuRequest
	res.CpuR = int(cpuTotal)
	res.MemR = int(memTotal / 1024 / 1024)
	res.CPULimitsR = strconv.FormatFloat(float64(res.CPULimits*100)/float64(res.CpuR*1000), 'f', 2, 64)
	res.CPURequestsR = strconv.FormatFloat(float64(res.CPURequests*100)/float64(res.CpuR*1000), 'f', 2, 64)
	res.MemoryLimits = memLimit / 1024 / 1024
	res.MemoryLimitsR = strconv.FormatFloat(float64(res.MemoryLimits*100)/float64(res.MemR), 'f', 2, 64)
	res.MemoryRequests = memRequest / 1024 / 1024
	res.MemoryRequestsR = strconv.FormatFloat(float64(res.MemoryRequests*100)/float64(res.MemR), 'f', 2, 64)
	return &res, nil
}

//CheckNode check node install status
func (n *NodeService) CheckNode(nodeUID string) (*model.InstallStatus, *utils.APIHandleError) {

	return nil, nil
}

func dealNext(task *model.ExecedTask, tasks []*model.Task) {
	for _, v := range tasks {
		if v.Temp.Depends != nil {
			for _, dep := range v.Temp.Depends {
				if dep.DependTaskID == task.ID {
					task.Next = append(task.Next, v.ID)
				}
			}
		}
	}
}

func dealDepend(result *model.ExecedTask, task *model.Task) {
	if task.Temp.Depends != nil {

		for _, v := range task.Temp.Depends {
			if v.DetermineStrategy == "SameNode" {
				result.Depends = append(result.Depends, v.DependTaskID)
			}
		}
	}
}
