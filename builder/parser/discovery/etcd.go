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

package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	c "github.com/coreos/etcd/clientv3"
	"github.com/sirupsen/logrus"
)

// Etcd implements Discoverier
type etcd struct {
	cli       *c.Client
	endpoints []string
	key       string
	username  string
	password  string
}

// NewEtcd creates a new Discorvery which implemeted by etcd.
func NewEtcd(info *Info) Discoverier {
	// TODO: validate endpoints
	return &etcd{
		endpoints: info.Servers,
		key:       info.Key,
		username:  info.Username,
		password:  info.Password,
	}
}

// Connect connects a etcdv3 client with a given configuration.
func (e *etcd) Connect() error {
	cli, err := c.New(c.Config{
		Endpoints:   e.endpoints,
		DialTimeout: 10 * time.Second,
		Username:    e.username,
		Password:    e.password,
	})
	if err != nil {
		logrus.Errorf("Endpoints: %s; error connecting etcd: %v", strings.Join(e.endpoints, ","), err)
		return err
	}
	e.cli = cli
	return nil
}

// Fetch fetches data from Etcd.
func (e *etcd) Fetch() ([]*Endpoint, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if e.cli == nil {
		return nil, fmt.Errorf("can't fetching data from etcd without etcdv3 client")
	}

	resp, err := e.cli.Get(ctx, e.key, c.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("error fetching endpoints form etcd: %v", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("error fetching endpoints form etcd: empty GetResponse")
	}

	var res []*Endpoint
	for _, kv := range resp.Kvs {
		var ep Endpoint
		if err := json.Unmarshal(kv.Value, &ep); err != nil {
			return nil, fmt.Errorf("error parsing the data from etcd: %v", err)
		}
		ep.Ep = strings.Replace(string(kv.Key), e.key+"/", "", -1)
		res = append(res, &ep)
	}
	return res, nil
}

// Close shuts down the client's etcd connections.
func (e *etcd) Close() error {
	if e.cli != nil {
		return nil
	}
	return e.cli.Close()
}
