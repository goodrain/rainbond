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

import "testing"

func TestPool_Equals(t *testing.T) {
	node1 := newFakeNode()
	node1.Name = "node-a"
	node2 := newFakeNode()
	node2.Name = "node-b"
	p := NewFakePoolWithoutNodes()
	p.Nodes = []*Node{
		node1,
		node2,
	}

	node3 := newFakeNode()
	node3.Name = "node-a"
	node4 := newFakeNode()
	node4.Name = "node-b"
	c := NewFakePoolWithoutNodes()
	c.Nodes = []*Node {
		node3,
		node4,
	}

	if !p.Equals(c) {
		t.Errorf("Pool p shoul equal Pool c")
	}
}

func NewFakePoolWithoutNodes() *Pool {
	return &Pool{
		Meta: Meta{
			Index:      888,
			Name:       "foo-pool",
			Namespace:  "gateway",
			PluginName: "Nginx",
		},
		ServiceID:         "foo-service-id",
		ServiceVersion:    "1.0.0",
		ServicePort:       80,
		Note:              "foo",
		NodeNumber:        8,
		LoadBalancingType: RoundRobin,
		Monitors: []Monitor{
			"monitor-a",
			"monitor-b",
			"monitor-c",
		},
	}
}
