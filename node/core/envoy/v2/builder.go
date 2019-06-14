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
	"time"

	"github.com/Sirupsen/logrus"
	apiv2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/cluster"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	listener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	route "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	http_rate_limit "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/rate_limit/v2"
	http_connection_manager "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	tcp_proxy "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"
	configratelimit "github.com/envoyproxy/go-control-plane/envoy/config/ratelimit/v2"
	_type "github.com/envoyproxy/go-control-plane/envoy/type"
	"github.com/envoyproxy/go-control-plane/pkg/util"
	v1 "github.com/goodrain/rainbond/node/core/envoy/v1"
)

//DefaultLocalhostListenerAddress -
var DefaultLocalhostListenerAddress = "127.0.0.1"

//CreateTCPListener listener builder
func CreateTCPListener(name, clusterName, address, statPrefix string, port uint32) *apiv2.Listener {
	if address == "" {
		address = DefaultLocalhostListenerAddress
	}
	tcpProxy := &tcp_proxy.TcpProxy{
		StatPrefix: statPrefix,
		//todo:TcpProxy_WeightedClusters
		ClusterSpecifier: &tcp_proxy.TcpProxy_Cluster{
			Cluster: clusterName,
		},
	}
	listener := &apiv2.Listener{
		Name:    name,
		Address: CreateSocketAddress("tcp", address, port),
		FilterChains: []listener.FilterChain{
			listener.FilterChain{
				Filters: []listener.Filter{
					listener.Filter{
						Name: util.TCPProxy,
						ConfigType: &listener.Filter_Config{
							Config: MessageToStruct(tcpProxy),
						},
					},
				},
			},
		},
	}
	if err := listener.Validate(); err != nil {
		logrus.Errorf("validate listener config failure %s", err.Error())
		return nil
	}
	return listener
}

//RateLimitOptions rate limit options
type RateLimitOptions struct {
	Enable                bool
	Domain                string
	RateServerClusterName string
	Stage                 uint32
}

//DefaultRateLimitServerClusterName default rate limit server cluster name
var DefaultRateLimitServerClusterName = "rate_limit_service_cluster"

//CreateHTTPRateLimit create http rate limit
func CreateHTTPRateLimit(option RateLimitOptions) *http_rate_limit.RateLimit {
	httpRateLimit := &http_rate_limit.RateLimit{
		Domain: option.Domain,
		Stage:  option.Stage,
		RateLimitService: &configratelimit.RateLimitServiceConfig{
			GrpcService: &core.GrpcService{
				TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
					EnvoyGrpc: &core.GrpcService_EnvoyGrpc{
						ClusterName: option.RateServerClusterName,
					},
				},
			},
		},
	}
	if err := httpRateLimit.Validate(); err != nil {
		logrus.Errorf("create http rate limit failure %s", err.Error())
		return nil
	}
	logrus.Debugf("service http rate limit for domain %s", httpRateLimit.Domain)
	return httpRateLimit
}

//CreateHTTPConnectionManager create http connection manager
func CreateHTTPConnectionManager(name, statPrefix string, rateOpt *RateLimitOptions, routes ...route.VirtualHost) *http_connection_manager.HttpConnectionManager {
	var httpFilters []*http_connection_manager.HttpFilter
	if rateOpt != nil && rateOpt.Enable {
		httpFilters = append(httpFilters, &http_connection_manager.HttpFilter{
			Name: util.HTTPRateLimit,
			ConfigType: &http_connection_manager.HttpFilter_Config{
				Config: MessageToStruct(CreateHTTPRateLimit(*rateOpt)),
			},
		})
	}
	httpFilters = append(httpFilters, &http_connection_manager.HttpFilter{
		Name: util.Router,
	})
	hcm := &http_connection_manager.HttpConnectionManager{
		StatPrefix: statPrefix,
		RouteSpecifier: &http_connection_manager.HttpConnectionManager_RouteConfig{
			RouteConfig: &apiv2.RouteConfiguration{
				Name:         name,
				VirtualHosts: routes,
			},
		},
		HttpFilters: httpFilters,
	}
	if err := hcm.Validate(); err != nil {
		logrus.Errorf("validate http connertion manager config failure %s", err.Error())
		return nil
	}
	return hcm
}

