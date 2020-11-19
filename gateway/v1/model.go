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

import "github.com/sirupsen/logrus"

//LoadBalancingType Load Balancing type
type LoadBalancingType string

//RoundRobin Assign requests in turn to each node.
var RoundRobin LoadBalancingType = "round-robin"

//CookieSessionAffinity session affinity by cookie
var CookieSessionAffinity LoadBalancingType = "cookie-session-affinity"

//GetLoadBalancingType get load balancing
func GetLoadBalancingType(s string) LoadBalancingType {
	switch s {
	case "round-robin":
		return RoundRobin
	case "cookie-session-affinity":
		return CookieSessionAffinity
	default:
		return RoundRobin
	}
}

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
	logrus.Debugf("len if cnf.L4VS = %d, l4vs = %d", len(cfg.L4VS), len(c.L4VS))
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
