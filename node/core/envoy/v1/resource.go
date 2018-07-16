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

package v1

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/duration"
)

const (
	// DefaultAccessLog is the name of the log channel (stdout in docker environment)
	DefaultAccessLog = "/dev/stdout"

	// DefaultLbType defines the default load balancer policy
	DefaultLbType = LbTypeRoundRobin

	// LDSName is the name of listener-discovery-service (LDS) cluster
	LDSName = "lds"

	// RDSName is the name of route-discovery-service (RDS) cluster
	RDSName = "rds"

	// SDSName is the name of service-discovery-service (SDS) cluster
	SDSName = "sds"

	// CDSName is the name of cluster-discovery-service (CDS) cluster
	CDSName = "cds"

	// RDSAll is the special name for HTTP PROXY route
	RDSAll = "http_proxy"

	// VirtualListenerName is the name for traffic capture listener
	VirtualListenerName = "virtual"

	// ClusterTypeStrictDNS name for clusters of type 'strict_dns'
	ClusterTypeStrictDNS = "strict_dns"

	// ClusterTypeStatic name for clusters of type 'static'
	ClusterTypeStatic = "static"

	// ClusterTypeOriginalDST name for clusters of type 'original_dst'
	ClusterTypeOriginalDST = "original_dst"

	// ClusterTypeSDS name for clusters of type 'sds'
	ClusterTypeSDS = "sds"

	// LbTypeRoundRobin is the name for round-robin LB
	LbTypeRoundRobin = "round_robin"

	// LbTypeLeastRequest is the name for least request LB
	LbTypeLeastRequest = "least_request"

	// LbTypeRingHash is the name for ring hash LB
	LbTypeRingHash = "ring_hash"

	// LbTypeRandom is the name for random LB
	LbTypeRandom = "random"

	// LbTypeOriginalDST is the name for LB of original_dst
	LbTypeOriginalDST = "original_dst_lb"

	// ClusterFeatureHTTP2 is the feature to use HTTP/2 for a cluster
	ClusterFeatureHTTP2 = "http2"

	// HTTPConnectionManager is the name of HTTP filter.
	HTTPConnectionManager = "http_connection_manager"

	// TCPProxyFilter is the name of the TCP Proxy network filter.
	TCPProxyFilter = "tcp_proxy"

	// CORSFilter is the name of the CORS network filter
	CORSFilter = "cors"

	// MongoProxyFilter is the name of the Mongo Proxy network filter.
	MongoProxyFilter = "mongo_proxy"

	// RedisProxyFilter is the name of the Redis Proxy network filter.
	RedisProxyFilter = "redis_proxy"

	// RedisDefaultOpTimeout is the op timeout used for Redis Proxy filter
	// Currently it is set to 30s (conversion happens in the filter)
	// TODO - Allow this to be configured.
	RedisDefaultOpTimeout = 30 * time.Second

	// WildcardAddress binds to all IP addresses
	WildcardAddress = "0.0.0.0"

	// LocalhostAddress for local binding
	LocalhostAddress = "127.0.0.1"

	// EgressTraceOperation denotes the name of trace operation for Envoy
	EgressTraceOperation = "egress"

	// IngressTraceOperation denotes the name of trace operation for Envoy
	IngressTraceOperation = "ingress"

	// ZipkinTraceDriverType denotes the Zipkin HTTP trace driver
	ZipkinTraceDriverType = "zipkin"

	// ZipkinCollectorCluster denotes the cluster where zipkin server is running
	ZipkinCollectorCluster = "zipkin"

	// ZipkinCollectorEndpoint denotes the REST endpoint where Envoy posts Zipkin spans
	ZipkinCollectorEndpoint = "/api/v1/spans"

	// MaxClusterNameLength is the maximum cluster name length
	MaxClusterNameLength = 189 // TODO: use MeshConfig.StatNameLength instead

	// Headers with special meaning in Envoy

	// HeaderMethod is the method header.
	HeaderMethod = ":method"
	// HeaderAuthority is the authority header.
	HeaderAuthority = ":authority"
	// HeaderScheme is the scheme header.
	HeaderScheme = ":scheme"
	// MixerFilter name and its attributes
	MixerFilter = "mixer"

	router  = "router"
	auto    = "auto"
	decoder = "decoder"
	read    = "read"
	both    = "both"
)

var (
	// ValidateClusters is an environment variable that can be set to false to disable
	// cluster validation in RDS, in case problems are discovered.
	ValidateClusters = true
)

// ListenersALPNProtocols denotes the the list of ALPN protocols that the listener
// should expose
var ListenersALPNProtocols = []string{"h2", "http/1.1"}

