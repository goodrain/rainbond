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
	"context"
	"strings"
	"testing"
	"time"

	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/goodrain/rainbond/node/nodem/service"
)

func TestManagerService_SetEndpoints(t *testing.T) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: time.Duration(5) * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	key := "/rainbond/endpoint/foobar"
	defer cli.Delete(ctx, key, clientv3.WithPrefix())

	m := &ManagerService{}
	srvs := []*service.Service{
		{
			Endpoints: []*service.Endpoint{
				{
					Name:     "foobar",
					Protocol: "http",
					Port:     "6442",
				},
			},
		},
	}
	m.services = srvs
	c := client.NewClusterClient(
		&option.Conf{
			EtcdCli: cli,
		},
	)
	m.cluster = c

	data := []string{
		"192.168.8.229",
		"192.168.8.230",
		"192.168.8.231",
	}

	m.SetEndpoints(data[0])
	m.SetEndpoints(data[1])
	m.SetEndpoints(data[2])

	edps := c.GetEndpoints("foobar")
	for _, d := range data {
		flag := false
		for _, edp := range edps {
			if d+":6442" == strings.Replace(edp, "http://", "", -1) {
				flag = true
			}
		}
		if !flag {
			t.Fatalf("Can not find \"%s\" in %v", d, edps)
		}
	}
}
