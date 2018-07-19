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
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/util"
	"time"
)

//Manager Manager
type Manager interface {
	GetServiceHealthy(serviceName string) *service.HealthStatus
	WatchServiceHealthy(serviceName string) Watcher
	CloseWatch(serviceName string, id string) error
	Start() error
	AddServices([]*service.Service) error
	Stop() error
}

func CreateManager() Manager {
	ctx, cancel := context.WithCancel(context.Background())
	statusChan := make(chan *service.HealthStatus, 100)
	status := make(map[string]*service.HealthStatus)
	watches := make(map[string]map[string]*watcher)
	m := &probeManager{
		ctx:        ctx,
		cancel:     cancel,
		statusChan: statusChan,
		status:     status,
		watches:    watches,
	}

	return m
}

type probeManager struct {
	services []*service.Service
	status   map[string]*service.HealthStatus
	ctx      context.Context
	cancel   context.CancelFunc
	watches  map[string]map[string]*watcher
	//lock sync.Mutex
	statusChan chan *service.HealthStatus
}

func (p *probeManager) AddServices(inner []*service.Service) error {
	p.services = inner
	return nil
}

func (p *probeManager) Start() (error) {

	logrus.Info("health mode start")

	for _, v := range p.services {
		if v.ServiceHealth.Model == "http" {
			h := &HttpProbe{
				name:           v.ServiceHealth.Name,
				address:        v.ServiceHealth.Address,
				ctx:            p.ctx,
				cancel:         p.cancel,
				resultsChan:    p.statusChan,
				TimeInterval:   v.ServiceHealth.TimeInterval,
				MaxErrorNumber: v.ServiceHealth.MaxErrorNumber,
			}
			go h.Check()
		}

	}
	go p.processResult()
	time.Sleep(time.Second*5)
	go p.SubscriptionPush()
	return nil
}

func (p *probeManager) processResult() {

	for {
		result := <-p.statusChan
		p.status[result.Name] = result
	}
}

func (p *probeManager) SubscriptionPush() {
	for {


	for _, service := range p.services {
		if watcherMap, ok := p.watches[service.Name]; ok {
			for _, watcher := range watcherMap {
				watcher.statusChan <- p.status[service.Name]
			}

		}
	}
}}

func (p *probeManager) Stop() error {
	p.cancel()
	return nil
}
func (p *probeManager) CloseWatch(serviceName string, id string) error {
	channel := p.watches[serviceName][id].statusChan
	close(channel)
	return nil
}
func (p *probeManager) GetServiceHealthy(serviceName string) *service.HealthStatus {
	for _, v := range p.services {
		if v.Name == serviceName {
			if v.ServiceHealth.Model == "http" {
				healthMap := GetHttpHealth(v.ServiceHealth.Address)

				return &service.HealthStatus{
					Status: healthMap["status"],
					Info:   healthMap["info"],
				}
			}
		}
	}
	return nil
}

type Watcher interface {
	Watch() *service.HealthStatus
	Close() error
}
type watcher struct {
	manager     Manager
	statusChan  chan *service.HealthStatus
	id          string
	serviceName string
}

func (w *watcher) Watch() *service.HealthStatus {
	return <-w.statusChan
}
func (w *watcher) Close() error {
	return w.manager.CloseWatch(w.serviceName, w.id)
}

func (p *probeManager) WatchServiceHealthy(serviceName string) Watcher {
	healthChannel := make(chan *service.HealthStatus, 10)
	w := &watcher{
		manager:     p,
		statusChan:  healthChannel,
		id:          util.NewUUID(),
		serviceName: serviceName,
	}
	if s, ok := p.watches[serviceName]; ok {
		s[w.id] = w
	} else {
		p.watches[serviceName] = map[string]*watcher{
			w.id: w,
		}
	}
	return w
}
