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
	"sync"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/goodrain/rainbond/node/nodem/envoy/conver"

	api_model "github.com/goodrain/rainbond/api/model"
	corev1 "k8s.io/api/core/v1"

	"github.com/Sirupsen/logrus"
	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	"github.com/envoyproxy/go-control-plane/pkg/server"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/kubecache"
	"google.golang.org/grpc"
)

//DiscoverServerManager envoy discover server
type DiscoverServerManager struct {
	server          server.Server
	conf            option.Conf
	grpcServer      *grpc.Server
	cacheManager    cache.SnapshotCache
	cacheNodeConfig []*NodeConfig
	kubecli         kubecache.KubeClient
	eventChan       chan *Event
	pool            *sync.Pool
	ctx             context.Context
	cancel          context.CancelFunc
}

//Event event
type Event struct {
	MethodType string
	Source     interface{}
}

// Hasher returns node ID as an ID
type Hasher struct {
}

// ID function
func (h Hasher) ID(node *core.Node) string {
	if node == nil {
		return "unknown"
	}
	return node.Cluster
}

//NodeConfig envoy node config cache struct
type NodeConfig struct {
	nodeID                         string
	namespace                      string
	serviceAlias                   string
	version                        int64
	config                         *corev1.ConfigMap
	configModel                    *api_model.ResourceSpec
	dependServices                 sync.Map
	listeners, clusters, endpoints []cache.Resource
}

//GetID get envoy node config id
func (n *NodeConfig) GetID() string {
	return n.nodeID
}

//TryUpdate try update resources, if don't care about,direct return false
//if return true, snapshot need update
func (n *NodeConfig) TryUpdate(obj interface{}) (needUpdate bool) {
	if service, ok := obj.(*corev1.Service); ok {
		if v, ok := service.Labels["creater"]; !ok || v != "Rainbond" {
			return false
		}
		if _, ok := n.dependServices.Load(service.Labels["service_id"]); ok {
			return true
		}
	}
	if endpoints, ok := obj.(*corev1.Endpoints); ok {
		if v, ok := endpoints.Labels["creater"]; !ok || v != "Rainbond" {
			return false
		}
		if _, ok := n.dependServices.Load(endpoints.Labels["service_id"]); ok {
			return true
		}
	}
	if configMap, ok := obj.(*corev1.ConfigMap); ok {
		if v, ok := configMap.Labels["creater"]; !ok || v != "Rainbond" {
			return false
		}
		if configMap.Name == n.config.Name {
			n.config = configMap
			return true
		}
	}
	if secret, ok := obj.(*corev1.Secret); ok {
		//do not support
		logrus.Debugf("add secret %s", secret.Name)
	}
	return false
}

//VersionUpdate add version index
func (n *NodeConfig) VersionUpdate() {
	newVersion := atomic.AddInt64(&n.version, 1)
	n.version = newVersion
}

//GetVersion get version
func (n *NodeConfig) GetVersion() string {
	return fmt.Sprintf("version_%d", n.version)
}

func createNodeID(namespace, pluginID, serviceAlias string) string {
	return fmt.Sprintf("%s_%s_%s", namespace, pluginID, serviceAlias)
}

//GetDependService get depend service
func (d *DiscoverServerManager) GetDependService(namespace, depServiceAlias string) ([]*corev1.Service, []*corev1.Endpoints) {
	labelname := fmt.Sprintf("name=%sService", depServiceAlias)
	selector, err := labels.Parse(labelname)
	if err != nil {
		logrus.Errorf("parse label name failure %s", err.Error())
		return nil, nil
	}
	services, err := d.kubecli.GetServices(namespace, selector)
	if err != nil {
		logrus.Errorf("get depend service failure %s", err.Error())
		return nil, nil
	}
	endpoints, err := d.kubecli.GetEndpoints(namespace, selector)
	if err != nil {
		logrus.Errorf("get depend service endpoints failure %s", err.Error())
		return nil, nil
	}
	return services, endpoints
}

