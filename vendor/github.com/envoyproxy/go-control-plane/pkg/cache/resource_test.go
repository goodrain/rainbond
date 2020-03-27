// Copyright 2018 Envoyproxy Authors
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package cache_test

import (
	"reflect"
	"testing"

	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	v2route "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	"github.com/envoyproxy/go-control-plane/pkg/test/resource"
)

const (
	clusterName  = "cluster0"
	routeName    = "route0"
	listenerName = "listener0"
	runtimeName  = "runtime0"
)

var (
	endpoint = resource.MakeEndpoint(clusterName, 8080)
	cluster  = resource.MakeCluster(resource.Ads, clusterName)
	route    = resource.MakeRoute(routeName, clusterName)
	listener = resource.MakeHTTPListener(resource.Ads, listenerName, 80, routeName)
	runtime  = resource.MakeRuntime(runtimeName)
)

func TestValidate(t *testing.T) {
	if err := endpoint.Validate(); err != nil {
		t.Error(err)
	}
	if err := cluster.Validate(); err != nil {
		t.Error(err)
	}
	if err := route.Validate(); err != nil {
		t.Error(err)
	}
	if err := listener.Validate(); err != nil {
		t.Error(err)
	}
	if err := runtime.Validate(); err != nil {
		t.Error(err)
	}

	invalidRoute := &v2.RouteConfiguration{
		Name: "test",
		VirtualHosts: []*v2route.VirtualHost{{
			Name:    "test",
			Domains: []string{},
		}},
	}

	if err := invalidRoute.Validate(); err == nil {
		t.Error("expected an error")
	}
	if err := invalidRoute.VirtualHosts[0].Validate(); err == nil {
		t.Error("expected an error")
	}
}

func TestGetResourceName(t *testing.T) {
	if name := cache.GetResourceName(endpoint); name != clusterName {
		t.Errorf("GetResourceName(%v) => got %q, want %q", endpoint, name, clusterName)
	}
	if name := cache.GetResourceName(cluster); name != clusterName {
		t.Errorf("GetResourceName(%v) => got %q, want %q", cluster, name, clusterName)
	}
	if name := cache.GetResourceName(route); name != routeName {
		t.Errorf("GetResourceName(%v) => got %q, want %q", route, name, routeName)
	}
	if name := cache.GetResourceName(listener); name != listenerName {
		t.Errorf("GetResourceName(%v) => got %q, want %q", listener, name, listenerName)
	}
	if name := cache.GetResourceName(runtime); name != runtimeName {
		t.Errorf("GetResourceName(%v) => got %q, want %q", runtime, name, runtimeName)
	}
	if name := cache.GetResourceName(nil); name != "" {
		t.Errorf("GetResourceName(nil) => got %q, want none", name)
	}
}

func TestGetResourceReferences(t *testing.T) {
	cases := []struct {
		in  cache.Resource
		out map[string]bool
	}{
		{
			in:  nil,
			out: map[string]bool{},
		},
		{
			in:  cluster,
			out: map[string]bool{clusterName: true},
		},
		{
			in: &v2.Cluster{Name: clusterName, ClusterDiscoveryType: &v2.Cluster_Type{Type: v2.Cluster_EDS},
				EdsClusterConfig: &v2.Cluster_EdsClusterConfig{ServiceName: "test"}},
			out: map[string]bool{"test": true},
		},
		{
			in:  resource.MakeHTTPListener(resource.Ads, listenerName, 80, routeName),
			out: map[string]bool{routeName: true},
		},
		{
			in:  resource.MakeTCPListener(listenerName, 80, clusterName),
			out: map[string]bool{},
		},
		{
			in:  route,
			out: map[string]bool{},
		},
		{
			in:  endpoint,
			out: map[string]bool{},
		},
		{
			in:  runtime,
			out: map[string]bool{},
		},
	}
	for _, cs := range cases {
		names := cache.GetResourceReferences(cache.IndexResourcesByName([]cache.Resource{cs.in}))
		if !reflect.DeepEqual(names, cs.out) {
			t.Errorf("GetResourceReferences(%v) => got %v, want %v", cs.in, names, cs.out)
		}
	}
}
