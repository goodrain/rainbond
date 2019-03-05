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

package prober

import (
	"context"
	"errors"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/prober/probes"
	"github.com/goodrain/rainbond/prober/types/v1"
	"github.com/goodrain/rainbond/util"
	"sync"
	"time"
)

//Manager Manager
type Manager interface {
	GetServiceHealthy(serviceName string) (*v1.HealthStatus, bool)
	GetCurrentServiceHealthy(serviceName string) (*v1.HealthStatus, error)
	WatchServiceHealthy(serviceName string) Watcher
	CloseWatch(serviceName string, id string) error
	Start()
	SetServices(*[]*v1.Service)
	GetServices() *[]*v1.Service
	GetServiceHealth() map[string]*v1.HealthStatus
	SetAndUpdateServices(*[]*v1.Service) error
	Stop() error
	DisableWatcher(serviceName, watcherID string)
	EnableWatcher(serviceName, watcherID string)
}

//CreateManager create manager
func CreateManager() Manager {
	ctx, cancel := context.WithCancel(context.Background())
	statusChan := make(chan *v1.HealthStatus, 100)
	status := make(map[string]*v1.HealthStatus)
	watches := make(map[string]map[string]*watcher)
	m := &probeManager{
		ctx:          ctx,
		cancel:       cancel,
		statusChan:   statusChan,
		status:       status,
		watches:      watches,
		serviceProbe: make(map[string]probe.Probe),
	}
	return m
}

//Watcher watcher
type Watcher interface {
	GetID() string
	GetServiceName() string
	Watch() <-chan *v1.HealthStatus
	Close() error
}

type probeManager struct {
	services     *[]*v1.Service
	serviceProbe map[string]probe.Probe
	status       map[string]*v1.HealthStatus
	ctx          context.Context
	cancel       context.CancelFunc
	watches      map[string]map[string]*watcher
	statusChan   chan *v1.HealthStatus
	lock         sync.Mutex
}

type watcher struct {
	manager     Manager
	statusChan  chan *v1.HealthStatus
	id          string
	serviceName string
	enable      bool
}

func (p *probeManager) Start() {
	go p.handleStatus()
	p.updateServiceProbe()
}

func (p *probeManager) Stop() error {
	p.cancel()
	return nil
}

func (p *probeManager) GetServiceHealthy(serviceName string) (*v1.HealthStatus, bool) {
	v, ok := p.status[serviceName]
	return v, ok
}

func (p *probeManager) GetCurrentServiceHealthy(serviceName string) (*v1.HealthStatus, error) {
	if len(*p.services) == 0 {
		return nil, errors.New("services list is empty")
	}
	for _, v := range *p.services {
		if v.Name == serviceName {
			if v.ServiceHealth.Model == "tcp" {
				statusMap := probe.GetTcpHealth(v.ServiceHealth.Address)
				result := &v1.HealthStatus{
					Name:   v.Name,
					Status: statusMap["status"],
					Info:   statusMap["info"],
				}
				return result, nil
			}
		}
	}
	return nil, errors.New("the service does not exist")
}

func (p *probeManager) WatchServiceHealthy(serviceName string) Watcher {
	healthCh := make(chan *v1.HealthStatus, 10)
	w := &watcher{
		manager:     p,
		statusChan:  healthCh,
		id:          util.NewUUID(),
		serviceName: serviceName,
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	if s, ok := p.watches[serviceName]; ok {
		s[w.id] = w
	} else {
		p.watches[serviceName] = map[string]*watcher{
			w.id: w,
		}
	}
	return w
}

func (p *probeManager) CloseWatch(serviceName string, id string) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	channel := p.watches[serviceName][id].statusChan
	close(channel)
	delete(p.watches[serviceName], id)
	return nil
}

func (p *probeManager) handleStatus() {
	for {
		select {
		case status := <-p.statusChan:
			p.updateServiceStatus(status)
			p.lock.Lock()
			if watcherMap, ok := p.watches[status.Name]; ok {
				for _, watcher := range watcherMap {
					if watcher.enable {
						watcher.statusChan <- status
					}
				}
			}
			p.lock.Unlock()
		case <-p.ctx.Done():
			return
		}
	}
}

func (p *probeManager) updateServiceStatus(status *v1.HealthStatus) {
	p.lock.Lock()
	defer p.lock.Unlock()
	exist, ok := p.status[status.Name]
	if !ok {
		p.status[status.Name] = status
		return
	}
	if status.Status != v1.StatHealthy {
		number := exist.ErrorNumber + 1
		status.ErrorNumber = number
		if exist.StartErrorTime.IsZero() {
			status.StartErrorTime = time.Now()
		} else {
			status.StartErrorTime = exist.StartErrorTime
		}
		status.ErrorDuration = time.Now().Sub(exist.StartErrorTime)
		p.status[status.Name] = status
	} else {
		status.ErrorNumber = 0
		status.ErrorDuration = 0
		var zero time.Time
		status.StartErrorTime = zero
		p.status[status.Name] = status
	}
}

func (p *probeManager) SetServices(in *[]*v1.Service) {
	p.services = in
}

func (p *probeManager) GetServices() *[]*v1.Service {
	return p.services
}

func (p *probeManager) SetAndUpdateServices(inner *[]*v1.Service) error {
	p.services = inner
	p.updateServiceProbe()
	return nil
}

func (p *probeManager) AddAndUpdateServices(inner *[]*v1.Service) error {
	//for _, svc := range *p.services {
	//
	//}
	p.updateServiceProbe()
	return nil
}

func (p *probeManager) GetServiceHealth() map[string]*v1.HealthStatus {
	return p.status
}

func (p *probeManager) EnableWatcher(serviceName, watcherID string) {
	logrus.Info("Enable check healthy status of service: ", serviceName)
	p.lock.Lock()
	defer p.lock.Unlock()
	if s, ok := p.watches[serviceName]; ok {
		if w, ok := s[watcherID]; ok {
			w.enable = true
		}
	} else {
		logrus.Error("Can not enable the watcher: Not found service: ", serviceName)
	}
}

func (p *probeManager) DisableWatcher(serviceName, watcherID string) {
	logrus.Info("Disable check healthy status of service: ", serviceName)
	p.lock.Lock()
	defer p.lock.Unlock()
	if s, ok := p.watches[serviceName]; ok {
		if w, ok := s[watcherID]; ok {
			w.enable = false
		}
	} else {
		logrus.Error("Can not disable the watcher: Not found service: ", serviceName)
	}
}

func (w *watcher) GetServiceName() string {
	return w.serviceName
}

func (w *watcher) GetID() string {
	return w.id
}

func (w *watcher) Watch() <-chan *v1.HealthStatus {
	return w.statusChan
}

func (w *watcher) Close() error {
	return w.manager.CloseWatch(w.serviceName, w.id)
}

func (p *probeManager) updateServiceProbe() {
	for _, pro := range p.serviceProbe {
		pro.Stop()
	}
	p.serviceProbe = make(map[string]probe.Probe, len(*p.services))
	for _, v := range *p.services {
		if v.ServiceHealth == nil {
			continue
		}
		if v.Disable {
			continue
		}
		serviceProbe := probe.CreateProbe(p.ctx, p.statusChan, v)
		if serviceProbe != nil {
			p.serviceProbe[v.Name] = serviceProbe
			serviceProbe.Check()
		}
	}
}
