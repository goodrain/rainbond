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
	"context"
	"errors"
	"github.com/goodrain/rainbond/pkg/gogo"
	"sync"
	"time"

	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/goodrain/rainbond/node/nodem/healthy/probe"
	"github.com/goodrain/rainbond/node/nodem/service"
	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
)

// Manager Manager
type Manager interface {
	GetServiceHealthy(serviceName string) (*service.HealthStatus, bool)
	GetCurrentServiceHealthy(serviceName string) (*service.HealthStatus, error)
	WatchServiceHealthy(serviceName string) Watcher
	CloseWatch(serviceName string, id string) error
	Start(hostNode *client.HostNode) error
	AddServices([]*service.Service) error
	GetServiceHealth() map[string]*service.HealthStatus
	AddServicesAndUpdate([]*service.Service) error
	Stop() error
	DisableWatcher(serviceName, watcherID string)
	EnableWatcher(serviceName, watcherID string)
}

// Watcher watcher
type Watcher interface {
	GetID() string
	GetServiceName() string
	Watch() <-chan *service.HealthStatus
	Close() error
}

type watcher struct {
	manager     Manager
	statusChan  chan *service.HealthStatus
	id          string
	serviceName string
	enable      bool
}

type probeManager struct {
	services     []*service.Service
	serviceProbe map[string]probe.Probe
	status       map[string]*service.HealthStatus
	ctx          context.Context
	cancel       context.CancelFunc
	watches      map[string]map[string]*watcher
	statusChan   chan *service.HealthStatus
	lock         sync.Mutex
	hostNode     *client.HostNode
}

// CreateManager create manager
func CreateManager() Manager {
	ctx, cancel := context.WithCancel(context.Background())
	statusChan := make(chan *service.HealthStatus, 100)
	status := make(map[string]*service.HealthStatus)
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

func (p *probeManager) AddServices(inner []*service.Service) error {
	p.services = inner
	return nil
}
func (p *probeManager) AddServicesAndUpdate(inner []*service.Service) error {
	p.services = inner
	p.updateServiceProbe()
	return nil
}

func (p *probeManager) Start(hostNode *client.HostNode) error {
	p.hostNode = hostNode
	go p.HandleStatus()
	_ = gogo.Go(func(ctx context.Context) error {
		p.HandleStatus()
		return nil
	})
	p.updateServiceProbe()
	return nil
}

func (p *probeManager) updateServiceProbe() {
	p.lock.Lock()
	defer p.lock.Unlock()
	//stop all probe
	for _, pro := range p.serviceProbe {
		pro.Stop()
	}
	//create new probe
	p.serviceProbe = make(map[string]probe.Probe, len(p.services))
	for i := range p.services {
		service := p.services[i]
		if service.ServiceHealth == nil {
			continue
		}
		if service.Disable {
			continue
		}
		serviceProbe, err := probe.CreateProbe(p.ctx, p.hostNode, p.statusChan, service)
		if err != nil {
			logrus.Warningf("create prose for service %s failure,%s", service.Name, err.Error())
		}
		if serviceProbe != nil {
			p.serviceProbe[service.Name] = serviceProbe
			serviceProbe.Check()
			logrus.Infof("create probe for service %s", service.Name)
		}
	}
}

func (p *probeManager) updateServiceStatus(status *service.HealthStatus) {
	p.lock.Lock()
	defer p.lock.Unlock()
	exist, ok := p.status[status.Name]
	if !ok {
		p.status[status.Name] = status
		return
	}
	if status.Status != service.Stat_healthy {
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
func (p *probeManager) updateWatcher(status *service.HealthStatus) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if watcherMap, ok := p.watches[status.Name]; ok {
		for _, watcher := range watcherMap {
			if watcher.enable {
				watcher.statusChan <- status
			}
		}
	}

}
func (p *probeManager) HandleStatus() {
	for {
		select {
		case status := <-p.statusChan:
			p.updateServiceStatus(status)
			p.updateWatcher(status)
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
	p.lock.Lock()
	defer p.lock.Unlock()
	channel := p.watches[serviceName][id].statusChan
	close(channel)
	delete(p.watches[serviceName], id)
	return nil
}
func (p *probeManager) GetServiceHealthy(serviceName string) (*service.HealthStatus, bool) {
	v, ok := p.status[serviceName]
	return v, ok
}

func (w *watcher) GetServiceName() string {
	return w.serviceName
}

func (w *watcher) GetID() string {
	return w.id
}

func (w *watcher) Watch() <-chan *service.HealthStatus {
	return w.statusChan
}

func (w *watcher) Close() error {
	return w.manager.CloseWatch(w.serviceName, w.id)
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

func (p *probeManager) WatchServiceHealthy(serviceName string) Watcher {
	healthChannel := make(chan *service.HealthStatus, 10)
	w := &watcher{
		manager:     p,
		statusChan:  healthChannel,
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

func (p *probeManager) GetCurrentServiceHealthy(serviceName string) (*service.HealthStatus, error) {
	if len(p.services) == 0 {
		return nil, errors.New("services list is empty")
	}
	for _, v := range p.services {
		if v.Name == serviceName {
			if v.ServiceHealth.Model == "http" {
				statusMap := probe.GetHTTPHealth(v.ServiceHealth.Address)
				result := &service.HealthStatus{
					Name:   v.Name,
					Status: statusMap["status"],
					Info:   statusMap["info"],
				}
				return result, nil
			}
			if v.ServiceHealth.Model == "tcp" {
				statusMap := probe.GetTcpHealth(v.ServiceHealth.Address)
				result := &service.HealthStatus{
					Name:   v.Name,
					Status: statusMap["status"],
					Info:   statusMap["info"],
				}
				return result, nil
			}
			if v.ServiceHealth.Model == "cmd" {
				statusMap := probe.GetShellHealth(v.ServiceHealth.Address)
				result := &service.HealthStatus{
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
func (p *probeManager) GetServiceHealth() map[string]*service.HealthStatus {
	p.lock.Lock()
	defer p.lock.Unlock()
	newstatus := make(map[string]*service.HealthStatus, len(p.services))
	for _, s := range p.services {
		if status, ok := p.status[s.Name]; ok {
			newstatus[s.Name] = status
		} else {
			//before not have status set service status is health
			newstatus[s.Name] = &service.HealthStatus{
				Name:   s.Name,
				Status: service.Stat_healthy,
				Info:   "not have health probe or not have status",
			}
		}
	}
	return newstatus
}
