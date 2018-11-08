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

package store

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/gateway/v1"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	istroe "k8s.io/ingress-nginx/ingress/controller/store"
	"time"
)

type EventType string

const (
	// CreateEvent event associated with new objects in an informer
	CreateEvent EventType = "CREATE"
	// UpdateEvent event associated with an object update in an informer
	UpdateEvent EventType = "UPDATE"
	// DeleteEvent event associated when an object is removed from an informer
	DeleteEvent EventType = "DELETE"
	// 创建或更新控制器配置对象时关联的ConfigurationEvent事件
	// ConfigurationEvent event associated when a controller configuration object is created or updated
	ConfigurationEvent EventType = "CONFIGURATION"
)

//EventMethod event method
type EventMethod string

//ADDEventMethod add method
const ADDEventMethod EventMethod = "ADD"

//UPDATEEventMethod add method
const UPDATEEventMethod EventMethod = "UPDATE"

//DELETEEventMethod add method
const DELETEEventMethod EventMethod = "DELETE"

var sslCertMap = make(map[string]*corev1.Secret)

//Storer is the interface that wraps the required methods to gather information
type Storer interface {
	// get endpoints pool by name
	//GetPool(name string) *v1.Pool

	// list endpoints pool
	ListPool() []*v1.Pool

	// get endpoint by name
	//GetNode(name string) *v1.Node

	// list endpoint
	//ListNode() []*v1.Node

	// get service access rule for http by name
	//GetHTTPRule(name string) *v1.HTTPRule

	// list service access rule for http
	//ListHTTPRule() []*v1.HTTPRule

	// get virtual service by name
	//GetVirtualService(name string) *v1.VirtualService

	// list virtual service
	ListVirtualService() []*v1.VirtualService

	// get SSL certificate by name
	//GetSSLCert(name string) *v1.SSLCert

	// list SSL certificates
	//ListSSLCert() []*v1.SSLCert

	//PoolUpdateMethod(func(*v1.Pool, EventMethod))
	//NodeUpdateMethod(func(*v1.Node, EventMethod))
	//HTTPRuleUpdateMethod(func(*v1.HTTPRule, EventMethod))
	//VirtualServiceUpdateMethod(func(*v1.VirtualService, EventMethod))
	//SSLCertUpdateMethod(func(*v1.SSLCert, EventMethod))

	ListIngresses() []*extensions.Ingress

	InitSecret()

	// Run initiates the synchronization of the controllers
	Run(stopCh chan struct{})
}

// Event holds the context of an event.
type Event struct {
	Type EventType
	Obj  interface{}
}

// Informer defines the required SharedIndexInformers that interact with the API server.
type Informer struct {
	Ingress  cache.SharedIndexInformer
	Service  cache.SharedIndexInformer
	Endpoint cache.SharedIndexInformer
	Secret   cache.SharedIndexInformer
}

// Lister contains object listers (stores).
type Lister struct {
	Ingress           istroe.IngressLister
	Service           istroe.ServiceLister
	Endpoint          istroe.EndpointLister
	Secret            istroe.SecretLister
}

// NotExistsError is returned when an object does not exist in a local store.
type NotExistsError string

// Error implements the error interface.
func (e NotExistsError) Error() string {
	return fmt.Sprintf("no object matching key %q in local store", string(e))
}

// Run initiates the synchronization of the informers against the API server.
func (i *Informer) Run(stopCh chan struct{}) {
	go i.Endpoint.Run(stopCh)
	go i.Service.Run(stopCh)
	go i.Secret.Run(stopCh)

	// wait for all involved caches to be synced before processing items
	// from the queue
	if !cache.WaitForCacheSync(stopCh,
		i.Endpoint.HasSynced,
		i.Service.HasSynced,
		i.Secret.HasSynced,
	) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
	}

	// in big clusters, deltas can keep arriving even after HasSynced
	// functions have returned 'true'
	time.Sleep(1 * time.Second)

	// we can start syncing ingress objects only after other caches are
	// ready, because ingress rules require content from other listers, and
	// 'add' events get triggered in the handlers during caches population.
	go i.Ingress.Run(stopCh)
	if !cache.WaitForCacheSync(stopCh,
		i.Ingress.HasSynced,
	) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
	}
}

type rbdStore struct {
	// informer contains the cache Informers
	informers *Informer
	// Lister contains object listers (stores).
	listers *Lister
}