// convertDuration converts to golang duration and logs errors
func convertDuration(d *duration.Duration) time.Duration {
	if d == nil {
		return 0
	}
	dur, err := ptypes.Duration(d)
	if err != nil {
		logrus.Warnf("error converting duration %#v, using 0: %v", d, err)
	}
	return dur
}

func protoDurationToMS(dur *duration.Duration) int64 {
	return int64(convertDuration(dur) / time.Millisecond)
}

// Config defines the schema for Envoy JSON configuration format
type Config struct {
	RootRuntime        *RootRuntime   `json:"runtime,omitempty"`
	Listeners          Listeners      `json:"listeners"`
	LDS                *LDSCluster    `json:"lds,omitempty"`
	Admin              Admin          `json:"admin"`
	ClusterManager     ClusterManager `json:"cluster_manager"`
	StatsdUDPIPAddress string         `json:"statsd_udp_ip_address,omitempty"`
	Tracing            *Tracing       `json:"tracing,omitempty"`

	// Special value used to hash all referenced values (e.g. TLS secrets)
	Hash []byte `json:"-"`
}

// Tracing definition
type Tracing struct {
	HTTPTracer HTTPTracer `json:"http"`
}

// HTTPTracer definition
type HTTPTracer struct {
	HTTPTraceDriver HTTPTraceDriver `json:"driver"`
}

// HTTPTraceDriver definition
type HTTPTraceDriver struct {
	HTTPTraceDriverType   string                `json:"type"`
	HTTPTraceDriverConfig HTTPTraceDriverConfig `json:"config"`
}

// HTTPTraceDriverConfig definition
type HTTPTraceDriverConfig struct {
	CollectorCluster  string `json:"collector_cluster"`
	CollectorEndpoint string `json:"collector_endpoint"`
}

// RootRuntime definition.
// See https://envoyproxy.github.io/envoy/configuration/overview/overview.html
type RootRuntime struct {
	SymlinkRoot          string `json:"symlink_root"`
	Subdirectory         string `json:"subdirectory"`
	OverrideSubdirectory string `json:"override_subdirectory,omitempty"`
}

// AbortFilter definition
type AbortFilter struct {
	Percent    int `json:"abort_percent,omitempty"`
	HTTPStatus int `json:"http_status,omitempty"`
}

// DelayFilter definition
type DelayFilter struct {
	Type     string `json:"type,omitempty"`
	Percent  int    `json:"fixed_delay_percent,omitempty"`
	Duration int64  `json:"fixed_duration_ms,omitempty"`
}

// AppendedHeader definition
type AppendedHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Header definition
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Regex bool   `json:"regex,omitempty"`
}

// FilterFaultConfig definition
type FilterFaultConfig struct {
	Abort           *AbortFilter `json:"abort,omitempty"`
	Delay           *DelayFilter `json:"delay,omitempty"`
	Headers         Headers      `json:"headers,omitempty"`
	UpstreamCluster string       `json:"upstream_cluster,omitempty"`
}

// FilterRouterConfig definition
type FilterRouterConfig struct {
	// DynamicStats defaults to true
	DynamicStats bool `json:"dynamic_stats,omitempty"`
}

// HTTPFilter definition
type HTTPFilter struct {
	Type   string      `json:"type"`
	Name   string      `json:"name"`
	Config interface{} `json:"config"`
}

// Runtime definition
type Runtime struct {
	Key     string `json:"key"`
	Default int    `json:"default"`
}

// Decorator definition
type Decorator struct {
	Operation string `json:"operation"`
}

// HTTPRoute definition
type HTTPRoute struct {
	Runtime *Runtime `json:"runtime,omitempty"`

	Path   string `json:"path,omitempty"`
	Prefix string `json:"prefix,omitempty"`
	Regex  string `json:"regex,omitempty"`

	PrefixRewrite string `json:"prefix_rewrite,omitempty"`
	HostRewrite   string `json:"host_rewrite,omitempty"`

	PathRedirect string `json:"path_redirect,omitempty"`
	HostRedirect string `json:"host_redirect,omitempty"`

	Cluster          string           `json:"cluster,omitempty"`
	WeightedClusters *WeightedCluster `json:"weighted_clusters,omitempty"`

	Headers      Headers           `json:"headers,omitempty"`
	TimeoutMS    int64             `json:"timeout_ms"`
	RetryPolicy  *RetryPolicy      `json:"retry_policy,omitempty"`
	OpaqueConfig map[string]string `json:"opaque_config,omitempty"`

	AutoHostRewrite  bool `json:"auto_host_rewrite,omitempty"`
	WebsocketUpgrade bool `json:"use_websocket,omitempty"`

	ShadowCluster *ShadowCluster `json:"shadow,omitempty"`

	HeadersToAdd []AppendedHeader `json:"request_headers_to_add,omitempty"`

	CORSPolicy *CORSPolicy `json:"cors,omitempty"`

	Decorator *Decorator `json:"decorator,omitempty"`

	// clusters contains the set of referenced clusters in the route; the field is special
	// and used only to aggregate cluster information after composing routes
	Clusters Clusters `json:"-"`

	// faults contains the set of referenced faults in the route; the field is special
	// and used only to aggregate fault filter information after composing routes
	faults []*HTTPFilter
}

