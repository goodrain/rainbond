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

package conver

import (
	"fmt"
	"time"

	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	api_model "github.com/goodrain/rainbond/api/model"
	envoyv2 "github.com/goodrain/rainbond/node/core/envoy/v2"
	corev1 "k8s.io/api/core/v1"
)

//OneNodeCluster conver cluster of on envoy node
func OneNodeCluster(serviceAlias, namespace string, configs []*corev1.ConfigMap, services []*corev1.Service) ([]v2.Cluster, error) {
	resources, _, err := getPluginConfigs(configs)
	if err != nil {
		return nil, err
	}
	var clusters []v2.Cluster
	if resources.BaseServices != nil && len(resources.BaseServices) > 0 {
		clusters = append(clusters, upstreamClusters(serviceAlias, namespace, resources.BaseServices, services)...)
	}
	if resources.BasePorts != nil && len(resources.BasePorts) > 0 {
		clusters = append(clusters, downstreamClusters(serviceAlias, namespace, resources.BasePorts)...)
	}
	return clusters, nil
}

// upstreamClusters handle upstream app cluster
// handle kubernetes inner service
func upstreamClusters(serviceAlias, namespace string, dependsServices []*api_model.BaseService, services []*corev1.Service) (cdsClusters []v2.Cluster) {
	var clusterConfig = make(map[string]*api_model.BaseService, len(dependsServices))
	for i, dService := range dependsServices {
		clusterName := fmt.Sprintf("%s_%s_%s_%d", namespace, serviceAlias, dService.DependServiceAlias, dService.Port)
		clusterConfig[clusterName] = dependsServices[i]
	}
	var portMap = make(map[int32]int)

	for _, service := range services {
		inner, ok := service.Labels["service_type"]
		destServiceAlias := service.Labels["service_alias"]
		port := service.Spec.Ports[0]
		if !ok || inner != "inner" {
			continue
		}
		clusterName := fmt.Sprintf("%s_%s_%s_%v", namespace, serviceAlias, service.Labels["service_alias"], port.Port)
		options := envoyv2.GetOptionValues(clusterConfig[clusterName].Options)
		createCluster := func(name string) v2.Cluster {
			return v2.Cluster{
				Name:           name,
				Type:           v2.Cluster_EDS,
				ConnectTimeout: time.Second * 250,
				LbPolicy:       v2.Cluster_ROUND_ROBIN,
				EdsClusterConfig: &v2.Cluster_EdsClusterConfig{
					EdsConfig:   &core.ConfigSource{},
					ServiceName: fmt.Sprintf("%s_%s_%s_%v", namespace, serviceAlias, destServiceAlias, port.Port),
				},
				OutlierDetection: envoyv2.CreatOutlierDetection(options),
				CircuitBreakers:  envoyv2.CreateCircuitBreaker(options),
			}
		}
		pcds := createCluster(clusterName)
		cdsClusters = append(cdsClusters, pcds)
		//create cluster base unique port
		if count, ok := portMap[port.Port]; ok && count == 1 {
			cdsClusters = append(cdsClusters, createCluster(fmt.Sprintf("%s_%s_%v", namespace, serviceAlias, port.Port)))
			portMap[port.Port] = 2
		} else {
			portMap[port.Port] = 1
		}
		continue
	}
	return
}

//downstreamClusters handle app self cluster
//only local port
func downstreamClusters(serviceAlias, namespace string, ports []*api_model.BasePort) (cdsClusters []v2.Cluster) {
	for i := range ports {
		port := ports[i]
		address := envoyv2.CreateSocketAddress(port.Protocol, "127.0.0.1", uint32(port.Port))
		pcds := v2.Cluster{
			Name:            fmt.Sprintf("%s_%s_%v", namespace, serviceAlias, port.Port),
			Type:            v2.Cluster_STATIC,
			ConnectTimeout:  time.Second * 250,
			LbPolicy:        v2.Cluster_ROUND_ROBIN,
			Hosts:           []*core.Address{&address},
			CircuitBreakers: envoyv2.CreateCircuitBreaker(envoyv2.GetOptionValues(port.Options)),
		}
		cdsClusters = append(cdsClusters, pcds)
		continue
	}
	return
}
