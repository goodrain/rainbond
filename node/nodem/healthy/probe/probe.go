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

package probe

import (
	"context"

	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/goodrain/rainbond/node/nodem/service"
)

//Probe probe
type Probe interface {
	Check()
	Stop()
}

//CreateProbe create probe
func CreateProbe(ctx context.Context, hostNode *client.HostNode, statusChan chan *service.HealthStatus, v *service.Service) Probe {
	ctx, cancel := context.WithCancel(ctx)
	if v.ServiceHealth.Model == "http" {
		h := &HttpProbe{
			Name:         v.ServiceHealth.Name,
			Address:      v.ServiceHealth.Address,
			Ctx:          ctx,
			Cancel:       cancel,
			ResultsChan:  statusChan,
			TimeInterval: v.ServiceHealth.TimeInterval,
			HostNode:     hostNode,
			MaxErrorsNum: v.ServiceHealth.MaxErrorsNum,
		}
		return h
	}
	if v.ServiceHealth.Model == "tcp" {
		t := &TcpProbe{
			Name:         v.ServiceHealth.Name,
			Address:      v.ServiceHealth.Address,
			Ctx:          ctx,
			Cancel:       cancel,
			ResultsChan:  statusChan,
			TimeInterval: v.ServiceHealth.TimeInterval,
			HostNode:     hostNode,
			MaxErrorsNum: v.ServiceHealth.MaxErrorsNum,
		}
		return t
	}
	if v.ServiceHealth.Model == "cmd" {
		s := &ShellProbe{
			Name:         v.ServiceHealth.Name,
			Address:      v.ServiceHealth.Address,
			Ctx:          ctx,
			Cancel:       cancel,
			ResultsChan:  statusChan,
			TimeInterval: v.ServiceHealth.TimeInterval,
			HostNode:     hostNode,
			MaxErrorsNum: v.ServiceHealth.MaxErrorsNum,
		}
		return s
	}
	cancel()
	return nil
}
