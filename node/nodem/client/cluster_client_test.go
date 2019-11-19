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

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/utils"
	"github.com/goodrain/rainbond/util"
)

func TestNormalEndpoint(t *testing.T) {
	value := []string{"http://192.168.2.37:9999", "192.168.2.203:8081", "http://:7171", ":7171"}

	for _, ep := range value {
		if !utils.FilterEndpoint(ep) {
			continue
		}

		t.Logf("ep is : %s", ep)
	}
}

func TestEmptyServieEndpoint(t *testing.T) {
	keys := []string{"METRICS_ENDPOINTS/", "METRICS_ENDPOINTS/192.168.2.203", "MONITOR_ENDPOINTS/"}
	for _, key := range keys {
		if !utils.FilterEndpointKey(key) {
			continue
		}
		t.Log(key)
	}
}

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
