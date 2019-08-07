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

package cmd

import (
	"fmt"
	"net"
	"testing"

	"github.com/goodrain/rainbond/node/nodem/client"
)

func TestAnsibleHostConfig(t *testing.T) {
	h1 := &AnsibleHost{
		AnsibleHostIP:   net.ParseIP("192.168.0.1"),
		AnsibleHostPort: 22,
		HostID:          "234t32134561",
		Role:            client.HostRule{"manage"},
	}
	h2 := &AnsibleHost{
		AnsibleHostIP:   net.ParseIP("192.168.0.2"),
		AnsibleHostPort: 22,
		HostID:          "234t32134562",
		Role:            client.HostRule{"manage,compute"},
	}
	h3 := &AnsibleHost{
		AnsibleHostIP:   net.ParseIP("192.168.0.3"),
		AnsibleHostPort: 22,
		HostID:          "234t32134563",
		Role:            client.HostRule{"manage,gateway"},
	}
	ansibleConfig := &AnsibleHostConfig{
		FileName: "/tmp/hosts",
		GroupList: map[string]*AnsibleHostGroup{
			"all": &AnsibleHostGroup{
				Name: "all",
				HostList: []*AnsibleHost{
					h1, h2, h3,
				},
			},
			"manage": &AnsibleHostGroup{
				Name: "manage",
				HostList: []*AnsibleHost{
					h1, h2, h3,
				},
			},
			"gateway": &AnsibleHostGroup{
				Name: "gateway",
				HostList: []*AnsibleHost{
					h3,
				},
			},
			"compute": &AnsibleHostGroup{
				Name: "compute",
				HostList: []*AnsibleHost{
					h2,
				},
			},
		},
	}
	fmt.Print(ansibleConfig.Content())
}
