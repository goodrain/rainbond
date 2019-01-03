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
	"github.com/goodrain/rainbond/gateway/annotations/l4"
	"github.com/goodrain/rainbond/gateway/util"
	"io/ioutil"
	"net"
	"os"
	"reflect"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/cmd/gateway/option"
	"github.com/goodrain/rainbond/gateway/annotations"
	"github.com/goodrain/rainbond/gateway/v1"
	apiv1 "k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	istroe "k8s.io/ingress-nginx/ingress/controller/store"
	"k8s.io/ingress-nginx/k8s"
)

// EventType -
type EventType string

const (
	// CreateEvent event associated with new objects in an informer
	CreateEvent EventType = "CREATE"
	// UpdateEvent event associated with an object update in an informer
	UpdateEvent EventType = "UPDATE"
	// DeleteEvent event associated when an object is removed from an informer
	DeleteEvent EventType = "DELETE"
	// CertificatePath is the default path of certificate file
	CertificatePath = "/run/nginx/conf/certificate"
	// DefVirSrvName is the default virtual service name
	DefVirSrvName = "_"
)

var l7PoolMap = make(map[string]struct{})

// l7PoolBackendMap is the mapping between backend and pool
var l7PoolBackendMap map[string][]backend

var l4PoolMap = make(map[string]struct{})

// l4PoolBackendMap is the mapping between backend and pool
var l4PoolBackendMap map[string][]backend

//Storer is the interface that wraps the required methods to gather information
type Storer interface {
	// list endpoints pool
	ListPool() ([]*v1.Pool, []*v1.Pool)

	// list virtual service
	ListVirtualService() ([]*v1.VirtualService, []*v1.VirtualService)

	ListIngresses() []*extensions.Ingress

	GetIngressAnnotations(key string) (*annotations.Ingress, error)

	// Run initiates the synchronization of the controllers
	Run(stopCh chan struct{})
}

