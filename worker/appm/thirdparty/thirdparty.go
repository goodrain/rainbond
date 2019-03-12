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

	"github.com/Sirupsen/logrus"
	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/worker/appm/f"
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
	clientset kubernetes.Interface
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
				key := func(evt *v1.Event) string {
					if evt.IsInner {
						return fmt.Sprintf("%s-%d-inner", evt.Sid, evt.Port)
					}
					return fmt.Sprintf("%s-%d-outer", evt.Sid, evt.Port)
				}(evt)
				if evt.Type == v1.StartEvent { // no need to distinguish between event types
					stopCh := t.svcStopCh[key]
					if stopCh != nil {
						logrus.Debugf("ServiceID: %s; already started.", evt.Sid)
						continue
					}
					t.svcStopCh[evt.Sid] = make(chan struct{})
					signal := make(chan struct{})
					go t.runStart(evt.Sid, signal)
				}
				if evt.Type == v1.StopEvent {
					stopCh := t.svcStopCh[key]
					if stopCh == nil {
						logrus.Warningf("ServiceID: %s; The third-party service has not started yet, cant't be stoped", evt.Sid)
						continue
					}
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
				break
			}
		}
	}()
}

func (t *thirdparty) runStart(sid string, signal chan<- struct{}) {
	logrus.Debugf("ServiceID: %s; run start...", sid)
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
	// TODO: empty rbdeps
	// add rbd endpoints to configmap
	as.SetRbdEndpionts(rbdeps)

	eps, err := t.createK8sEndpoints(as)
	if err != nil {
		logrus.Errorf("ServiceID: %s; error creating k8s endpoints: %s", sid, err.Error())
		return
	}
	logrus.Debugf("k8s eps: %+v", eps)
	// find out old endpoints, and delete it.
	old := as.GetEndpoints()
	del := findDeletedEndpoints(old, eps)
	for _, ep := range del {
		deleteEndpoints(ep, t.clientset)
	}
	for _, ep := range eps {
		ensureEndpoints(ep, t.clientset)
	}
	ensureConfigMap(as.GetRbdEndpiontsCM(), t.clientset)

	signal <- struct{}{}
	i.Watch()
}

func findDeletedEndpoints(old, new []*corev1.Endpoints) []*corev1.Endpoints {
	if old == nil {
		logrus.Debugf("empty old endpoints.")
		return nil
	}
	var res []*corev1.Endpoints
	for _, o := range old {
		del := true
		for _, n := range new {
			if o.Name == n.Name {
				del = false
				break
			}
		}
		if del {
			res = append(res, o)
		}
	}
	return res
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
	b, _ := json.Marshal(epinfos)
	logrus.Debugf("ep infos: %s", string(b))
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

func deleteEndpoints(ep *corev1.Endpoints, clientset kubernetes.Interface) {
	err := clientset.CoreV1().Endpoints(ep.Namespace).Delete(ep.Name, &metav1.DeleteOptions{})
	if err != nil {
		logrus.Debugf("Ignore; error deleting endpoints%+v: %v", ep, err)
	}
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

func ensureConfigMap(cm *corev1.ConfigMap, clientSet kubernetes.Interface) {
	logrus.Debugf("ensure cm: %+v", cm)
	_, err := clientSet.CoreV1().ConfigMaps(cm.Namespace).Update(cm)

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			_, err := clientSet.CoreV1().ConfigMaps(cm.Namespace).Create(cm)
			if err != nil {
				logrus.Warningf("error creating ConfigMaps %+v: %v", cm, err)
			}
			return
		}
		logrus.Warningf("error updating ConfigMaps %+v: %v", cm, err)
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
		// find out old endpoints, and delete it.
		old := as.GetEndpoints()
		for _, ep := range endpoints {
			ensureEndpoints(ep, t.clientset)
		}
		del := findDeletedEndpoints(old, endpoints)
		for _, ep := range del {
			deleteEndpoints(ep, t.clientset)
		}
		ensureConfigMap(as.GetRbdEndpiontsCM(), t.clientset)
	case discovery.UpdateEvent:
		// TODO: Compare old and new endpoints
		// TODO: delete old endpoints
		as.UpdRbdEndpionts(ep)
		endpoints, err := t.createK8sEndpoints(as)
		if err != nil {
			logrus.Warningf("ServiceID: %s; error creating k8s endpoints struct: %s",
				ep.Sid, err.Error())
			return
		}
		// find out old endpoints, and delete it.
		old := as.GetEndpoints()
		del := findDeletedEndpoints(old, endpoints)
		for _, ep := range del {
			deleteEndpoints(ep, t.clientset)
		}
		ensureConfigMap(as.GetRbdEndpiontsCM(), t.clientset)
	case discovery.DeleteEvent:
		as.DelRbdEndpiont(ep.UUID)
		endpoints, err := t.createK8sEndpoints(as)
		if err != nil {
			logrus.Warningf("ServiceID: %s; error creating k8s endpoints struct: %s",
				ep.Sid, err.Error())
			return
		}
		// find out old endpoints, and delete it.
		old := as.GetEndpoints()
		for _, ep := range endpoints {
			ensureEndpoints(ep, t.clientset)
		}
		del := findDeletedEndpoints(old, endpoints)
		for _, ep := range del {
			deleteEndpoints(ep, t.clientset)
		}
		ensureConfigMap(as.GetRbdEndpiontsCM(), t.clientset)
	case discovery.HealthEvent:
		logrus.Debugf("Health event; Sid: %s; IP: %s", ep.Sid, ep.IP)
		eps := as.GetRbdEndpiontByIP(ep.IP)
		if eps == nil || len(eps) == 0 {
			logrus.Warningf("Sid: %s; IP: %s; Empty rbd endpoints", ep.Sid, ep.IP)
			return
		}
		for _, e := range eps {
			e.Status = ep.Status
			as.UpdRbdEndpionts(e)
		}
		ensureConfigMap(as.GetRbdEndpiontsCM(), t.clientset)
	case discovery.OfflineEvent:
		logrus.Debugf("Offline event; Sid: %s; IP: %s", ep.Sid, ep.IP)
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
				logrus.Warningf("error deleting endpoinempty old app servicets: %v", err)
			}
			t.store.OnDelete(ep)
		}
	}
}

// CreateRbdEpConfigmap creates a configmap to store rbd endpoints.
func CreateRbdEpConfigmap(as *v1.AppService, eps []*v1.RbdEndpoint) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: as.GetTenant().Name,
			Name:      as.ServiceID + "-rbd-endpoints",
		},
	}
	cm.Data = make(map[string]string)
	for _, ep := range eps {
		b, _ := json.Marshal(ep)
		cm.Data[ep.UUID] = string(b)
	}
	return cm
}
