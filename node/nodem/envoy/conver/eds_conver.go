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
	"strconv"

	"github.com/Sirupsen/logrus"

	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	envoyv2 "github.com/goodrain/rainbond/node/core/envoy/v2"
	corev1 "k8s.io/api/core/v1"
)

//OneNodeClusterLoadAssignment one envoy node endpoints
func OneNodeClusterLoadAssignment(serviceAlias, namespace string, endpoints []*corev1.Endpoints, services []*corev1.Service) (clusterLoadAssignment []cache.Resource) {
	for i := range services {
		service := services[i]
		destServiceAlias := GetServiceAliasByService(service)
		if destServiceAlias == "" {
			logrus.Errorf("service alias is empty in k8s service %s", service.Name)
			continue
		}
		clusterName := fmt.Sprintf("%s_%s_%s_%d", namespace, serviceAlias, destServiceAlias, service.Spec.Ports[0].Port)
		name := fmt.Sprintf("%sService", destServiceAlias)
		if destServiceAlias == serviceAlias {
			name = fmt.Sprintf("%sServiceOUT", destServiceAlias)
		}
		selectEndpoint := getEndpointsByLables(endpoints, map[string]string{"name": name})
		var lendpoints []endpoint.LocalityLbEndpoints
		for _, en := range selectEndpoint {
			for _, subset := range en.Subsets {
				if len(subset.Ports) < 1 {
					continue
				}
				toport := int(subset.Ports[0].Port)
				if serviceAlias == destServiceAlias {
					if originPort, ok := service.Labels["origin_port"]; ok {
						origin, err := strconv.Atoi(originPort)
						if err == nil {
							toport = origin
						}
					}
				}
				protocol := string(subset.Ports[0].Protocol)
				addressList := subset.Addresses
				var notready bool
				if len(addressList) == 0 {
					notready = true
					addressList = subset.NotReadyAddresses
				}
				getHealty := func() *endpoint.Endpoint_HealthCheckConfig {
					if notready {
						return nil
					}
					return &endpoint.Endpoint_HealthCheckConfig{
						PortValue: uint32(toport),
					}
				}
				var lbe []endpoint.LbEndpoint
				for _, address := range addressList {
					envoyAddress := envoyv2.CreateSocketAddress(protocol, address.IP, uint32(toport))
					lbe = append(lbe, endpoint.LbEndpoint{
						HostIdentifier: &endpoint.LbEndpoint_Endpoint{
							Endpoint: &endpoint.Endpoint{
								Address:           &envoyAddress,
								HealthCheckConfig: getHealty(),
							},
						},
					})
				}
				lendpoints = append(lendpoints, endpoint.LocalityLbEndpoints{LbEndpoints: lbe})
			}
		}
		cla := &v2.ClusterLoadAssignment{
			ClusterName: clusterName,
			Endpoints:   lendpoints,
		}
		if err := cla.Validate(); err != nil {
			logrus.Errorf("endpoints discover validate failure %s", err.Error())
		} else {
			clusterLoadAssignment = append(clusterLoadAssignment, cla)
		}
	}
	if len(clusterLoadAssignment) == 0 {
		logrus.Warn("create clusterLoadAssignment zero length")
	}
	return clusterLoadAssignment
}

func getEndpointsByLables(endpoints []*corev1.Endpoints, slabels map[string]string) (re []*corev1.Endpoints) {
	for _, en := range endpoints {
		existLength := 0
		for k, v := range slabels {
			v2, ok := en.Labels[k]
			if ok && v == v2 {
				existLength++
			}
		}
		if existLength == len(slabels) {
			re = append(re, en)
		}
	}
	return
}