func New(client kubernetes.Interface,
	namespace string,
	updateCh *channels.RingChannel) Storer {
	store := &rbdStore{
		informers: &Informer{},
		listers:   &Lister{},
	}

	// create informers factory, enable and assign required informers
	infFactory := informers.NewFilteredSharedInformerFactory(client, time.Second, namespace,
		func(*metav1.ListOptions) {})

	store.informers.Ingress = infFactory.Extensions().V1beta1().Ingresses().Informer()
	store.listers.Ingress.Store = store.informers.Ingress.GetStore()

	store.informers.Service = infFactory.Core().V1().Services().Informer()
	store.listers.Service.Store = store.informers.Service.GetStore()

	store.informers.Endpoint = infFactory.Core().V1().Endpoints().Informer()
	store.listers.Endpoint.Store = store.informers.Endpoint.GetStore()

	store.informers.Secret = infFactory.Core().V1().Secrets().Informer()
	store.listers.Secret.Store = store.informers.Secret.GetStore()

	// 定义Ingress Event Handler: Add, Delete, Update
	ingEventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			logrus.Debug("Ingress AddFunc is called.\n")
			//recorder.Eventf(ing, corev1.EventTypeNormal, "CREATE", fmt.Sprintf("Ingress %s/%s", ing.Namespace, ing.Name))
			// 将obj加到Event中, 并把这个Event发送给*channels.RingChannel的input
			updateCh.In() <- Event{
				Type: CreateEvent,
				Obj:  obj,
			}
		},
		DeleteFunc: func(obj interface{}) {
			logrus.Debug("Ingress DeleteFunc is called.\n")
			updateCh.In() <- Event{
				Type: DeleteEvent,
				Obj:  obj,
			}
		},
		UpdateFunc: func(old, new interface{}) {
			logrus.Debug("Ingress UpdateFunc is called.\n")
			updateCh.In() <- Event{
				Type: UpdateEvent,
				Obj:  new,
			}
		},
	}

	svcEventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			logrus.Debug("Service AddFunc is called.\n")
			updateCh.In() <- Event{
				Type: CreateEvent,
				Obj:  obj,
			}
		},
		DeleteFunc: func(obj interface{}) {
			logrus.Debug("Service DeleteFunc is called.\n")
			updateCh.In() <- Event{
				Type: DeleteEvent,
				Obj:  obj,
			}
		},
	}

	store.informers.Ingress.AddEventHandler(ingEventHandler)
	store.informers.Service.AddEventHandler(svcEventHandler)

	return store
}

// TODO test
func (s *rbdStore) ListPool() []*v1.Pool {
	var pools []*v1.Pool // TODO 需不需要用指针?
	for _, item := range s.listers.Endpoint.List() {
		endpoint := item.(*corev1.Endpoints)

		pool := &v1.Pool{
			Nodes: []v1.Node{},
		}
		pool.Name = endpoint.ObjectMeta.Name
		for _, ss := range endpoint.Subsets { // TODO 这个SubSets为什么是slice?
			for _, address := range ss.Addresses {
				pool.Nodes = append(pool.Nodes, v1.Node{ // TODO 需不需要用指针?
					Host: address.IP,
					Port: ss.Ports[0].Port,
				})
			}
		}
		pools = append(pools, pool)
	}

	return pools
}

func (s *rbdStore) ListVirtualService() []*v1.VirtualService {
	var virtualServices []*v1.VirtualService
	for _, item := range s.listers.Ingress.List() {
		ing := item.(*extensions.Ingress)
		if !s.ingressIsValid(ing) {
			continue
		}

		vs := &v1.VirtualService{
			CertificateMapping: make(map[string]string),
		}

		httpsEnabled := ing.ObjectMeta.Annotations["ingress.kubernetes.io/ssl-redirect"]
		if ing.Spec.Backend != nil { // stream
			vs.Protocol = "stream" // TODO
			vs.Listening = []string{fmt.Sprintf("%v", ing.Spec.Backend.ServicePort.IntVal)}
			vs.PoolName = ing.Spec.Backend.ServiceName
		} else if httpsEnabled == "True" { // TODO
			vs.Protocol = "https" // TODO
			for _, rule := range ing.Spec.Rules {
				vs.ServerName = rule.Host
				var locations []*v1.Location
				for _, path := range rule.IngressRuleValue.HTTP.Paths {
					location := &v1.Location{
						Path:     path.Path,
						PoolName: path.Backend.ServiceName,
					}
					locations = append(locations, location)
				}
				vs.Locations = locations
			}
			secret := sslCertMap[ing.Spec.TLS[0].SecretName]
			vs.CertificateMapping["tlscrt"] = string(secret.Data["tls.crt"])
			vs.CertificateMapping["tlskey"] = string(secret.Data["tls.key"])
		} else { // http
			vs.Protocol = "http" // TODO
			for _, rule := range ing.Spec.Rules {
				vs.ServerName = rule.Host
				var locations []*v1.Location
				for _, path := range rule.IngressRuleValue.HTTP.Paths {
					location := &v1.Location{
						Path:     path.Path,
						PoolName: path.Backend.ServiceName,
					}
					locations = append(locations, location)
				}
				vs.Locations = locations
			}
		}
		virtualServices = append(virtualServices, vs)
	}
	return virtualServices
}

func (s *rbdStore) ingressIsValid(ing *extensions.Ingress) bool {
	var endpointKey string
	if ing.Spec.Backend != nil { // stream
		endpointKey = fmt.Sprintf("%s/%s", "gateway", ing.Spec.Backend.ServiceName)
	} else { // http or https
	Loop:
		for _, rule := range ing.Spec.Rules {
			for _, path := range rule.IngressRuleValue.HTTP.Paths {
				endpointKey = fmt.Sprintf("%s/%s", "gateway", path.Backend.ServiceName)
				if endpointKey != "" {
					break Loop
				}
			}
		}
	}
	_, exists, _ := s.listers.Endpoint.GetByKey(endpointKey)
	if !exists {
		logrus.Infof("Endpoint \"%s\" does not exist.", endpointKey)
		return false
	}
	return true
}

func (s *rbdStore) InitSecret() {
	for _, item := range s.listers.Secret.List() {
		secret := item.(*corev1.Secret)
		sslCertMap[secret.ObjectMeta.Name] = secret
	}
}

// ListIngresses returns the list of Ingresses
func (s *rbdStore) ListIngresses() []*extensions.Ingress {
	// filter ingress rules
	var ingresses []*extensions.Ingress
	for _, item := range s.listers.Ingress.List() {
		ing := item.(*extensions.Ingress)

		ingresses = append(ingresses, ing)
	}

	return ingresses
}

// Run initiates the synchronization of the informers.
func (s *rbdStore) Run(stopCh chan struct{}) {
	// start informers
	s.informers.Run(stopCh)
}
