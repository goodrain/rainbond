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
	"sync"
	"time"

	"github.com/goodrain/rainbond/util"
	probe "github.com/goodrain/rainbond/util/prober/probes"
	v1 "github.com/goodrain/rainbond/util/prober/types/v1"
	"github.com/sirupsen/logrus"
)

//Prober Prober
type Prober interface {
	GetServiceHealthy(serviceName string) (*v1.HealthStatus, bool)
	WatchServiceHealthy(serviceName string) Watcher
	CloseWatch(serviceName string, id string) error
	Start()
	AddServices(in []*v1.Service)
	CheckIfExist(in *v1.Service) bool
	SetServices([]*v1.Service)
	GetServices() []*v1.Service
	GetServiceHealth() map[string]*v1.HealthStatus
	SetAndUpdateServices([]*v1.Service) error
	AddAndUpdateServices([]*v1.Service) error
	UpdateServiceProbe(service *v1.Service)
	UpdateServicesProbe(services []*v1.Service)
	Stop() error
	DisableWatcher(serviceName, watcherID string)
	EnableWatcher(serviceName, watcherID string)
	GetProbe(name string) probe.Probe
	StopProbes(names []string)
}

// NewProber creates a new prober.
func NewProber(ctx context.Context, cancel context.CancelFunc) Prober {
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
		services:     []*v1.Service{},
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
	services     []*v1.Service
	serviceProbe map[string]probe.Probe
	status       map[string]*v1.HealthStatus
	ctx          context.Context
	cancel       context.CancelFunc
	watches      map[string]map[string]*watcher
	statusChan   chan *v1.HealthStatus
	lock         sync.Mutex
}

type watcher struct {
	manager     Prober
	statusChan  chan *v1.HealthStatus
	id          string
	serviceName string
	enable      bool
}

func (p *probeManager) Start() {
	go p.handleStatus()
	p.updateAllServicesProbe()
}

func (p *probeManager) Stop() error {
	p.cancel()
	return nil
}

func (p *probeManager) GetServiceHealthy(serviceName string) (*v1.HealthStatus, bool) {
	v, ok := p.status[serviceName]
	return v, ok
}

func (p *probeManager) WatchServiceHealthy(serviceName string) Watcher {
	logrus.Debugf("service name: %s;watch service healthy...", serviceName)
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
	logrus.Debugf("start handling status...")
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
	logrus.Debugf("who: %s, status; %s, info: %s", status.Name, status.Status, status.Info)
	p.lock.Lock()
	defer p.lock.Unlock()
	exist, ok := p.status[status.Name]
	if !ok {
		p.status[status.Name] = status
		return
	}
	if exist.LastStatus != status.Status {
		status.StatusChange = true
	} else {
		status.StatusChange = false
	}
	status.LastStatus = status.Status
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

func (p *probeManager) SetServices(in []*v1.Service) {
	p.services = in
}

func (p *probeManager) GetServices() []*v1.Service {
	return p.services
}

func (p *probeManager) SetAndUpdateServices(inner []*v1.Service) error {
	p.services = inner
	p.updateAllServicesProbe()
	return nil
}

// AddAndUpdateServices adds services, then updates all services.
func (p *probeManager) AddAndUpdateServices(in []*v1.Service) error {
	for _, i := range in {
		exist := false
		for index, svc := range p.services {
			if svc.Name == i.Name {
				(p.services)[index] = i
				exist = true
			}
		}
		if !exist {
			p.services = append(p.services, i)
		}
	}
	p.services = append(p.services, in...)
	p.updateAllServicesProbe()
	return nil
}

// AddServices adds services.
func (p *probeManager) AddServices(in []*v1.Service) {
	for _, i := range in {
		exist := false
		for index, svc := range p.services {
			if svc.Name == i.Name {
				(p.services)[index] = i
				exist = true
			}
		}
		if !exist {
			p.services = append(p.services, i)
		}
	}
	p.services = append(p.services, in...)
}

// CheckAndAddService checks if the input service exists, if it does not exist, add it.
func (p *probeManager) CheckIfExist(in *v1.Service) bool {
	exist := false
	for _, svc := range p.services {
		if svc.Name == in.Name {
			exist = true
		}
	}
	return exist
}

func (p *probeManager) isNeedUpdate(new *v1.Service) bool {
	var old *v1.Service
	for _, svc := range p.services {
		if svc.Name == new.Name {
			old = svc
		}
	}
	if old == nil {
		// not found old one
		return true
	}
	return !new.Equal(old)
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

func (p *probeManager) updateAllServicesProbe() {
	if p.services == nil || len(p.services) == 0 {
		return
	}
	for _, pro := range p.serviceProbe {
		pro.Stop()
	}
	p.serviceProbe = make(map[string]probe.Probe, len(p.services))
	for _, v := range p.services {
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

// UpdateServiceProbe updates and runs one service probe.
func (p *probeManager) UpdateServiceProbe(service *v1.Service) {
	if service.ServiceHealth == nil || service.Disable || !p.isNeedUpdate(service) {
		return
	}
	logrus.Debugf("Probe: %s; update probe.", service.Name)
	// stop old probe
	old := p.serviceProbe[service.Name]
	if old != nil {
		old.Stop()
	}
	// create new probe
	serviceProbe := probe.CreateProbe(p.ctx, p.statusChan, service)
	if serviceProbe != nil {
		p.serviceProbe[service.Name] = serviceProbe
		serviceProbe.Check()
	}
}

// UpdateServicesProbe updates and runs services probe.
func (p *probeManager) UpdateServicesProbe(services []*v1.Service) {
	if services == nil || len(services) == 0 {
		return
	}
	oldSvcs := p.ListServicesBySid(services[0].Sid)
	for _, v := range services {
		delete(oldSvcs, v.Name)
		if v.ServiceHealth == nil || !p.isNeedUpdate(v) {
			continue
		}
		if v.Disable {
			p.StopProbes([]string{v.Name})
			continue
		}
		logrus.Debugf("Probe: %s; update probe.", v.Name)
		// stop old probe
		old := p.serviceProbe[v.Name]
		if old != nil {
			old.Stop()
		}
		if !p.CheckIfExist(v) {
			p.services = append(p.services, v)
		}
		// create new probe
		serviceProbe := probe.CreateProbe(p.ctx, p.statusChan, v)
		if serviceProbe != nil {
			p.serviceProbe[v.Name] = serviceProbe
			serviceProbe.Check()
		}
	}
	for _, svc := range oldSvcs {
		p.StopProbes([]string{svc.Name})
	}
	logrus.Debugf("current have probe of third apps number is  %d", len(p.serviceProbe))
}

// ListServicesBySid lists services corresponding to sid.
func (p *probeManager) ListServicesBySid(sid string) map[string]*v1.Service {
	res := make(map[string]*v1.Service)
	for _, svc := range p.services {
		if svc.Sid == sid {
			res[svc.Name] = svc
		}
	}
	return res
}

// GetProbe returns a probe associated with name.
func (p *probeManager) GetProbe(name string) probe.Probe {
	return p.serviceProbe[name]
}

func (p *probeManager) StopProbes(names []string) {
	for _, name := range names {
		probe := p.serviceProbe[name]
		if probe == nil {
			continue
		}
		probe.Stop()
		delete(p.serviceProbe, name)
		for idx, svc := range p.services {
			if svc.Name == name {
				p.services = append(p.services[:idx], p.services[idx+1:]...)
			}
		}
		logrus.Debugf("Name: %s; Probe stopped", name)
	}
}
