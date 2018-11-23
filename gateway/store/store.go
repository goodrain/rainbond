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
	"bytes"
	"fmt"
	"github.com/golang/glog"
	"github.com/goodrain/rainbond/gateway/annotations"
	"io/ioutil"
	"k8s.io/ingress-nginx/k8s"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/gateway/v1"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	istroe "k8s.io/ingress-nginx/ingress/controller/store"
)

type EventType string

const (
	// CreateEvent event associated with new objects in an informer
	CreateEvent EventType = "CREATE"
	// UpdateEvent event associated with an object update in an informer
	UpdateEvent EventType = "UPDATE"
	// DeleteEvent event associated when an object is removed from an informer
	DeleteEvent EventType = "DELETE"
	// ConfigurationEvent event associated when a controller configuration object is created or updated
	ConfigurationEvent EventType = "CONFIGURATION"
	CertificatePath              = "/export/servers/nginx/certificate"
	DefServerName                = "_"
)

var l7PoolMap = make(map[string]struct{})
var l4PoolMap = make(map[string]struct{})

//Storer is the interface that wraps the required methods to gather information
type Storer interface {
	// get endpoints pool by name
	//GetPool(name string) *v1.Pool

	// list endpoints pool
	ListPool() ([]*v1.Pool, []*v1.Pool)

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
	ListVirtualService() ([]*v1.VirtualService, []*v1.VirtualService)

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

	GetIngressAnnotations(key string) (*annotations.Ingress, error)

	// Run initiates the synchronization of the controllers
	Run(stopCh chan struct{})
}

// Event holds the context of an event.
type Event struct {
	Type EventType
	Obj  interface{}
}

// Lister contains object listers (stores).
type Lister struct {
	Ingress           istroe.IngressLister
	Service           istroe.ServiceLister
	Endpoint          istroe.EndpointLister
	Secret            istroe.SecretLister
	IngressAnnotation IngressAnnotationsLister
}

type rbdStore struct {
	// informer contains the cache Informers
	informers *Informer
	// Lister contains object listers (stores).
	listers          *Lister
	secretIngressMap *secretIngressMap
	// sslStore local store of SSL certificates (certificates used in ingress)
	// this is required because the certificates must be present in the
	// container filesystem
	sslStore    *SSLCertTracker
	annotations annotations.Extractor
}

// New creates a new Storer
func New(client kubernetes.Interface,
	updateCh *channels.RingChannel) Storer {
	store := &rbdStore{
		informers: &Informer{},
		listers:   &Lister{},
		secretIngressMap: &secretIngressMap{
			make(map[string][]string),
		},
		sslStore: NewSSLCertTracker(),
	}

	store.annotations = annotations.NewAnnotationExtractor(store)
	store.listers.IngressAnnotation.Store = cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc)

	// create informers factory, enable and assign required informers
	infFactory := informers.NewFilteredSharedInformerFactory(client, time.Second, "",
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
			ing := obj.(*extensions.Ingress)

			// updating annotations information for ingress
			store.extractAnnotations(ing)
			// takes an Ingress and updates all Secret objects it references in secretIngressMap.
			store.secretIngressMap.update(ing)
			// synchronizes data from all Secrets referenced by the given Ingress with the local store and file system.
			store.syncSecrets(ing)

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
		//UpdateFunc: func(old, cur interface{}) {
		//	curIng := cur.(*extensions.Ingress)
		//
		//	store.secretIngressMap.update(curIng)
		//	store.syncSecrets(curIng)
		//
		//	updateCh.In() <- Event{
		//		Type: UpdateEvent,
		//		Obj:  cur,
		//	}
		//},
	}

	secEventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			logrus.Debug("Secret AddFunc is called.\n")
			sec := obj.(*corev1.Secret)
			key := k8s.MetaNamespaceKey(sec)

			// find references in ingresses and update local ssl certs
			if ings := store.secretIngressMap.getSecretKeys(key); len(ings) > 0 {
				glog.Infof("secret %v was added and it is used in ingress annotations. Parsing...", key)
				for _, ingKey := range ings {
					ing, err := store.GetIngress(ingKey)
					if err != nil {
						glog.Errorf("could not find Ingress %v in local store", ingKey)
						continue
					}
					store.syncSecrets(ing)
				}
				updateCh.In() <- Event{
					Type: CreateEvent,
					Obj:  obj,
				}
			}
		},
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				sec := cur.(*corev1.Secret)
				key := k8s.MetaNamespaceKey(sec)

				// find references in ingresses and update local ssl certs
				if ings := store.secretIngressMap.getSecretKeys(key); len(ings) > 0 {
					glog.Infof("secret %v was updated and it is used in ingress annotations. Parsing...", key)
					for _, ingKey := range ings {
						ing, err := store.GetIngress(ingKey)
						if err != nil {
							glog.Errorf("could not find Ingress %v in local store", ingKey)
							continue
						}
						store.syncSecrets(ing)
					}
					updateCh.In() <- Event{
						Type: UpdateEvent,
						Obj:  cur,
					}
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			sec, ok := obj.(*corev1.Secret)
			if !ok {
				// If we reached here it means the secret was deleted but its final state is unrecorded.
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					glog.Errorf("couldn't get object from tombstone %#v", obj)
					return
				}
				sec, ok = tombstone.Obj.(*corev1.Secret)
				if !ok {
					glog.Errorf("Tombstone contained object that is not a Secret: %#v", obj)
					return
				}
			}

			store.sslStore.Delete(k8s.MetaNamespaceKey(sec))

			key := k8s.MetaNamespaceKey(sec)

			// find references in ingresses
			if ings := store.secretIngressMap.getSecretKeys(key); len(ings) > 0 {
				glog.Infof("secret %v was deleted and it is used in ingress annotations. Parsing...", key)
				updateCh.In() <- Event{
					Type: DeleteEvent,
					Obj:  obj,
				}
			}
		},
	}

	store.informers.Ingress.AddEventHandler(ingEventHandler)
	store.informers.Secret.AddEventHandler(secEventHandler)

	return store
}

