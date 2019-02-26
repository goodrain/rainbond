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

	"github.com/envoyproxy/go-control-plane/pkg/util"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"

	"google.golang.org/grpc"

	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
)

func TestClientListener(t *testing.T) {
	cli, err := grpc.Dial("127.0.0.1:6102", grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	listenerDiscover := v2.NewListenerDiscoveryServiceClient(cli)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	res, err := listenerDiscover.FetchListeners(ctx, &v2.DiscoveryRequest{
		Node: &core.Node{
			Cluster: "6ab5725e1ca34cfba7762b7ac10c0dee_9d379258e0bc4fc581331780b0541ac6_grc69d9c",
			Id:      "6ab5725e1ca34cfba7762b7ac10c0dee_9d379258e0bc4fc581331780b0541ac6_grc69d9c",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(util.MessageToStruct(res))
}

func TestClientCluster(t *testing.T) {
	cli, err := grpc.Dial("127.0.0.1:6101", grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	clusterDiscover := v2.NewClusterDiscoveryServiceClient(cli)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	res, err := clusterDiscover.FetchClusters(ctx, &v2.DiscoveryRequest{
		Node: &core.Node{
			Cluster: "6ab5725e1ca34cfba7762b7ac10c0dee_9d379258e0bc4fc581331780b0541ac6_grc69d9c",
			Id:      "6ab5725e1ca34cfba7762b7ac10c0dee_9d379258e0bc4fc581331780b0541ac6_grc69d9c",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(util.MessageToStruct(res))
}

func TestClientEndpoint(t *testing.T) {
	cli, err := grpc.Dial("127.0.0.1:6101", grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	endpointDiscover := v2.NewEndpointDiscoveryServiceClient(cli)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	res, err := endpointDiscover.FetchEndpoints(ctx, &v2.DiscoveryRequest{
		Node: &core.Node{
			Cluster: "6ab5725e1ca34cfba7762b7ac10c0dee_9d379258e0bc4fc581331780b0541ac6_grc69d9c",
			Id:      "6ab5725e1ca34cfba7762b7ac10c0dee_9d379258e0bc4fc581331780b0541ac6_grc69d9c",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(util.MessageToStruct(res))
}
