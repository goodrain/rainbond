// RAINBOND, Application Management Platform
// Copyright (C) 2021-2024 Goodrain Co., Ltd.

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
	"context"
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/pkg/gogo"
	etcdutil "github.com/goodrain/rainbond/util/etcd"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"time"
)

var defaultEtcdComponent *Component

// Component -
type Component struct {
	EtcdClient *clientv3.Client
}

// Etcd -
func Etcd() *Component {
	defaultEtcdComponent = &Component{}
	return defaultEtcdComponent
}

// Default -
func Default() *Component {
	return defaultEtcdComponent
}

// Start -
func (e *Component) Start(ctx context.Context, cfg *configs.Config) error {
	logrus.Info("start etcd client...")
	clientArgs := &etcdutil.ClientArgs{
		Endpoints: cfg.APIConfig.EtcdEndpoint,
		CaFile:    cfg.APIConfig.EtcdCaFile,
		CertFile:  cfg.APIConfig.EtcdCertFile,
		KeyFile:   cfg.APIConfig.EtcdKeyFile,
	}
	if clientArgs.DialTimeout <= 5 {
		clientArgs.DialTimeout = 5 * time.Second
	}
	if clientArgs.AutoSyncInterval <= 30 {
		clientArgs.AutoSyncInterval = 10 * time.Second
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
			return err
		}
		config.TLS = tlsConfig
	} else {
		config.DialOptions = []grpc.DialOption{grpc.WithInsecure()}
	}
	gogo.Go(func(ctx context.Context) error {
		var err error
		for {
			e.EtcdClient, err = clientv3.New(config)
			if err == nil {
				logrus.Infof("create etcd.v3 client success")
				return nil
			}
			logrus.Errorf("create etcd.v3 client failed, try time is %d,%s", 10, err.Error())
			time.Sleep(10 * time.Second)
		}
	})
	time.Sleep(5 * time.Second)
	return nil
}

// CloseHandle -
func (e *Component) CloseHandle() {
}
