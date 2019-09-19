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

package thirdparty

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	validation "github.com/goodrain/rainbond/util/endpoint"
	"github.com/goodrain/rainbond/worker/appm/f"
	"github.com/goodrain/rainbond/worker/appm/store"
	"github.com/goodrain/rainbond/worker/appm/thirdparty/discovery"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ThirdPartier is the interface that wraps the required methods to update status
// about upstream servers(Endpoints) associated with a third-party service.
type ThirdPartier interface {
	Start()
}

// NewThirdPartier creates a new ThirdPartier.
func NewThirdPartier(clientset *kubernetes.Clientset,
	store store.Storer,
	startCh *channels.RingChannel,
	updateCh *channels.RingChannel,
	stopCh chan struct{}) ThirdPartier {
	t := &thirdparty{
		clientset: clientset,
		store:     store,

		svcStopCh: make(map[string]chan struct{}),
		startCh:   startCh,
		updateCh:  updateCh,
		stopCh:    stopCh,
	}
	return t
}

type thirdparty struct {
	clientset *kubernetes.Clientset
	store     store.Storer

	// a collection of stop channel for every service.
	svcStopCh map[string]chan struct{}

	startCh  *channels.RingChannel
	updateCh *channels.RingChannel
	stopCh   chan struct{}
}

// Start starts receiving event that update k8s endpoints status from start channel(startCh).
func (t *thirdparty) Start() {
	go func() {
		for {
			select {
			case event := <-t.startCh.Out():
				evt, ok := event.(*v1.Event)
				if !ok {
					logrus.Warningf("Unexpected event received %+v", event)
					continue
				}
				logrus.Debugf("Received event: %+v", evt)
				if evt.Type == v1.StartEvent { // no need to distinguish between event types
					needWatch := false
					stopCh := t.svcStopCh[evt.Sid]
					if stopCh == nil {
						logrus.Debugf("ServiceID: %s; already started.", evt.Sid)
						needWatch = true
						t.svcStopCh[evt.Sid] = make(chan struct{})
					}
					go t.runStart(evt.Sid, needWatch)
				}
				if evt.Type == v1.StopEvent {
					stopCh := t.svcStopCh[evt.Sid]
					if stopCh == nil {
						logrus.Warningf("ServiceID: %s; The third-party service has not started yet, cant't be stoped", evt.Sid)
						continue
					}
					t.runDelete(evt.Sid)
					close(stopCh)
					delete(t.svcStopCh, evt.Sid)
				}
			case event := <-t.updateCh.Out():
				devent, ok := event.(discovery.Event)
				if !ok {
					logrus.Warningf("Unexpected event received %+v", event)
					continue
				}
				go t.runUpdate(devent)
			case <-t.stopCh:
				for _, stopCh := range t.svcStopCh {
					close(stopCh)
				}
				return
			}
		}
	}()
}

func (t *thirdparty) runStart(sid string, needWatch bool) {
	as := t.store.GetAppService(sid)
	if as == nil {
		logrus.Warnf("get app service from store failure, sid=%s", sid)
		return
	}
	var err error
	for i := 3; i > 0; i-- { // retry 3 times
		rbdeps, ir := t.ListRbdEndpoints(sid)
		if rbdeps == nil || len(rbdeps) == 0 {
			logrus.Warningf("ServiceID: %s;Empty rbd endpoints, stop starting third-party service.", sid)
			continue
		}

		var eps []*corev1.Endpoints
		eps, err = t.k8sEndpoints(as, rbdeps)
		if err != nil {
			logrus.Warningf("ServiceID: %s; error creating k8s endpoints: %s", sid, err.Error())
			continue
		}
		for _, ep := range eps {
			f.EnsureEndpoints(ep, t.clientset)
		}

		for _, service := range as.GetServices() {
			f.EnsureService(service, t.clientset)
		}

		if needWatch && ir != nil {
			ir.Watch()
		}
		logrus.Infof("ServiceID: %s; successfully running start task", sid)
		return
	}
	logrus.Errorf("ServiceID: %s; error running start task: %v", sid, err)
}

