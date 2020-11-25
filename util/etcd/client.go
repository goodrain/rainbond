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

package etcd

import (
	"errors"
	"time"

	"github.com/coreos/etcd/pkg/transport"
	"github.com/sirupsen/logrus"

	"github.com/coreos/etcd/clientv3"
	v3 "github.com/coreos/etcd/clientv3"
	spb "github.com/coreos/etcd/mvcc/mvccpb"
	"golang.org/x/net/context"
)

var (
	// ErrKeyExists key already exists
	ErrKeyExists = errors.New("key already exists")
	// ErrWaitMismatch unexpected wait result
	ErrWaitMismatch = errors.New("unexpected wait result")
	// ErrTooManyClients too many clients
	ErrTooManyClients = errors.New("too many clients")
	// ErrNoWatcher no watcher channel
	ErrNoWatcher = errors.New("no watcher channel")
	//ErrNoEndpoints no etcd endpoint
	ErrNoEndpoints = errors.New("no etcd endpoint")
)

// deleteRevKey deletes a key by revision, returning false if key is missing
func deleteRevKey(ctx context.Context, kv v3.KV, key string, rev int64) (bool, error) {
	cmp := v3.Compare(v3.ModRevision(key), "=", rev)
	req := v3.OpDelete(key)
	txnresp, err := kv.Txn(ctx).If(cmp).Then(req).Commit()
	if err != nil {
		return false, err
	} else if !txnresp.Succeeded {
		return false, nil
	}
	return true, nil
}

//claimFirstKey 获取队列第一个key,并从队列删除
func claimFirstKey(ctx context.Context, kv v3.KV, kvs []*spb.KeyValue) (*spb.KeyValue, error) {
	for _, k := range kvs {
		ok, err := deleteRevKey(ctx, kv, string(k.Key), k.ModRevision)
		if err != nil {
			return nil, err
		} else if ok {
			return k, nil
		}
	}
	return nil, nil
}

// ClientArgs etcd client arguments
type ClientArgs struct {
	Endpoints        []string      // args for clientv3.Config
	DialTimeout      time.Duration // args for clientv3.Config
	AutoSyncInterval time.Duration // args for clientv3.Config
	CaFile           string        // args for clientv3.Config.TLS
	CertFile         string        // args for clientv3.Config.TLS
	KeyFile          string        // args for clientv3.Config.TLS
}

var (
	// for parsing ca from k8s object
	defaultDialTimeout      = 5 * time.Second
	defaultAotuSyncInterval = 10 * time.Second
)

// NewClient new etcd client v3 for all rainbond module, attention: do not support v2
func NewClient(ctx context.Context, clientArgs *ClientArgs) (*v3.Client, error) {
	if clientArgs.DialTimeout <= 5 {
		clientArgs.DialTimeout = defaultDialTimeout
	}
	if clientArgs.AutoSyncInterval <= 30 {
		clientArgs.AutoSyncInterval = defaultAotuSyncInterval
	}

	config := clientv3.Config{
		Context:              ctx,
		Endpoints:            clientArgs.Endpoints,
		DialTimeout:          clientArgs.DialTimeout,
		DialKeepAliveTime:    time.Second * 2,
		DialKeepAliveTimeout: time.Second * 6,
		AutoSyncInterval:     clientArgs.AutoSyncInterval,
	}

	if clientArgs.CaFile != "" && clientArgs.CertFile != "" && clientArgs.KeyFile != "" {
		// create etcd client with tls
		tlsInfo := transport.TLSInfo{
			CertFile:      clientArgs.CertFile,
			KeyFile:       clientArgs.KeyFile,
			TrustedCAFile: clientArgs.CaFile,
		}
		tlsConfig, err := tlsInfo.ClientConfig()
		if err != nil {
			return nil, err
		}
		config.TLS = tlsConfig
	}
	var etcdClient *v3.Client
	var err error
	for {
		etcdClient, err = clientv3.New(config)
		if err == nil {
			logrus.Infof("etcd.v3 client is ready")
			return etcdClient, nil
		}
		logrus.Errorf("create etcd.v3 client failed, try time is %d,%s", 10, err.Error())
		time.Sleep(10 * time.Second)
	}
}
