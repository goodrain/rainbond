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

package store

import (
	"errors"
	"strings"
	"time"

	client "github.com/coreos/etcd/clientv3"

	conf "github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/utils"

	"context"

	etcdutil "github.com/goodrain/rainbond/util/etcd"
	"github.com/sirupsen/logrus"
)

var (
	DefalutClient *Client
)

// Client etcd client
type Client struct {
	*client.Client
	reqTimeout time.Duration
}

// NewClient 创建client
func NewClient(ctx context.Context, cfg *conf.Conf, etcdClientArgs *etcdutil.ClientArgs) (err error) {
	cli, err := etcdutil.NewClient(ctx, etcdClientArgs)
	if err != nil {
		return
	}
	if cfg.ReqTimeout < 3 {
		cfg.ReqTimeout = 3
	}
	c := &Client{
		Client:     cli,
		reqTimeout: time.Duration(cfg.ReqTimeout) * time.Second,
	}
	logrus.Infof("init etcd client, endpoint is:%v", cfg.EtcdEndpoints)
	DefalutClient = c
	return
}

// ErrKeyExists key exist error
var ErrKeyExists = errors.New("key already exists")

// Post attempts to create the given key, only succeeding if the key did
// not yet exist.
func (c *Client) Post(key, val string, opts ...client.OpOption) (*client.PutResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.reqTimeout)
	defer cancel()
	cmp := client.Compare(client.Version(key), "=", 0)
	req := client.OpPut(key, val, opts...)
	txnresp, err := c.Client.Txn(ctx).If(cmp).Then(req).Commit()
	if err != nil {
		return nil, err
	}
	if !txnresp.Succeeded {
		return nil, ErrKeyExists
	}
	return txnresp.OpResponse().Put(), nil
}

// Put etcd v3 Put
func (c *Client) Put(key, val string, opts ...client.OpOption) (*client.PutResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.reqTimeout)
	defer cancel()
	return c.Client.Put(ctx, key, val, opts...)
}

// NewRunnable NewRunnable
func (c *Client) NewRunnable(key, val string, opts ...client.OpOption) (*client.PutResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.reqTimeout)
	defer cancel()
	return c.Client.Put(ctx, key, val, opts...)
}

// DelRunnable DelRunnable
func (c *Client) DelRunnable(key string, opts ...client.OpOption) (*client.DeleteResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.reqTimeout)
	defer cancel()
	return c.Client.Delete(ctx, key, opts...)
}

// PutWithModRev PutWithModRev
func (c *Client) PutWithModRev(key, val string, rev int64) (*client.PutResponse, error) {
	if rev == 0 {
		return c.Put(key, val)
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.reqTimeout)
	tresp, err := DefalutClient.Txn(ctx).
		If(client.Compare(client.ModRevision(key), "=", rev)).
		Then(client.OpPut(key, val)).
		Commit()
	cancel()
	if err != nil {
		return nil, err
	}

	if !tresp.Succeeded {
		return nil, utils.ErrValueMayChanged
	}

	resp := client.PutResponse(*tresp.Responses[0].GetResponsePut())
	return &resp, nil
}

// IsRunnable IsRunnable
func (c *Client) IsRunnable(key string, opts ...client.OpOption) bool {
	ctx, cancel := context.WithTimeout(context.Background(), c.reqTimeout)
	defer cancel()
	resp, err := c.Client.Get(ctx, key, opts...)
	if err != nil {
		logrus.Infof("get key %s from etcd failed ,details %s", key, err.Error())
		return false
	}
	if resp.Count <= 0 {
		logrus.Infof("get nothing from etcd by key %s", key)
		return false
	}
	return true
}

// Get get
func (c *Client) Get(key string, opts ...client.OpOption) (*client.GetResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.reqTimeout)
	defer cancel()
	return c.Client.Get(ctx, key, opts...)
}

// Delete delete v3 etcd
func (c *Client) Delete(key string, opts ...client.OpOption) (*client.DeleteResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.reqTimeout)
	defer cancel()
	return c.Client.Delete(ctx, key, opts...)
}

// Watch etcd v3 watch
func (c *Client) Watch(key string, opts ...client.OpOption) client.WatchChan {
	return c.Client.Watch(context.Background(), key, opts...)
}

// WatchByCtx watch by ctx
func (c *Client) WatchByCtx(ctx context.Context, key string, opts ...client.OpOption) client.WatchChan {
	return c.Client.Watch(ctx, key, opts...)
}

// KeepAliveOnce etcd v3 KeepAliveOnce
func (c *Client) KeepAliveOnce(id client.LeaseID) (*client.LeaseKeepAliveResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.reqTimeout)
	defer cancel()
	return c.Client.KeepAliveOnce(ctx, id)
}

// GetLock GetLock
func (c *Client) GetLock(key string, id client.LeaseID) (bool, error) {
	key = conf.Config.LockPath + key
	ctx, cancel := context.WithTimeout(context.Background(), c.reqTimeout)
	resp, err := DefalutClient.Txn(ctx).
		If(client.Compare(client.CreateRevision(key), "=", 0)).
		Then(client.OpPut(key, "", client.WithLease(id))).
		Commit()
	cancel()

	if err != nil {
		return false, err
	}

	return resp.Succeeded, nil
}

// DelLock DelLock
func (c *Client) DelLock(key string) error {
	_, err := c.Delete(conf.Config.LockPath + key)
	return err
}

// Grant etcd v3 Grant
func (c *Client) Grant(ttl int64) (*client.LeaseGrantResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.reqTimeout)
	defer cancel()
	return c.Client.Grant(ctx, ttl)
}

// IsValidAsKeyPath IsValidAsKeyPath
func IsValidAsKeyPath(s string) bool {
	return strings.IndexByte(s, '/') == -1
}
