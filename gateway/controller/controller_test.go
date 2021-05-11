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

package controller

import (
	v1 "github.com/goodrain/rainbond/gateway/v1"
)

func poolsIsEqual(old []*v1.Pool, new []*v1.Pool) bool {
	if len(old) != len(new) {
		return false
	}
	for _, rp := range old {
		flag := false
		for _, cp := range new {
			if rp.Equals(cp) {
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

func newFakePoolWithoutNodes(name string) *v1.Pool {
	return &v1.Pool{
		Meta: v1.Meta{
			Index:      888,
			Name:       name,
			Namespace:  "gateway",
			PluginName: "Nginx",
		},
		ServiceID:         "foo-service-id",
		ServiceVersion:    "1.0.0",
		ServicePort:       80,
		Note:              "foo",
		NodeNumber:        8,
		LoadBalancingType: v1.RoundRobin,
		Monitors: []v1.Monitor{
			"monitor-a",
			"monitor-b",
			"monitor-c",
		},
	}
}
