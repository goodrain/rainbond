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
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
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
				if evt.Type == v1.StartEvent { // no need to distinguish between event types
					stopCh := t.svcStopCh[evt.Sid]
					if stopCh != nil {
						logrus.Debugf("ServiceID: %s; already started.", evt.Sid)
						continue
					}
					t.svcStopCh[evt.Sid] = make(chan struct{})
					go t.runStart(evt.Sid)
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
				break
			}
		}
	}()
}

func (t *thirdparty) runStart(sid string) {
	logrus.Debugf("ServiceID: %s; run start...", sid)
	as := t.store.GetAppService(sid)
	// TODO: when an error occurs, consider retrying.
	rbdeps, d := t.ListRbdEndpoints(sid)
	b, _ := json.Marshal(rbdeps)
	logrus.Debugf("ServiceID: %s; rbd endpoints: %+v", sid, string(b))
	// TODO: empty rbdeps
	if rbdeps == nil || len(rbdeps) == 0 {
		logrus.Warningf("ServiceID: %s;Empty rbd endpoints, stop starting third-party service.", sid)
		return
	}

	eps, err := t.createK8sEndpoints(as, rbdeps)
	if err != nil {
		logrus.Errorf("ServiceID: %s; error creating k8s endpoints: %s", sid, err.Error())
		return
	}
	// find out old endpoints, and delete it.
	old := as.GetEndpoints()
	// TODO: can do better
	del := findDeletedEndpoints(old, eps)
	for _, ep := range del {
		deleteEndpoints(ep, t.clientset)
	}
	for _, ep := range eps {
		ensureEndpoints(ep, t.clientset)
	}

	if d != nil {
		d.Watch()
	}
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

func (t *thirdparty) createK8sEndpoints(as *v1.AppService, epinfo []*v1.RbdEndpoint) ([]*corev1.Endpoints, error) {
	ports, err := db.GetManager().TenantServicesPortDao().GetPortsByServiceID(as.ServiceID)
	if err != nil {
		return nil, err
	}
	// third-party service can only have one port
	if ports == nil || len(ports) == 0 {
		return nil, fmt.Errorf("Port not found")
	}
	p := ports[0]

	logrus.Debugf("create outer third-party service")
	f := func() []*corev1.Endpoints {
		var eps []*corev1.Endpoints
		for _, epi := range epinfo {
			port, realport := func(targetPort int, realPort int) (int32, bool) { // final port
				if realPort == 0 {
					return int32(targetPort), false
				}
				return int32(realPort), true
			}(p.ContainerPort, epi.Port)
			ep := corev1.Endpoints{}
			ep.Namespace = as.TenantID
			// one ep - one ip:port
			if p.IsInnerService {
				ep.Name = epi.UUID
				ep.Labels = as.GetCommonLabels(map[string]string{
					"name":         as.ServiceAlias + "Service",
					"service-kind": model.ServiceKindThirdParty.String(),
				})
			}
			if p.IsOuterService {
				ep.Name = epi.UUID + "out"
				ep.Labels = as.GetCommonLabels(map[string]string{
					"name":         as.ServiceAlias + "ServiceOUT",
					"service-kind": model.ServiceKindThirdParty.String(),
				})
			}
			ep.Labels["uuid"] = epi.UUID
			ep.Labels["ip"] = epi.IP
			ep.Labels["port"] = strconv.Itoa(int(port))
			ep.Labels["real-port"] = strconv.FormatBool(realport)
			subset := corev1.EndpointSubset{
				Ports: []corev1.EndpointPort{
					{
						Port: port,
					},
				},
				Addresses: []corev1.EndpointAddress{
					{
						IP: epi.IP,
					},
				},
			}
			ep.Subsets = append(ep.Subsets, subset)
			eps = append(eps, &ep)
		}
		return eps
	}

	var res []*corev1.Endpoints
	if p.IsInnerService {
		res = append(res, f()...)
	}
	if p.IsOuterService {
		res = append(res, f()...)
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
		b, _ := json.Marshal(ep)
		logrus.Debugf("Run update; Event received: Type: %v; Body: %s", event.Type, string(b))
		endpoints, err := t.createK8sEndpoints(as, []*v1.RbdEndpoint{ep})
		if err != nil {
			logrus.Warningf("ServiceID: %s; error creating k8s endpoints struct: %s",
				ep.Sid, err.Error())
			return
		}
		for _, ep := range endpoints {
			ensureEndpoints(ep, t.clientset)
		}
	case discovery.UpdateEvent:
		b, _ := json.Marshal(ep)
		logrus.Debugf("Run update; Event received: Type: %v; Body: %s", event.Type, string(b))
		// TODO: Compare old and new endpoints
		// TODO: delete old endpoints
		if !ep.IsOnline {
			eps := ListOldEndpoints(as, ep)
			for _, item := range eps {
				deleteEndpoints(item, t.clientset)
			}
			return
		}

		endpoints, err := t.createK8sEndpoints(as, []*v1.RbdEndpoint{ep})
		if err != nil {
			logrus.Warningf("ServiceID: %s; error creating k8s endpoints struct: %s",
				ep.Sid, err.Error())
			return
		}
		for _, ep := range endpoints {
			ensureEndpoints(ep, t.clientset)
		}
	case discovery.DeleteEvent:
		b, _ := json.Marshal(ep)
		logrus.Debugf("Run update; Event received: Type: %v; Body: %s", event.Type, string(b))
		eps := ListOldEndpoints(as, ep)
		for _, item := range eps {
			deleteEndpoints(item, t.clientset)
		}
	case discovery.HealthEvent:
		subset := createSubset(ep, false)
		eps := ListOldEndpoints(as, ep)
		for _, ep := range eps {
			if !isHealth(ep) {
				ep.Subsets = []corev1.EndpointSubset{
					subset,
				}
				ensureEndpoints(ep, t.clientset)
			}
		}
	case discovery.UnhealthyEvent:
		subset := createSubset(ep, false)
		eps := ListOldEndpoints(as, ep)
		for _, ep := range eps {
			if isHealth(ep) {
				ep.Subsets = []corev1.EndpointSubset{
					subset,
				}
				ensureEndpoints(ep, t.clientset)
			}
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
				logrus.Warningf("error deleting endpoin empty old app servicets: %v", err)
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

func ListOldEndpoints(as *v1.AppService, ep *v1.RbdEndpoint) []*corev1.Endpoints {
	var res []*corev1.Endpoints
	for _, item := range as.GetEndpoints() {
		if item.GetLabels()["uuid"] == ep.UUID {
			res = append(res, item)
		}
	}
	return res
}

func createSubset(ep *v1.RbdEndpoint, notReady bool) corev1.EndpointSubset {
	address := corev1.EndpointAddress{
		IP: ep.IP,
	}
	port := corev1.EndpointPort{
		Port: int32(ep.Port),
	}
	subset := corev1.EndpointSubset{}
	if notReady {
		subset.NotReadyAddresses = append(subset.Addresses, address)
	} else {
		subset.Addresses = append(subset.Addresses, address)
	}
	subset.Ports = append(subset.Ports, port)
	return subset
}

func isHealth(ep *corev1.Endpoints) bool {
	if ep.Subsets == nil || len(ep.Subsets) == 0 {
		return false
	}
	if ep.Subsets[0].Addresses != nil && len(ep.Subsets[0].Addresses) > 0 {
		return true
	}
	return false
}