// extractAnnotations parses ingress annotations converting the value of the
// annotation to a go struct and also information about the referenced secrets
func (s *rbdStore) extractAnnotations(ing *extensions.Ingress) {
	key := k8s.MetaNamespaceKey(ing)
	logrus.Infof("updating annotations information for ingress %v", key)

	anns := s.annotations.Extract(ing)

	err := s.listers.IngressAnnotation.Update(anns)
	if err != nil {
		logrus.Error(err)
	}
}

// ListPool returns the list of Pools
func (s *rbdStore) ListPool() ([]*v1.Pool, []*v1.Pool) {
	var httpPools []*v1.Pool
	var tcpPools []*v1.Pool
	for _, item := range s.listers.Endpoint.List() {
		endpoint := item.(*corev1.Endpoints)

		pool := &v1.Pool{
			Nodes: []*v1.Node{},
		}
		pool.Name = endpoint.ObjectMeta.Name
		for _, ss := range endpoint.Subsets {
			for _, address := range ss.Addresses {
				pool.Nodes = append(pool.Nodes, &v1.Node{
					Host: address.IP,
					Port: ss.Ports[0].Port,
				})
			}
		}
		if _, ok := l7PoolMap[pool.Name]; ok {
			httpPools = append(httpPools, pool)
		}
		if _, ok := l4PoolMap[pool.Name]; ok {
			tcpPools = append(tcpPools, pool)
		}
	}
	return httpPools, tcpPools
}

// ListVirtualService list l7 virtual service and l4 virtual service
func (s *rbdStore) ListVirtualService() (l7vs []*v1.VirtualService, l4vs []*v1.VirtualService) {
	l7vsMap := make(map[string]*v1.VirtualService)
	l4vsMap := make(map[string]*v1.VirtualService)
	// ServerName-LocationPath -> location
	srvLocMap := make(map[string]*v1.Location)
	for _, item := range s.listers.Ingress.List() {
		ing := item.(*extensions.Ingress)
		if !s.ingressIsValid(ing) {
			continue
		}

		ingKey := k8s.MetaNamespaceKey(ing)
		anns, err := s.GetIngressAnnotations(ingKey)
		if err != nil {
			logrus.Errorf("Error getting Ingress annotations %q: %v", ingKey, err)
		}

		if anns.L4.L4Enable && anns.L4.L4Host != "" && anns.L4.L4Port != 0 { // l4
			listening := fmt.Sprintf("%s:%v", anns.L4.L4Host, anns.L4.L4Port)
			vs := l4vsMap[listening]
			if vs != nil {
				logrus.Info("already have a ingress the same as %s, ignore %s", ingKey, ingKey)
				return
			}
			vs = &v1.VirtualService{
				Listening: []string{listening},
				PoolName:  ing.Spec.Backend.ServiceName,
			}
			l4PoolMap[vs.PoolName] = struct{}{}
			l4vsMap[listening] = vs
			l4vs = append(l4vs, vs)
		} else { // l7
			// parse TLS into a map
			hostSSLMap := make(map[string]*v1.SSLCert)
			for _, tls := range ing.Spec.TLS {
				secrKey := fmt.Sprintf("%s/%s", ing.Namespace, tls.SecretName)
				item, exists := s.sslStore.Get(secrKey)
				if !exists {
					logrus.Warnf("Secret named %s does not exist", secrKey)
				}
				sslCert := item.(*v1.SSLCert)
				for _, host := range tls.Hosts {
					hostSSLMap[host] = sslCert
				}
				// take first SSLCert as default
				if _, exists := hostSSLMap[DefServerName]; !exists {
					hostSSLMap[DefServerName] = sslCert
				}
			}

			for _, rule := range ing.Spec.Rules {
				var vs *v1.VirtualService
				serverName := strings.Replace(rule.Host, " ", "", -1)
				if serverName == "" {
					serverName = DefServerName
				}
				vs = l7vsMap[serverName]
				if vs == nil {
					vs = &v1.VirtualService{
						Listening:        []string{"80"},
						ServerName:       serverName,
						Protocol:         v1.HTTP,
						Locations:        []*v1.Location{},
						ForceSSLRedirect: anns.Rewrite.ForceSSLRedirect,
					}
					if len(hostSSLMap) != 0 {
						vs.Listening = []string{"443", "ssl"}
						if hostSSLMap[serverName] != nil {
							vs.SSLCert = hostSSLMap[serverName]
						} else {
							vs.SSLCert = hostSSLMap[DefServerName]
						}
					}

					l7vsMap[serverName] = vs
					l7vs = append(l7vs, vs)
				}

				for _, path := range rule.IngressRuleValue.HTTP.Paths {
					key := fmt.Sprintf("%s%s", serverName, path.Path)
					location := srvLocMap[key]
					l7PoolMap[path.Backend.ServiceName] = struct{}{}
					if srvLocMap[key] == nil {
						location = &v1.Location{
							Path:          path.Path,
							NameCondition: map[string]*v1.Condition{},
						}
						srvLocMap[key] = location
						vs.Locations = append(vs.Locations, location)
					}

					// If their ServiceName is the same, then the new one will overwrite the old one.
					nameCondition := &v1.Condition{}
					if anns.Header.Header != nil {
						nameCondition.Type = v1.HeaderType
						nameCondition.Value = anns.Header.Header
					} else if anns.Cookie.Cookie != nil {
						nameCondition.Type = v1.CookieType
						nameCondition.Value = anns.Cookie.Cookie
					} else {
						nameCondition.Type = v1.DefaultType
						nameCondition.Value = map[string]string{"1": "1"}
					}
					location.NameCondition[path.Backend.ServiceName] = nameCondition
				}
			}
		}
	}
	return l7vs, l4vs
}

