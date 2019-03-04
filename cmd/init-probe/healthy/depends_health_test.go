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

package healthy

import (
	"context"
	"testing"

	yaml "gopkg.in/yaml.v2"

	envoyv2 "github.com/goodrain/rainbond/node/core/envoy/v2"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"

	"google.golang.org/grpc"

	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
)

var testClusterID = "5dbea040f5cb437c82e3bf02943fb02e_c5618104b2aa4c508390e4f18f316500_gr439125"

var testXDSHost = "39.96.17.249:6101"

// var testClusterID = "6ab5725e1ca34cfba7762b7ac10c0dee_9d379258e0bc4fc581331780b0541ac6_grc69d9c"
//var testXDSHost = "127.0.0.1:6101"

func TestClientListener(t *testing.T) {
	cli, err := grpc.Dial(testXDSHost, grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	listenerDiscover := v2.NewListenerDiscoveryServiceClient(cli)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	res, err := listenerDiscover.FetchListeners(ctx, &v2.DiscoveryRequest{
		Node: &core.Node{
			Cluster: testClusterID,
			Id:      testClusterID,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Resources) == 0 {
		t.Fatal("no listeners")
	}
	t.Logf("version %s", res.GetVersionInfo())
	listeners := envoyv2.ParseListenerResource(res.Resources)
	printYaml(t, listeners)
}

func TestClientCluster(t *testing.T) {
	cli, err := grpc.Dial(testXDSHost, grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	clusterDiscover := v2.NewClusterDiscoveryServiceClient(cli)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	res, err := clusterDiscover.FetchClusters(ctx, &v2.DiscoveryRequest{
		Node: &core.Node{
			Cluster: testClusterID,
			Id:      testClusterID,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Resources) == 0 {
		t.Fatal("no clusters")
	}
	t.Logf("version %s", res.GetVersionInfo())
	clusters := envoyv2.ParseClustersResource(res.Resources)
	printYaml(t, clusters)
}

func printYaml(t *testing.T, data interface{}) {
	out, _ := yaml.Marshal(data)
	t.Log(string(out))
}

func TestClientEndpoint(t *testing.T) {
	cli, err := grpc.Dial(testXDSHost, grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	endpointDiscover := v2.NewEndpointDiscoveryServiceClient(cli)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	res, err := endpointDiscover.FetchEndpoints(ctx, &v2.DiscoveryRequest{
		Node: &core.Node{
			Cluster: testClusterID,
			Id:      testClusterID,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Resources) == 0 {
		t.Fatal("no endpoints")
	}
	t.Logf("version %s", res.GetVersionInfo())
	endpoints := envoyv2.ParseLocalityLbEndpointsResource(res.Resources)
	printYaml(t, endpoints)
}

func TestNewDependServiceHealthController(t *testing.T) {
	controller, err := NewDependServiceHealthController()
	if err != nil {
		t.Fatal(err)
	}
	controller.Check()
}