type backend struct {
	name   string
	weight int
	hashBy string
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
	conf *option.Config
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
	updateCh *channels.RingChannel,
	conf *option.Config) Storer {
	store := &rbdStore{
		informers: &Informer{},
		listers:   &Lister{},
		secretIngressMap: &secretIngressMap{
			make(map[string][]string),
		},
		sslStore: NewSSLCertTracker(),
		conf:     conf,
	}

	store.annotations = annotations.NewAnnotationExtractor(store)
	store.listers.IngressAnnotation.Store = cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc)

	// create informers factory, enable and assign required informers
	infFactory := informers.NewFilteredSharedInformerFactory(client, conf.ResyncPeriod, corev1.NamespaceAll,
		func(options *metav1.ListOptions) {
			options.LabelSelector = "creater=Rainbond"
		})

	store.informers.Ingress = infFactory.Extensions().V1beta1().Ingresses().Informer()
	store.listers.Ingress.Store = store.informers.Ingress.GetStore()

	store.informers.Service = infFactory.Core().V1().Services().Informer()
	store.listers.Service.Store = store.informers.Service.GetStore()

	store.informers.Endpoint = infFactory.Core().V1().Endpoints().Informer()
	store.listers.Endpoint.Store = store.informers.Endpoint.GetStore()

	store.informers.Secret = infFactory.Core().V1().Secrets().Informer()
	store.listers.Secret.Store = store.informers.Secret.GetStore()

	ingEventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ing := obj.(*extensions.Ingress)
			logrus.Debugf("Received ingress: %v", ing)

			//if !store.checkIngress(ing) {
			//	return
			//}

			// updating annotations information for ingress
			store.extractAnnotations(ing)
			// takes an Ingress and updates all Secret objects it references in secretIngressMap.
			store.secretIngressMap.update(ing)
			// synchronizes data from all Secrets referenced by the given Ingress with the local store and file system.
			store.syncSecrets(ing)

			updateCh.In() <- Event{
				Type: CreateEvent,
				Obj:  obj,
			}
		},
		DeleteFunc: func(obj interface{}) {
			updateCh.In() <- Event{
				Type: DeleteEvent,
				Obj:  obj,
			}
		},
		UpdateFunc: func(old, cur interface{}) {
			oldIng := old.(*extensions.Ingress)
			curIng := cur.(*extensions.Ingress)
			// ignore the same secret as the old one
			if oldIng.ResourceVersion == curIng.ResourceVersion || reflect.DeepEqual(oldIng, curIng) {
				return
			}
			logrus.Debugf("Received ingress: %v", curIng)

			//if !store.checkIngress(curIng) {
			//	return
			//}

			store.extractAnnotations(curIng)
			store.secretIngressMap.update(curIng)
			store.syncSecrets(curIng)

			updateCh.In() <- Event{
				Type: UpdateEvent,
				Obj:  cur,
			}
		},
	}

	secEventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			sec := obj.(*corev1.Secret)
			key := k8s.MetaNamespaceKey(sec)

			// find references in ingresses and update local ssl certs
			if ings := store.secretIngressMap.getSecretKeys(key); len(ings) > 0 {
				logrus.Infof("secret %v was added and it is used in ingress annotations. Parsing...", key)
				for _, ingKey := range ings {
					ing, err := store.GetIngress(ingKey)
					if err != nil {
						logrus.Errorf("could not find Ingress %v in local store", ingKey)
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
				curSec := cur.(*corev1.Secret)
				oldSec := old.(*corev1.Secret)
				// ignore the same secret as the old one
				if oldSec.ResourceVersion == curSec.ResourceVersion || reflect.DeepEqual(curSec, oldSec) {
					return
				}
				key := k8s.MetaNamespaceKey(curSec)

				// find references in ingresses and update local ssl certs
				if ings := store.secretIngressMap.getSecretKeys(key); len(ings) > 0 {
					logrus.Infof("secret %v was updated and it is used in ingress annotations. Parsing...", key)
					for _, ingKey := range ings {
						ing, err := store.GetIngress(ingKey)
						if err != nil {
							logrus.Errorf("could not find Ingress %v in local store", ingKey)
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
					logrus.Errorf("couldn't get object from tombstone %#v", obj)
					return
				}
				sec, ok = tombstone.Obj.(*corev1.Secret)
				if !ok {
					logrus.Errorf("Tombstone contained object that is not a Secret: %#v", obj)
					return
				}
			}

			store.sslStore.Delete(k8s.MetaNamespaceKey(sec))

			key := k8s.MetaNamespaceKey(sec)

			// find references in ingresses
			if ings := store.secretIngressMap.getSecretKeys(key); len(ings) > 0 {
				logrus.Infof("secret %v was deleted and it is used in ingress annotations. Parsing...", key)
				updateCh.In() <- Event{
					Type: DeleteEvent,
					Obj:  obj,
				}
			}
		},
	}

	// Endpoint Event Handler
	epEventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			updateCh.In() <- Event{
				Type: CreateEvent,
				Obj:  obj,
			}
		},
		DeleteFunc: func(obj interface{}) {
			updateCh.In() <- Event{
				Type: DeleteEvent,
				Obj:  obj,
			}
		},
		UpdateFunc: func(old, cur interface{}) {
			oep := old.(*corev1.Endpoints)
			cep := cur.(*corev1.Endpoints)
			if cep.ResourceVersion != oep.ResourceVersion && !reflect.DeepEqual(cep.Subsets, oep.Subsets) {
				updateCh.In() <- Event{
					Type: UpdateEvent,
					Obj:  cur,
				}
			}
		},
	}

	store.informers.Ingress.AddEventHandler(ingEventHandler)
	store.informers.Secret.AddEventHandler(secEventHandler)
	store.informers.Endpoint.AddEventHandler(epEventHandler)
	store.informers.Service.AddEventHandler(cache.ResourceEventHandlerFuncs{})

	return store
}

// checkIngress checks whether the given ing is valid.
func (s *rbdStore) checkIngress(ing *extensions.Ingress) bool {
	i, err := l4.NewParser(s).Parse(ing)
	if err != nil {
		logrus.Warningf("Uxpected error with ingress: %v", err)
		return false
	}

	cfg := i.(*l4.Config)
	if cfg.L4Enable {
		_, err := net.Dial("tcp", fmt.Sprintf("%s:%d", cfg.L4Host, cfg.L4Port))
		if err == nil {
			logrus.Warningf("%s, in Ingress(%v), is already in use.",
				fmt.Sprintf("%s:%d", cfg.L4Host, cfg.L4Port), ing)
			return false
		}
		return true
	}

	return true
}

