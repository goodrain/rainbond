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
	"github.com/goodrain/rainbond/pkg/node/masterserver"
	"github.com/goodrain/rainbond/pkg/node/utils"
	"github.com/twinj/uuid"
)

//NodeService node service
type NodeService struct {
	c           *option.Conf
	nodecluster *masterserver.NodeCluster
}

//CreateNodeService create
func CreateNodeService(c *option.Conf, nodecluster *masterserver.NodeCluster) *NodeService {
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
	rbnode.CreateTime = time.Now()
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

	}
	_, err := node.DeleteNode()
	if err != nil {
		return utils.CreateAPIHandleErrorFromDBError("delete node", err)
	}
	return nil
}

//GetAllNode get all node
func (n *NodeService) GetAllNode() ([]*model.HostNode, *utils.APIHandleError) {
	if n.nodecluster == nil {
		return nil, utils.CreateAPIHandleError(400, fmt.Errorf("this node can not support this api"))
	}
	return n.nodecluster.GetAllNode(), nil
}
