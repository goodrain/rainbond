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
	"fmt"

	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	validation "github.com/goodrain/rainbond/util/endpoint"
	"github.com/goodrain/rainbond/worker/appm/f"
	"github.com/goodrain/rainbond/worker/appm/prober"
	"github.com/goodrain/rainbond/worker/appm/store"
	"github.com/goodrain/rainbond/worker/appm/thirdparty/discovery"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/sirupsen/logrus"
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
func NewThirdPartier(clientset kubernetes.Interface,
	store store.Storer,
	startCh *channels.RingChannel,
	updateCh *channels.RingChannel,
	stopCh chan struct{},
	prober prober.Prober) ThirdPartier {
	t := &thirdparty{
		clientset: clientset,
		store:     store,
		svcStopCh: make(map[string]chan struct{}),
		startCh:   startCh,
		updateCh:  updateCh,
		stopCh:    stopCh,
		prober:    prober,
	}
	return t
}

type thirdparty struct {
	clientset kubernetes.Interface
	store     store.Storer
	prober    prober.Prober
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
			case event := <-t.updateCh.Out():
				devent, ok := event.(discovery.Event)
				if !ok {
					logrus.Warningf("Unexpected event received %+v", event)
					continue
				}
				t.runUpdate(devent)
			case <-t.stopCh:
				for _, stopCh := range t.svcStopCh {
					close(stopCh)
				}
				return
			}
		}
	}()
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
	for i := 3; i > 0; i-- {
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
			if err := f.EnsureEndpoints(ep, t.clientset); err != nil {
				logrus.Errorf("create or update endpoint %s failure %s", ep.Name, err.Error())
			}
		}

		for _, service := range as.GetServices(true) {
			if err := f.EnsureService(service, t.clientset); err != nil {
				logrus.Errorf("create or update service %s failure %s", service.Name, err.Error())
			}
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
	eps := as.GetEndpoints(true)
	for _, ep := range eps {
		for idx, item := range ep.Subsets {
			if item.Ports[0].Name == rbdep.UUID {
				logrus.Debugf("UUID: %s; subset deleted", rbdep.UUID)
				ep.Subsets[idx] = ep.Subsets[len(ep.Subsets)-1]
				ep.Subsets = ep.Subsets[:len(ep.Subsets)-1]
			}
			isDomain := false
			for _, addr := range item.Addresses {
				if addr.IP == "1.1.1.1" {
					isDomain = true
				}
			}
			for _, addr := range item.NotReadyAddresses {
				if addr.IP == "1.1.1.1" {
					isDomain = true
				}
			}
			if isDomain {
				for _, service := range as.GetServices(true) {
					if service.Annotations != nil {
						if rbdep.IP == service.Annotations["domain"] {
							delete(service.Annotations, "domain")
						}
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
		}
		eaddressIP := epi.IP
		address := validation.SplitEndpointAddress(epi.IP)
		if validation.IsDomainNotIP(address) {
			if len(as.GetServices(false)) > 0 {
				annotations := as.GetServices(false)[0].Annotations
				if annotations == nil {
					annotations = make(map[string]string)
				}
				annotations["domain"] = epi.IP
				as.GetServices(false)[0].Annotations = annotations
			}
			eaddressIP = "1.1.1.1"
		}
		eaddress := []corev1.EndpointAddress{
			{
				IP: eaddressIP,
			},
		}
		useProbe := t.prober.IsUsedProbe(as.ServiceID)
		if useProbe {
			subset.NotReadyAddresses = eaddress
		} else {
			subset.Addresses = eaddress
		}
		subsets = append(subsets, subset)
	}
	//all endpoint for one third app is same
	for _, item := range res {
		item.Subsets = subsets
	}
	return res, nil
}

func (t *thirdparty) createSubsetForAllEndpoint(as *v1.AppService, rbdep *v1.RbdEndpoint) error {
	port, err := db.GetManager().TenantServicesPortDao().GetPortsByServiceID(as.ServiceID)
	if err != nil {
		return err
	}
	// third-party service can only have one port
	if port == nil || len(port) == 0 {
		return fmt.Errorf("Port not found")
	}
	ipAddress := rbdep.IP
	address := validation.SplitEndpointAddress(rbdep.IP)
	if validation.IsDomainNotIP(address) {
		//domain endpoint set ip is 1.1.1.1
		ipAddress = "1.1.1.1"
		if len(as.GetServices(false)) > 0 {
			annotations := as.GetServices(false)[0].Annotations
			if annotations == nil {
				annotations = make(map[string]string)
			}
			annotations["domain"] = rbdep.IP
			as.GetServices(false)[0].Annotations = annotations
		}
	}

	subset := corev1.EndpointSubset{
		Ports: []corev1.EndpointPort{
			{
				Name: rbdep.UUID,
				Port: func() int32 {
					//if endpoint have port, will ues this port
					//or use service port
					if rbdep.Port != 0 {
						return int32(rbdep.Port)
					}
					return int32(port[0].ContainerPort)
				}(),
				Protocol: corev1.ProtocolTCP,
			},
		},
	}
	eaddress := []corev1.EndpointAddress{
		{
			IP: ipAddress,
		},
	}
	useProbe := t.prober.IsUsedProbe(as.ServiceID)
	if useProbe {
		subset.NotReadyAddresses = eaddress
	} else {
		subset.Addresses = eaddress
	}

	for _, ep := range as.GetEndpoints(true) {
		existPort := false
		existAddress := false
		for i, item := range ep.Subsets {
			for _, port := range item.Ports {
				if port.Port == int32(subset.Ports[0].Port) && len(item.Ports) < 2 {
					for _, a := range item.Addresses {
						if a.IP == ipAddress {
							existAddress = true
							break
						}
					}
					for _, a := range item.NotReadyAddresses {
						if a.IP == ipAddress {
							existAddress = true
							break
						}
					}
					if !existAddress {
						if useProbe {
							ep.Subsets[i].NotReadyAddresses = append(ep.Subsets[i].NotReadyAddresses, subset.NotReadyAddresses...)
						} else {
							ep.Subsets[i].Addresses = append(ep.Subsets[i].NotReadyAddresses, subset.Addresses...)
						}
					}
					existPort = true
				}
			}
		}
		if !existPort {
			ep.Subsets = append(ep.Subsets, subset)
		}
		if err := f.EnsureEndpoints(ep, t.clientset); err != nil {
			logrus.Errorf("update endpoint %s failure %s", ep.Name, err.Error())
		}
	}
	return nil
}

func (t *thirdparty) runUpdate(event discovery.Event) {

	updateAddress := func(as *v1.AppService, rbdep *v1.RbdEndpoint, ready bool) {
		ad := validation.SplitEndpointAddress(rbdep.IP)
		for _, ep := range as.GetEndpoints(true) {
			var needUpdate bool
			for idx, subset := range ep.Subsets {
				for _, port := range subset.Ports {
					address := subset.Addresses
					if ready {
						address = subset.NotReadyAddresses
					}
					for i, addr := range address {
						ipequal := fmt.Sprintf("%s_%d", addr.IP, port.Port) == fmt.Sprintf("%s_%d", rbdep.IP, rbdep.Port)
						if (addr.IP == "1.1.1.1" && validation.IsDomainNotIP(ad)) || ipequal {
							if validation.IsDomainNotIP(ad) {
								rbdep.IP = "1.1.1.1"
							}
							ep.Subsets[idx] = updateSubsetAddress(ready, subset, address[i])
							needUpdate = true
							break
						}
					}
					logrus.Debugf("not found need update address by %s", fmt.Sprintf("%s_%d", rbdep.IP, rbdep.Port))
				}
			}
			if needUpdate {
				if err := f.EnsureEndpoints(ep, t.clientset); err != nil {
					logrus.Errorf("update endpoint %s failure %s", ep.Name, err.Error())
				}
			}
		}
	}
	// do not  have multiple ports, multiple addresses
	removeAddress := func(as *v1.AppService, rbdep *v1.RbdEndpoint) {

		ad := validation.SplitEndpointAddress(rbdep.IP)
		for _, ep := range as.GetEndpoints(true) {
			var needUpdate bool
			var newSubsets []corev1.EndpointSubset
			for idx, subset := range ep.Subsets {
				var handleSubset bool
				for i, port := range subset.Ports {
					address := append(subset.Addresses, subset.NotReadyAddresses...)
					for j, addr := range address {
						ipequal := fmt.Sprintf("%s_%d", addr.IP, port.Port) == fmt.Sprintf("%s_%d", rbdep.IP, rbdep.Port)
						if (addr.IP == "1.1.1.1" && validation.IsDomainNotIP(ad)) || ipequal {
							//multiple port remove port, Instead remove the address
							if len(subset.Ports) > 1 {
								subset.Ports = append(subset.Ports[:i], subset.Ports[:i]...)
								newSubsets = append(newSubsets, subset)
							} else {
								if validation.IsDomainNotIP(ad) {
									rbdep.IP = "1.1.1.1"
								}
								newsub := removeSubsetAddress(ep.Subsets[idx], address[j])
								if len(newsub.Addresses) != 0 || len(newsub.NotReadyAddresses) != 0 {
									newSubsets = append(newSubsets, newsub)
								}
							}
							needUpdate = true
							handleSubset = true
							break
						}
					}
				}
				if !handleSubset {
					newSubsets = append(newSubsets, subset)
				}
			}
			ep.Subsets = newSubsets
			if needUpdate {
				if err := f.EnsureEndpoints(ep, t.clientset); err != nil {
					logrus.Errorf("update endpoint %s failure %s", ep.Name, err.Error())
				}
			}
		}
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
	//rbdep.IP may be set "1.1.1.1" if it is domain
	//so cache doamin address for show after handle complete
	showEndpointIP := rbdep.IP
	switch event.Type {
	case discovery.UpdateEvent, discovery.CreateEvent:
		err := t.createSubsetForAllEndpoint(as, rbdep)
		if err != nil {
			logrus.Warningf("ServiceID: %s; error adding subset: %s",
				rbdep.Sid, err.Error())
			return
		}
		for _, service := range as.GetServices(true) {
			if err := f.EnsureService(service, t.clientset); err != nil {
				logrus.Errorf("create or update service %s failure %s", service.Name, err.Error())
			}
		}
		logrus.Debugf("upgrade endpoints and service for third app %s", as.ServiceAlias)
	case discovery.DeleteEvent:
		removeAddress(as, rbdep)
		logrus.Debugf("third endpoint %s ip %s is deleted", rbdep.UUID, showEndpointIP)
	case discovery.HealthEvent:
		updateAddress(as, rbdep, true)
		logrus.Debugf("third endpoint %s ip %s is onlined", rbdep.UUID, showEndpointIP)
	case discovery.UnhealthyEvent:
		logrus.Debugf("third endpoint %s ip %s is offlined", rbdep.UUID, showEndpointIP)
		updateAddress(as, rbdep, false)
	}
}

func (t *thirdparty) runDelete(sid string) {
	as := t.store.GetAppService(sid) // TODO: need to delete?
	if eps := as.GetEndpoints(true); eps != nil {
		for _, ep := range eps {
			logrus.Debugf("Endpoints delete: %+v", ep)
			err := t.clientset.CoreV1().Endpoints(as.TenantID).Delete(ep.Name, &metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				logrus.Warningf("error deleting endpoint empty old app endpoints: %v", err)
			}
		}
	}
}

func updateSubsetAddress(ready bool, subset corev1.EndpointSubset, address corev1.EndpointAddress) corev1.EndpointSubset {
	if ready {
		for i, a := range subset.NotReadyAddresses {
			if a.IP == address.IP {
				subset.NotReadyAddresses = append(subset.NotReadyAddresses[:i], subset.NotReadyAddresses[i+1:]...)
			}
		}
		var exist bool
		for _, a := range subset.Addresses {
			if a.IP == address.IP {
				exist = true
				break
			}
		}
		if !exist {
			subset.Addresses = append(subset.Addresses, address)
		}
	} else {
		for i, a := range subset.Addresses {
			if a.IP == address.IP {
				subset.Addresses = append(subset.Addresses[:i], subset.Addresses[i+1:]...)
			}
		}
		var exist bool
		for _, a := range subset.NotReadyAddresses {
			if a.IP == address.IP {
				exist = true
				break
			}
		}
		if !exist {
			subset.NotReadyAddresses = append(subset.NotReadyAddresses, address)
		}
	}
	return subset
}

func removeSubsetAddress(subset corev1.EndpointSubset, address corev1.EndpointAddress) corev1.EndpointSubset {
	for i, a := range subset.Addresses {
		if a.IP == address.IP {
			subset.Addresses = append(subset.Addresses[:i], subset.Addresses[i+1:]...)
		}
	}
	for i, a := range subset.NotReadyAddresses {
		if a.IP == address.IP {
			subset.NotReadyAddresses = append(subset.NotReadyAddresses[:i], subset.NotReadyAddresses[i+1:]...)
		}
	}
	return subset
}

func isHealthy(subset corev1.EndpointSubset) bool {
	if subset.Addresses != nil && len(subset.Addresses) > 0 {
		return true
	}
	return false
}