// ListRbdEndpoints lists all rbd endpoints, include static and dynamic.
func (t *thirdparty) ListRbdEndpoints(sid string) ([]*v1.RbdEndpoint, Interacter) {
	var res []*v1.RbdEndpoint
	// static
	s := NewStaticInteracter(sid)
	slist, err := s.List()
	if err != nil {
		logrus.Warningf("ServiceID: %s;error listing static rbd endpoints: %v", sid, err)
	}
	if slist != nil && len(slist) > 0 {
		res = append(res, slist...)
	}
	d := NewDynamicInteracter(sid, t.updateCh, t.stopCh)
	if d != nil {
		dlist, err := d.List()
		if err != nil {
			logrus.Warningf("ServiceID: %s;error listing dynamic rbd endpoints: %v", sid, err)
		}
		if dlist != nil && len(dlist) > 0 {
			res = append(res, dlist...)
		}
	}
	return res, d
}

func deleteSubset(as *v1.AppService, rbdep *v1.RbdEndpoint) {
	eps := as.GetEndpoints()
	for _, ep := range eps {
		for idx, item := range ep.Subsets {
			if item.Ports[0].Name == rbdep.UUID {
				logrus.Debugf("UUID: %s; subset deleted", rbdep.UUID)
				ep.Subsets[idx] = ep.Subsets[len(ep.Subsets)-1]
				ep.Subsets = ep.Subsets[:len(ep.Subsets)-1]
			}
			isDomain := false
			for _, addr := range item.Addresses {
				if addr.IP == "8.8.8.8" {
					isDomain = true
				}
			}
			for _, addr := range item.NotReadyAddresses {
				if addr.IP == "8.8.8.8" {
					isDomain = true
				}
			}
			if isDomain {
				for _, service := range as.GetServices() {
					if service.Annotations != nil {
						delete(service.Annotations, "domain")
					}
				}
			}
		}
	}
}

func (t *thirdparty) k8sEndpoints(as *v1.AppService, epinfo []*v1.RbdEndpoint) ([]*corev1.Endpoints, error) {
	ports, err := db.GetManager().TenantServicesPortDao().GetPortsByServiceID(as.ServiceID)
	if err != nil {
		return nil, err
	}
	// third-party service can only have one port
	if ports == nil || len(ports) == 0 {
		return nil, fmt.Errorf("Port not found")
	}
	p := ports[0]

	var res []*corev1.Endpoints
	if *p.IsInnerService {
		ep := &corev1.Endpoints{}
		ep.Namespace = as.TenantID
		// inner or outer
		if *p.IsInnerService {
			ep.Name = fmt.Sprintf("service-%d-%d", p.ID, p.ContainerPort)
			ep.Labels = as.GetCommonLabels(map[string]string{
				"name":         as.ServiceAlias + "Service",
				"service-kind": model.ServiceKindThirdParty.String(),
			})
		}
		res = append(res, ep)
	}
	if *p.IsOuterService {
		ep := &corev1.Endpoints{}
		ep.Namespace = as.TenantID
		// inner or outer
		if *p.IsOuterService {
			ep.Name = fmt.Sprintf("service-%d-%dout", p.ID, p.ContainerPort)
			ep.Labels = as.GetCommonLabels(map[string]string{
				"name":         as.ServiceAlias + "ServiceOUT",
				"service-kind": model.ServiceKindThirdParty.String(),
			})
		}
		res = append(res, ep)
	}

	var subsets []corev1.EndpointSubset
	for _, epi := range epinfo {
		logrus.Debugf("make endpoints[address: %s] subset", epi.IP)
		address := validation.SplitEndpointAddress(epi.IP)
		if errs := validation.ValidateEndpointIP(address); len(errs) > 0 {
			logrus.Debug("domain endpoint")
			if len(as.GetServices()) > 0 {
				annotations := as.GetServices()[0].Annotations
				if annotations == nil {
					annotations = make(map[string]string)
				}
				annotations["domain"] = epi.IP
				as.GetServices()[0].Annotations = annotations
			}
			subsets = []corev1.EndpointSubset{corev1.EndpointSubset{
				Ports: []corev1.EndpointPort{
					{
						Name: epi.UUID,
						Port: func(targetPort int, realPort int) int32 {
							if realPort == 0 {
								return int32(targetPort)
							}
							return int32(realPort)
						}(p.ContainerPort, epi.Port),
						Protocol: corev1.ProtocolUDP,
					},
				},
				Addresses: []corev1.EndpointAddress{
					{
						IP: "8.8.8.8",
					},
				},
			}}
			for _, item := range res {
				item.Subsets = subsets
			}
			return res, nil
		}
		subset := corev1.EndpointSubset{
			Ports: []corev1.EndpointPort{
				{
					Name: epi.UUID,
					Port: func(targetPort int, realPort int) int32 {
						if realPort == 0 {
							return int32(targetPort)
						}
						return int32(realPort)
					}(p.ContainerPort, epi.Port),
					Protocol: corev1.ProtocolTCP,
				},
			},
			Addresses: []corev1.EndpointAddress{
				{
					IP: epi.IP,
				},
			},
		}
		subsets = append(subsets, subset)
	}
	for _, item := range res {
		item.Subsets = subsets
	}

	return res, nil
}

