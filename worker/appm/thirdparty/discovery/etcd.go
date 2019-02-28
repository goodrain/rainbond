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
	"context"
	"encoding/json"
	"fmt"
	c "github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/db/model"
	"strings"
	"time"
)

type etcd struct {
	cli *c.Client

	endpoints []string
	key       string
	username  string
	password  string
}

// NewEtcd creates a new Discorvery which implemeted by etcd.
func NewEtcd(cfg *model.ThirdPartySvcDiscoveryCfg) Discoverier {
	// TODO: validate endpoints
	return &etcd{
		endpoints: strings.Split(cfg.Servers, ","),
		key:       cfg.Key,
		username:  cfg.Username,
		password:  cfg.Password,
	}
}

// Connect connects a etcdv3 client with a given configuration.
func (e *etcd) Connect() error {
	cli, err := c.New(c.Config{
		Endpoints:   e.endpoints,
		DialTimeout: 5,
		Username:    e.username,
		Password:    e.password,
	})
	if err != nil {
		return fmt.Errorf("error connecting etcd: %v", err)
	}
	e.cli = cli
	return nil
}

// Fetch fetches data from Etcd.
func (e *etcd) Fetch() ([]*model.Endpoint, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if e.cli == nil {
		return nil, fmt.Errorf("can't fetching data from etcd without etcdv3 client")
	}

	resp, err := e.cli.Get(ctx, e.key)
	if err != nil {
		return nil, fmt.Errorf("error fetching endpoints form etcd: %v", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("error fetching endpoints form etcd: empty GetResponse")
	}

	type ep struct {
		Endpint  string `json:"endpoint"`
		IsOnline bool   `json:"is_online"`
	}
	var res []*model.Endpoint
	for _, kv := range resp.Kvs {
		var eps []*model.Endpoint
		if err := json.Unmarshal([]byte(kv.Value), &eps); err != nil {
			return nil, fmt.Errorf("error getting data from etcd: %v", err)
		}
		res = append(res, eps...)
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