// extractAnnotations parses ingress annotations converting the value of the
// annotation to a go struct and also information about the referenced secrets
func (s *rbdStore) extractAnnotations(ing *extensions.Ingress) {
	key := k8s.MetaNamespaceKey(ing)
	logrus.Debugf("updating annotations information for ingress %v", key)

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
	l7Pools := make(map[string]*v1.Pool)
	l4Pools := make(map[string]*v1.Pool)
	for _, item := range s.listers.Endpoint.List() {
		ep := item.(*corev1.Endpoints)

		if ep.Subsets != nil || len(ep.Subsets) != 0 {
			epn := ep.ObjectMeta.Name
			// l7
			backends := l7PoolBackendMap[ep.ObjectMeta.Name]
			for _, backend := range backends {
				pool := l7Pools[backend.name]
				if pool == nil {
					pool = &v1.Pool{
						Nodes: []*v1.Node{},
					}
					pool.Name = backend.name
					pool.UpstreamHashBy = backend.hashBy
					l7Pools[backend.name] = pool
				}
				for _, ss := range ep.Subsets {
					var addresses []corev1.EndpointAddress
					if ss.Addresses != nil && len(ss.Addresses) > 0 {
						addresses = append(addresses, ss.Addresses...)
					} else {
						addresses = append(addresses, ss.NotReadyAddresses...)
					}
					for _, address := range addresses {
						if _, ok := l7PoolMap[epn]; ok { // l7
							pool.Nodes = append(pool.Nodes, &v1.Node{
								Host:   address.IP,
								Port:   ss.Ports[0].Port,
								Weight: backend.weight,
							})
						}
					}
				}
			}
			// l4
			backends = l4PoolBackendMap[ep.ObjectMeta.Name]
			for _, backend := range backends {
				pool := l4Pools[backend.name]
				if pool == nil {
					pool = &v1.Pool{
						Nodes: []*v1.Node{},
					}
					pool.Name = backend.name
					l4Pools[backend.name] = pool
				}
				for _, ss := range ep.Subsets {
					var addresses []corev1.EndpointAddress
					if ss.Addresses != nil && len(ss.Addresses) > 0 {
						addresses = append(addresses, ss.Addresses...)
					} else {
						addresses = append(addresses, ss.NotReadyAddresses...)
					}
					for _, address := range addresses {
						if _, ok := l4PoolMap[epn]; ok { // l7
							pool.Nodes = append(pool.Nodes, &v1.Node{
								Host:   address.IP,
								Port:   ss.Ports[0].Port,
								Weight: backend.weight,
							})
						}
					}
				}
			}
		}
	}
	// change map to slice TODO: use map directly
	for _, pool := range l7Pools {
		httpPools = append(httpPools, pool)
	}
	for _, pool := range l4Pools {
		tcpPools = append(tcpPools, pool)
	}
	return httpPools, tcpPools
}

