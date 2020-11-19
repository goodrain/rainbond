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

import (
	"strconv"

	"github.com/sirupsen/logrus"

	v1 "github.com/goodrain/rainbond/gateway/v1"
	apiv1 "k8s.io/api/core/v1"
)

//Config update config
type Config struct {
	Backends []*Backend `json:"backends"`
}

// Backend describes one or more remote server/s (endpoints) associated with a service
type Backend struct {
	// Name represents an unique apiv1.Service name formatted as <namespace>-<name>-<port>
	Name string `json:"name"`

	Endpoints []Endpoint `json:"endpoints,omitempty"`
	// StickySessionAffinitySession contains the StickyConfig object with stickyness configuration
	SessionAffinity SessionAffinityConfig `json:"sessionAffinityConfig"`
	// Consistent hashing by NGINX variable
	UpstreamHashBy string `json:"upstream-hash-by,omitempty"`
	// LB algorithm configuration per ingress
	LoadBalancing string `json:"load-balance,omitempty"`
}

// SessionAffinityConfig describes different affinity configurations for new sessions.
// Once a session is mapped to a backend based on some affinity setting, it
// retains that mapping till the backend goes down, or the ingress controller
// restarts. Exactly one of these values will be set on the upstream, since multiple
// affinity values are incompatible. Once set, the backend makes no guarantees
// about honoring updates.
type SessionAffinityConfig struct {
	AffinityType          string                `json:"name"`
	CookieSessionAffinity CookieSessionAffinity `json:"cookieSessionAffinity"`
}

// CookieSessionAffinity defines the structure used in Affinity configured by Cookies.
// +k8s:deepcopy-gen=true
type CookieSessionAffinity struct {
	Name      string              `json:"name"`
	Hash      string              `json:"hash"`
	Expires   string              `json:"expires,omitempty"`
	MaxAge    string              `json:"maxage,omitempty"`
	Locations map[string][]string `json:"locations,omitempty"`
	Path      string              `json:"path,omitempty"`
}

// Endpoint describes a kubernetes endpoint in a backend
// +k8s:deepcopy-gen=true
type Endpoint struct {
	// Address IP address of the endpoint
	Address string `json:"address"`
	// Port number of the TCP port
	Port string `json:"port"`
	// Weight weight of the endpoint
	Weight int `json:"weight"`
	// Target returns a reference to the object providing the endpoint
	Target *apiv1.ObjectReference `json:"target,omitempty"`
}

//CreateBackendByPool create backend by pool
func CreateBackendByPool(pool *v1.Pool) *Backend {
	var backend = Backend{
		Name: pool.Name,
	}
	switch pool.LoadBalancingType {
	case v1.RoundRobin:
		backend.LoadBalancing = "round_robin"
	case v1.CookieSessionAffinity:
		logrus.Infof("pool %s use cookie-session-affinity load balance", pool.Name)
		backend.SessionAffinity = SessionAffinityConfig{
			AffinityType: "cookie",
			CookieSessionAffinity: CookieSessionAffinity{
				Name: "rainbond-route",
			},
		}
	}
	backend.UpstreamHashBy = pool.UpstreamHashBy
	var endpoints []Endpoint
	for _, node := range pool.Nodes {
		endpoints = append(endpoints, Endpoint{
			Address: node.Host,
			Port:    strconv.Itoa(int(node.Port)),
			Weight:  node.Weight,
		})
	}
	backend.Endpoints = endpoints
	return &backend
}