// Redirect returns true if route contains redirect logic
func (route *HTTPRoute) Redirect() bool {
	return route.HostRedirect != "" || route.PathRedirect != ""
}

// CatchAll returns true if the route matches all requests
func (route *HTTPRoute) CatchAll() bool {
	return len(route.Headers) == 0 && route.Path == "" && route.Prefix == "/"
}

//BasicHash returns hash string by route path\prefix\header
func (route *HTTPRoute) BasicHash() string {
	key := sha256.New()
	var header string
	sort.Sort(route.Headers)
	for _, h := range route.Headers {
		header += h.Name + h.Value
	}
	key.Write([]byte(route.Path + route.Prefix + header))
	return string(key.Sum(nil))
}

// CombinePathPrefix checks that the route applies for a given path and prefix
// match and updates the path and the prefix in the route. If the route is
// incompatible with the path or the prefix, returns nil.  Either path or
// prefix must be set but not both.  The resulting route must match exactly the
// requests that match both the original route and the supplied path and
// prefix.
func (route *HTTPRoute) CombinePathPrefix(path, prefix string) *HTTPRoute {
	switch {
	case path == "" && route.Path == "" && strings.HasPrefix(route.Prefix, prefix):
		// pick the longest prefix if both are prefix matches
		return route
	case path == "" && route.Path == "" && strings.HasPrefix(prefix, route.Prefix):
		route.Prefix = prefix
		return route
	case prefix == "" && route.Prefix == "" && route.Path == path:
		// pick only if path matches if both are path matches
		return route
	case path == "" && route.Prefix == "" && strings.HasPrefix(route.Path, prefix):
		// if mixed, pick if route path satisfies the prefix
		return route
	case prefix == "" && route.Path == "" && strings.HasPrefix(path, route.Prefix):
		// if mixed, pick if route prefix satisfies the path and change route to path
		route.Path = path
		route.Prefix = ""
		return route
	default:
		return nil
	}
}

// CORSPolicy definition
// See: https://www.envoyproxy.io/envoy/configuration/http_filters/cors_filter.html#config-http-filters-cors
type CORSPolicy struct {
	Enabled          bool     `json:"enabled,omitempty"`
	AllowCredentials bool     `json:"allow_credentials,omitempty"`
	AllowMethods     string   `json:"allow_methods,omitempty"`
	AllowHeaders     string   `json:"allow_headers,omitempty"`
	ExposeHeaders    string   `json:"expose_headers,omitempty"`
	MaxAge           int      `json:"max_age,string,omitempty"`
	AllowOrigin      []string `json:"allow_origin,omitempty"`
}

// RetryPolicy definition
// See: https://lyft.github.io/envoy/docs/configuration/http_conn_man/route_config/route.html#retry-policy
type RetryPolicy struct {
	Policy          string `json:"retry_on"` //if unset, set to 5xx,connect-failure,refused-stream
	NumRetries      int    `json:"num_retries,omitempty"`
	PerTryTimeoutMS int64  `json:"per_try_timeout_ms,omitempty"`
}

// ShadowCluster definition
// See: https://www.envoyproxy.io/envoy/configuration/http_conn_man/route_config/route.html?
// highlight=shadow#config-http-conn-man-route-table-route-shadow
type ShadowCluster struct {
	Cluster string `json:"cluster"`
}

// WeightedCluster definition
// See https://envoyproxy.github.io/envoy/configuration/http_conn_man/route_config/route.html
type WeightedCluster struct {
	Clusters         []*WeightedClusterEntry `json:"clusters"`
	RuntimeKeyPrefix string                  `json:"runtime_key_prefix,omitempty"`
}

// WeightedClusterEntry definition. Describes the format of each entry in the WeightedCluster
type WeightedClusterEntry struct {
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}

// VirtualHost definition
type VirtualHost struct {
	Name    string       `json:"name"`
	Domains []string     `json:"domains"`
	Routes  []*HTTPRoute `json:"routes"`
}

func (host *VirtualHost) clusters() Clusters {
	out := make(Clusters, 0)
	for _, route := range host.Routes {
		out = append(out, route.Clusters...)
	}
	return out
}

