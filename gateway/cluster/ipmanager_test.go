// RAINBOND, Application Management Platform
// Copyright (C) 2014-2019 Goodrain Co., Ltd.

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

package cluster

import (
	"net"
	"testing"
	"time"

	"github.com/goodrain/rainbond/cmd/gateway/option"
)

func TestCreateIPManager(t *testing.T) {
	i, err := CreateIPManager(option.Config{
		ListenPorts: option.ListenPorts{
			HTTP: 80,
		},
		EtcdEndpoint: []string{"http://127.0.0.1:2379"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := i.Start(); err != nil {
		t.Fatal(err)
	}
	t.Log(i.IPInCurrentHost(net.ParseIP("192.168.2.15")))
	time.Sleep(time.Second * 10)
}
