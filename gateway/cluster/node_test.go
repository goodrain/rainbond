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
	"testing"

	"github.com/goodrain/rainbond/cmd/gateway/option"
)

func TestCreateNodeManager(t *testing.T) {
	nm, err := CreateNodeManager(option.Config{
		ListenPorts: option.ListenPorts{
			HTTP: 80,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(nm.localV4Hosts)
	if ok := nm.checkGatewayPort(); !ok {
		t.Log("port check is not pass")
	} else {
		t.Log("port check is passed")
	}
}
