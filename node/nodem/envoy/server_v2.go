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

package envoy

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/envoyproxy/go-control-plane/pkg/cache/types"

	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoy_api_v2_core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v2"
	"github.com/envoyproxy/go-control-plane/pkg/server/v2"
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/nodem/envoy/conver"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	kcache "k8s.io/client-go/tools/cache"
)

// DiscoverServerManager envoy discover server
type DiscoverServerManager struct {
	server          server.Server
	conf            option.Conf
	grpcServer      *grpc.Server
	cacheManager    cache.SnapshotCache
	cacheNodeConfig []*NodeConfig
	kubecli         kubernetes.Interface
	eventChan       chan *Event
	pool            *sync.Pool
	ctx             context.Context
	cancel          context.CancelFunc
	services        cacheHandler
	endpoints       cacheHandler
	configmaps      cacheHandler
	queue           Queue
}

// Hasher returns node ID as an ID
type Hasher struct {
}

// ID function
func (h Hasher) ID(node *envoy_api_v2_core.Node) string {
	if node == nil {
		return "unknown"
	}
	return node.Cluster
}

// NodeConfig envoy node config cache struct
type NodeConfig struct {
	nodeID                         string
	namespace                      string
	serviceAlias                   string
	version                        int64
	config                         *corev1.ConfigMap
	configModel                    *api_model.ResourceSpec
	dependServices                 sync.Map
	listeners, clusters, endpoints []types.Resource
}

// GetID get envoy node config id
func (n *NodeConfig) GetID() string {
	return n.nodeID
}

// TryUpdate try update resources, if don't care about,direct return false
// if return true, snapshot need update
func (n *NodeConfig) TryUpdate(obj interface{}) (needUpdate bool) {
	if service, ok := obj.(*corev1.Service); ok {
		if v, ok := service.Labels["creator"]; !ok || v != "Rainbond" {
			return false
		}
		if _, ok := n.dependServices.Load(service.Labels["service_id"]); ok {
			return true
		}
	}
	if endpoints, ok := obj.(*corev1.Endpoints); ok {
		if v, ok := endpoints.Labels["creator"]; !ok || v != "Rainbond" {
			return false
		}
		if _, ok := n.dependServices.Load(endpoints.Labels["service_id"]); ok {
			return true
		}
	}
	return false
}

// VersionUpdate add version index
func (n *NodeConfig) VersionUpdate() {
	newVersion := atomic.AddInt64(&n.version, 1)
	n.version = newVersion
}

// GetVersion get version
func (n *NodeConfig) GetVersion() string {
	return fmt.Sprintf("version_%d", n.version)
}

func createNodeID(namespace, pluginID, serviceAlias string) string {
	return fmt.Sprintf("%s_%s_%s", namespace, pluginID, serviceAlias)
}

type cacheHandler struct {
	informer kcache.SharedIndexInformer
	handler  *ChainHandler
}

// GetServicesAndEndpoints get service and endpoint
func (d *DiscoverServerManager) GetServicesAndEndpoints(namespace string, labelSelector labels.Selector) (ret []*corev1.Service, eret []*corev1.Endpoints) {
	kcache.ListAllByNamespace(d.services.informer.GetIndexer(), namespace, labelSelector, func(s interface{}) {
		ret = append(ret, s.(*corev1.Service))
	})
	kcache.ListAllByNamespace(d.endpoints.informer.GetIndexer(), namespace, labelSelector, func(s interface{}) {
		eret = append(eret, s.(*corev1.Endpoints))
	})
	return
}

// NewNodeConfig new NodeConfig
func (d *DiscoverServerManager) NewNodeConfig(config *corev1.ConfigMap) (*NodeConfig, error) {
	logrus.Debugf("cm name: %s; plugin-config: %s", config.GetName(), config.Data["plugin-config"])
	servicaAlias := config.Labels["service_alias"]
	namespace := config.Namespace
	configs, pluginID, err := conver.GetPluginConfigs(config)
	if err != nil {
		return nil, err
	}
	nc := &NodeConfig{
		nodeID:         createNodeID(namespace, pluginID, servicaAlias),
		serviceAlias:   servicaAlias,
		namespace:      namespace,
		version:        0,
		config:         config,
		configModel:    configs,
		dependServices: sync.Map{},
	}
	return nc, nil
}

