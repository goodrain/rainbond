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
	"crypto/x509"
	"time"
)

//LoadBalancingType Load Balancing type
type LoadBalancingType string

//RoundRobin Assign requests in turn to each node.
var RoundRobin LoadBalancingType = "RoundRobin"

//WeightedRoundRobin Assign requests in turn to each node, in proportion to their weights.
var WeightedRoundRobin LoadBalancingType = "WeightedRoundRobin"

//Perceptive Predict the most appropriate node using a combination of historical and current data.
var Perceptive LoadBalancingType = "Perceptive"

//LeastConnections Assign each request to the node with the fewest connections.
var LeastConnections LoadBalancingType = "LeastConnections"

//WeightedLeastConnections Assign each request to a node based on the number of concurrent connections to the node and its weight.
var WeightedLeastConnections LoadBalancingType = "WeightedLeastConnections"

//FastestResponseTime Assign each request to the node with the fastest response time.
var FastestResponseTime LoadBalancingType = "FastestResponseTime"

//RandomNode Choose a random node for each request.
var RandomNode LoadBalancingType = "RandomNode"

//Monitor monitor type
type Monitor string

//ConnectMonitor tcp connect monitor
var ConnectMonitor Monitor = "connect"

//PingMonitor ping monitor
var PingMonitor Monitor = "ping"

//SimpleHTTP http monitor
var SimpleHTTP Monitor = "simple http"

//SimpleHTTPS http monitor
var SimpleHTTPS Monitor = "simple https"

//Meta Common meta
type Meta struct {
	Index      int64  `json:"index"`
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	PluginName string `json:"plugin_name"`
}

//Pool Application service endpoints pool
type Pool struct {
	Meta
	//application service id
	ServiceID string `json:"service_id"`
	//application service version
	ServiceVersion string `json:"service_version"`
	//application service port
	ServicePort int `json:"service_port"`
	//pool instructions
	Note              string            `json:"note"`
	NodeNumber        int               `json:"node_number"`
	LoadBalancingType LoadBalancingType `json:"load_balancing_type"`
	Monitors          []Monitor         `json:"monitors"`
}

//Node Application service endpoint
type Node struct {
	Meta
	Host     string `json:"host"`
	Port     int32  `json:"port"`
	Protocol string `json:"protocol"`
	State    string `json:"state"`     //Active Draining Disabled
	PoolName string `json:"pool_name"` //Belong to the pool
	Ready    bool   `json:"ready"`     //Whether ready
	Weight   int    `json:"weight"`
}

//HTTPRule Application service access rule for http
type HTTPRule struct {
	Meta
	Domain       string            `json:"domain"`
	Path         string            `json:"path"`
	Headers      map[string]string `json:"headers"`
	Redirect     RedirectConfig    `json:"redirect,omitempty"`
	HTTPSEnabale bool              `json:"https_enable"`
	SSLCertName  string            `json:"ssl_cert_name"`
	PoolName     string            `json:"pool_name"`
}

//RedirectConfig Config returns the redirect configuration for an  rule
type RedirectConfig struct {
	URL       string `json:"url"`
	Code      int    `json:"code"`
	FromToWWW bool   `json:"fromToWWW"`
}

//VirtualService VirtualService
type VirtualService struct {
	Meta
	Enabled  bool   `json:"enable"`
	Protocol string `json:"protocol"` //default stream
	// BackendProtocol indicates which protocol should be used to communicate with the service
	BackendProtocol        string            `json:"backend-protocol"`
	Port                   int32             `json:"port"`
	Listening              []string          `json:"listening"` //if Listening is nil,will listen all
	Note                   string            `json:"note"`
	DefaultPoolName        string            `json:"default_pool_name"`
	RuleNames              []string          `json:"rule_names"`
	SSLdecrypt             bool              `json:"ssl_decrypt"`
	DefaultCertificateName string            `json:"default_certificate_name"`
	CertificateMapping     map[string]string `json:"certificate_mapping"`
	RequestLogEnable       bool              `json:"request_log_enable"`
	RequestLogFileName     string            `json:"request_log_file_name"`
	RequestLogFormat       string            `json:"request_log_format"`
	//ConnectTimeout The time, in seconds, to wait for data from a new connection. If no data is received within this time, the connection will be closed. A value of 0 (zero) will disable the timeout.
	ConnectTimeout int `json:"connect_timeout"`
	//Timeout A connection should be closed if no additional data has been received for this period of time. A value of 0 (zero) will disable this timeout. Note that the default value may vary depending on the protocol selected.
	Timeout int `json:"timeout"`
}

// SSLCert describes a SSL certificate
type SSLCert struct {
	Meta
	CertificateStr string            `json:"certificate_str"`
	Certificate    *x509.Certificate `json:"certificate,omitempty"`
	PrivateKey     string            `json:"private_key"`
	// CN contains all the common names defined in the SSL certificate
	CN []string `json:"cn"`
	// ExpiresTime contains the expiration of this SSL certificate in timestamp format
	ExpireTime time.Time `json:"expires"`
}
