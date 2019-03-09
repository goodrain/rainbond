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
	"encoding/json"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	uitlprober "github.com/goodrain/rainbond/util/prober"
	"github.com/goodrain/rainbond/util/prober/types/v1"
	"github.com/goodrain/rainbond/worker/appm/f"
	"github.com/goodrain/rainbond/worker/appm/store"
	"github.com/goodrain/rainbond/worker/appm/thirdparty/discovery"
	appmv1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	workerutil "github.com/goodrain/rainbond/worker/util"
)

// Prober is the interface that wraps the required methods to maintain status
// about upstream servers(Endpoints) associated with a third-party service.
type Prober interface {
	Init() error
	Start()
	Stop()
	AddProbe(sid string, eps []*appmv1.RbdEndpoint)
	StopProbe(sid string)
}

// NewProber creates a new third-party service prober.
func NewProber(store store.Storer, updateCh *channels.RingChannel) Prober {
	ctx, cancel := context.WithCancel(context.Background())
	return &tpProbe{
		utilprober: uitlprober.NewProber(ctx, cancel),
		dbm:        db.GetManager(),
		store:      store,

		updateCh: updateCh,

		ctx:    ctx,
		cancel: cancel,
	}
}

// third-party service probe
type tpProbe struct {
	utilprober uitlprober.Prober
	dbm        db.Manager
	store      store.Storer

	updateCh *channels.RingChannel

	ctx    context.Context
	cancel context.CancelFunc
}

// Init initiates probes.
func (t *tpProbe) Init() error {
	return nil
}

func createService(probe *model.TenantServiceProbe) *v1.Service {
	return &v1.Service{
		Disable: false,
		ServiceHealth: &v1.Health{
			Port:         probe.Port,
			Model:        probe.Scheme,
			TimeInterval: probe.PeriodSecond,
			MaxErrorsNum: probe.FailureThreshold,
		},
	}
}

func (t *tpProbe) Start() {
	t.utilprober.Start()
}

// Stop stops prober.
func (t *tpProbe) Stop() {
	t.cancel()
}

func (t *tpProbe) AddProbe(sid string, eps []*appmv1.RbdEndpoint) {
	probeInfo, err := t.GetProbeInfo(sid)
	if err != nil {
		logrus.Warningf("ServiceID: %s; Unexpected error occurred, ignore the creation of "+
			"probes: %s", sid, err.Error())
		return
	}
	var services []*v1.Service
	for _, ep := range eps {
		service := createService(probeInfo)
		service.Name = ep.UUID
		service.ServiceHealth.Name = ep.UUID // TODO: unused ServiceHealth.Name, consider to delete it.
		service.ServiceHealth.Address = func() string {
			return fmt.Sprintf("%s:%d", ep.IP, service.ServiceHealth.Port)
		}()
		services = append(services, service)
	}
	if services == nil || len(services) == 0 {
		logrus.Debugf("empty services, stop creating probe")
		return
	}
	t.utilprober.AddServices(services)
	// watch
	logrus.Debugf("enable watcher...")
	for _, service := range services {
		go func() { // TODO: abstract out
			watcher := t.utilprober.WatchServiceHealthy(service.Name)
			t.utilprober.EnableWatcher(watcher.GetServiceName(), watcher.GetID())
			defer watcher.Close()
			defer t.utilprober.DisableWatcher(watcher.GetServiceName(), watcher.GetID())
			for {
				select {
				case event := <-watcher.Watch():
					b, _ := json.Marshal(event)
					logrus.Debugf("Received event: %s", string(b))
					if event == nil {
						return
					}
					switch event.Status {
					case v1.StatHealthy:
						logrus.Debugf("is [%s] of service %s.", event.Status, event.Name)
					case v1.StatDeath, v1.StatUnhealthy:
						if event.ErrorNumber > service.ServiceHealth.MaxErrorsNum {
							// TODO: better msg, consider logrus.infof
							logrus.Debugf("Name: %s; Status: %s; ErrorNumber: %d; ErrorDuration: %v; StartErrorTime: %v; Info: %s",
								event.Name, event.Status, event.ErrorNumber, event.ErrorDuration, event.StartErrorTime, event.Info)
							if probeInfo.FailureAction == model.OfflineFailureAction.String() {
								logrus.Infof("Name: %s; Status: %s; ErrorNumber: %d. Offline.", event.Status, event.Name, event.ErrorNumber)
								obj := &appmv1.RbdEndpoint{
									UUID: service.Name,
									Sid:  sid,
								}
								t.updateCh.In() <- discovery.Event{
									Type: discovery.DeleteEvent,
									Obj:  obj,
								}
							}
						}
					}
				case <-t.ctx.Done():
					return
				}
			}
		}()
	}
	// start
	t.utilprober.UpdateServicesProbe(services)
}

func (t *tpProbe) StopProbe(sid string) {
	as := t.store.GetAppService(sid)
	if as == nil || as.GetRbdEndpionts() == nil || len(as.GetRbdEndpionts()) == 0 {
		return
	}
	eps, err := f.ConvRbdEndpoint(as.GetRbdEndpionts())
	if err != nil {
		logrus.Warningf("error stopping probe: %v", err)
		return
	}
	var services []*v1.Service
	for _, ep := range eps {
		if ep.IPs == nil || len(ep.IPs) == 0 {
			continue
		}
		for _, ip := range ep.IPs {
			service := &v1.Service{
				Name: workerutil.GenServiceName(sid, ip),
			}
			services = append(services, service)
		}
	}
	for _, svc := range services {
		probe := t.utilprober.GetProbe(svc.Name)
		if probe == nil {
			logrus.Warningf("Name: %s; error stopping probe: Probe not found", svc.Name)
			return
		}
		probe.Stop()
	}

}

// GetProbeInfo returns probe info associated with sid.
// If there is a probe in the database, return directly
// If there is no probe in the database, return a default probe
func (t *tpProbe) GetProbeInfo(sid string) (*model.TenantServiceProbe, error) {
	probes, err := t.dbm.ServiceProbeDao().GetServiceProbes(sid)
	if err != nil || probes == nil || len(probes) == 0 {
		// no defined probe, use default one
		logrus.Debugf("no defined probe, use default one")
		ports, err := t.dbm.TenantServicesPortDao().GetOpenedPorts(sid)
		if err != nil {
			return nil, fmt.Errorf("error getting opened ports: %v", err)
		}
		if ports == nil || len(ports) == 0 {
			return nil, fmt.Errorf("Ports not found")
		}
		logrus.Debugf("default port: %d", ports[0].ContainerPort)
		return &model.TenantServiceProbe{
			Port:             ports[0].ContainerPort,
			Scheme:           "tcp",
			PeriodSecond:     5,
			FailureThreshold: 3,
			FailureAction:    model.OfflineFailureAction.String(),
		}, nil
	}
	return probes[0], nil
}
