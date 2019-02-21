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

package v2

import (
	apiv2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/cluster"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	listener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	route "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	http_connection_manager "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	tcp_proxy "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"
)

var defaultListenerAddress = "127.0.0.1"

//CreateTCPListener listener builder
func CreateTCPListener(name, clusterName, address string, port uint32) apiv2.Listener {
	if address == "" {
		address = defaultListenerAddress
	}
	tcpProxy := &tcp_proxy.TcpProxy{
		StatPrefix: name,
		//todo:TcpProxy_WeightedClusters
		ClusterSpecifier: &tcp_proxy.TcpProxy_Cluster{
			Cluster: clusterName,
		},
	}
	return apiv2.Listener{
		Name:    name,
		Address: CreateSocketAddress("tcp", address, port),
		FilterChains: []listener.FilterChain{
			listener.FilterChain{
				Filters: []listener.Filter{
					listener.Filter{
						Name: "envoy.tcp_proxy",
						ConfigType: &listener.Filter_Config{
							Config: MessageToStruct(tcpProxy),
						},
					},
				},
			},
		},
	}
}

//CreateHTTPListener create http manager listener
func CreateHTTPListener(name, address string, port uint32, routes ...route.VirtualHost) apiv2.Listener {
	hcm := &http_connection_manager.HttpConnectionManager{
		StatPrefix: name,
		RouteSpecifier: &http_connection_manager.HttpConnectionManager_RouteConfig{
			RouteConfig: &apiv2.RouteConfiguration{
				Name:         name,
				VirtualHosts: routes,
			},
		},
		HttpFilters: []*http_connection_manager.HttpFilter{
			&http_connection_manager.HttpFilter{
				Name: "envoy.router",
			},
		},
	}
	return apiv2.Listener{
		Name: name,
		Address: core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Protocol: core.TCP,
					Address:  defaultListenerAddress,
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: port,
					},
				},
			},
		},

		FilterChains: []listener.FilterChain{
			listener.FilterChain{
				Filters: []listener.Filter{
					listener.Filter{
						Name: "envoy.http_connection_manager",
						ConfigType: &listener.Filter_Config{
							Config: MessageToStruct(hcm),
						},
					},
				},
			},
		},
	}
}

//CreateSocketAddress create socket address
func CreateSocketAddress(protocol, address string, port uint32) core.Address {
	return core.Address{
		Address: &core.Address_SocketAddress{
			SocketAddress: &core.SocketAddress{
				Protocol: func(protocol string) core.SocketAddress_Protocol {
					if protocol == "udp" {
						return core.UDP
					}
					return core.TCP
				}(protocol),
				Address: defaultListenerAddress,
				PortSpecifier: &core.SocketAddress_PortValue{
					PortValue: port,
				},
			},
		},
	}
}

//CreateCircuitBreaker create down cluster circuitbreaker
func CreateCircuitBreaker(options RainbondPluginOptions) *cluster.CircuitBreakers {
	return &cluster.CircuitBreakers{
		Thresholds: []*cluster.CircuitBreakers_Thresholds{
			&cluster.CircuitBreakers_Thresholds{
				MaxConnections:     ConversionUInt32(uint32(options.MaxConnections)),
				MaxRequests:        ConversionUInt32(uint32(options.MaxRequests)),
				MaxRetries:         ConversionUInt32(uint32(options.MaxActiveRetries)),
				MaxPendingRequests: ConversionUInt32(uint32(options.MaxPendingRequests)),
			},
		},
	}
}

//CreatOutlierDetection create up cluster OutlierDetection
func CreatOutlierDetection(options RainbondPluginOptions) *cluster.OutlierDetection {
	return &cluster.OutlierDetection{
		Interval:           ConverTimeDuration(options.IntervalMS / 1000),
		BaseEjectionTime:   ConverTimeDuration(options.BaseEjectionTimeMS / 1000),
		MaxEjectionPercent: ConversionUInt32(uint32(options.MaxEjectionPercent)),
		Consecutive_5Xx:    ConversionUInt32(uint32(options.ConsecutiveErrors)),
	}
}