//UniqVirtualHost according to the rules of VirtualHost in http route
//merge the VirtualHost that have same domain
//if have same domain, prifix, path and header,support weight
func UniqVirtualHost(vhs []*VirtualHost) (revhs []*VirtualHost) {
	var domains = make(map[string]*VirtualHost, 0)
	for _, vh := range vhs {
		for _, domain := range vh.Domains {
			if cahcevh, ok := domains[domain]; ok {
				cahcevh.Routes = append(cahcevh.Routes, vh.Routes...)
			} else {
				domains[domain] = vh
			}
		}
	}
	for _, v := range domains {
		//supprot weight if have same prifix, path and header
		var keys = make(map[string]*HTTPRoute, 0)
		for _, route := range v.Routes {
			key := route.BasicHash()
			if cacheroute, ok := keys[key]; ok {
				cacheroute.WeightedClusters.Clusters = append(cacheroute.WeightedClusters.Clusters, route.WeightedClusters.Clusters...)
			} else {
				keys[key] = route
			}
		}
		var routes []*HTTPRoute
		for _, v := range keys {
			var total int
			var i int
			for i = 0; i < len(v.WeightedClusters.Clusters)-1; i++ {
				total += v.WeightedClusters.Clusters[i].Weight
				if total >= 100 {
					break
				}
			}
			if total > 100 {
				v.WeightedClusters.Clusters[i].Weight = v.WeightedClusters.Clusters[i].Weight - (total - 100)
				if i+1 < len(v.WeightedClusters.Clusters) {
					for j := i + 1; j < len(v.WeightedClusters.Clusters); j++ {
						v.WeightedClusters.Clusters[j].Weight = 0
					}
				}
			}
			if total == 100 && i+1 < len(v.WeightedClusters.Clusters) {
				for j := i + 1; j < len(v.WeightedClusters.Clusters); j++ {
					v.WeightedClusters.Clusters[j].Weight = 0
				}
			}
			if total < 100 {
				v.WeightedClusters.Clusters[i].Weight = 100 - total
			}
			routes = append(routes, v)
		}
		v.Routes = routes
		revhs = append(revhs, v)
	}
	return
}

// HTTPRouteConfig definition
type HTTPRouteConfig struct {
	ValidateClusters bool           `json:"validate_clusters"`
	VirtualHosts     []*VirtualHost `json:"virtual_hosts"`
}

// HTTPRouteConfigs is a map from the port number to the route config
type HTTPRouteConfigs map[int]*HTTPRouteConfig

// EnsurePort creates a route config if necessary
func (routes HTTPRouteConfigs) EnsurePort(port int) *HTTPRouteConfig {
	config, ok := routes[port]
	if !ok {
		config = &HTTPRouteConfig{ValidateClusters: ValidateClusters}
		routes[port] = config
	}
	return config
}

// Clusters returns the clusters corresponding to the given routes.
func (routes HTTPRouteConfigs) Clusters() Clusters {
	out := make(Clusters, 0)
	for _, config := range routes {
		out = append(out, config.Clusters()...)
	}
	return out
}

// Normalize normalizes the route configs.
func (routes HTTPRouteConfigs) Normalize() HTTPRouteConfigs {
	out := make(HTTPRouteConfigs)

	// sort HTTP routes by virtual hosts, rest should be deterministic
	for port, routeConfig := range routes {
		out[port] = routeConfig.Normalize()
	}

	return out
}

// Combine creates a new route config that is the union of all HTTP routes.
// note that the virtual hosts without an explicit port suffix (IP:PORT) are stripped
// for all routes except the route for port 80.
func (routes HTTPRouteConfigs) Combine() *HTTPRouteConfig {
	out := &HTTPRouteConfig{ValidateClusters: ValidateClusters}
	for port, config := range routes {
		for _, host := range config.VirtualHosts {
			vhost := &VirtualHost{
				Name:   host.Name,
				Routes: host.Routes,
			}
			for _, domain := range host.Domains {
				if port == 80 || strings.Contains(domain, ":") {
					vhost.Domains = append(vhost.Domains, domain)
				}
			}

			if len(vhost.Domains) > 0 {
				out.VirtualHosts = append(out.VirtualHosts, vhost)
			}
		}
	}
	return out.Normalize()
}

// faults aggregates fault filters across virtual hosts in single http_conn_man
func (rc *HTTPRouteConfig) faults() []*HTTPFilter {
	out := make([]*HTTPFilter, 0)
	for _, host := range rc.VirtualHosts {
		for _, route := range host.Routes {
			out = append(out, route.faults...)
		}
	}
	return out
}

// Clusters returns the clusters for the given route config.
func (rc *HTTPRouteConfig) Clusters() Clusters {
	out := make(Clusters, 0)
	for _, host := range rc.VirtualHosts {
		out = append(out, host.clusters()...)
	}
	return out
}