//GetSelfService get self service
func (d *DiscoverServerManager) GetSelfService(namespace, serviceAlias string) ([]*corev1.Service, []*corev1.Endpoints) {
	labelname := fmt.Sprintf("name=%sServiceOUT", serviceAlias)
	selector, err := labels.Parse(labelname)
	if err != nil {
		logrus.Errorf("parse label name failure %s", err.Error())
		return nil, nil
	}
	services, err := d.kubecli.GetServices(namespace, selector)
	if err != nil {
		logrus.Errorf("get self service failure %s", err.Error())
		return nil, nil
	}
	endpoints, err := d.kubecli.GetEndpoints(namespace, selector)
	if err != nil {
		logrus.Errorf("get self service endpoints failure %s", err.Error())
		return nil, nil
	}
	return services, endpoints
}

//NewNodeConfig new NodeConfig
func (d *DiscoverServerManager) NewNodeConfig(config *corev1.ConfigMap) (*NodeConfig, error) {
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
		version:        1,
		config:         config,
		configModel:    configs,
		dependServices: sync.Map{},
	}
	return nc, d.UpdateNodeConfig(nc)
}

//UpdateNodeConfig update node config
func (d *DiscoverServerManager) UpdateNodeConfig(nc *NodeConfig) error {
	var services []*corev1.Service
	var endpoint []*corev1.Endpoints
	for _, dep := range nc.configModel.BaseServices {
		nc.dependServices.Store(dep.DependServiceID, true)
		upServices, upEndpoints := d.GetDependService(nc.namespace, dep.DependServiceAlias)
		services = append(services, upServices...)
		endpoint = append(endpoint, upEndpoints...)
	}
	if nc.configModel.BasePorts != nil && len(nc.configModel.BasePorts) > 0 {
		downService, downEndpoint := d.GetSelfService(nc.namespace, nc.serviceAlias)
		services = append(services, downService...)
		endpoint = append(endpoint, downEndpoint...)
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
	if err != nil {
		logrus.Errorf("create envoy endpoints failure %s", err.Error())
	} else {
		nc.endpoints = clusterLoadAssignment
	}
	return d.setSnapshot(nc)
}

func (d *DiscoverServerManager) setSnapshot(nc *NodeConfig) error {
	if len(nc.clusters) < 1 || len(nc.listeners) < 1 {
		logrus.Warn("node config cluster length is zero or listener length is zero,not set snapshot")
		return nil
	}
	snapshot := cache.NewSnapshot(nc.GetVersion(), nc.endpoints, nc.clusters, nil, nc.listeners)
	err := d.cacheManager.SetSnapshot(nc.nodeID, snapshot)
	if err != nil {
		return err
	}
	logrus.Infof("cache envoy node %s config,version: %s", nc.GetID(), nc.GetVersion())
	//TODO: If the resource has not changed, there is no need to cache the new version
	nc.VersionUpdate()
	return nil
}

//CreateDiscoverServerManager create discover server manager
func CreateDiscoverServerManager(kubecli kubecache.KubeClient, conf option.Conf) (*DiscoverServerManager, error) {
	configcache := cache.NewSnapshotCache(false, Hasher{}, logrus.WithField("module", "config-cache"))
	ctx, cancel := context.WithCancel(context.Background())
	dsm := &DiscoverServerManager{
		server:       server.NewServer(configcache, nil),
		cacheManager: configcache,
		kubecli:      kubecli,
		conf:         conf,
		eventChan:    make(chan *Event, 100),
		pool: &sync.Pool{
			New: func() interface{} {
				return &Event{}
			},
		},
		ctx:    ctx,
		cancel: cancel,
	}
	kubecli.AddEventWatch("all", dsm)
	return dsm, nil
}

const grpcMaxConcurrentStreams = 1000000

//Start server start
func (d *DiscoverServerManager) Start(errch chan error) error {

	// start handle event
	go d.handleEvent()

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
	go func() {
		logrus.Infof("envoy grpc management server listening %s", d.conf.GrpcAPIAddr)
		lis, err := net.Listen("tcp", d.conf.GrpcAPIAddr)
		if err != nil {
			errch <- err
		}
		if err = d.grpcServer.Serve(lis); err != nil {
			errch <- err
		}
	}()
	return nil
}

//Stop stop grpc server
func (d *DiscoverServerManager) Stop() {
	//d.grpcServer.GracefulStop()
	d.cancel()
}

//AddNodeConfig add node config cache
func (d *DiscoverServerManager) AddNodeConfig(nc *NodeConfig) {
	for i, existNC := range d.cacheNodeConfig {
		if existNC.nodeID == nc.nodeID {
			nc.version = existNC.version
			d.cacheNodeConfig[i] = nc
			return
		}
	}
	d.cacheNodeConfig = append(d.cacheNodeConfig, nc)
}

//DeleteNodeConfig delete node config cache
func (d *DiscoverServerManager) DeleteNodeConfig(nodeID string) {
	for i, existNC := range d.cacheNodeConfig {
		if existNC.nodeID == nodeID {
			d.cacheManager.ClearSnapshot(existNC.nodeID)
			d.cacheNodeConfig = append(d.cacheNodeConfig[:i], d.cacheNodeConfig[i+1:]...)
		}
	}
}

//OnAdd on add for k8s
func (d *DiscoverServerManager) OnAdd(obj interface{}) {
	event := d.pool.Get().(*Event)
	event.MethodType = "update"
	event.Source = obj
	d.eventChan <- event
}
func checkIsHandleResource(configMap *corev1.ConfigMap) bool {
	if value, ok := configMap.Data["plugin-model"]; ok && (value == "net-plugin:down" || value == "net-plugin:up") {
		return true
	}
	return false
}

//OnAdd on add resource
func (d *DiscoverServerManager) onAdd(obj interface{}) {
	if configMap, ok := obj.(*corev1.ConfigMap); ok {
		if checkIsHandleResource(configMap) {
			nc, err := d.NewNodeConfig(configMap)
			if err != nil {
				logrus.Errorf("create envoy node config failure %s", err.Error())
			}
			if nc != nil {
				d.AddNodeConfig(nc)
			}
		}
		return
	}
	for i, nodeConfig := range d.cacheNodeConfig {
		if nodeConfig.TryUpdate(obj) {
			err := d.UpdateNodeConfig(d.cacheNodeConfig[i])
			if err != nil {
				logrus.Errorf("update envoy node config failure %s", err.Error())
			}
		}
	}
}

func (d *DiscoverServerManager) handleEvent() {
	for {
		select {
		case event := <-d.eventChan:
			switch event.MethodType {
			case "update":
				d.onAdd(event.Source)
			case "delete":
				d.onDelete(event.Source)
			}
			d.pool.Put(event)
		case <-d.ctx.Done():
			return
		}
	}
}

//OnUpdate on update resource
func (d *DiscoverServerManager) OnUpdate(oldObj, newObj interface{}) {
	d.OnAdd(newObj)
}

//OnDelete on delete resource
func (d *DiscoverServerManager) OnDelete(obj interface{}) {
	event := d.pool.Get().(*Event)
	event.MethodType = "delete"
	event.Source = obj
	d.eventChan <- event
}

//OnDelete on delete resource
func (d *DiscoverServerManager) onDelete(obj interface{}) {
	if configMap, ok := obj.(*corev1.ConfigMap); ok {
		if checkIsHandleResource(configMap) {
			nodeID := createNodeID(configMap.Namespace, configMap.Labels["plugin_id"], configMap.Labels["service_alias"])
			d.DeleteNodeConfig(nodeID)
		}
		return
	}
	for i, nodeConfig := range d.cacheNodeConfig {
		if nodeConfig.TryUpdate(obj) {
			err := d.UpdateNodeConfig(d.cacheNodeConfig[i])
			if err != nil {
				logrus.Errorf("update envoy node config failure %s", err.Error())
			}
		}
	}
}