//CreateHTTPListener create http manager listener
func CreateHTTPListener(name, address, statPrefix string, port uint32, rateOpt *RateLimitOptions, routes ...route.VirtualHost) *apiv2.Listener {
	hcm := CreateHTTPConnectionManager(name, statPrefix, rateOpt, routes...)
	if hcm == nil {
		logrus.Warningf("create http connection manager failure %s", name)
		return nil
	}
	listener := &apiv2.Listener{
		Name: name,
		Address: core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Protocol: core.TCP,
					Address:  address,
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
						Name: util.HTTPConnectionManager,
						ConfigType: &listener.Filter_Config{
							Config: MessageToStruct(hcm),
						},
					},
				},
			},
		},
	}
	if err := listener.Validate(); err != nil {
		logrus.Errorf("validate listener config failure %s", err.Error())
		return nil
	}
	return listener
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
				Address: address,
				PortSpecifier: &core.SocketAddress_PortValue{
					PortValue: port,
				},
			},
		},
	}
}

//CreateCircuitBreaker create down cluster circuitbreaker
func CreateCircuitBreaker(options RainbondPluginOptions) *cluster.CircuitBreakers {
	circuitBreakers := &cluster.CircuitBreakers{
		Thresholds: []*cluster.CircuitBreakers_Thresholds{
			&cluster.CircuitBreakers_Thresholds{
				MaxConnections:     ConversionUInt32(uint32(options.MaxConnections)),
				MaxRequests:        ConversionUInt32(uint32(options.MaxRequests)),
				MaxRetries:         ConversionUInt32(uint32(options.MaxActiveRetries)),
				MaxPendingRequests: ConversionUInt32(uint32(options.MaxPendingRequests)),
			},
		},
	}
	if err := circuitBreakers.Validate(); err != nil {
		logrus.Errorf("validate envoy config circuitBreakers failure %s", err.Error())
		return nil
	}
	return circuitBreakers
}

//CreatOutlierDetection create up cluster OutlierDetection
func CreatOutlierDetection(options RainbondPluginOptions) *cluster.OutlierDetection {
	outlierDetection := &cluster.OutlierDetection{
		Interval:           ConverTimeDuration(options.Interval),
		BaseEjectionTime:   ConverTimeDuration(options.BaseEjectionTimeMS / 1000),
		MaxEjectionPercent: ConversionUInt32(uint32(options.MaxEjectionPercent)),
		Consecutive_5Xx:    ConversionUInt32(uint32(options.ConsecutiveErrors)),
	}
	if err := outlierDetection.Validate(); err != nil {
		logrus.Errorf("validate envoy config outlierDetection failure %s", err.Error())
		return nil
	}
	return outlierDetection
}

//CreateRouteVirtualHost create route virtual host
func CreateRouteVirtualHost(name string, domains []string, rateLimits []*route.RateLimit, routes ...route.Route) *route.VirtualHost {
	pvh := &route.VirtualHost{
		Name:       name,
		Domains:    domains,
		Routes:     routes,
		RateLimits: rateLimits,
	}
	if err := pvh.Validate(); err != nil {
		logrus.Errorf("route virtualhost config validate failure %s domains %s", err.Error(), domains)
		return nil
	}
	return pvh
}

