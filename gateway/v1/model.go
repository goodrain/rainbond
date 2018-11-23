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

// Config contains all the configuration of the gateway
type Config struct {
	HTTPPools []*Pool
	TCPPools  []*Pool
	L7VS      []*VirtualService
	L4VS      []*VirtualService
}

// Equals determines if cfg is equal to c
func (cfg *Config) Equals(c *Config) bool {
	if cfg == c {
		return true
	}

	if cfg == nil || c == nil {
		return false
	}

	if len(cfg.TCPPools) != len(c.TCPPools) {
		return false
	}
	for _, cfgp := range cfg.TCPPools {
		flag := false
		for _, cp := range c.TCPPools {
			if cfgp.Equals(cp) {
				flag = true
				break
			}
		}
		if !flag {
			return false
		}
	}

	if len(cfg.L7VS) != len(c.L7VS) {
		return false
	}
	for _, cfgv := range cfg.L7VS {
		flag := false
		for _, cv := range c.L7VS {
			if cfgv.Equals(cv) {
				flag = true
				break
			}
		}
		if !flag {
			return false
		}
	}

	if len(cfg.L4VS) != len(c.L4VS) {
		return false
	}
	for _, cfgv := range cfg.L4VS {
		flag := false
		for _, cv := range c.L4VS {
			if cfgv.Equals(cv) {
				flag = true
				break
			}
		}
		if !flag {
			return false
		}
	}

	return true
}