// Normalize normalizes the route config.
func (rc *HTTPRouteConfig) Normalize() *HTTPRouteConfig {
	hosts := make([]*VirtualHost, len(rc.VirtualHosts))
	copy(hosts, rc.VirtualHosts)
	sort.Slice(hosts, func(i, j int) bool { return hosts[i].Name < hosts[j].Name })
	return &HTTPRouteConfig{ValidateClusters: ValidateClusters, VirtualHosts: hosts}
}

// AccessLog definition.
type AccessLog struct {
	Path   string `json:"path"`
	Format string `json:"format,omitempty"`
	Filter string `json:"filter,omitempty"`
}

// HTTPFilterConfig definition
type HTTPFilterConfig struct {
	CodecType         string                 `json:"codec_type"`
	StatPrefix        string                 `json:"stat_prefix"`
	GenerateRequestID bool                   `json:"generate_request_id,omitempty"`
	UseRemoteAddress  bool                   `json:"use_remote_address,omitempty"`
	Tracing           *HTTPFilterTraceConfig `json:"tracing,omitempty"`
	RouteConfig       *HTTPRouteConfig       `json:"route_config,omitempty"`
	RDS               *RDS                   `json:"rds,omitempty"`
	Filters           []HTTPFilter           `json:"filters"`
	AccessLog         []AccessLog            `json:"access_log,omitempty"`
}

// IsNetworkFilterConfig marks HTTPFilterConfig as an implementation of NetworkFilterConfig
func (*HTTPFilterConfig) IsNetworkFilterConfig() {}

// HTTPFilterTraceConfig definition
type HTTPFilterTraceConfig struct {
	OperationName string `json:"operation_name"`
}

// TCPRoute definition
type TCPRoute struct {
	Cluster           string   `json:"cluster"`
	DestinationIPList []string `json:"destination_ip_list,omitempty"`
	DestinationPorts  string   `json:"destination_ports,omitempty"`
	SourceIPList      []string `json:"source_ip_list,omitempty"`
	SourcePorts       string   `json:"source_ports,omitempty"`

	// special value to retain dependent cluster definition for TCP routes.
	clusterRef *Cluster
}

// TCPRouteByRoute sorts TCP routes over all route sub fields.
type TCPRouteByRoute []*TCPRoute

func (r TCPRouteByRoute) Len() int {
	return len(r)
}

func (r TCPRouteByRoute) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r TCPRouteByRoute) Less(i, j int) bool {
	if r[i].Cluster != r[j].Cluster {
		return r[i].Cluster < r[j].Cluster
	}

	compare := func(a, b []string) bool {
		lenA, lenB := len(a), len(b)
		min := lenA
		if min > lenB {
			min = lenB
		}
		for k := 0; k < min; k++ {
			if a[k] != b[k] {
				return a[k] < b[k]
			}
		}
		return lenA < lenB
	}

	if less := compare(r[i].DestinationIPList, r[j].DestinationIPList); less {
		return less
	}
	if r[i].DestinationPorts != r[j].DestinationPorts {
		return r[i].DestinationPorts < r[j].DestinationPorts
	}
	if less := compare(r[i].SourceIPList, r[j].SourceIPList); less {
		return less
	}
	if r[i].SourcePorts != r[j].SourcePorts {
		return r[i].SourcePorts < r[j].SourcePorts
	}
	return false
}

// TCPProxyFilterConfig definition
type TCPProxyFilterConfig struct {
	StatPrefix  string          `json:"stat_prefix"`
	RouteConfig *TCPRouteConfig `json:"route_config"`
}

// IsNetworkFilterConfig marks TCPProxyFilterConfig as an implementation of NetworkFilterConfig
func (*TCPProxyFilterConfig) IsNetworkFilterConfig() {}

// TCPRouteConfig (or generalize as RouteConfig or L4RouteConfig for TCP/UDP?)
type TCPRouteConfig struct {
	Routes []*TCPRoute `json:"routes"`
}

// MongoProxyFilterConfig definition
type MongoProxyFilterConfig struct {
	StatPrefix string `json:"stat_prefix"`
}

// IsNetworkFilterConfig marks MongoProxyFilterConfig as an implementation of NetworkFilterConfig
func (*MongoProxyFilterConfig) IsNetworkFilterConfig() {}

// CORSFilterConfig definition
// See: https://www.envoyproxy.io/envoy/configuration/http_filters/cors_filter.html#config-http-filters-cors
type CORSFilterConfig struct{}

// IsNetworkFilterConfig marks CORSFilterConfig as an implementation of NetworkFilterConfig
func (*CORSFilterConfig) IsNetworkFilterConfig() {}

// RedisConnPool definition
type RedisConnPool struct {
	OperationTimeoutMS int64 `json:"op_timeout_ms"`
}

