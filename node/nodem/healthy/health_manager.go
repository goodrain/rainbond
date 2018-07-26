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
	"github.com/goodrain/rainbond/node/nodem/client"
	"sync"
)

//Manager Manager
type Manager interface {
	GetServiceHealthy(serviceName string) (*service.HealthStatus, bool)
	WatchServiceHealthy(serviceName string) Watcher
	CloseWatch(serviceName string, id string) error
	Start(hostNode *client.HostNode) error
	AddServices([]*service.Service) error
	Stop() error
}

type Watcher interface {
	Watch() <-chan *service.HealthStatus
	Close() error
}
type watcher struct {
	manager     Manager
	statusChan  chan *service.HealthStatus
	id          string
	serviceName string
}

type probeManager struct {
	services   []*service.Service
	status     map[string]*service.HealthStatus
	ctx        context.Context
	cancel     context.CancelFunc
	watches    map[string]map[string]*watcher
	statusChan chan *service.HealthStatus
	errorNum   map[string]int
	errorTime  map[string]time.Time
	errorFlag  map[string]bool
	lock       sync.Mutex
}

func CreateManager() Manager {
	ctx, cancel := context.WithCancel(context.Background())
	statusChan := make(chan *service.HealthStatus, 100)
	status := make(map[string]*service.HealthStatus)
	watches := make(map[string]map[string]*watcher)
	errorNum := make(map[string]int)
	errorTime := make(map[string]time.Time)
	errorFlag := make(map[string]bool)
	m := &probeManager{
		ctx:        ctx,
		cancel:     cancel,
		statusChan: statusChan,
		status:     status,
		watches:    watches,
		errorNum:   errorNum,
		errorTime:  errorTime,
		errorFlag:  errorFlag,
	}

	return m
}

func (p *probeManager) AddServices(inner []*service.Service) error {
	p.services = inner
	return nil
}

func (p *probeManager) Start(hostNode *client.HostNode) (error) {

	logrus.Info("health mode start")
	go p.HandleStatus()
	logrus.Info("services length===>",len(p.services))
	for _,v :=range p.services{
		logrus.Info("Need to check===>",v.ServiceHealth.Name,v.ServiceHealth.Model)
	}
	for _, v := range p.services {
		if v.ServiceHealth.Model == "http" {
			h := &HttpProbe{
				name:         v.ServiceHealth.Name,
				address:      v.ServiceHealth.Address,
				ctx:          p.ctx,
				cancel:       p.cancel,
				resultsChan:  p.statusChan,
				TimeInterval: v.ServiceHealth.TimeInterval,
				hostNode:     hostNode,
			}
			go h.Check()
		}
		if v.ServiceHealth.Model == "tcp" {
			t := &TcpProbe{
				name:         v.ServiceHealth.Name,
				address:      v.ServiceHealth.Address,
				ctx:          p.ctx,
				cancel:       p.cancel,
				resultsChan:  p.statusChan,
				TimeInterval: v.ServiceHealth.TimeInterval,
				hostNode:     hostNode,
			}
			go t.TcpCheck()
		}
		if v.ServiceHealth.Model == "cmd" {
			s := &ShellProbe{
				name:         v.ServiceHealth.Name,
				address:      v.ServiceHealth.Address,
				ctx:          p.ctx,
				cancel:       p.cancel,
				resultsChan:  p.statusChan,
				TimeInterval: v.ServiceHealth.TimeInterval,
				hostNode:     hostNode,
			}
			go s.ShellCheck()
		}

	}
	return nil
}

func (p *probeManager) updateServiceStatus(status *service.HealthStatus) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if status.Status != service.Stat_healthy {
		number := p.errorNum[status.Name] + 1
		p.errorNum[status.Name] = number
		status.ErrorNumber = number
		if !p.errorFlag[status.Name] {
			p.errorTime[status.Name] = time.Now()
			p.errorFlag[status.Name] = true
		}
		status.ErrorTime = time.Now().Sub(p.errorTime[status.Name])
		p.status[status.Name] = status

	} else {
		p.errorNum[status.Name] = 0
		status.ErrorNumber = 0
		p.errorFlag[status.Name] = false
		status.ErrorTime = 0
		p.status[status.Name] = status
	}

}
func (p *probeManager) HandleStatus() {
	for {
		select {
		case status := <-p.statusChan:
			p.updateServiceStatus(status)
			if watcherMap, ok := p.watches[status.Name]; ok {
				for _, watcher := range watcherMap {
					watcher.statusChan <- status
				}
			}
		case <-p.ctx.Done():
			return
		}
	}
}

func (p *probeManager) Stop() error {
	p.cancel()
	return nil
}
func (p *probeManager) CloseWatch(serviceName string, id string) error {
	channel := p.watches[serviceName][id].statusChan
	close(channel)
	return nil
}
func (p *probeManager) GetServiceHealthy(serviceName string) (*service.HealthStatus, bool) {
	v, ok := p.status[serviceName]
	return v, ok

}

func (w *watcher) Watch() <-chan *service.HealthStatus {
	return w.statusChan
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
		p.lock.Lock()
		p.watches[serviceName] = map[string]*watcher{
			w.id: w,
		}
		p.lock.Unlock()
	}
	return w
}
