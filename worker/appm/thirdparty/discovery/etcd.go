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
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/appm/types/v1"
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
		DialTimeout: 5 * time.Second,
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

	resp, err := e.cli.Get(ctx, e.key, c.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("error fetching endpoints form etcd: %v", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("error fetching endpoints form etcd: empty GetResponse")
	}

	type ep struct {
		IP       string `json:"ip"`
		Port     int    `json:"port"`
		IsOnline bool   `json:"is_online"`
	}
	var res []*model.Endpoint
	for _, kv := range resp.Kvs {
		var ep ep
		if err := json.Unmarshal(kv.Value, &ep); err != nil {
			return nil, fmt.Errorf("error getting data from etcd: %v", err)
		}
		res = append(res, &model.Endpoint{
			UUID:     strings.Replace(string(kv.Key), e.key+"/", "", -1),
			IP:       ep.IP,
			Port:     ep.Port,
			IsOnline: &ep.IsOnline,
		})
	}
	return res, nil
}

func (e *etcd) Add(dbm db.Manager, req v1.AddEndpointReq) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if e.cli == nil {
		return fmt.Errorf("can't fetching data from etcd without etcdv3 client")
	}
	type foo struct {
		IP       string `json:"ip"`
		Port     int    `json:"port"`
		IsOnline bool   `json:"is_online"`
	}
	f := foo{
		IP:       req.IP,
		Port:     req.Port,
		IsOnline: req.IsOnline,
	}
	b, _ := json.Marshal(f)
	_, err := e.cli.Put(ctx, e.key+"/"+util.NewUUID(), string(b))
	if err != nil {
		return fmt.Errorf("error adding endpoints to etcd: %v", err)
	}
	return nil
}

func (e *etcd) Update(dbm db.Manager, req v1.UpdEndpointReq) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if e.cli == nil {
		return fmt.Errorf("can't fetching data from etcd without etcdv3 client")
	}
	resp, err := e.cli.Get(ctx, e.key+"/"+req.EpID)
	if err != nil {
		return fmt.Errorf("IP UUID: %s; error getting data from etcd: %v", req.EpID, err)
	}
	if resp == nil || resp.Kvs == nil || len(resp.Kvs) == 0 {
		return fmt.Errorf("IP UUID: %s; empty data from etcd", req.EpID)
	}
	type ep struct {
		IP       string `json:"endpoint"`
		IsOnline bool   `json:"is_online"`
	}
	var foo ep
	if err := json.Unmarshal(resp.Kvs[0].Value, &foo); err != nil {
		return fmt.Errorf("error getting data from etcd: %v", err)
	}
	foo.IsOnline = req.IsOnline
	if req.IP != "" {
		foo.IP = req.IP
	}
	b, _ := json.Marshal(foo)
	_, err = e.cli.Put(ctx, e.key+"/"+req.EpID, string(b))
	if err != nil {
		return fmt.Errorf("error fetching endpoints form etcd: %v", err)
	}
	return nil
}

// Fetch fetches data from Etcd.
func (e *etcd) Delete(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if e.cli == nil {
		return fmt.Errorf("can't deleting data from etcd without etcdv3 client")
	}
	_, err := e.cli.Delete(ctx, e.key+"/"+id)
	if err != nil {
		return fmt.Errorf("error deleting endpoints form etcd: %v", err)
	}
	return nil
}

// Close shuts down the client's etcd connections.
func (e *etcd) Close() error {
	if e.cli != nil {
		return nil
	}
	return e.cli.Close()
}
