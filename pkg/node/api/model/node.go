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

package model

import (
	"os"
	"strconv"
	"syscall"
	"time"
	"strings"

	"k8s.io/client-go/pkg/api/v1"

	client "github.com/coreos/etcd/clientv3"

	conf "github.com/goodrain/rainbond/cmd/node/option"
	store "github.com/goodrain/rainbond/pkg/node/core/store"
)

//HostNode 集群节点实体
type HostNode struct {
	ID              string            `json:"uuid"`
	HostName        string            `json:"host_name"`
	InternalIP      string            `json:"internal_ip"`
	ExternalIP      string            `json:"external_ip"`
	AvailableMemory int64             `json:"available_memory"`
	AvailableCPU    int64             `json:"available_cpu"`
	Role            HostRule          `json:"role"`          //节点属性 compute manage storage
	Status          string            `json:"status"`        //节点状态 create,init,running,stop,delete
	Labels          map[string]string `json:"labels"`        //节点标签 内置标签+用户自定义标签
	Unschedulable   bool              `json:"unschedulable"` //不可调度
	NodeStatus      *v1.NodeStatus    `json:"node_status,omitempty"`
	ClusterNode
}

//HostRule 节点角色
type HostRule []string

//HasRule 是否具有什么角色
func (h HostRule) HasRule(rule string) bool {
	for _, v := range h {
		if v == rule {
			return true
		}
	}
	return false
}
func (h HostRule) String()string{
	return strings.Join(h,",")
}

//NodeConditionType NodeConditionType
type NodeConditionType string

// These are valid conditions of node.
const (
	// NodeReady means this node is working
	NodeReady NodeConditionType = "Ready"
	// InstallNotReady means  the installation task was not completed in this node.
	InstallNotReady NodeConditionType = "InstallNotReady"
)

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

// ClusterNode 集群节点实体
type ClusterNode struct {
	PID        string          `json:"pid"` // 进程 pid
	Version    string          `json:"version"`
	UpTime     time.Time       `json:"up"`        // 启动时间
	DownTime   time.Time       `json:"down"`      // 上次关闭时间
	Alived     bool            `json:"alived"`    // 是否可用
	Connected  bool            `json:"connected"` // 当 Alived 为 true 时有效，表示心跳是否正常
	Conditions []NodeCondition `json:"conditions"`
}

func (n *HostNode) String() string {
	return "node[" + n.ID + "] pid[" + n.PID + "]"
}

//Put 节点更新
func (n *HostNode) Put(opts ...client.OpOption) (*client.PutResponse, error) {
	return store.DefalutClient.Put(conf.Config.Node+n.ID, n.PID, opts...)
}

//PutMaster 注册管理节点
func (n *HostNode) PutMaster(opts ...client.OpOption) (*client.PutResponse, error) {
	return store.DefalutClient.Put(conf.Config.Master+n.ID, n.PID, opts...)
}

//Del 删除
func (n *HostNode) Del() (*client.DeleteResponse, error) {
	return store.DefalutClient.Delete(conf.Config.Node + n.ID)
}

// Exist 判断 node 是否已注册到 etcd
// 存在则返回进行 pid，不存在返回 -1
func (n *HostNode) Exist() (pid int, err error) {
	resp, err := store.DefalutClient.Get(conf.Config.Node + n.ID)
	if err != nil {
		return
	}

	if len(resp.Kvs) == 0 {
		return -1, nil
	}

	if pid, err = strconv.Atoi(string(resp.Kvs[0].Value)); err != nil {
		if _, err = store.DefalutClient.Delete(conf.Config.Node + n.ID); err != nil {
			return
		}
		return -1, nil
	}

	p, err := os.FindProcess(pid)
	if err != nil {
		return -1, nil
	}

	// TODO: 暂时不考虑 linux/unix 以外的系统
	if p != nil && p.Signal(syscall.Signal(0)) == nil {
		return
	}

	return -1, nil
}

//GetNodes 获取节点
func GetNodes() (nodes []*HostNode, err error) {
	return nil, nil
}

// Down 节点下线
func (n *HostNode) Down() {
	n.Alived, n.DownTime = false, time.Now()
	//if err := mgoDB.Upsert(Coll_Node, bson.M{"_id": n.ID}, n); err != nil {
	//	logrus.Errorf(err.Error())
	//}
}
