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

package nodem

import (
	"context"

	"github.com/goodrain/rainbond/node/nodem/logger"

	"github.com/goodrain/rainbond/cmd/node-proxy/option"
	"github.com/goodrain/rainbond/node/api"
	"github.com/goodrain/rainbond/node/nodem/monitor"
)

var sandboxImage = "k8s.gcr.io/pause-amd64:latest"

//NodeManager node manager
type NodeManager struct {
	ctx     context.Context
	monitor monitor.Manager
	cfg     *option.Conf
	apim    *api.Manager
	clm     *logger.ContainerLogManage
}

//NewNodeManager new a node manager
func NewNodeManager(ctx context.Context, conf *option.Conf) (*NodeManager, error) {
	monitor, err := monitor.CreateManager(ctx, conf)
	if err != nil {
		return nil, err
	}
	clm := logger.CreatContainerLogManage(conf)

	nodem := &NodeManager{
		cfg:     conf,
		ctx:     ctx,
		monitor: monitor,
		clm:     clm,
	}
	return nodem, nil
}

//AddAPIManager AddApiManager
func (n *NodeManager) AddAPIManager(apim *api.Manager) error {
	n.apim = apim
	return n.monitor.SetAPIRoute(apim)
}

//Start start
func (n *NodeManager) Start(errchan chan error) error {
	if n.cfg.EnableCollectLog {
		if err := n.clm.Start(); err != nil {
			return err
		}
	}
	go n.monitor.Start(errchan)
	return nil
}

//Stop Stop
func (n *NodeManager) Stop() {
	if n.monitor != nil {
		n.monitor.Stop()
	}
	if n.clm != nil && n.cfg.EnableCollectLog {
		n.clm.Stop()
	}
}

//GetMonitorManager get monitor manager
func (n *NodeManager) GetMonitorManager() monitor.Manager {
	return n.monitor
}