//CreateRoute create http route
func CreateRoute(clusterName, prefix string, headers []*route.HeaderMatcher, weight uint32) *route.Route {
	route := &route.Route{
		Match: route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{
				Prefix: prefix,
			},
			Headers: headers,
		},
		Action: &route.Route_Route{
			Route: &route.RouteAction{
				ClusterSpecifier: &route.RouteAction_WeightedClusters{
					WeightedClusters: &route.WeightedCluster{
						Clusters: []*route.WeightedCluster_ClusterWeight{
							&route.WeightedCluster_ClusterWeight{
								Name:   clusterName,
								Weight: ConversionUInt32(weight),
							},
						},
					},
				},
			},
		},
	}
	if err := route.Validate(); err != nil {
		logrus.Errorf("route http route config validate failure %s", err.Error())
		return nil
	}
	return route
}

//CreateHeaderMatcher create http route config header matcher
func CreateHeaderMatcher(header v1.Header) *route.HeaderMatcher {
	if header.Name == "" {
		return nil
	}
	headerMatcher := &route.HeaderMatcher{
		Name: header.Name,
		HeaderMatchSpecifier: &route.HeaderMatcher_PrefixMatch{
			PrefixMatch: header.Value,
		},
	}
	if err := headerMatcher.Validate(); err != nil {
		logrus.Errorf("route http header(%s) matcher config validate failure %s", header.Name, err.Error())
		return nil
	}
	return headerMatcher
}

//CreateEDSClusterConfig create grpc eds cluster config
func CreateEDSClusterConfig(serviceName string) *apiv2.Cluster_EdsClusterConfig {
	edsClusterConfig := &apiv2.Cluster_EdsClusterConfig{
		EdsConfig: &core.ConfigSource{
			ConfigSourceSpecifier: &core.ConfigSource_ApiConfigSource{
				ApiConfigSource: &core.ApiConfigSource{
					ApiType: core.ApiConfigSource_GRPC,
					GrpcServices: []*core.GrpcService{
						&core.GrpcService{
							TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
								EnvoyGrpc: &core.GrpcService_EnvoyGrpc{
									ClusterName: "rainbond_xds_cluster",
								},
							},
						},
					},
				},
			},
		},
		ServiceName: serviceName,
	}
	if err := edsClusterConfig.Validate(); err != nil {
		logrus.Errorf("validate eds cluster config failure %s", err.Error())
		return nil
	}
	return edsClusterConfig
}

//ClusterOptions cluster options
type ClusterOptions struct {
	Name                     string
	ServiceName              string
	ClusterType              apiv2.Cluster_DiscoveryType
	MaxRequestsPerConnection *uint32
	OutlierDetection         *cluster.OutlierDetection
	CircuitBreakers          *cluster.CircuitBreakers
	Hosts                    []*core.Address
	HealthyPanicThreshold    int64
}

//CreateCluster create cluster config
func CreateCluster(options ClusterOptions) *apiv2.Cluster {
	var edsClusterConfig *apiv2.Cluster_EdsClusterConfig
	if options.ClusterType == apiv2.Cluster_EDS {
		edsClusterConfig = CreateEDSClusterConfig(options.ServiceName)
		if edsClusterConfig == nil {
			logrus.Errorf("create eds cluster config failure")
			return nil
		}
	}
	cluster := &apiv2.Cluster{
		Name:             options.Name,
		Type:             options.ClusterType,
		ConnectTimeout:   time.Second * 250,
		LbPolicy:         apiv2.Cluster_ROUND_ROBIN,
		EdsClusterConfig: edsClusterConfig,
		Hosts:            options.Hosts,
		OutlierDetection: options.OutlierDetection,
		CircuitBreakers:  options.CircuitBreakers,
		CommonLbConfig: &apiv2.Cluster_CommonLbConfig{
			HealthyPanicThreshold: &_type.Percent{Value: float64(options.HealthyPanicThreshold) / 100},
		},
	}
	if options.MaxRequestsPerConnection != nil {
		cluster.MaxRequestsPerConnection = ConversionUInt32(*options.MaxRequestsPerConnection)
	}
	if err := cluster.Validate(); err != nil {
		logrus.Errorf("validate cluster config failure %s", err.Error())
		return nil
	}
	return cluster
}
