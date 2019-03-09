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
	"github.com/goodrain/rainbond/worker/appm/f"
	"github.com/goodrain/rainbond/worker/appm/prober"

	"github.com/Sirupsen/logrus"
	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/worker/appm/store"
	"github.com/goodrain/rainbond/worker/appm/thirdparty/discovery"
	"github.com/goodrain/rainbond/worker/appm/types/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
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
	prober prober.Prober,
	startCh *channels.RingChannel,
	updateCh *channels.RingChannel,
	stopCh chan struct{}) ThirdPartier {
	t := &thirdparty{
		clientset: clientset,
		store:     store,
		prober:    prober,

		svcStopCh: make(map[string]chan struct{}),
		startCh:   startCh,
		updateCh:  updateCh,
		stopCh:    stopCh,
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
			case event := <-t.startCh.Out(): // TODO: rename
				evt, ok := event.(*v1.Event)
				if !ok {
					logrus.Warningf("Unexpected event received %+v", event)
					continue
				}
				logrus.Debugf("Received event: %+v", evt)
				if evt.Type == v1.StartEvent {
					stopCh := t.svcStopCh[evt.Sid]
					if stopCh != nil {
						logrus.Debugf("ServiceID: %s; already started.", evt.Sid)
						continue
					}
					t.svcStopCh[evt.Sid] = make(chan struct{})
					signal := make(chan struct{})
					go t.runStart(evt.Sid, signal)
					// health test
					go t.runProbe(evt.Sid, signal)
				}
				if evt.Type == v1.StopEvent {
					stopCh := t.svcStopCh[evt.Sid]
					if stopCh == nil {
						logrus.Warningf("ServiceID: %s; The third-party service has not started yet, cant't be stoped", evt.Sid)
						continue
					}
					close(stopCh)
					delete(t.svcStopCh, evt.Sid)

					go t.prober.StopProbe(evt.Sid)
					go t.runDelete(evt.Sid)

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
				break
			}
		}
	}()
}

func (t *thirdparty) runStart(sid string, signal chan<- struct{}) {
	logrus.Debugf("ServiceID: %s; run start...", sid)
	// TODO: nil pointer
	as := t.store.GetAppService(sid)
	// TODO: when an error occurs, consider retrying.
	i := NewInteracter(sid, t.updateCh, t.svcStopCh[sid])
	rbdeps, err := i.List()
	b, _ := json.Marshal(rbdeps)
	logrus.Debugf("ServiceID: %s; rbd endpoints: %+v", sid, string(b))
	if err != nil {
		logrus.Errorf("ServiceID: %s; error listing endpoints infos: %s", sid, err.Error())
		return
	}
	as.SetRbdEndpionts(rbdeps)

	eps, err := t.createK8sEndpoints(as)
	if err != nil {
		logrus.Errorf("ServiceID: %s; error creating k8s endpoints: %s", sid, err.Error())
		return
	}
	for _, ep := range eps {
		ensureEndpoints(ep, t.clientset)
	}

	signal <- struct{}{}
	i.Watch()
}

func (t *thirdparty) runProbe(sid string, signal <-chan struct{}) {
	<-signal
	logrus.Debugf("ServiceID: %s; run probe...", sid)
	// TODO: nil pointer
	as := t.store.GetAppService(sid)
	if as == nil {
		// TODO: warning
		return
	}
	rbdEndpoints := as.GetRbdEndpionts()
	b, _ := json.Marshal(rbdEndpoints)
	logrus.Debugf("rbd endpoints: %s", string(b))
	if rbdEndpoints == nil || len(rbdEndpoints) == 0 {
		// TODO: warning
		return
	}
	t.prober.AddProbe(as.ServiceID, rbdEndpoints)
}