// RedisProxyFilterConfig definition
type RedisProxyFilterConfig struct {
	ClusterName string         `json:"cluster_name"`
	ConnPool    *RedisConnPool `json:"conn_pool"`
	StatPrefix  string         `json:"stat_prefix"`
}

// IsNetworkFilterConfig marks RedisProxyFilterConfig as an implementation of NetworkFilterConfig
func (*RedisProxyFilterConfig) IsNetworkFilterConfig() {}

// NetworkFilter definition
type NetworkFilter struct {
	Type   string              `json:"-"`
	Name   string              `json:"name"`
	Config NetworkFilterConfig `json:"config"`
}

// NetworkFilterConfig is a marker interface
type NetworkFilterConfig interface {
	IsNetworkFilterConfig()
}

// NetworkFilterTypes maps filter names to types of structs that implement them. It is used when unmarshaling JSON data.
// To add your own NetworkFilter types, add additional entries to this map prior to calling json.Unmarshal.
var NetworkFilterTypes = map[string]reflect.Type{
	RedisProxyFilter:      reflect.TypeOf(RedisProxyFilterConfig{}),
	CORSFilter:            reflect.TypeOf(CORSFilterConfig{}),
	MongoProxyFilter:      reflect.TypeOf(MongoProxyFilterConfig{}),
	TCPProxyFilter:        reflect.TypeOf(TCPProxyFilterConfig{}),
	HTTPConnectionManager: reflect.TypeOf(HTTPFilterConfig{}),
	MixerFilter:           reflect.TypeOf(FilterMixerConfig{}),
}

// UnmarshalJSON handles custom unmarshal logic for the NetworkFilter struct. This is needed because the config field
// depends on the filter name.
func (nf *NetworkFilter) UnmarshalJSON(b []byte) error {

	// First, unmarshal to a generic data structure so we can get the name.
	var j interface{}
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	m := j.(map[string]interface{})
	n, ok := m["name"].(string)
	if !ok {
		return errors.New("filter missing name field")
	}

	// Once we have the name, we can look up the concrete type of the config field.
	t, ok := NetworkFilterTypes[n]
	if !ok {
		return fmt.Errorf("unknown filter name: %s", n)
	}
	v := reflect.New(t)

	// Since Unmarshal takes a []byte, we re-marshall the config and then call Unmarshal on it.
	cfgBytes, err := json.Marshal(m["config"])
	if err != nil {
		return err
	}
	err = json.Unmarshal(cfgBytes, v.Interface())
	if err != nil {
		return err
	}

	// Fill in the NetworkFilter
	nf.Name = n
	nf.Type = m["type"].(string)
	nf.Config = v.Interface().(NetworkFilterConfig)
	return nil
}

// Listener definition
type Listener struct {
	Address        string           `json:"address"`
	Name           string           `json:"name,omitempty"`
	Filters        []*NetworkFilter `json:"filters"`
	SSLContext     *SSLContext      `json:"ssl_context,omitempty"`
	BindToPort     bool             `json:"bind_to_port"`
	UseOriginalDst bool             `json:"use_original_dst,omitempty"`
}

// Listeners is a collection of listeners
type Listeners []*Listener

//Append append some listeners
func (l *Listeners) Append(new Listeners) {
	*l = append(*l, new...)
}

//CreateHTTPCommonListener create simple http common listener
//listen port 80
func CreateHTTPCommonListener(name string, vh ...*VirtualHost) *Listener {
	rcg := &HTTPRouteConfig{
		VirtualHosts: vh,
	}
	hsf := HTTPFilter{
		Type:   "decoder",
		Name:   "router",
		Config: make(map[string]string),
	}
	lhc := &HTTPFilterConfig{
		CodecType:   "auto",
		StatPrefix:  "ingress_http",
		RouteConfig: rcg,
		Filters:     []HTTPFilter{hsf},
	}
	lfs := &NetworkFilter{
		Name:   "http_connection_manager",
		Config: lhc,
	}
	plds := &Listener{
		Name:       name,
		Address:    fmt.Sprintf("tcp://127.0.0.1:%d", 80),
		Filters:    []*NetworkFilter{lfs},
		BindToPort: true,
	}
	return plds
}

//CreateTCPCommonListener create tcp simple common listener
//listen the specified port
//associate the specified cluster.
func CreateTCPCommonListener(listenerName, clusterName string, address string) *Listener {
	ptr := &TCPRoute{
		Cluster: clusterName,
	}
	lrs := &TCPRouteConfig{
		Routes: []*TCPRoute{ptr},
	}
	lcg := &TCPProxyFilterConfig{
		StatPrefix:  listenerName,
		RouteConfig: lrs,
	}
	lfs := &NetworkFilter{
		Name:   "tcp_proxy",
		Config: lcg,
	}
	plds := &Listener{
		Name:       listenerName,
		Address:    address,
		Filters:    []*NetworkFilter{lfs},
		BindToPort: true,
	}
	return plds
}

