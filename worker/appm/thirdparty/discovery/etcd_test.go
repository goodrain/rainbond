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

package discovery

import (
	"fmt"
	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/db/model"
	"testing"
	"time"
)

func TestEtcd_Watch(t *testing.T) {
	cfg := &model.ThirdPartySvcDiscoveryCfg{
		Type:    model.DiscorveryTypeEtcd.String(),
		Servers: "http://127.0.0.1:2379",
		Key:     "/foobar/eps",
	}
	updateCh := channels.NewRingChannel(1024)
	stopCh := make(chan struct{})
	defer close(stopCh)

	go func() {
		for {
			select {
			case event := <-updateCh.Out():
				fmt.Printf("%+v", event)
			case <-stopCh:
				break
			}
		}
	}()

	etcd := NewEtcd(cfg, updateCh, stopCh)
	if err := etcd.Connect(); err != nil {
		t.Fatalf("error connecting etcd: %v", err)
	}
	defer etcd.Close()

	etcd.Watch()

	time.Sleep(10 * time.Second)
}