// ListVirtualService list l7 virtual service and l4 virtual service
func (s *rbdStore) ListVirtualService() (l7vs []*v1.VirtualService, l4vs []*v1.VirtualService) {
	l7PoolBackendMap = make(map[string][]backend)
	l4PoolBackendMap = make(map[string][]backend)
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
		if anns.L4.L4Enable && anns.L4.L4Port != 0 {
			// region l4
			host := strings.Replace(anns.L4.L4Host, " ", "", -1)
			if host == "" {
				host = s.conf.IP
			}
			host = s.conf.IP
			svcKey := fmt.Sprintf("%v/%v", ing.Namespace, ing.Spec.Backend.ServiceName)
			protocol := s.GetServiceProtocol(svcKey, ing.Spec.Backend.ServicePort.IntVal)
			listening := fmt.Sprintf("%s:%v", host, anns.L4.L4Port)
			if string(protocol) == string(v1.ProtocolUDP) {
				listening = fmt.Sprintf("%s %s", listening, "udp")
			}

			backendName := util.BackendName(listening, ing.Namespace)
			vs := l4vsMap[listening]
			if vs == nil {
				vs = &v1.VirtualService{
					Listening: []string{listening},
					PoolName:  backendName,
				}
			}

			l4PoolMap[ing.Spec.Backend.ServiceName] = struct{}{}
			l4vsMap[listening] = vs
			l4vs = append(l4vs, vs)
			backend := backend{name: backendName, weight: anns.Weight.Weight}
			l4PoolBackendMap[ing.Spec.Backend.ServiceName] = append(l4PoolBackendMap[ing.Spec.Backend.ServiceName], backend)
			// endregion
		} else {
			// region l7
			// parse TLS into a map
			hostSSLMap := make(map[string]*v1.SSLCert)
			for _, tls := range ing.Spec.TLS {
				secrKey := fmt.Sprintf("%s/%s", ing.Namespace, tls.SecretName)
				item, exists := s.sslStore.Get(secrKey)
				if !exists {
					logrus.Warnf("Secret named %s does not exist", secrKey)
					continue
				}
				sslCert := item.(*v1.SSLCert)
				for _, host := range tls.Hosts {
					hostSSLMap[host] = sslCert
				}
				// make the first SSLCert as default
				if _, exists := hostSSLMap[DefVirSrvName]; !exists {
					hostSSLMap[DefVirSrvName] = sslCert
				}
			}

			for _, rule := range ing.Spec.Rules {
				var vs *v1.VirtualService
				// virtual service name
				virSrvName := strings.Replace(rule.Host, " ", "", -1)
				if virSrvName == "" {
					virSrvName = DefVirSrvName
				}
				vs = l7vsMap[virSrvName]
				if vs == nil {
					vs = &v1.VirtualService{
						Listening:        []string{"80"},
						ServerName:       virSrvName,
						Locations:        []*v1.Location{},
						ForceSSLRedirect: anns.Rewrite.ForceSSLRedirect,
					}
					if len(hostSSLMap) != 0 {
						vs.Listening = []string{"443", "ssl"}
						if hostSSLMap[virSrvName] != nil {
							vs.SSLCert = hostSSLMap[virSrvName]
						} else { // TODO: if there is necessary to provide a default virtual service name
							vs.SSLCert = hostSSLMap[DefVirSrvName]
						}
					}

					l7vsMap[virSrvName] = vs
					l7vs = append(l7vs, vs)
				}

				for _, path := range rule.IngressRuleValue.HTTP.Paths {
					locKey := fmt.Sprintf("%s_%s", virSrvName, path.Path)
					location := srvLocMap[locKey]
					l7PoolMap[path.Backend.ServiceName] = struct{}{}
					// if location do not exists, then creates a new one
					if location == nil {
						location = &v1.Location{
							Path:          path.Path,
							NameCondition: map[string]*v1.Condition{},
						}
						srvLocMap[locKey] = location
						vs.Locations = append(vs.Locations, location)
					}
					// If their ServiceName is the same, then the new one will overwrite the old one.
					nameCondition := &v1.Condition{}
					var backendName string
					if anns.Header.Header != nil {
						nameCondition.Type = v1.HeaderType
						nameCondition.Value = anns.Header.Header
						backendName = fmt.Sprintf("%s_%s", locKey, v1.HeaderType)
					} else if anns.Cookie.Cookie != nil {
						nameCondition.Type = v1.CookieType
						nameCondition.Value = anns.Cookie.Cookie
						backendName = fmt.Sprintf("%s_%s", locKey, v1.CookieType)
					} else {
						nameCondition.Type = v1.DefaultType
						nameCondition.Value = map[string]string{"1": "1"}
						backendName = fmt.Sprintf("%s_%s", locKey, v1.DefaultType)
					}
					backendName = util.BackendName(backendName, ing.Namespace)
					location.NameCondition[backendName] = nameCondition
					backend := backend{name: backendName, weight: anns.Weight.Weight}
					if anns.UpstreamHashBy != "" {
						backend.hashBy = anns.UpstreamHashBy
					}
					l7PoolBackendMap[path.Backend.ServiceName] = append(l7PoolBackendMap[path.Backend.ServiceName], backend)
				}
			}
			// endregion
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
	item, exists, err := s.listers.Endpoint.GetByKey(endpointKey)
	if err != nil {
		logrus.Errorf("Can not get endpoint by key(%s): %v", endpointKey, err)
		return false
	}
	if !exists {
		logrus.Warningf("Endpoint \"%s\" does not exist.", endpointKey)
		return false
	}
	endpoint, ok := item.(*apiv1.Endpoints)
	if !ok {
		logrus.Errorf("Cant not convert %v to %v", reflect.TypeOf(item), reflect.TypeOf(endpoint))
		return false
	}
	if endpoint.Subsets == nil || len(endpoint.Subsets) == 0 {
		logrus.Warningf("Endpoints(%s) is empty, ignore it", endpointKey)
		return false
	}
	for _, ep := range endpoint.Subsets {
		if (ep.Addresses == nil || len(ep.Addresses) == 0) && (ep.NotReadyAddresses == nil || len(ep.NotReadyAddresses) == 0) {
			logrus.Warningf("Endpoints(%s) is empty, ignore it", endpointKey)
			return false
		}
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

// GetServiceProtocol returns the Service matching key and port.
func (s *rbdStore) GetServiceProtocol(key string, port int32) corev1.Protocol {
	svcs, err := s.listers.Service.ByKey(key)
	if err != nil {
		return corev1.ProtocolTCP
	}
	for _, p := range svcs.Spec.Ports {
		if p.Port == port {
			return p.Protocol
		}
	}

	return corev1.ProtocolTCP
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
	buffer.Write([]byte("\n"))
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