// ingressIsValid checks if the specified ingress is valid
func (s *rbdStore) ingressIsValid(ing *extensions.Ingress) bool {
	var endpointKey string
	if ing.Spec.Backend != nil { // stream
		endpointKey = fmt.Sprintf("%s/%s", ing.Namespace, ing.Spec.Backend.ServiceName)
	} else { // http
	Loop:
		for _, rule := range ing.Spec.Rules {
			for _, path := range rule.IngressRuleValue.HTTP.Paths {
				endpointKey = fmt.Sprintf("%s/%s", ing.Namespace, path.Backend.ServiceName)
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

// GetIngress returns the Ingress matching key.
func (s *rbdStore) GetIngress(key string) (*extensions.Ingress, error) {
	return s.listers.Ingress.ByKey(key)
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

// GetIngressAnnotations returns the parsed annotations of an Ingress matching key.
func (s rbdStore) GetIngressAnnotations(key string) (*annotations.Ingress, error) {
	ia, err := s.listers.IngressAnnotation.ByKey(key)
	if err != nil {
		return &annotations.Ingress{}, err
	}

	return ia, nil
}

// Run initiates the synchronization of the informers.
func (s *rbdStore) Run(stopCh chan struct{}) {
	// start informers
	s.informers.Run(stopCh)
}

// syncSecrets synchronizes data from all Secrets referenced by the given
// Ingress with the local store and file system.
func (s *rbdStore) syncSecrets(ing *extensions.Ingress) {
	key := k8s.MetaNamespaceKey(ing)
	// 获取所有关联的secret key
	for _, secrKey := range s.secretIngressMap.getSecretKeys(key) {
		s.syncSecret(secrKey)
	}
}

func (s *rbdStore) syncSecret(secrKey string) {
	sslCert, err := s.getCertificatePem(secrKey)
	if err != nil {
		logrus.Errorf("fail to get certificate pem: %v", err)
		return
	}

	old, exists := s.sslStore.Get(secrKey)
	if exists {
		oldSSLCert := old.(*v1.SSLCert)
		if sslCert.Equals(oldSSLCert) {
			logrus.Debugf("no need to update SSLCert named %s", secrKey)
			return
		}
		s.sslStore.Delete(secrKey)
	}

	s.sslStore.Add(secrKey, sslCert)
}

func (s *rbdStore) getCertificatePem(secrKey string) (*v1.SSLCert, error) {
	item, exists, err := s.listers.Secret.GetByKey(secrKey)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("the secret named %s does not exists", secrKey)
	}
	secret := item.(*corev1.Secret)
	crt := secret.Data[corev1.TLSCertKey]
	key := secret.Data[corev1.TLSPrivateKeyKey]

	var buffer bytes.Buffer
	buffer.Write(crt)
	buffer.Write(key)

	secrKey = strings.Replace(secrKey, "/", "-", 1)
	filename := fmt.Sprintf("%s/%s.pem", CertificatePath, secrKey)

	if e := os.MkdirAll(CertificatePath, 0777); e != nil {
		return nil, fmt.Errorf("cant not create directory %s: %v", CertificatePath, e)
	}

	if e := ioutil.WriteFile(filename, buffer.Bytes(), 0666); e != nil {
		return nil, fmt.Errorf("cant not write data to %s: %v", filename, e)
	}

	return &v1.SSLCert{
		CertificatePem: filename,
	}, nil
}
