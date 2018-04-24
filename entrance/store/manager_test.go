// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

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

package store

import (
	"github.com/goodrain/rainbond/entrance/cluster"
	"github.com/goodrain/rainbond/cmd/entrance/option"
	"github.com/goodrain/rainbond/entrance/core/object"
	"sync"
	"testing"
)

var ct = &cluster.Manager{
	Prefix: "/entrance",
	Name:   "test",
}

func TestAddSource(t *testing.T) {
	manager, err := NewManager(option.Config{
		EtcdEndPoints: []string{"http://127.0.0.1:2379"},
		EtcdTimeout:   5,
	}, ct)
	if err != nil {
		t.Fatal(err)
	}
	source := &object.NodeObject{
		Index:    10001,
		Host:     "127.0.0.1",
		Port:     2333,
		NodeName: "pool2-127.0.0.1",
		PoolName: "pool3",
		State:    "Active",
	}
	ok, err := manager.AddSource(source)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(ok)
	ok, err = manager.AddSource(source)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(ok)
}

func TestGetSource(t *testing.T) {
	manager, err := NewManager(option.Config{
		EtcdEndPoints: []string{"http://127.0.0.1:2379"},
		EtcdTimeout:   5,
	}, ct)
	if err != nil {
		t.Fatal(err)
	}
	node := &object.NodeObject{NodeName: "pool2-127.0.0.1"}
	nodeNew, err := manager.GetSource(node)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(nodeNew)
}

func TestUpdateSource(t *testing.T) {
	manager, err := NewManager(option.Config{
		EtcdEndPoints: []string{"http://127.0.0.1:2379"},
		EtcdTimeout:   5,
	}, ct)
	if err != nil {
		t.Fatal(err)
	}
	source := &object.NodeObject{
		Index:    10004,
		Host:     "127.0.0.1",
		Port:     2333,
		NodeName: "pool2-127.0.0.1",
		PoolName: "pool3",
		State:    "Active",
	}
	wait := sync.WaitGroup{}
	wait.Add(1)
	go func() {
		ok, err := manager.UpdateSource(source)
		if err != nil {
			t.Fatal(err)
		}
		t.Log(ok)
		wait.Done()
	}()
	ok, err := manager.UpdateSource(source)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(ok)
	wait.Wait()
}

func TestDeleteSource(t *testing.T) {
	manager, err := NewManager(option.Config{
		EtcdEndPoints: []string{"http://127.0.0.1:2379"},
		EtcdTimeout:   5,
	}, ct)
	if err != nil {
		t.Fatal(err)
	}
	node := &object.NodeObject{NodeName: "pool2-127.0.0.1", PoolName: "pool3"}
	ok, err := manager.DeleteSource(node)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(ok)
	ok, err = manager.DeleteSource(node)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(ok)
}
