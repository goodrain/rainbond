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

package client

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/core/store"
	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
)

func TestEtcdClusterClient_GetEndpoints(t *testing.T) {
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

	type data struct {
		hostID string
		values []string
	}

	testCase := []string{
		"192.168.8.229:8081",
		"192.168.8.230:8081",
		"192.168.8.231:6443",
	}
	datas := []data{
		{
			hostID: util.NewUUID(),
			values: []string{
				testCase[0],
			},
		},
		{
			hostID: util.NewUUID(),
			values: []string{
				testCase[1],
			},
		},
		{
			hostID: util.NewUUID(),
			values: []string{
				testCase[2],
			},
		},
	}
	for _, d := range datas {
		s, err := json.Marshal(d.values)
		if err != nil {
			logrus.Errorf("Can not marshal %s endpoints to json.", "foobar")
			return
		}
		_, err = cli.Put(ctx, key+"/"+d.hostID, string(s))
		if err != nil {
			t.Fatal(err)
		}
	}

	c := etcdClusterClient{
		conf: &option.Conf{
			EtcdCli: cli,
		},
	}

	edps := c.GetEndpoints("foobar")
	for _, tc := range testCase {
		flag := false
		for _, edp := range edps {
			if tc == edp {
				flag = true
			}
		}
		if !flag {
			t.Fatalf("Can not find \"%s\" in %v", tc, edps)
		}
	}
}

func TestSetEndpoints(t *testing.T) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: time.Duration(5) * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	c := NewClusterClient(&option.Conf{EtcdCli: cli})
	c.SetEndpoints("etcd", "DSASD", []string{"http://:8080"})
	c.SetEndpoints("etcd", "192.168.1.1", []string{"http://:8080"})
	c.SetEndpoints("etcd", "192.168.1.1", []string{"http://192.168.1.1:8080"})
	c.SetEndpoints("node", "192.168.2.137", []string{"192.168.2.137:10252"})
	t.Logf("check: %v", checkURL("192.168.2.137:10252"))
}

func TestGetEndpoints(t *testing.T) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: time.Duration(5) * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	c := NewClusterClient(&option.Conf{EtcdCli: cli})
	t.Log(c.GetEndpoints("/etcd/"))
}
func TestEtcdClusterClient_ListEndpointKeys(t *testing.T) {
	cfg := &option.Conf{
		EtcdEndpoints:   []string{"192.168.3.3:2379"},
		EtcdDialTimeout: 5 * time.Second,
	}

	if err := store.NewClient(context.Background(), cfg, nil); err != nil {
		t.Fatalf("error create etcd client: %v", err)
	}

	hostNode := HostNode{
		InternalIP: "192.168.2.76",
	}

	keys, err := hostNode.listEndpointKeys()
	if err != nil {
		t.Errorf("unexperted error: %v", err)
	}
	t.Logf("keys: %#v", keys)
}