// UpdateNodeConfig update node config
func (d *DiscoverServerManager) UpdateNodeConfig(nc *NodeConfig) error {
	var services []*corev1.Service
	var endpoint []*corev1.Endpoints
	for _, dep := range nc.configModel.BaseServices {
		nc.dependServices.Store(dep.DependServiceID, true)
		labelname := fmt.Sprintf("name=%sService", dep.DependServiceAlias)
		selector, err := labels.Parse(labelname)
		if err != nil {
			logrus.Errorf("parse selector %s failure %s", labelname, err.Error())
		}
		if selector != nil {
			upServices, upEndpoints := d.GetServicesAndEndpoints(nc.namespace, selector)
			for i, service := range upServices {
				for _, p := range service.Spec.Ports {
					listenPort := p.Port
					if value, ok := service.Labels["origin_port"]; ok {
						origin, _ := strconv.Atoi(value)
						if origin != 0 {
							listenPort = int32(origin)
						}
					}
					if listenPort == int32(dep.Port) {
						services = append(services, upServices[i])
					}
				}
			}
			for i, end := range upEndpoints {
				if len(end.Subsets) == 0 || len(end.Subsets[0].Ports) == 0 {
					continue
				}
				endpoint = append(endpoint, upEndpoints[i])
			}
		}
	}
	if nc.configModel.BasePorts != nil && len(nc.configModel.BasePorts) > 0 {
		labelname := fmt.Sprintf("name=%sServiceOUT", nc.serviceAlias)
		selector, err := labels.Parse(labelname)
		if err != nil {
			logrus.Errorf("parse selector %s failure %s", labelname, err.Error())
		}
		if selector != nil {
			downService, downEndpoint := d.GetServicesAndEndpoints(nc.namespace, selector)
			services = append(services, downService...)
			endpoint = append(endpoint, downEndpoint...)
		}
	}
	listeners, err := conver.OneNodeListerner(nc.serviceAlias, nc.namespace, nc.config, services)
	if err != nil {
		logrus.Errorf("create envoy listeners failure %s", err.Error())
	} else {
		nc.listeners = listeners
	}
	clusters, err := conver.OneNodeCluster(nc.serviceAlias, nc.namespace, nc.config, services)
	if err != nil {
		logrus.Errorf("create envoy clusters failure %s", err.Error())
	} else {
		nc.clusters = clusters
	}
	clusterLoadAssignment := conver.OneNodeClusterLoadAssignment(nc.serviceAlias, nc.namespace, endpoint, services)
	if len(clusterLoadAssignment) == 0 {
		logrus.Warningf("configmap name: %s; plugin-config: %s; empty clusterLoadAssignment", nc.config.Name, nc.config.Data["plugin-config"])
	}
	if err != nil {
		logrus.Errorf("create envoy endpoints failure %s", err.Error())
	} else {
		nc.endpoints = clusterLoadAssignment
	}
	//Fill the configuration information and inject envoy
	nc.VersionUpdate()
	return d.setSnapshot(nc)
}

func (d *DiscoverServerManager) setSnapshot(nc *NodeConfig) error {
	if len(nc.clusters) < 1 || len(nc.listeners) < 1 {
		logrus.Warningf("node id: %s; node config cluster length is zero or listener length is zero,not set snapshot", nc.GetID())
		return nil
	}
	snapshot := cache.NewSnapshot(nc.GetVersion(), nc.endpoints, nc.clusters, nil, nc.listeners, nil)
	err := d.cacheManager.SetSnapshot(nc.nodeID, snapshot)
	if err != nil {
		return err
	}
	logrus.Infof("cache envoy node %s config,version: %s", nc.GetID(), nc.GetVersion())
	return nil
}