func (t *thirdparty) createK8sEndpoints(as *v1.AppService) ([]*corev1.Endpoints, error) {
	ports, err := db.GetManager().TenantServicesPortDao().GetPortsByServiceID(as.ServiceID)
	if err != nil {
		return nil, err
	}
	epinfos, err := f.ConvRbdEndpoint(as.GetRbdEndpionts())
	if err != nil {
		return nil, err
	}
	var res []*corev1.Endpoints
	for _, p := range ports {
		for _, item := range epinfos {
			port := func(targetPort int, realPort int) int32 { // final port
				if realPort == 0 {
					return int32(targetPort)
				}
				return int32(realPort)
			}(p.ContainerPort, item.Port)
			ep := corev1.Endpoints{}
			ep.Namespace = as.TenantID
			if p.IsInnerService {
				ep.Name = fmt.Sprintf("%s-%s-%d", as.TenantName, as.ServiceAlias, port)
				logrus.Debugf("create inner third-party service")
				ep.Labels = as.GetCommonLabels(map[string]string{
					"name": as.ServiceAlias + "Service",
				})
			}
			if p.IsOuterService {
				ep.Name = fmt.Sprintf("%s-%s-%d-out", as.TenantName, as.ServiceAlias, port)
				logrus.Debugf("create outer third-party service")
				ep.Labels = as.GetCommonLabels(map[string]string{
					"name": as.ServiceAlias + "ServiceOUT",
				})
			}
			subset := corev1.EndpointSubset{
				Ports: []corev1.EndpointPort{
					{
						Port: port,
					},
				},
			}
			for _, ip := range item.IPs {
				address := corev1.EndpointAddress{
					IP: ip,
				}
				subset.Addresses = append(subset.Addresses, address)
			}
			for _, ip := range item.NotReadyIPs {
				address := corev1.EndpointAddress{
					IP: ip,
				}
				subset.NotReadyAddresses = append(subset.NotReadyAddresses, address)
			}
			ep.Subsets = []corev1.EndpointSubset{subset}
			res = append(res, &ep)
		}
	}
	return res, nil
}

func ensureEndpoints(ep *corev1.Endpoints, clientSet kubernetes.Interface) {
	_, err := clientSet.CoreV1().Endpoints(ep.Namespace).Update(ep)

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			_, err := clientSet.CoreV1().Endpoints(ep.Namespace).Create(ep)
			if err != nil {
				logrus.Warningf("error creating endpoints %+v: %v", ep, err)
			}
			return
		}
		logrus.Warningf("error updating endpoints %+v: %v", ep, err)
	}
}

func (t *thirdparty) runUpdate(event discovery.Event) {
	ep := event.Obj.(*v1.RbdEndpoint)
	as := t.store.GetAppService(ep.Sid)
	switch event.Type {
	case discovery.CreateEvent:
		as.AddRbdEndpiont(ep)
		endpoints, err := t.createK8sEndpoints(as)
		if err != nil {
			logrus.Warningf("ServiceID: %s; error creating k8s endpoints struct: %s",
				ep.Sid, err.Error())
			return
		}
		for _, item := range endpoints {
			ensureEndpoints(item, t.clientset)
		}
	case discovery.UpdateEvent:
		// TODO: Compare old and new endpoints
		as.UpdRbdEndpionts(ep)
		endpoints, err := t.createK8sEndpoints(as)
		if err != nil {
			logrus.Warningf("ServiceID: %s; error creating k8s endpoints struct: %s",
				ep.Sid, err.Error())
			return
		}
		for _, item := range endpoints {
			ensureEndpoints(item, t.clientset)
		}
	case discovery.DeleteEvent:
		as.DelRbdEndpiont(ep.UUID)
		endpoints, err := t.createK8sEndpoints(as)
		if err != nil {
			logrus.Warningf("ServiceID: %s; error creating k8s endpoints struct: %s",
				ep.Sid, err.Error())
			return
		}
		for _, item := range endpoints {
			ensureEndpoints(item, t.clientset)
		}
	}
}

func (t *thirdparty) runDelete(sid string) {
	as := t.store.GetAppService(sid) // TODO: need to delete?

	if services := as.GetServices(); services != nil {
		for _, service := range services {
			err := t.clientset.CoreV1().Services(as.TenantID).Delete(service.Name, &metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				logrus.Warningf("error deleting service: %v", err)
			}
		}
	}
	if secrets := as.GetSecrets(); secrets != nil {
		for _, secret := range secrets {
			if secret != nil {
				err := t.clientset.CoreV1().Secrets(as.TenantID).Delete(secret.Name, &metav1.DeleteOptions{})
				if err != nil && !errors.IsNotFound(err) {
					logrus.Warningf("error deleting secrets: %v", err)
				}
				t.store.OnDelete(secret)
			}
		}
	}
	if ingresses := as.GetIngress(); ingresses != nil {
		for _, ingress := range ingresses {
			err := t.clientset.ExtensionsV1beta1().Ingresses(as.TenantID).Delete(ingress.Name, &metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				logrus.Warningf("error deleting ingress: %v", err)
			}
			t.store.OnDelete(ingress)
		}
	}
	if configs := as.GetConfigMaps(); configs != nil {
		for _, config := range configs {
			err := t.clientset.CoreV1().ConfigMaps(as.TenantID).Delete(config.Name, &metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				logrus.Warningf("error deleting config map: %v", err)
			}
			t.store.OnDelete(config)
		}
	}
	if eps := as.GetEndpoints(); eps != nil {
		for _, ep := range eps {
			err := t.clientset.CoreV1().Endpoints(as.TenantID).Delete(ep.Name, &metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				logrus.Warningf("error deleting endpoints: %v", err)
			}
			t.store.OnDelete(ep)
		}
	}
}
