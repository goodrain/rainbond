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

package prober

import (
	"context"
	"fmt"
	"github.com/goodrain/rainbond/util/prober/types/v1"
	"testing"
)

func TestProbeManager_Start(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := NewProber(ctx, cancel)

	serviceList := make([]*v1.Service, 0, 10)

	h := &v1.Service{
		Name: "etcd",
		ServiceHealth: &v1.Health{
			Name:         "etcd",
			Model:        "tcp",
			Address:      "192.168.1.107:23790",
			TimeInterval: 3,
		},
	}
	serviceList = append(serviceList, h)
	m.SetServices(serviceList)
	watcher := m.WatchServiceHealthy("etcd")
	m.EnableWatcher(watcher.GetServiceName(), watcher.GetID())

	m.Start()

	for {
		v := <-watcher.Watch()
		if v != nil {
			fmt.Println("----", v.Name, v.Status, v.Info, v.ErrorNumber, v.ErrorNumber)
		} else {
			t.Log("nil nil nil")
		}
	}
}