// CreateDiscoverServerManager create discover server manager
func CreateDiscoverServerManager(clientset kubernetes.Interface, conf option.Conf) (*DiscoverServerManager, error) {
	configcache := cache.NewSnapshotCache(false, Hasher{}, logrus.WithField("module", "config-cache"))
	ctx, cancel := context.WithCancel(context.Background())
	dsm := &DiscoverServerManager{
		server:       server.NewServer(ctx, configcache, nil),
		cacheManager: configcache,
		kubecli:      clientset,
		conf:         conf,
		eventChan:    make(chan *Event, 100),
		pool: &sync.Pool{
			New: func() interface{} {
				return &Task{}
			},
		},
		ctx:    ctx,
		cancel: cancel,
		queue:  NewQueue(1 * time.Second),
	}
	sharedInformers := informers.NewFilteredSharedInformerFactory(dsm.kubecli, time.Second*10, corev1.NamespaceAll, func(options *meta_v1.ListOptions) {
		options.LabelSelector = "creator=Rainbond"
	})
	svcInformer := sharedInformers.Core().V1().Services().Informer()
	dsm.services = dsm.createCacheHandler(svcInformer, "Services")
	epInformer := sharedInformers.Core().V1().Endpoints().Informer()
	dsm.endpoints = dsm.createEDSCacheHandler(epInformer, "Endpoints")
	configsInformer := sharedInformers.Core().V1().ConfigMaps().Informer()
	dsm.configmaps = dsm.createCacheHandler(configsInformer, "ConfigMaps")
	dsm.configmaps.handler.Append(dsm.configHandle)
	dsm.endpoints.handler.Append(dsm.resourceSimpleHandle)
	dsm.services.handler.Append(dsm.resourceSimpleHandle)
	return dsm, nil
}

const grpcMaxConcurrentStreams = 1000000

// Start server start
func (d *DiscoverServerManager) Start(errch chan error) error {
	go func() {
		go d.queue.Run(d.ctx.Done())
		go d.services.informer.Run(d.ctx.Done())
		go d.endpoints.informer.Run(d.ctx.Done())
		//waiting service and endpoint resource loading is complete
		logrus.Infof("waiting kube service and endpoint resource loading")
		kcache.WaitForCacheSync(d.ctx.Done(), d.services.informer.HasSynced, d.endpoints.informer.HasSynced)
		logrus.Infof("kube service and endpoint resource loading success")
		//loading rule config resource
		go d.configmaps.informer.Run(d.ctx.Done())
		// gRPC golang library sets a very small upper bound for the number gRPC/h2
		// streams over a single TCP connection. If a proxy multiplexes requests over
		// a single connection to the management server, then it might lead to
		// availability problems.
		var grpcOptions []grpc.ServerOption
		grpcOptions = append(grpcOptions, grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams))
		d.grpcServer = grpc.NewServer(grpcOptions...)
		// register services
		discovery.RegisterAggregatedDiscoveryServiceServer(d.grpcServer, d.server)
		v2.RegisterEndpointDiscoveryServiceServer(d.grpcServer, d.server)
		v2.RegisterClusterDiscoveryServiceServer(d.grpcServer, d.server)
		v2.RegisterRouteDiscoveryServiceServer(d.grpcServer, d.server)
		v2.RegisterListenerDiscoveryServiceServer(d.grpcServer, d.server)
		discovery.RegisterSecretDiscoveryServiceServer(d.grpcServer, d.server)
		logrus.Infof("envoy grpc management server listening %s", d.conf.GrpcAPIAddr)
		lis, err := net.Listen("tcp", d.conf.GrpcAPIAddr)
		if err != nil {
			errch <- err
			return
		}
		if err = d.grpcServer.Serve(lis); err != nil {
			errch <- err
		}
	}()
	return nil
}