// Normalize sorts and de-duplicates listeners by address
func (listeners Listeners) normalize() Listeners {
	out := make(Listeners, 0, len(listeners))
	set := make(map[string]*Listener)
	for _, listener := range listeners {
		if l, collision := set[listener.Address]; collision {
			ol, _ := json.Marshal(*l)
			ll, _ := json.Marshal(*listener)
			logrus.Errorf("Listener collision for %s\n---\n%s\n--- rejected ---\n%s", listener.Address,
				string(ol), string(ll))
			continue
		}
		out = append(out, listener)
		set[listener.Address] = listener
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Address < out[j].Address })
	return out
}

// GetByAddress returns a listener by its address
func (listeners Listeners) GetByAddress(addr string) *Listener {
	for _, listener := range listeners {
		if listener.Address == addr {
			return listener
		}
	}
	return nil
}

// SSLContext definition
type SSLContext struct {
	CertChainFile            string `json:"cert_chain_file"`
	PrivateKeyFile           string `json:"private_key_file"`
	CaCertFile               string `json:"ca_cert_file,omitempty"`
	RequireClientCertificate bool   `json:"require_client_certificate"`
	ALPNProtocols            string `json:"alpn_protocols,omitempty"`
}

// SSLContextExternal definition
type SSLContextExternal struct {
	CaCertFile string `json:"ca_cert_file,omitempty"`
}

// SSLContextWithSAN definition, VerifySubjectAltName cannot be nil.
type SSLContextWithSAN struct {
	CertChainFile        string   `json:"cert_chain_file"`
	PrivateKeyFile       string   `json:"private_key_file"`
	CaCertFile           string   `json:"ca_cert_file,omitempty"`
	VerifySubjectAltName []string `json:"verify_subject_alt_name"`
}

// Admin definition
type Admin struct {
	AccessLogPath string `json:"access_log_path"`
	Address       string `json:"address"`
}

// Host definition
type Host struct {
	URL string `json:"url"`
}

// Cluster definition
type Cluster struct {
	Name                     string            `json:"name"`
	ServiceName              string            `json:"service_name,omitempty"`
	ConnectTimeoutMs         int64             `json:"connect_timeout_ms"`
	Type                     string            `json:"type"`
	LbType                   string            `json:"lb_type"`
	MaxRequestsPerConnection int               `json:"max_requests_per_connection,omitempty"`
	Hosts                    []Host            `json:"hosts,omitempty"`
	SSLContext               interface{}       `json:"ssl_context,omitempty"`
	Features                 string            `json:"features,omitempty"`
	CircuitBreaker           *CircuitBreaker   `json:"circuit_breakers,omitempty"`
	OutlierDetection         *OutlierDetection `json:"outlier_detection,omitempty"`
}

// CircuitBreaker definition
// See: https://lyft.github.io/envoy/docs/configuration/cluster_manager/cluster_circuit_breakers.html#circuit-breakers
type CircuitBreaker struct {
	Default DefaultCBPriority `json:"default"`
}

// DefaultCBPriority defines the circuit breaker for default cluster priority
type DefaultCBPriority struct {
	MaxConnections     int `json:"max_connections"`
	MaxPendingRequests int `json:"max_pending_requests"`
	MaxRequests        int `json:"max_requests"`
	MaxRetries         int `json:"max_retries"`
}

// OutlierDetection definition
// See: https://lyft.github.io/envoy/docs/configuration/cluster_manager/cluster_runtime.html#outlier-detection
type OutlierDetection struct {
	ConsecutiveErrors  int   `json:"consecutive_5xx,omitempty"`
	IntervalMS         int64 `json:"interval_ms,omitempty"`
	BaseEjectionTimeMS int64 `json:"base_ejection_time_ms,omitempty"`
	MaxEjectionPercent int   `json:"max_ejection_percent,omitempty"`
}

// Clusters is a collection of clusters
type Clusters []*Cluster

//Append append some clusters
func (c *Clusters) Append(new Clusters) {
	*c = append(*c, new...)
}

