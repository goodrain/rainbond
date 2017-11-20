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

package model

//SDS strcut
type SDS struct {
	Hosts []*PieceSDS `json:"hosts"`
}

//PieceSDS struct
type PieceSDS struct {
	IPAddress string `json:"ip_address"`
	Port      int32  `json:"port"`
	//Tags      sdsTags `json:"tags"`
}

type sdsTags struct {
	AZ                  string `json:"az"`
	Canary              string `json:"canary"`
	LoadBalancingWeight string `json:"load_balancing_weight"`
}

//CDS struct
type CDS struct {
	Clusters []*PieceCDS `json:"clusters"`
}

//PieceCDS struct
type PieceCDS struct {
	Name             string           `json:"name"`
	Type             string           `json:"type"`
	ConnectTimeoutMS int              `json:"connect_timeout_ms"`
	LBType           string           `json:"lb_type"`
	ServiceName      string           `json:"service_name"`
	CircuitBreakers  *CircuitBreakers `json:"circuit_breakers"`
	//HealthCheck      cdsHealthCheckt `json:"health_check"`
}

//MaxConnections circuit
type MaxConnections struct {
	MaxConnections int `json:"max_connections"`
}

//CircuitBreakers circuit
type CircuitBreakers struct {
	Default *MaxConnections `json:"default"`
}

type cdsHealthCheckt struct {
	Type               string `json:"type"`
	TimeoutMS          int    `json:"timeout_ms"`
	IntervalMS         int    `json:"interval_ms"`
	IntervalJitterMS   int    `json:"interval_jitter_ms"`
	UnhealthyThreshold int    `json:"unhealthy_threshold"`
	HealthyThreshold   int    `json:"healthy_threshold"`
	Path               string `json:"path"`
	ServiceName        string `json:"service_name"`
}

//LDS struct
type LDS struct {
	Listeners []*PieceLDS `json:"listeners"`
}

//PieceLDS struct
type PieceLDS struct {
	Name    string        `json:"name"`
	Address string        `json:"address"`
	Filters []*LDSFilters `json:"filters"`
}

//LDSFilters LDSFilters
type LDSFilters struct {
	Name   string      `json:"name"`
	Config interface{} `json:"config"`
}

//LDSTCPConfig LDSTCPConfig
type LDSTCPConfig struct {
	StatPrefix  string        `json:"stat_prefix"`
	RouteConfig *LDSTCPRoutes `json:"route_config"`
}

//LDSTCPRoutes LDSTCPRoutes
type LDSTCPRoutes struct {
	Routes []*PieceTCPRoute `json:"routes"`
}

//PieceTCPRoute PieceTCPRoute
type PieceTCPRoute struct {
	Cluster string `json:"cluster"`
}

//LDSHTTPConfig LDSHTTPConfig
type LDSHTTPConfig struct {
	CodecType   string               `json:"codec_type"`
	StatPrefix  string               `json:"stat_prefix"`
	RouteConfig *RouteConfig         `json:"route_config"`
	Filters     []*HTTPSingleFileter `json:"filters"`
}

//RouteConfig RouteConfig
type RouteConfig struct {
	VirtualHosts []*PieceHTTPVirtualHost `json:"virtual_hosts"`
}

//PieceHTTPVirtualHost PieceHTTPVirtualHost
type PieceHTTPVirtualHost struct {
	Name    string             `json:"name"`
	Domains []string           `json:"domains"`
	Routes  []*PieceHTTPRoutes `json:"routes"`
}

//PieceHTTPRoutes PieceHTTPRoutes
type PieceHTTPRoutes struct {
	TimeoutMS int            `json:"timeout_ms"`
	Prefix    string         `json:"prefix"`
	Cluster   string         `json:"cluster"`
	Headers   []*PieceHeader `json:"headers"`
}

//PieceHeader PieceHeader
type PieceHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

//HTTPSingleFileter HttpSingleFileter
type HTTPSingleFileter struct {
	Type   string            `json:"type"`
	Name   string            `json:"name"`
	Config map[string]string `json:"config"`
}

const (
	//PREFIX PREFIX
	PREFIX string = "PREFIX"
	//HEADERS HEADERS
	HEADERS string = "HEADERS"
	//DOMAINS DOMAINS
	DOMAINS string = "DOMAINS"
	//LIMITS LIMITS
	LIMITS string = "LIMITS"
	//UPSTREAM upStream
	UPSTREAM string = "upStream"
	//DOWNSTREAM downStream
	DOWNSTREAM string = "downStream"
)
