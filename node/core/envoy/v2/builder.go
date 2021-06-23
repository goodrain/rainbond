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
	"fmt"
	"strings"

	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/sirupsen/logrus"

	apiv2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	cluster "github.com/envoyproxy/go-control-plane/envoy/api/v2/cluster"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	envoy_api_v2_listener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	route "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	http_rate_limit "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/rate_limit/v2"
	http_connection_manager "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	tcp_proxy "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"
	envoy_config_filter_udp_udp_proxy_v2alpha "github.com/envoyproxy/go-control-plane/envoy/config/filter/udp/udp_proxy/v2alpha"
	configratelimit "github.com/envoyproxy/go-control-plane/envoy/config/ratelimit/v2"
	corev1 "k8s.io/api/core/v1"

	_type "github.com/envoyproxy/go-control-plane/envoy/type"

	v1 "github.com/goodrain/rainbond/node/core/envoy/v1"
)

//DefaultLocalhostListenerAddress -
var DefaultLocalhostListenerAddress = "127.0.0.1"

// DefaultLocalhostListenerPort -
var DefaultLocalhostListenerPort uint32 = 80

//CreateTCPListener listener builder
func CreateTCPListener(name, clusterName, address, statPrefix string, port uint32, idleTimeout int64) *apiv2.Listener {
	if address == "" {
		address = DefaultLocalhostListenerAddress
	}
	tcpProxy := &tcp_proxy.TcpProxy{
		StatPrefix: statPrefix,
		//todo:TcpProxy_WeightedClusters
		ClusterSpecifier: &tcp_proxy.TcpProxy_Cluster{
			Cluster: clusterName,
		},
		IdleTimeout: ConverTimeDuration(idleTimeout),
	}
	if err := tcpProxy.Validate(); err != nil {
		logrus.Errorf("validate listener tcp proxy config failure %s", err.Error())
		return nil
	}
	listener := &apiv2.Listener{
		Name:    name,
		Address: CreateSocketAddress("tcp", address, port),
		FilterChains: []*envoy_api_v2_listener.FilterChain{
			{
				Filters: []*envoy_api_v2_listener.Filter{
					{
						Name:       wellknown.TCPProxy,
						ConfigType: &envoy_api_v2_listener.Filter_TypedConfig{TypedConfig: Message2Any(tcpProxy)},
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

//CreateUDPListener create udp listenner
func CreateUDPListener(name, clusterName, address, statPrefix string, port uint32) *apiv2.Listener {
	if address == "" {
		address = DefaultLocalhostListenerAddress
	}
	config := &envoy_config_filter_udp_udp_proxy_v2alpha.UdpProxyConfig{
		StatPrefix: statPrefix,
		RouteSpecifier: &envoy_config_filter_udp_udp_proxy_v2alpha.UdpProxyConfig_Cluster{
			Cluster: clusterName,
		},
	}
	if err := config.Validate(); err != nil {
		logrus.Errorf("validate listener udp config failure %s", err.Error())
		return nil
	}
	anyConfig, err := ptypes.MarshalAny(config)
	if err != nil {
		logrus.Errorf("marshal any failure %s", err.Error())
		return nil
	}
	listener := &apiv2.Listener{
		Name:    name,
		Address: CreateSocketAddress("udp", address, port),
		ListenerFilters: []*envoy_api_v2_listener.ListenerFilter{
			{
				Name: "envoy.filters.udp_listener.udp_proxy",
				ConfigType: &envoy_api_v2_listener.ListenerFilter_TypedConfig{
					TypedConfig: anyConfig,
				},
			},
		},
		// Listening on UDP without SO_REUSEPORT socket option may result to unstable packet proxying. Consider configuring the reuse_port listener option.
		ReusePort: true,
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
func CreateHTTPConnectionManager(name, statPrefix string, rateOpt *RateLimitOptions, routes ...*route.VirtualHost) *http_connection_manager.HttpConnectionManager {
	var httpFilters []*http_connection_manager.HttpFilter
	if rateOpt != nil && rateOpt.Enable {
		httpFilters = append(httpFilters, &http_connection_manager.HttpFilter{
			Name: wellknown.HTTPRateLimit,
			ConfigType: &http_connection_manager.HttpFilter_Config{
				Config: MessageToStruct(CreateHTTPRateLimit(*rateOpt)),
			},
		})
	}
	httpFilters = append(httpFilters, &http_connection_manager.HttpFilter{
		Name: wellknown.Router,
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
func CreateHTTPListener(name, address, statPrefix string, port uint32, rateOpt *RateLimitOptions, routes ...*route.VirtualHost) *apiv2.Listener {
	hcm := CreateHTTPConnectionManager(name, statPrefix, rateOpt, routes...)
	if hcm == nil {
		logrus.Warningf("create http connection manager failure %s", name)
		return nil
	}
	listener := &apiv2.Listener{
		Name: name,
		Address: &core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Protocol: core.SocketAddress_TCP,
					Address:  address,
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: port,
					},
				},
			},
		},

		FilterChains: []*envoy_api_v2_listener.FilterChain{
			{
				Filters: []*envoy_api_v2_listener.Filter{
					{
						Name:       wellknown.HTTPConnectionManager,
						ConfigType: &envoy_api_v2_listener.Filter_TypedConfig{TypedConfig: Message2Any(hcm)},
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
func CreateSocketAddress(protocol, address string, port uint32) *core.Address {
	if strings.HasPrefix(address, "https://") {
		address = strings.Split(address, "https://")[1]
	}
	if strings.HasPrefix(address, "http://") {
		address = strings.Split(address, "http://")[1]
	}
	return &core.Address{
		Address: &core.Address_SocketAddress{
			SocketAddress: &core.SocketAddress{
				Protocol: func(protocol string) core.SocketAddress_Protocol {
					if protocol == "udp" {
						return core.SocketAddress_UDP
					}
					return core.SocketAddress_TCP
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
			{
				Priority:           core.RoutingPriority_DEFAULT,
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
func CreateRouteVirtualHost(name string, domains []string, rateLimits []*route.RateLimit, routes ...*route.Route) *route.VirtualHost {
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

//CreateRouteWithHostRewrite create route with hostRewrite
func CreateRouteWithHostRewrite(host, clusterName, prefix string, headers []*route.HeaderMatcher, weight uint32) *route.Route {
	var rout *route.Route
	if host != "" {
		var hostRewriteSpecifier *route.RouteAction_HostRewrite
		var clusterSpecifier *route.RouteAction_Cluster
		if strings.HasPrefix(host, "https://") {
			host = strings.Split(host, "https://")[1]
		}
		if strings.HasPrefix(host, "http://") {
			host = strings.Split(host, "http://")[1]
		}
		hostRewriteSpecifier = &route.RouteAction_HostRewrite{
			HostRewrite: host,
		}
		clusterSpecifier = &route.RouteAction_Cluster{
			Cluster: clusterName,
		}
		rout = &route.Route{
			Match: &route.RouteMatch{
				PathSpecifier: &route.RouteMatch_Prefix{
					Prefix: prefix,
				},
				Headers: headers,
			},
			Action: &route.Route_Route{
				Route: &route.RouteAction{
					ClusterSpecifier:     clusterSpecifier,
					Priority:             core.RoutingPriority_DEFAULT,
					HostRewriteSpecifier: hostRewriteSpecifier,
				},
			},
		}
		if err := rout.Validate(); err != nil {
			logrus.Errorf("route http route config validate failure %s", err.Error())
			return nil
		}

	}
	return rout
}

//CreateRoute create http route
func CreateRoute(clusterName, prefix string, headers []*route.HeaderMatcher, weight uint32) *route.Route {
	rout := &route.Route{
		Match: &route.RouteMatch{
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
							{
								Name:   clusterName,
								Weight: ConversionUInt32(weight),
							},
						},
					},
				},
				Priority: core.RoutingPriority_DEFAULT,
			},
		},
	}

	if err := rout.Validate(); err != nil {
		logrus.Errorf("route http route config validate failure %s", err.Error())
		return nil
	}
	return rout
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
						{
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
	ConnectionTimeout        *duration.Duration
	ClusterType              apiv2.Cluster_DiscoveryType
	MaxRequestsPerConnection *uint32
	OutlierDetection         *cluster.OutlierDetection
	CircuitBreakers          *cluster.CircuitBreakers
	Hosts                    []*core.Address
	HealthyPanicThreshold    int64
	TransportSocket          *core.TransportSocket
	LoadAssignment           *apiv2.ClusterLoadAssignment
	Protocol                 string
	// grpc service name of health check
	GrpcHealthServiceName string
	//health check
	HealthTimeout  int64
	HealthInterval int64
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
		Name:                 options.Name,
		ClusterDiscoveryType: &apiv2.Cluster_Type{Type: options.ClusterType},
		ConnectTimeout:       options.ConnectionTimeout,
		LbPolicy:             apiv2.Cluster_ROUND_ROBIN,
		EdsClusterConfig:     edsClusterConfig,
		Hosts:                options.Hosts,
		OutlierDetection:     options.OutlierDetection,
		CircuitBreakers:      options.CircuitBreakers,
		CommonLbConfig: &apiv2.Cluster_CommonLbConfig{
			HealthyPanicThreshold: &_type.Percent{Value: float64(options.HealthyPanicThreshold) / 100},
		},
	}
	if options.Protocol == "http2" || options.Protocol == "grpc" {
		cluster.Http2ProtocolOptions = &core.Http2ProtocolOptions{}
		// set grpc health check
		if options.Protocol == "grpc" && options.GrpcHealthServiceName != "" {
			cluster.HealthChecks = append(cluster.HealthChecks, &core.HealthCheck{
				Timeout:  ConverTimeDuration(options.HealthTimeout),
				Interval: ConverTimeDuration(options.HealthInterval),
				//The number of unhealthy health checks required before a host is marked unhealthy.
				//Note that for http health checking if a host responds with 503 this threshold is ignored and the host is considered unhealthy immediately.
				UnhealthyThreshold: ConversionUInt32(2),
				//The number of healthy health checks required before a host is marked healthy.
				//Note that during startup, only a single successful health check is required to mark a host healthy.
				HealthyThreshold: ConversionUInt32(1),
				HealthChecker: &core.HealthCheck_GrpcHealthCheck_{
					GrpcHealthCheck: &core.HealthCheck_GrpcHealthCheck{
						ServiceName: options.GrpcHealthServiceName,
					},
				}})
		}
	}
	if options.TransportSocket != nil {
		cluster.TransportSocket = options.TransportSocket
	}
	if options.LoadAssignment != nil {
		cluster.LoadAssignment = options.LoadAssignment
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

//GetServiceAliasByService get service alias from k8s service
func GetServiceAliasByService(service *corev1.Service) string {
	//v5.1 and later
	if serviceAlias, ok := service.Labels["service_alias"]; ok {
		return serviceAlias
	}
	//version before v5.1
	if serviceAlias, ok := service.Spec.Selector["name"]; ok {
		return serviceAlias
	}
	return ""
}

//CreateDNSLoadAssignment create dns loadAssignment
func CreateDNSLoadAssignment(serviceAlias, namespace, domain string, service *corev1.Service) *apiv2.ClusterLoadAssignment {
	destServiceAlias := GetServiceAliasByService(service)
	if destServiceAlias == "" {
		logrus.Errorf("service alias is empty in k8s service %s", service.Name)
		return nil
	}

	clusterName := fmt.Sprintf("%s_%s_%s_%d", namespace, serviceAlias, destServiceAlias, service.Spec.Ports[0].Port)
	var lendpoints []*endpoint.LocalityLbEndpoints
	protocol := service.Labels["port_protocol"]
	port := service.Spec.Ports[0].Port
	var lbe []*endpoint.LbEndpoint
	envoyAddress := CreateSocketAddress(protocol, domain, uint32(port))
	lbe = append(lbe, &endpoint.LbEndpoint{
		HostIdentifier: &endpoint.LbEndpoint_Endpoint{
			Endpoint: &endpoint.Endpoint{
				Address:           envoyAddress,
				HealthCheckConfig: &endpoint.Endpoint_HealthCheckConfig{PortValue: uint32(port)},
			},
		},
	})
	lendpoints = append(lendpoints, &endpoint.LocalityLbEndpoints{LbEndpoints: lbe})
	cla := &apiv2.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints:   lendpoints,
	}
	if err := cla.Validate(); err != nil {
		logrus.Errorf("endpoints discover validate failure %s", err.Error())
	}

	return cla
}
