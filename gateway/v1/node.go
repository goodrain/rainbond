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

//Node Application service endpoint
type Node struct {
	Meta
	Host        string `json:"host"`
	Port        int32  `json:"port"`
	Protocol    string `json:"protocol"`
	State       string `json:"state"`     //Active Draining Disabled
	PoolName    string `json:"pool_name"` //Belong to the pool
	Ready       bool   `json:"ready"`     //Whether ready
	Weight      int    `json:"weight"`
	MaxFails    int    `json:"max_fails"`
	FailTimeout string `json:"fail_timeout"`
}

func (n *Node) Equals(c *Node) bool { //
	if n == c {
		return true
	}
	if n == nil || c == nil {
		return false
	}
	if n.Meta != c.Meta {
		return false
	}
	if n.Host != c.Host {
		return false
	}
	if n.Protocol != c.Protocol {
		return false
	}
	if n.State != c.State {
		return false
	}
	if n.PoolName != c.PoolName {
		return false
	}
	if n.Ready != c.Ready {
		return false
	}
	if n.Weight != c.Weight {
		return false
	}
	return true
}
