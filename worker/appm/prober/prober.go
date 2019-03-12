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
	"fmt"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	uitlprober "github.com/goodrain/rainbond/util/prober"
	"github.com/goodrain/rainbond/util/prober/types/v1"
	"github.com/goodrain/rainbond/worker/appm/store"
	"github.com/goodrain/rainbond/worker/appm/thirdparty/discovery"
	appmv1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	corev1 "k8s.io/api/core/v1"
)

// Prober is the interface that wraps the required methods to maintain status
// about upstream servers(Endpoints) associated with a third-party service.
type Prober interface {
	Start()
	Stop()
	AddProbe(ep *corev1.Endpoints)
	StopProbe(ep *corev1.Endpoints)
}

// NewProber creates a new third-party service prober.
func NewProber(store store.Storer,
	probeCh *channels.RingChannel,
	updateCh *channels.RingChannel) Prober {
	ctx, cancel := context.WithCancel(context.Background())
	return &tpProbe{
		utilprober: uitlprober.NewProber(ctx, cancel),
		dbm:        db.GetManager(),
		store:      store,

		updateCh: updateCh,
		probeCh:  probeCh,

		ctx:    ctx,
		cancel: cancel,
	}
}

// third-party service probe
type tpProbe struct {
	utilprober uitlprober.Prober
	dbm        db.Manager
	store      store.Storer

	probeCh  *channels.RingChannel
	updateCh *channels.RingChannel

	ctx    context.Context
	cancel context.CancelFunc
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

	go func() {
		for {
			select {
			case event := <-t.probeCh.Out():
				logrus.Debugf("Probe event received: %+v", event)
				if event == nil {
					return
				}
				evt := event.(store.Event)
				switch evt.Type {
				case store.CreateEvent:
					logrus.Debug("create probe")
					ep := evt.Obj.(*corev1.Endpoints)
					t.AddProbe(ep)
				case store.UpdateEvent:
					logrus.Debug("update probe")
					old := evt.Old.(*corev1.Endpoints)
					ep := evt.Obj.(*corev1.Endpoints)
					t.StopProbe(old)
					t.AddProbe(ep)
				case store.DeleteEvent:
					logrus.Debug("delete probe")
					ep := evt.Obj.(*corev1.Endpoints)
					t.StopProbe(ep)
				}
			case <-t.ctx.Done():
				return
			}
		}
	}()
}

// Stop stops prober.
func (t *tpProbe) Stop() {
	t.cancel()
}

func (t *tpProbe) AddProbe(ep *corev1.Endpoints) {
	services, probeInfo, sid := t.createServices(ep)
	if services == nil || len(services) == 0 {
		logrus.Debugf("empty services, stop creating probe")
		return
	}
	// watch
	logrus.Debugf("Service len: %d; enable watcher...", len(services))
	for _, service := range services {
		if t.utilprober.CheckAndAddService(service) {
			logrus.Debugf("Service: %+v; Exists", service)
			return
		}
		go func(service *v1.Service) {
			watcher := t.utilprober.WatchServiceHealthy(service.Name)
			t.utilprober.EnableWatcher(watcher.GetServiceName(), watcher.GetID())
			defer watcher.Close()
			defer t.utilprober.DisableWatcher(watcher.GetServiceName(), watcher.GetID())
			for {
				select {
				case event := <-watcher.Watch():
					if event == nil {
						return
					}
					switch event.Status {
					case v1.StatHealthy:
						if event.StatusChange {
							logrus.Debugf("is [%s] of service %s.", event.Status, event.Name)
							obj := &appmv1.RbdEndpoint{
								IP:       strings.Replace(service.Name, sid+"-", "", 1),
								Sid:      sid,
								Status:   "healthy",
								IsOnline: true,
							}
							t.updateCh.In() <- discovery.Event{
								Type: discovery.HealthEvent,
								Obj:  obj,
							}
						}
					case v1.StatDeath, v1.StatUnhealthy:
						if event.ErrorNumber > service.ServiceHealth.MaxErrorsNum {
							if probeInfo.FailureAction == model.OfflineFailureAction.String() {
								logrus.Infof("Name: %s; Status: %s; ErrorNumber: %d. Offline.", event.Status, event.Name, event.ErrorNumber)
								obj := &appmv1.RbdEndpoint{
									IP:       strings.Replace(service.Name, sid+"-", "", 1),
									Sid:      sid,
									IsOnline: false,
								}
								t.updateCh.In() <- discovery.Event{
									Type: discovery.OfflineEvent,
									Obj:  obj,
								}
							} else {
								logrus.Infof("Name: %s; Status: %s; ErrorNumber: %d. Change.", event.Status, event.Name, event.ErrorNumber)
								obj := &appmv1.RbdEndpoint{
									IP:     strings.Replace(service.Name, sid+"-", "", 1),
									Sid:    sid,
									Status: "unhealthy",
								}
								t.updateCh.In() <- discovery.Event{
									Type: discovery.HealthEvent,
									Obj:  obj,
								}
							}
						}
					}
				case <-t.ctx.Done():
					return
				}
			}
		}(service)
	}
	// start
	t.utilprober.UpdateServicesProbe(services)
}

func (t *tpProbe) StopProbe(ep *corev1.Endpoints) {
	names := t.createServiceNames(ep)
	if names == nil || len(names) == 0 {
		logrus.Warningf("empty services, stop stoping probes")
		return
	}
	logrus.Debugf("Names: %+v; Stop probes.", names)
	for _, name := range names {
		probe := t.utilprober.GetProbe(name)
		if probe == nil {
			continue
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
			FailureAction:    model.IgnoreFailureAction.String(),
		}, nil
	}
	return probes[0], nil
}

func (t *tpProbe) createServices(ep *corev1.Endpoints) ([]*v1.Service, *model.TenantServiceProbe, string) {
	sid := ep.GetLabels()["service_id"]
	if strings.TrimSpace(sid) == "" {
		logrus.Warningf("Endpoints key: %s; ServiceID not found, stop creating probe",
			fmt.Sprintf("%s/%s", ep.Namespace, ep.Name))
		return nil, nil, ""
	}
	probeInfo, err := t.GetProbeInfo(sid)
	if err != nil {
		logrus.Warningf("ServiceID: %s; Unexpected error occurred, ignore the creation of "+
			"probes: %s", sid, err.Error())
		return nil, nil, ""
	}
	var services []*v1.Service
	for _, subset := range ep.Subsets {
		for _, address := range subset.Addresses {
			service := createService(probeInfo)
			service.Name = fmt.Sprintf("%s-%s", sid, address.IP)
			service.ServiceHealth.Name = fmt.Sprintf("%s-%s", sid, address.IP)
			service.ServiceHealth.Address = func() string {
				return fmt.Sprintf("%s:%d", address.IP, service.ServiceHealth.Port)
			}()
			services = append(services, service)
		}
	}
	return services, probeInfo, sid
}

func (t *tpProbe) createServiceNames(ep *corev1.Endpoints) []string {
	sid := ep.GetLabels()["service_id"]
	if strings.TrimSpace(sid) == "" {
		logrus.Warningf("Endpoints key: %s; ServiceID not found, stop creating probe",
			fmt.Sprintf("%s/%s", ep.Namespace, ep.Name))
		return nil
	}
	var names []string
	for _, subset := range ep.Subsets {
		for _, address := range subset.Addresses {
			names = append(names, fmt.Sprintf("%s-%s", sid, address.IP))
		}
	}
	return names
}