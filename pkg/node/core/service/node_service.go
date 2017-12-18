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

package service

import (
	"fmt"
	"time"

	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/node/api/model"
	"github.com/goodrain/rainbond/pkg/node/core/k8s"
	"github.com/goodrain/rainbond/pkg/node/masterserver/node"
	"github.com/goodrain/rainbond/pkg/node/utils"
	"github.com/twinj/uuid"
)

//NodeService node service
type NodeService struct {
	c           *option.Conf
	nodecluster *node.NodeCluster
}

//CreateNodeService create
func CreateNodeService(c *option.Conf, nodecluster *node.NodeCluster) *NodeService {
	return &NodeService{
		c:           c,
		nodecluster: nodecluster,
	}
}

//AddNode add node
func (n *NodeService) AddNode(node *model.APIHostNode) *utils.APIHandleError {
	if n.nodecluster == nil {
		return utils.CreateAPIHandleError(400, fmt.Errorf("this node can not support this api"))
	}
	if node.ID == "" {
		node.ID = uuid.NewV4().String()
	}
	if node.InternalIP == "" {
		return utils.CreateAPIHandleError(400, fmt.Errorf("node internal ip can not be empty"))
	}
	existNode := n.nodecluster.GetAllNode()
	for _, en := range existNode {
		if node.InternalIP == en.InternalIP {
			return utils.CreateAPIHandleError(400, fmt.Errorf("node internal ip %s is exist", node.InternalIP))
		}
	}
	rbnode := node.Clone()
	rbnode.Status = "init"
	rbnode.CreateTime = time.Now()
	rbnode.Status = "create"

	rbnode.Conditions = make([]model.NodeCondition, 0)
	if _, err := rbnode.Update(); err != nil {
		return utils.CreateAPIHandleErrorFromDBError("save node", err)
	}
	//判断是否需要安装
	n.nodecluster.CheckNodeInstall(rbnode)
	return nil
}

//DeleteNode 删除节点信息
//只有节点状态属于（离线状态）才能删除
func (n *NodeService) DeleteNode(nodeID string) *utils.APIHandleError {
	node := n.nodecluster.GetNode(nodeID)
	if node.Alived {
		return utils.CreateAPIHandleError(400, fmt.Errorf("node is online, can not delete"))
	}
	//TODO:计算节点，判断节点是否下线
	if node.Role.HasRule(model.ComputeNode) {
		if node.NodeStatus != nil {
			return utils.CreateAPIHandleError(400, fmt.Errorf("node is k8s compute node, can not delete"))
		}
	}
	_, err := node.DeleteNode()
	if err != nil {
		return utils.CreateAPIHandleErrorFromDBError("delete node", err)
	}
	return nil
}

//GetNode 获取node
func (n *NodeService) GetNode(nodeID string) (*model.HostNode, *utils.APIHandleError) {
	node := n.nodecluster.GetNode(nodeID)
	if node == nil {
		return nil, utils.CreateAPIHandleError(404, fmt.Errorf("node no found"))
	}
	return node, nil
}

//GetAllNode get all node
func (n *NodeService) GetAllNode() ([]*model.HostNode, *utils.APIHandleError) {
	if n.nodecluster == nil {
		return nil, utils.CreateAPIHandleError(400, fmt.Errorf("this node can not support this api"))
	}
	return n.nodecluster.GetAllNode(), nil
}

//CordonNode 设置节点不可调度熟悉
func (n *NodeService) CordonNode(nodeID string, unschedulable bool) *utils.APIHandleError {
	hostNode, apierr := n.GetNode(nodeID)
	if apierr != nil {
		return apierr
	}
	if !hostNode.Role.HasRule(model.ComputeNode) {
		return utils.CreateAPIHandleError(400, fmt.Errorf("this node can not support this api"))
	}
	//更新节点状态
	hostNode.Unschedulable = unschedulable
	//k8s节点存在
	if unschedulable {
		hostNode.Status = "unschedulable"
	} else {
		hostNode.Status = "schedulable"
	}
	if hostNode.NodeStatus != nil {
		//true表示drain，不可调度
		node, err := k8s.CordonOrUnCordon(hostNode.ID, unschedulable)
		if err != nil {
			return utils.CreateAPIHandleError(500, fmt.Errorf("set node schedulable info error,%s", err.Error()))
		}
		hostNode.NodeStatus = &node.Status
	}
	n.nodecluster.UpdateNode(hostNode)
	return nil
}

//PutNodeLabel 更新node label
func (n *NodeService) PutNodeLabel(nodeID string, labels map[string]string) *utils.APIHandleError {
	hostNode, apierr := n.GetNode(nodeID)
	if apierr != nil {
		return apierr
	}
	if hostNode.Role.HasRule(model.ComputeNode) && hostNode.NodeStatus != nil {
		node, err := k8s.UpdateLabels(nodeID, labels)
		if err != nil {
			return utils.CreateAPIHandleError(500, fmt.Errorf("update k8s node labels error,%s", err.Error()))
		}
		hostNode.NodeStatus = &node.Status
	}
	hostNode.Labels = labels
	n.nodecluster.UpdateNode(hostNode)
	return nil
}

//DownNode 节点下线
func (n *NodeService) DownNode(nodeID string) (*model.HostNode, *utils.APIHandleError) {
	hostNode, apierr := n.GetNode(nodeID)
	if apierr != nil {
		return nil, apierr
	}
	if !hostNode.Role.HasRule(model.ComputeNode) || hostNode.NodeStatus == nil {
		return nil, utils.CreateAPIHandleError(400, fmt.Errorf("node is not k8s node or it not up"))
	}
	hostNode.Status = "down"
	err := k8s.DeleteNode(hostNode.ID)
	if err != nil {
		return nil, utils.CreateAPIHandleError(500, fmt.Errorf("k8s node down error,%s", err.Error()))
	}
	hostNode.NodeStatus = nil
	n.nodecluster.UpdateNode(hostNode)
	return hostNode, nil
}

//UpNode 节点上线
func (n *NodeService) UpNode(nodeID string) (*model.HostNode, *utils.APIHandleError) {
	hostNode, apierr := n.GetNode(nodeID)
	if apierr != nil {
		return nil, apierr
	}
	if !hostNode.Role.HasRule(model.ComputeNode) || hostNode.NodeStatus != nil {
		return nil, utils.CreateAPIHandleError(400, fmt.Errorf("node is not k8s node or it not down"))
	}
	node, err := k8s.CreatK8sNodeFromRainbonNode(hostNode)
	if err != nil {
		return nil, utils.CreateAPIHandleError(500, fmt.Errorf("k8s node up error,%s", err.Error()))
	}
	hostNode.NodeStatus = &node.Status
	n.nodecluster.UpdateNode(hostNode)
	return hostNode, nil
}
