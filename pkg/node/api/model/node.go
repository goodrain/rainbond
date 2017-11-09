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
	"strings"
	"time"

	"k8s.io/client-go/pkg/api/v1"

	"github.com/Sirupsen/logrus"
	client "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/pquerna/ffjson/ffjson"

	conf "github.com/goodrain/rainbond/cmd/node/option"
	store "github.com/goodrain/rainbond/pkg/node/core/store"
)

//HostNode 集群节点实体
type HostNode struct {
	ID              string            `json:"uuid"`
	HostName        string            `json:"host_name"`
	InternalIP      string            `json:"internal_ip"`
	ExternalIP      string            `json:"external_ip"`
	RootPass        string            `json:"root_pass,omitempty"`
	AvailableMemory int64             `json:"available_memory"`
	AvailableCPU    int64             `json:"available_cpu"`
	Role            HostRule          `json:"role"`          //节点属性 compute manage storage
	Status          string            `json:"status"`        //节点状态 create,init,running,stop,delete
	Labels          map[string]string `json:"labels"`        //节点标签 内置标签+用户自定义标签
	Unschedulable   bool              `json:"unschedulable"` //不可调度
	NodeStatus      *v1.NodeStatus    `json:"node_status,omitempty"`
	ClusterNode
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
func (h *HostNode) UpdataK8sCondition(conditions []v1.NodeCondition) {
	for _, con := range conditions {
		rbcon := NodeCondition{
			Type:               NodeConditionType(con.Type),
			Status:             ConditionStatus(con.Status),
			LastHeartbeatTime:  con.LastHeartbeatTime.Time,
			LastTransitionTime: con.LastTransitionTime.Time,
			Reason:             con.Reason,
			Message:            con.Message,
		}
		h.UpdataCondition(rbcon)
	}
}

//UpdataCondition 更新状态
func (h *HostNode) UpdataCondition(conditions ...NodeCondition) {
	for _, newcon := range conditions {
		for i, con := range h.Conditions {
			if con.Type == newcon.Type {
				h.Conditions[i] = newcon
				continue
			}
		}
		h.Conditions = append(h.Conditions, newcon)
	}
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
func (h HostRule) String() string {
	return strings.Join(h, ",")
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

//String string
func (h *HostNode) String() string {
	res, _ := ffjson.Marshal(h)
	return string(res)
}

//Put 节点上线更新
func (h *HostNode) Put(opts ...client.OpOption) (*client.PutResponse, error) {
	return store.DefalutClient.Put(conf.Config.OnlineNodePath+"/"+h.ID, h.PID, opts...)
}

//Update 更新节点信息，由节点启动时调用
func (h *HostNode) Update() (*client.PutResponse, error) {
	return store.DefalutClient.Put(conf.Config.NodePath+"/"+h.ID, h.String())
}

//Del 删除
func (h *HostNode) Del() (*client.DeleteResponse, error) {
	return store.DefalutClient.Delete(conf.Config.OnlineNodePath + h.ID)
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
