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

package appm

import (
	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/event"
	"fmt"
	"os"
	"testing"

	"github.com/pquerna/ffjson/ffjson"
)

func TestStartStatefulSet(t *testing.T) {
	manager, err := NewManager(option.Config{
		KubeConfig: "../../admin.kubeconfig",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	event.NewManager(event.EventConfig{
		EventLogServers: []string{"tcp://127.0.0.1:6366"},
	})
	os.Setenv("CUR_NET", "midonet")
	result, err := manager.StartStatefulSet("889bb1f028f655bebd545f24aa184a0b", event.GetManager().GetLogger("system"))
	if err != nil {
		t.Fatal(err)
	}
	re, _ := ffjson.Marshal(result)
	fmt.Println(string(re))
}

func TestHorizontalScaling(t *testing.T) {
	manager, err := NewManager(option.Config{
		KubeConfig: "../../admin.kubeconfig",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	event.NewManager(event.EventConfig{
		EventLogServers: []string{"tcp://127.0.0.1:6366"},
	})
	os.Setenv("CUR_NET", "midonet")
	err = manager.HorizontalScaling("37f6cc84da449882104687130e868196", 4, event.GetManager().GetLogger("system"))
	if err != nil {
		t.Fatal(err)
	}
}
func TestStopStatefulSet(t *testing.T) {
	manager, err := NewManager(option.Config{
		KubeConfig: "../../admin.kubeconfig",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	event.NewManager(event.EventConfig{
		EventLogServers: []string{"tcp://127.0.0.1:6366"},
	})
	os.Setenv("CUR_NET", "midonet")
	err = manager.StopStatefulSet("889bb1f028f655bebd545f24aa184a0b", event.GetManager().GetLogger("system"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestStartService(t *testing.T) {
	manager, err := NewManager(option.Config{
		KubeConfig: "../../admin.kubeconfig",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	event.NewManager(event.EventConfig{
		EventLogServers: []string{"tcp://127.0.0.1:6366"},
	})
	os.Setenv("CUR_NET", "midonet")
	os.Setenv("EX_DOMAIN", "test-ali.goodrain.net:10080")
	err = manager.StartService("37f6cc84da449882104687130e868196", event.GetManager().GetLogger("system"), "", "")
	if err != nil {
		t.Fatal(err)
	}
	err = manager.StartService("889bb1f028f655bebd545f24aa184a0b", event.GetManager().GetLogger("system"), "", "")
	if err != nil {
		t.Fatal(err)
	}
}

func TestStopService(t *testing.T) {
	manager, err := NewManager(option.Config{
		KubeConfig: "../../admin.kubeconfig",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	event.NewManager(event.EventConfig{
		EventLogServers: []string{"tcp://127.0.0.1:6366"},
	})
	os.Setenv("CUR_NET", "midonet")
	os.Setenv("EX_DOMAIN", "test-ali.goodrain.net:10080")
	err = manager.StopService("37f6cc84da449882104687130e868196", event.GetManager().GetLogger("system"))
	if err != nil {
		t.Fatal(err)
	}
	err = manager.StopService("889bb1f028f655bebd545f24aa184a0b", event.GetManager().GetLogger("system"))
	if err != nil {
		t.Fatal(err)
	}
}
func TestStartReplicationController(t *testing.T) {
	manager, err := NewManager(option.Config{
		KubeConfig: "../../admin.kubeconfig",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	event.NewManager(event.EventConfig{
		EventLogServers: []string{"tcp://127.0.0.1:6366"},
	})
	os.Setenv("CUR_NET", "midonet")
	os.Setenv("EX_DOMAIN", "test-ali.goodrain.net:10080")
	re, err := manager.StartReplicationController("59fbd0a74e7dfbf594fba0f8953593f8", event.GetManager().GetLogger("system"))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(re)
}

func TestStopReplicationController(t *testing.T) {
	manager, err := NewManager(option.Config{
		KubeConfig: "../../admin.kubeconfig",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	event.NewManager(event.EventConfig{
		EventLogServers: []string{"tcp://127.0.0.1:6366"},
	})
	os.Setenv("CUR_NET", "midonet")
	os.Setenv("EX_DOMAIN", "test-ali.goodrain.net:10080")
	err = manager.StopReplicationController("59fbd0a74e7dfbf594fba0f8953593f8", event.GetManager().GetLogger("system"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestRollingUpgradeReplicationController(t *testing.T) {
	manager, err := NewManager(option.Config{
		KubeConfig: "../../admin.kubeconfig",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	event.NewManager(event.EventConfig{
		EventLogServers: []string{"tcp://127.0.0.1:6366"},
	})
	os.Setenv("CUR_NET", "midonet")
	os.Setenv("EX_DOMAIN", "test-ali.goodrain.net:10080")
	stop := make(chan struct{})
	_, err = manager.RollingUpgradeReplicationController("59fbd0a74e7dfbf594fba0f8953593f8", stop, event.GetManager().GetLogger("system"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestSyncData(t *testing.T) {
	manager, err := NewManager(option.Config{
		KubeConfig: "../../admin.kubeconfig",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	event.NewManager(event.EventConfig{
		EventLogServers: []string{"tcp://127.0.0.1:6366"},
	})
	manager.SyncData()
}