func updateSubset(as *v1.AppService, rbdep *v1.RbdEndpoint) error {
	ports, err := db.GetManager().TenantServicesPortDao().GetPortsByServiceID(as.ServiceID)
	if err != nil {
		return err
	}
	// third-party service can only have one port
	if ports == nil || len(ports) == 0 {
		return fmt.Errorf("Port not found")
	}
	p := ports[0]
	ipAddress := rbdep.IP
	address := validation.SplitEndpointAddress(rbdep.IP)
	if errs := validation.ValidateEndpointIP(address); len(errs) > 0 {
		ipAddress = "8.8.8.8"
		if len(as.GetServices()) > 0 {
			annotations := as.GetServices()[0].Annotations
			if annotations == nil {
				annotations = make(map[string]string)
			}
			annotations["domain"] = rbdep.IP
			as.GetServices()[0].Annotations = annotations
		}
	}
	subset := corev1.EndpointSubset{
		Ports: []corev1.EndpointPort{
			{
				Name: rbdep.UUID,
				Port: func(targetPort int, realPort int) int32 {
					if realPort == 0 {
						return int32(targetPort)
					}
					return int32(realPort)
				}(p.ContainerPort, rbdep.Port),
				Protocol: corev1.ProtocolTCP,
			},
		},
		Addresses: []corev1.EndpointAddress{
			{
				IP: ipAddress,
			},
		},
	}

	for _, ep := range as.GetEndpoints() {
		exist := false
		for idx, item := range ep.Subsets {
			if item.Ports[0].Name == subset.Ports[0].Name {
				ep.Subsets[idx] = item
				exist = true
				break
			}
		}
		if !exist {
			ep.Subsets = append(ep.Subsets, subset)
		}
	}
	return nil
}