// Normalize deduplicates and sorts clusters by name
func (c Clusters) Normalize() Clusters {
	out := make(Clusters, 0, len(c))
	set := make(map[string]bool)
	for _, cluster := range c {
		if !set[cluster.Name] {
			set[cluster.Name] = true
			out = append(out, cluster)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// RoutesByPath sorts routes by their path and/or prefix, such that:
// - Exact path routes are "less than" than prefix path routes
// - Exact path routes are sorted lexicographically
// - Prefix path routes are sorted anti-lexicographically
//
// This order ensures that prefix path routes do not shadow more
// specific routes which share the same prefix.
type RoutesByPath []*HTTPRoute

func (r RoutesByPath) Len() int {
	return len(r)
}

func (r RoutesByPath) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r RoutesByPath) Less(i, j int) bool {
	if r[i].Path != "" {
		if r[j].Path != "" {
			// i and j are both path
			return r[i].Path < r[j].Path
		}
		// i is path and j is prefix => i is "less than" j
		return true
	}
	if r[j].Path != "" {
		// i is prefix nad j is path => j is "less than" i
		return false
	}
	// i and j are both prefix
	return r[i].Prefix > r[j].Prefix
}

// Headers sorts headers
type Headers []Header

func (s Headers) Len() int {
	return len(s)
}

func (s Headers) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s Headers) Less(i, j int) bool {
	if s[i].Name == s[j].Name {
		if s[i].Regex == s[j].Regex {
			return s[i].Value < s[j].Value
		}
		// true is less, false is more
		return s[i].Regex
	}
	return s[i].Name < s[j].Name
}

// DiscoveryCluster is a service discovery service definition
type DiscoveryCluster struct {
	Cluster        *Cluster `json:"cluster"`
	RefreshDelayMs int64    `json:"refresh_delay_ms"`
}

// LDSCluster is a reference to LDS cluster by name
type LDSCluster struct {
	Cluster        string `json:"cluster"`
	RefreshDelayMs int64  `json:"refresh_delay_ms"`
}

// CDSCluter is result struct for cds api
type CDSCluter struct {
	Clusters Clusters `json:"clusters"`
}

// DiscoverHost is hosts that make up the service.
type DiscoverHost struct {
	Address string `json:"ip_address"`
	Port    int    `json:"port"`
	Tags    *Tags  `json:"tags,omitempty"`
}

// Tags is Discover host tags
type Tags struct {
	AZ     string `json:"az,omitempty"`
	Canary bool   `json:"canary,omitempty"`

	// Weight is an integer in the range [1, 100] or empty
	Weight int `json:"load_balancing_weight,omitempty"`
}

//DiscoverHosts  is a collection of DiscoverHost
type DiscoverHosts []*DiscoverHost

//SDSHost is result struct for sds api
type SDSHost struct {
	Hosts DiscoverHosts `json:"hosts"`
}

// LDSListener is result struct for lds api
type LDSListener struct {
	Listeners Listeners `json:"listeners"`
}

// RDS definition
type RDS struct {
	Cluster         string `json:"cluster"`
	RouteConfigName string `json:"route_config_name"`
	RefreshDelayMs  int64  `json:"refresh_delay_ms"`
}

// ClusterManager definition
type ClusterManager struct {
	Clusters         Clusters          `json:"clusters"`
	SDS              *DiscoveryCluster `json:"sds,omitempty"`
	CDS              *DiscoveryCluster `json:"cds,omitempty"`
	LocalClusterName string            `json:"local_cluster_name,omitempty"`
}

// FilterMixerConfig definition.
//
// NOTE: all fields marked as DEPRECATED are part of the original v1
// mixerclient configuration. They are deprecated and will be
// eventually removed once proxies are updated.
//
// Going forwards all mixerclient configuration should represeted by
// istio.io/api/mixer/v1/config/client/mixer_filter_config.proto and
// encoded in the `V2` field below.
//
type FilterMixerConfig struct {
	// DEPRECATED: MixerAttributes specifies the static list of attributes that are sent with
	// each request to Mixer.
	MixerAttributes map[string]string `json:"mixer_attributes,omitempty"`

	// DEPRECATED: ForwardAttributes specifies the list of attribute keys and values that
	// are forwarded as an HTTP header to the server side proxy
	ForwardAttributes map[string]string `json:"forward_attributes,omitempty"`

	// DEPRECATED: QuotaName specifies the name of the quota bucket to withdraw tokens from;
	// an empty name means no quota will be charged.
	QuotaName string `json:"quota_name,omitempty"`

	// DEPRECATED: If set to true, disables mixer check calls for TCP connections
	DisableTCPCheckCalls bool `json:"disable_tcp_check_calls,omitempty"`

	// istio.io/api/mixer/v1/config/client configuration protobuf
	// encoded as a generic map using canonical JSON encoding.
	//
	// If `V2` field is not empty, the DEPRECATED fields above should
	// be discarded.
	V2 map[string]interface{} `json:"v2,omitempty"`
}

// IsNetworkFilterConfig marks FilterMixerConfig as an implementation of NetworkFilterConfig
func (*FilterMixerConfig) IsNetworkFilterConfig() {}
