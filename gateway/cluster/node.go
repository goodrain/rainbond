// RAINBOND, Application Management Platform
// Copyright (C) 2014-2019 Goodrain Co., Ltd.

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

package cluster

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/cmd/gateway/option"
	"github.com/sirupsen/logrus"
)

//NodeManager node manager
type NodeManager struct {
	config    option.Config
	ipManager IPManager
}

//CreateNodeManager create node manager
func CreateNodeManager(ctx context.Context, config option.Config, etcdcli *clientv3.Client) (*NodeManager, error) {
	nm := &NodeManager{
		config: config,
	}
	ipManager, err := CreateIPManager(ctx, config, etcdcli)
	if err != nil {
		return nil, err
	}
	nm.ipManager = ipManager
	return nm, nil
}

// Start -
func (n *NodeManager) Start() error {
	if err := n.ipManager.Start(); err != nil {
		return err
	}
	if ok := n.checkGatewayPort(); !ok {
		return fmt.Errorf("Check gateway node port failure")
	}
	return nil
}

// Stop -
func (n *NodeManager) Stop() {
	n.ipManager.Stop()
}

func (n *NodeManager) checkGatewayPort() bool {
	ports := []uint32{
		uint32(n.config.ListenPorts.Health),
		uint32(n.config.ListenPorts.HTTP),
		uint32(n.config.ListenPorts.HTTPS),
		uint32(n.config.ListenPorts.Status),
		uint32(n.config.ListenPorts.Stream),
	}
	return n.CheckPortAvailable("tcp", ports...)
}

//CheckPortAvailable checks whether the specified port is available
func (n *NodeManager) CheckPortAvailable(protocol string, ports ...uint32) bool {
	if protocol == "" {
		protocol = "tcp"
	}
	timeout := time.Second * 3
	for _, port := range ports {
		c, _ := net.DialTimeout(protocol, fmt.Sprintf("0.0.0.0:%d", port), timeout)
		if c != nil {
			logrus.Errorf("Gateway must need listen port %d, but it has been uesd.", port)
			return false
		}
	}
	return true
}

//IPManager ip manager
func (n *NodeManager) IPManager() IPManager {
	return n.ipManager
}
