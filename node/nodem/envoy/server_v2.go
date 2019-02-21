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
	"net"

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
	server     server.Server
	conf       option.Conf
	grpcServer *grpc.Server
	cache      cache.SnapshotCache
}

// Hasher returns node ID as an ID
type Hasher struct {
}

// ID function
func (h Hasher) ID(node *core.Node) string {
	if node == nil {
		return "unknown"
	}
	return node.Id
}

//CreateDiscoverServerManager create discover server manager
func CreateDiscoverServerManager(kubecli kubecache.KubeClient, conf option.Conf) (*DiscoverServerManager, error) {
	configcache := cache.NewSnapshotCache(false, Hasher{}, logrus.WithField("module", "config-cache"))
	dsm := &DiscoverServerManager{
		server: server.NewServer(configcache, nil),
		cache:  configcache,
	}
	kubecli.AddEventWatch("all", dsm)
	return dsm, nil
}

const grpcMaxConcurrentStreams = 1000000

//Start server start
func (d *DiscoverServerManager) Start(errch chan error) error {
	// gRPC golang library sets a very small upper bound for the number gRPC/h2
	// streams over a single TCP connection. If a proxy multiplexes requests over
	// a single connection to the management server, then it might lead to
	// availability problems.
	var grpcOptions []grpc.ServerOption
	grpcOptions = append(grpcOptions, grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams))
	d.grpcServer = grpc.NewServer(grpcOptions...)

	lis, err := net.Listen("tcp", d.conf.GrpcAPIAddr)
	if err != nil {
		return err
	}
	// register services
	discovery.RegisterAggregatedDiscoveryServiceServer(d.grpcServer, d.server)
	v2.RegisterEndpointDiscoveryServiceServer(d.grpcServer, d.server)
	v2.RegisterClusterDiscoveryServiceServer(d.grpcServer, d.server)
	v2.RegisterRouteDiscoveryServiceServer(d.grpcServer, d.server)
	v2.RegisterListenerDiscoveryServiceServer(d.grpcServer, d.server)
	discovery.RegisterSecretDiscoveryServiceServer(d.grpcServer, d.server)
	logrus.Info("management server listening")
	go func() {
		if err = d.grpcServer.Serve(lis); err != nil {
			errch <- err
		}
	}()
	return nil
}

//Stop stop grpc server
func (d *DiscoverServerManager) Stop() {
	d.grpcServer.GracefulStop()
}

//OnAdd on add resource
func (d *DiscoverServerManager) OnAdd(obj interface{}) {

}

//OnUpdate on update resource
func (d *DiscoverServerManager) OnUpdate(oldObj, newObj interface{}) {

}

//OnDelete on delete resource
func (d *DiscoverServerManager) OnDelete(obj interface{}) {

}
