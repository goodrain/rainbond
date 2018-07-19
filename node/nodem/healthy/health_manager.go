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

package healthy

import (
	"github.com/goodrain/rainbond/node/nodem/service"
	"context"
	"github.com/goodrain/rainbond/util"
	"time"
	"github.com/Sirupsen/logrus"
)

//Manager Manager
type Manager interface {
	GetServiceHealthy(serviceName string) *service.HealthStatus
	WatchServiceHealthy() <-chan *service.HealthStatus
	Start() error
	AddServices([]*service.Service) error
	Stop() error
}

type ProbeManager struct {
	services []*service.Service
	ctx      context.Context
	cancel   context.CancelFunc
}

func (p *ProbeManager) AddServices(inner []*service.Service) error {
	ctx, cancel := context.WithCancel(context.Background())
	p.ctx = ctx
	p.cancel = cancel
	p.services = inner
	return nil
}


func (p *ProbeManager) Start() (chan service.HealthStatus, error) {

	logrus.Info("health mode start")
	ResultsChan := make(chan service.HealthStatus, 100)
	for _, v := range p.services {
		if v.ServiceHealth.Model == "http" {
			h := &HttpProbe{
				name:        v.ServiceHealth.Name,
				address:     v.ServiceHealth.Address,
				path:        v.ServiceHealth.Path,
				ctx:         p.ctx,
				cancel:      p.cancel,
				resultsChan: ResultsChan,
			}
			go h.Check()
		}

	}
	return ResultsChan, nil
}

func (p *ProbeManager) Stop() error {
	p.cancel()
	return nil
}

func (p *ProbeManager) GetServiceHealthy(serviceName string) *service.HealthStatus {
	for _, v := range p.services {
		if v.Name == serviceName {
			if v.ServiceHealth.Model == "http" {
				healthMap := GetHttpHealth(v.ServiceHealth.Address, v.ServiceHealth.Path)

				return &service.HealthStatus{
					Status: healthMap["status"],
					Info:   healthMap["info"],
				}
			}
		}
	}
	return nil
}

func (p *ProbeManager) WatchServiceHealthy() <-chan *service.HealthStatus {
	healthChannel := make(chan *service.HealthStatus, 100)
	function := func() {
		util.Exec(p.ctx, func() error {
			for _, v := range p.services {
				if v.ServiceHealth.Model == "http" {
					healthMap := GetHttpHealth(v.ServiceHealth.Address, v.ServiceHealth.Path)
					result := &service.HealthStatus{
						Name:   v.ServiceHealth.Name,
						Status: healthMap["status"],
						Info:   healthMap["info"],
					}
					healthChannel <- result
				}
			}
			return nil
		}, time.Second*8)
	}
	go function()

	return healthChannel
}
