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

package client

import (
	"fmt"
	"strings"
	"time"

	client "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	conf "github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/core/store"
	"github.com/goodrain/rainbond/util"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

//LabelOS node label about os
var LabelOS = "beta.kubernetes.io/os"

//APIHostNode api host node
type APIHostNode struct {
	ID          string            `json:"uuid" validate:"uuid"`
	HostName    string            `json:"host_name" validate:"host_name"`
	InternalIP  string            `json:"internal_ip" validate:"internal_ip|ip"`
	ExternalIP  string            `json:"external_ip" validate:"external_ip|ip"`
	RootPass    string            `json:"root_pass,omitempty"`
	Privatekey  string            `json:"private_key,omitempty"`
	Role        HostRule          `json:"role" validate:"role|required"`
	PodCIDR     string            `json:"podCIDR"`
	AutoInstall bool              `json:"auto_install"`
	Labels      map[string]string `json:"labels"`
}

//Clone Clone
func (a APIHostNode) Clone() *HostNode {
	hn := &HostNode{
		ID:           a.ID,
		HostName:     a.HostName,
		InternalIP:   a.InternalIP,
		ExternalIP:   a.ExternalIP,
		RootPass:     a.RootPass,
		KeyPath:      a.Privatekey,
		Role:         a.Role,
		Labels:       map[string]string{"rainbond_node_hostname": a.HostName},
		CustomLabels: map[string]string{},
		NodeStatus:   NodeStatus{Status: "not_installed", Conditions: make([]NodeCondition, 0)},
		Status:       "not_installed",
		PodCIDR:      a.PodCIDR,
		//node default unscheduler
		Unschedulable: true,
	}
	return hn
}

//HostNode rainbond node entity
type HostNode struct {
	ID              string            `json:"uuid"`
	HostName        string            `json:"host_name"`
	CreateTime      time.Time         `json:"create_time"`
	InternalIP      string            `json:"internal_ip"`
	ExternalIP      string            `json:"external_ip"`
	RootPass        string            `json:"root_pass,omitempty"`
	KeyPath         string            `json:"key_path,omitempty"` //管理节点key文件路径
	AvailableMemory int64             `json:"available_memory"`
	AvailableCPU    int64             `json:"available_cpu"`
	Mode            string            `json:"mode"`
	Role            HostRule          `json:"role"` //compute, manage, storage, gateway
	Status          string            `json:"status"`
	Labels          map[string]string `json:"labels"`        // system labels
	CustomLabels    map[string]string `json:"custom_labels"` // custom labels
	Unschedulable   bool              `json:"unschedulable"` // 设置值
	PodCIDR         string            `json:"podCIDR"`
	NodeStatus      NodeStatus        `json:"node_status"`
}

//Resource 资源
type Resource struct {
	CPU  int `json:"cpu"`
	MemR int `json:"mem"`
}

// NodePodResource -
type NodePodResource struct {
	AllocatedResources `json:"allocatedresources"`
	Resource           `json:"allocatable"`
}

// AllocatedResources -
type AllocatedResources struct {
	CPURequests     int64
	CPULimits       int64
	MemoryRequests  int64
	MemoryLimits    int64
	MemoryRequestsR string
	MemoryLimitsR   string
	CPURequestsR    string
	CPULimitsR      string
}

//NodeStatus node status
type NodeStatus struct {
	//worker maintenance
	Version string `json:"version"`
	//worker maintenance example: unscheduler, offline
	//Initiate a recommendation operation to the master based on the node state
	AdviceAction []string `json:"advice_actions"`
	//worker maintenance
	Status string `json:"status"` //installed running offline unknown
	//master maintenance
	CurrentScheduleStatus bool `json:"current_scheduler"`
	//master maintenance
	NodeHealth bool `json:"node_health"`
	//worker maintenance
	NodeUpdateTime time.Time `json:"node_update_time"`
	//master maintenance
	KubeUpdateTime time.Time `json:"kube_update_time"`
	//worker maintenance node progress down time
	LastDownTime time.Time `json:"down_time"`
	//worker and master maintenance
	Conditions []NodeCondition `json:"conditions,omitempty"`
	//master maintenance
	KubeNode *v1.Node
	//worker and master maintenance
	NodeInfo NodeSystemInfo `json:"nodeInfo,omitempty" protobuf:"bytes,7,opt,name=nodeInfo"`
}

//UpdateK8sNodeStatus update rainbond node status by k8s node
func (n *HostNode) UpdateK8sNodeStatus(k8sNode v1.Node) {
	status := k8sNode.Status
	n.UpdataK8sCondition(status.Conditions)
	n.NodeStatus.NodeInfo = NodeSystemInfo{
		MachineID:               status.NodeInfo.MachineID,
		SystemUUID:              status.NodeInfo.SystemUUID,
		BootID:                  status.NodeInfo.BootID,
		KernelVersion:           status.NodeInfo.KernelVersion,
		OSImage:                 status.NodeInfo.OSImage,
		OperatingSystem:         status.NodeInfo.OperatingSystem,
		ContainerRuntimeVersion: status.NodeInfo.ContainerRuntimeVersion,
		Architecture:            status.NodeInfo.Architecture,
	}
}

// MergeLabels merges custom lables into labels.
func (n *HostNode) MergeLabels() map[string]string {
	// TODO: Parallel
	labels := make(map[string]string, len(n.Labels)+len(n.CustomLabels))
	// copy labels
	for k, v := range n.Labels {
		labels[k] = v
	}
	for k, v := range n.CustomLabels {
		if _, ok := n.Labels[k]; !ok {
			labels[k] = v
		}
	}
	return labels
}

// NodeSystemInfo is a set of ids/uuids to uniquely identify the node.
type NodeSystemInfo struct {
	// MachineID reported by the node. For unique machine identification
	// in the cluster this field is preferred. Learn more from man(5)
	// machine-id: http://man7.org/linux/man-pages/man5/machine-id.5.html
	MachineID string `json:"machineID"`
	// SystemUUID reported by the node. For unique machine identification
	// MachineID is preferred. This field is specific to Red Hat hosts
	// https://access.redhat.com/documentation/en-US/Red_Hat_Subscription_Management/1/html/RHSM/getting-system-uuid.html
	SystemUUID string `json:"systemUUID"`
	// Boot ID reported by the node.
	BootID string `json:"bootID" protobuf:"bytes,3,opt,name=bootID"`
	// Kernel Version reported by the node from 'uname -r' (e.g. 3.16.0-0.bpo.4-amd64).
	KernelVersion string `json:"kernelVersion" `
	// OS Image reported by the node from /etc/os-release (e.g. Debian GNU/Linux 7 (wheezy)).
	OSImage string `json:"osImage"`
	// ContainerRuntime Version reported by the node through runtime remote API (e.g. docker://1.5.0).
	ContainerRuntimeVersion string `json:"containerRuntimeVersion"`
	// The Operating System reported by the node
	OperatingSystem string `json:"operatingSystem"`
	// The Architecture reported by the node
	Architecture string `json:"architecture"`

	MemorySize uint64 `json:"memorySize"`
	NumCPU     int64  `json:"cpu_num"`
}

const (
	//Running node running status
	Running = "running"
	//Offline node offline status
	Offline = "offline"
	//Unknown node unknown status
	Unknown = "unknown"
	//Error node error status
	Error = "error"
	//Init node init status
	Init = "init"
	//InstallSuccess node install success status
	InstallSuccess = "install_success"
	//InstallFailed node install failure status
	InstallFailed = "install_failed"
	//Installing node installing status
	Installing = "installing"
	//NotInstalled node not install status
	NotInstalled = "not_installed"
)

//Decode decode node info
func (n *HostNode) Decode(data []byte) error {
	if err := ffjson.Unmarshal(data, n); err != nil {
		logrus.Error("decode node info error:", err.Error())
		return err
	}
	return nil
}

//NodeList node list
type NodeList []*HostNode

func (list NodeList) Len() int {
	return len(list)
}

func (list NodeList) Less(i, j int) bool {
	return list[i].InternalIP < list[j].InternalIP
}

func (list NodeList) Swap(i, j int) {
	var temp = list[i]
	list[i] = list[j]
	list[j] = temp
}

//GetNodeFromKV 从etcd解析node信息
func GetNodeFromKV(kv *mvccpb.KeyValue) *HostNode {
	var node HostNode
	if err := ffjson.Unmarshal(kv.Value, &node); err != nil {
		logrus.Error("parse node info error:", err.Error())
		return nil
	}
	return &node
}

//UpdataK8sCondition 更新k8s节点的状态到rainbond节点
func (n *HostNode) UpdataK8sCondition(conditions []v1.NodeCondition) {
	for _, con := range conditions {
		var rbcon NodeCondition
		if NodeConditionType(con.Type) == "Ready" {
			rbcon = NodeCondition{
				Type:               KubeNodeReady,
				Status:             ConditionStatus(con.Status),
				LastHeartbeatTime:  con.LastHeartbeatTime.Time,
				LastTransitionTime: con.LastTransitionTime.Time,
				Reason:             con.Reason,
				Message:            con.Message,
			}
		} else {
			if con.Status != v1.ConditionFalse {
				rbcon = NodeCondition{
					Type:               KubeNodeReady,
					Status:             ConditionFalse,
					LastHeartbeatTime:  con.LastHeartbeatTime.Time,
					LastTransitionTime: con.LastTransitionTime.Time,
					Reason:             con.Reason,
					Message:            con.Message,
				}
			}
		}
		n.UpdataCondition(rbcon)
	}
}

//DeleteCondition DeleteCondition
func (n *HostNode) DeleteCondition(types ...NodeConditionType) {
	for _, t := range types {
		for i, c := range n.NodeStatus.Conditions {
			if c.Type.Compare(t) {
				n.NodeStatus.Conditions = append(n.NodeStatus.Conditions[:i], n.NodeStatus.Conditions[i+1:]...)
				break
			}
		}
	}
}

// UpdateReadyStatus UpdateReadyStatus
func (n *HostNode) UpdateReadyStatus() {
	var status = ConditionTrue
	var Reason, Message string
	for _, con := range n.NodeStatus.Conditions {
		if con.Status != ConditionTrue && con.Type != "" && con.Type != NodeReady {
			logrus.Debugf("because %s id false, will set node %s(%s) health is false", con.Type, n.ID, n.InternalIP)
			status = ConditionFalse
			Reason = con.Reason
			Message = con.Message
			break
		}
	}
	n.GetAndUpdateCondition(NodeReady, status, Reason, Message)
}

//GetCondition get condition
func (n *HostNode) GetCondition(ctype NodeConditionType) *NodeCondition {
	for _, con := range n.NodeStatus.Conditions {
		if con.Type.Compare(ctype) {
			return &con
		}
	}
	return nil
}

// GetAndUpdateCondition get old condition and update it, if old condition is nil and then create it
func (n *HostNode) GetAndUpdateCondition(condType NodeConditionType, status ConditionStatus, reason, message string) {
	oldCond := n.GetCondition(condType)
	now := time.Now()
	var lastTransitionTime time.Time
	if oldCond == nil {
		lastTransitionTime = now
	} else {
		if oldCond.Status != status {
			lastTransitionTime = now
		} else {
			lastTransitionTime = oldCond.LastTransitionTime
		}
	}
	cond := NodeCondition{
		Type:               condType,
		Status:             status,
		LastHeartbeatTime:  now,
		LastTransitionTime: lastTransitionTime,
		Reason:             reason,
		Message:            message,
	}
	n.UpdataCondition(cond)
}

//UpdataCondition 更新状态
func (n *HostNode) UpdataCondition(conditions ...NodeCondition) {
	for _, newcon := range conditions {
		if newcon.Type == "" {
			continue
		}
		var update bool
		if n.NodeStatus.Conditions != nil {
			for i, con := range n.NodeStatus.Conditions {
				if con.Type.Compare(newcon.Type) {
					n.NodeStatus.Conditions[i] = newcon
					update = true
					break
				}
			}
		}
		if !update {
			n.NodeStatus.Conditions = append(n.NodeStatus.Conditions, newcon)
		}
	}
}

//HostRule 节点角色
type HostRule []string

//SupportNodeRule -
var SupportNodeRule = []string{ComputeNode, ManageNode, StorageNode, GatewayNode}

//ComputeNode 计算节点
var ComputeNode = "compute"

//ManageNode 管理节点
var ManageNode = "manage"

//StorageNode 存储节点
var StorageNode = "storage"

//GatewayNode 边缘负载均衡节点
var GatewayNode = "gateway"

//HasRule 是否具有什么角色
func (h HostRule) HasRule(rule string) bool {
	for _, v := range h {
		if v == rule {
			return true
		}
	}
	return false
}
func (h HostRule) String() string {
	return strings.Join(h, ",")
}

//Add add role
func (h *HostRule) Add(role ...string) {
	for _, r := range role {
		if !util.StringArrayContains(*h, r) {
			*h = append(*h, r)
		}
	}
}

//Validation host rule validation
func (h HostRule) Validation() error {
	if len(h) == 0 {
		return fmt.Errorf("node rule must be seted")
	}
	for _, role := range h {
		if !util.StringArrayContains(SupportNodeRule, role) {
			return fmt.Errorf("node role %s can not be supported", role)
		}
	}
	return nil
}

//NodeConditionType NodeConditionType
type NodeConditionType string

// These are valid conditions of node.
const (
	// NodeReady means this node is working
	NodeReady     NodeConditionType = "Ready"
	KubeNodeReady NodeConditionType = "KubeNodeReady"
	NodeUp        NodeConditionType = "NodeUp"
	// InstallNotReady means  the installation task was not completed in this node.
	InstallNotReady NodeConditionType = "InstallNotReady"
	OutOfDisk       NodeConditionType = "OutOfDisk"
	MemoryPressure  NodeConditionType = "MemoryPressure"
	DiskPressure    NodeConditionType = "DiskPressure"
	PIDPressure     NodeConditionType = "PIDPressure"
)

var masterCondition = []NodeConditionType{NodeReady, KubeNodeReady, NodeUp, InstallNotReady, OutOfDisk, MemoryPressure, DiskPressure, PIDPressure}

//IsMasterCondition Whether it is a preset condition of the system
func IsMasterCondition(con NodeConditionType) bool {
	for _, c := range masterCondition {
		if c.Compare(con) {
			return true
		}
	}
	return false
}

//Compare 比较
func (nt NodeConditionType) Compare(ent NodeConditionType) bool {
	return string(nt) == string(ent)
}

//ConditionStatus ConditionStatus
type ConditionStatus string

// These are valid condition statuses. "ConditionTrue" means a resource is in the condition.
// "ConditionFalse" means a resource is not in the condition. "ConditionUnknown" means kubernetes
// can't decide if a resource is in the condition or not. In the future, we could add other
// intermediate conditions, e.g. ConditionDegraded.
const (
	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
	ConditionUnknown ConditionStatus = "Unknown"
)

// NodeCondition contains condition information for a node.
type NodeCondition struct {
	// Type of node condition.
	Type NodeConditionType `json:"type" `
	// Status of the condition, one of True, False, Unknown.
	Status ConditionStatus `json:"status" `
	// Last time we got an update on a given condition.
	// +optional
	LastHeartbeatTime time.Time `json:"lastHeartbeatTime,omitempty" `
	// Last time the condition transit from one status to another.
	// +optional
	LastTransitionTime time.Time `json:"lastTransitionTime,omitempty" `
	// (brief) reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`
	// Human readable message indicating details about last transition.
	// +optional
	Message string `json:"message,omitempty"`
}

//String string
func (n *HostNode) String() string {
	res, _ := ffjson.Marshal(n)
	return string(res)
}

//Update update node info
func (n *HostNode) Update() (*client.PutResponse, error) {
	savenode := *n
	savenode.NodeStatus.KubeNode = nil
	return store.DefalutClient.Put(conf.Config.NodePath+"/"+n.ID, savenode.String())
}

//DeleteNode delete node
func (n *HostNode) DeleteNode() (*client.DeleteResponse, error) {
	return store.DefalutClient.Delete(conf.Config.NodePath + "/" + n.ID)
}

// DelEndpoints -
func (n *HostNode) DelEndpoints() {
	keys, err := n.listEndpointKeys()
	if err != nil {
		logrus.Warningf("error deleting endpoints: %v", err)
		return
	}
	for _, key := range keys {
		_, err := store.DefalutClient.Delete(key)
		if err != nil {
			logrus.Warnf("key: %s; error delete endpoints: %v", key, err)
		}
	}
}

func (n *HostNode) listEndpointKeys() ([]string, error) {
	resp, err := store.DefalutClient.Get(RainbondEndpointPrefix, client.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("prefix: %s; error list rainbond endpoint keys by prefix: %v", RainbondEndpointPrefix, err)
	}
	var res []string
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		if strings.Contains(key, n.InternalIP) {
			res = append(res, key)
		}
	}
	return res, nil
}
