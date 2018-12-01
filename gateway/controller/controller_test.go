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
	"github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/gateway/v1"
	"testing"
	"time"
)

func TestController_GetDelUpdPools(t *testing.T) {
	delPools := []*v1.Pool{
		newFakePoolWithoutNodes("pool-a"),
		newFakePoolWithoutNodes("pool-b"),
	}
	updPools := []*v1.Pool{
		newFakePoolWithoutNodes("pool-c"),
		newFakePoolWithoutNodes("pool-d"),
	}
	// fooPools don't need to be change.
	fooPools := []*v1.Pool{
		newFakePoolWithoutNodes("pool-e"),
		newFakePoolWithoutNodes("pool-f"),
	}

	var runningHttpPools []*v1.Pool
	runningHttpPools = append(runningHttpPools, delPools...)
	runningHttpPools = append(runningHttpPools, fooPools...)

	var currentHttpPools []*v1.Pool
	currentHttpPools = append(currentHttpPools, updPools...)
	currentHttpPools = append(currentHttpPools, fooPools...)

	gwc := &GWController{
		rhp: runningHttpPools,
	}
	del, upd := gwc.getDelUpdPools(currentHttpPools)
	if !poolsIsEqual(delPools, del) {
		t.Errorf("del should equal delPools.")
	}
	if !poolsIsEqual(updPools, upd) {
		t.Errorf("upd should equal udpPools.")
	}

	gwc.rhp = fooPools
	currentHttpPools = fooPools
	del, upd = gwc.getDelUpdPools(currentHttpPools)
	if len(del) != 0 {
		t.Errorf("Expected del length to be 0, but returned %v", len(del))
	}
	if len(upd) != 0 {
		t.Errorf("Expected del length to be 0, but returned %v", len(upd))
	}
}

func TestGWController_WatchRbdEndpoints(t *testing.T) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 3 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	gwc := GWController{
		EtcdCli: cli,
	}
	go gwc.watchRbdEndpoints()
}

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