func (t *thirdparty) runUpdate(event discovery.Event) {
	fc := func(as *v1.AppService, rbdep *v1.RbdEndpoint, ready bool, msg string, condition func(subset corev1.EndpointSubset) bool) {
		var wait sync.WaitGroup
		go func() {
			wait.Add(1)
			defer wait.Done()
			for _, ep := range as.GetEndpoints() {
				for idx, subset := range ep.Subsets {
					if subset.Ports[0].Name == rbdep.UUID && condition(subset) {
						logrus.Debugf("Executed; health: %v; msg: %s", ready, msg)
						address := validation.SplitEndpointAddress(rbdep.IP)
						if errs := validation.ValidateEndpointIP(address); len(errs) > 0 {
							rbdep.IP = "8.8.8.8"
						}
						ep.Subsets[idx] = createSubset(rbdep, ready)
						f.UpgradeEndpoints(t.clientset, as, as.GetEndpoints(), []*corev1.Endpoints{ep}, func(msg string, err error) error {
							logrus.Errorf("update endpoint[%+v] failure: %+v",ep, err)
							return err
						})
					}
				}
			}

		}()
		wait.Wait()
	}

	rbdep := event.Obj.(*v1.RbdEndpoint)
	if rbdep == nil {
		logrus.Warning("update event obj transfer to *v1.RbdEndpoint failure")
		return
	}
	as := t.store.GetAppService(rbdep.Sid)
	if as == nil {
		logrus.Warnf("get app service from store failure, sid=%s", rbdep.Sid)
		return
	}
	b, _ := json.Marshal(rbdep)
	msg := fmt.Sprintf("Run update; Event received: Type: %v; Body: %s", event.Type, string(b))
	switch event.Type {
	case discovery.CreateEvent, discovery.UpdateEvent:
		logrus.Debug(msg)
		err := updateSubset(as, rbdep)
		if err != nil {
			logrus.Warningf("ServiceID: %s; error adding subset: %s",
				rbdep.Sid, err.Error())
			return
		}
		logrus.Debug("upgrade endpoints and service")
		for _, service := range as.GetServices() {
			f.EnsureService(service, t.clientset)
		}

		_ = f.UpgradeEndpoints(t.clientset, as, as.GetEndpoints(), as.GetEndpoints(),
			func(msg string, err error) error {
				logrus.Warning(msg)
				return nil
			})
	case discovery.DeleteEvent:
		logrus.Debug(msg)
		deleteSubset(as, rbdep)
		for _, service := range as.GetServices() {
			f.EnsureService(service, t.clientset)
		}
		eps := as.GetEndpoints()
		_ = f.UpgradeEndpoints(t.clientset, as, as.GetEndpoints(), eps,
			func(msg string, err error) error {
				logrus.Warning(msg)
				return nil
			})
	case discovery.HealthEvent:
		isUnhealthy := func(subset corev1.EndpointSubset) bool {
			return !isHealthy(subset)
		}
		fc(as, rbdep, true, msg, isUnhealthy)
	case discovery.UnhealthyEvent:
		fc(as, rbdep, false, msg, isHealthy)
	}
}

func (t *thirdparty) runDelete(sid string) {
	as := t.store.GetAppService(sid) // TODO: need to delete?
	if eps := as.GetEndpoints(); eps != nil {
		for _, ep := range eps {
			logrus.Debugf("Endpoints delete: %+v", ep)
			err := t.clientset.CoreV1().Endpoints(as.TenantID).Delete(ep.Name, &metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				logrus.Warningf("error deleting endpoint empty old app endpoints: %v", err)
			}
			t.store.OnDelete(ep)
		}
	}
}

func createSubset(ep *v1.RbdEndpoint, ready bool) corev1.EndpointSubset {
	address := corev1.EndpointAddress{
		IP: ep.IP,
	}
	port := corev1.EndpointPort{
		Name:     ep.UUID,
		Port:     int32(ep.Port),
		Protocol: corev1.ProtocolTCP,
	}
	subset := corev1.EndpointSubset{}
	if ready {
		subset.Addresses = append(subset.Addresses, address)
	} else {
		subset.NotReadyAddresses = append(subset.NotReadyAddresses, address)
	}
	subset.Ports = append(subset.Ports, port)
	return subset
}

func isHealthy(subset corev1.EndpointSubset) bool {
	if subset.Addresses != nil && len(subset.Addresses) > 0 {
		return true
	}
	return false
}