// Stop stop grpc server
func (d *DiscoverServerManager) Stop() {
	//d.grpcServer.GracefulStop()
	d.cancel()
}
func (d *DiscoverServerManager) createCacheHandler(informer kcache.SharedIndexInformer, otype string) cacheHandler {
	handler := &ChainHandler{funcs: []Handler{}}

	informer.AddEventHandler(
		kcache.ResourceEventHandlerFuncs{
			// TODO: filtering functions to skip over un-referenced resources (perf)
			AddFunc: func(obj interface{}) {
				d.queue.Push(Task{handler: handler.Apply, obj: obj, event: EventAdd})
			},
			UpdateFunc: func(old, cur interface{}) {
				if !reflect.DeepEqual(old, cur) {
					d.queue.Push(Task{handler: handler.Apply, obj: cur, event: EventUpdate})
				}
			},
			DeleteFunc: func(obj interface{}) {
				d.queue.Push(Task{handler: handler.Apply, obj: obj, event: EventDelete})
			},
		})

	return cacheHandler{informer: informer, handler: handler}
}
func (d *DiscoverServerManager) createEDSCacheHandler(informer kcache.SharedIndexInformer, otype string) cacheHandler {
	handler := &ChainHandler{funcs: []Handler{}}

	informer.AddEventHandler(
		kcache.ResourceEventHandlerFuncs{
			// TODO: filtering functions to skip over un-referenced resources (perf)
			AddFunc: func(obj interface{}) {
				d.queue.Push(Task{handler: handler.Apply, obj: obj, event: EventAdd})
			},
			UpdateFunc: func(old, cur interface{}) {
				// Avoid pushes if only resource version changed (kube-scheduller, cluster-autoscaller, etc)
				oldE := old.(*corev1.Endpoints)
				curE := cur.(*corev1.Endpoints)

				if !reflect.DeepEqual(oldE.Subsets, curE.Subsets) {
					d.queue.Push(Task{handler: handler.Apply, obj: cur, event: EventUpdate})
				}
			},
			DeleteFunc: func(obj interface{}) {
				// Deleting the endpoints results in an empty set from EDS perspective - only
				// deleting the service should delete the resources. The full sync replaces the
				// maps.
				// c.updateEDS(obj.(*v1.Endpoints))
				d.queue.Push(Task{handler: handler.Apply, obj: obj, event: EventDelete})
			},
		})

	return cacheHandler{informer: informer, handler: handler}
}

// AddNodeConfig add node config cache
func (d *DiscoverServerManager) AddNodeConfig(nc *NodeConfig) {
	var exist bool
	for i, existNC := range d.cacheNodeConfig {
		if existNC.nodeID == nc.nodeID {
			nc.version = existNC.version
			d.cacheNodeConfig[i] = nc
			exist = true
			break
		}
	}
	if !exist {
		d.cacheNodeConfig = append(d.cacheNodeConfig, nc)
	}
	if err := d.UpdateNodeConfig(nc); err != nil {
		logrus.Errorf("update envoy node(%s) config failue %s", nc.GetID(), err.Error())
	}
}

// DeleteNodeConfig delete node config cache
func (d *DiscoverServerManager) DeleteNodeConfig(nodeID string) {
	for i, existNC := range d.cacheNodeConfig {
		if existNC.nodeID == nodeID {
			d.cacheManager.ClearSnapshot(existNC.nodeID)
			d.cacheNodeConfig = append(d.cacheNodeConfig[:i], d.cacheNodeConfig[i+1:]...)
		}
	}
}

func checkIsHandleResource(configMap *corev1.ConfigMap) bool {
	if value, ok := configMap.Data["plugin-model"]; ok &&
		(value == "net-plugin:up" || value == "net-plugin:down" || value == "net-plugin:in-and-out") {
		return true
	}
	return false
}

func (d *DiscoverServerManager) configHandle(obj interface{}, event Event) error {
	configMap, ok := obj.(*corev1.ConfigMap)
	if !ok {
		return fmt.Errorf("Illegal resources")
	}
	switch event {
	case EventAdd, EventUpdate:
		if checkIsHandleResource(configMap) {
			nc, err := d.NewNodeConfig(configMap)
			if err != nil {
				logrus.Errorf("create envoy node config failure %s", err.Error())
			}
			if nc != nil {
				d.AddNodeConfig(nc)
			}
		}
	case EventDelete:
		if checkIsHandleResource(configMap) {
			nodeID := createNodeID(configMap.Namespace, configMap.Labels["plugin_id"], configMap.Labels["service_alias"])
			d.DeleteNodeConfig(nodeID)
		}
		return nil
	}
	return nil
}

func (d *DiscoverServerManager) resourceSimpleHandle(obj interface{}, event Event) error {
	switch event {
	case EventAdd, EventUpdate, EventDelete:
		for i, nodeConfig := range d.cacheNodeConfig {
			if nodeConfig.TryUpdate(obj) {
				err := d.UpdateNodeConfig(d.cacheNodeConfig[i])
				if err != nil {
					logrus.Errorf("update envoy node config failure %s", err.Error())
				}
			}
		}
	}
	return nil
}
