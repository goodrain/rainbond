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

	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/node/api/model"
	"github.com/goodrain/rainbond/pkg/node/masterserver"
	"github.com/goodrain/rainbond/pkg/node/utils"
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
func (n *NodeService) AddNode(node *model.HostNode) *utils.APIHandleError {
	if n.nodecluster == nil {
		return utils.CreateAPIHandleError(400, fmt.Errorf("this node can not support this api"))
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
