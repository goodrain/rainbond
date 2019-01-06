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
	UpstreamHashBy    string            `json:"upstream_hash_by"`
	LeastConn         bool              `json:"least_conn"`
	Monitors          []Monitor         `json:"monitors"`
	Nodes             []*Node
}

func (p *Pool) Equals(c *Pool) bool {
	if p == c {
		return true
	}
	if p == nil || c == nil {
		return false
	}
	if !p.Meta.Equals(&c.Meta) {
		return false
	}
	if p.ServiceID != c.ServiceID {
		return false
	}
	if p.ServiceVersion != c.ServiceVersion {
		return false
	}
	if p.ServicePort != c.ServicePort {
		return false
	}
	if p.Note != c.Note {
		return false
	}
	if p.NodeNumber != c.NodeNumber {
		return false
	}
	if p.LoadBalancingType != c.LoadBalancingType {
		return false
	}

	if len(p.Monitors) != len(c.Monitors) {
		return false
	}
	for _, a := range p.Monitors {
		flag := false
		for _, b := range c.Monitors {
			if a == b {
				flag = true
				break
			}
		}
		if !flag {
			return false
		}
	}

	if len(p.Nodes) != len(c.Nodes) {
		return false
	}
	for _, a := range p.Nodes {
		flag := false
		for _, b := range c.Nodes {
			if a.Equals(b) {
				flag = true
			}
		}
		if !flag {
			return false
		}
	}

	return true
}
