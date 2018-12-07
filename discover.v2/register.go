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

package discover

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/clientv3"
	client "github.com/coreos/etcd/clientv3"
	etcdnaming "github.com/coreos/etcd/clientv3/naming"
	"github.com/goodrain/rainbond/util"
	"google.golang.org/grpc/naming"
)

//KeepAlive 服务注册
type KeepAlive struct {
	EtcdEndpoint []string
	ServerName   string
	HostName     string
	Endpoint     string
	TTL          int64
	LID          clientv3.LeaseID
	Done         chan struct{}
	etcdClient   *client.Client
	gRPCResolver *etcdnaming.GRPCResolver
	once         sync.Once
}

//CreateKeepAlive create keepalive for server
func CreateKeepAlive(EtcdEndpoint []string, ServerName string, Protocol string, HostIP string, Port int) (*KeepAlive, error) {
	if len(EtcdEndpoint) == 0 {
		EtcdEndpoint = []string{"127.0.0.1:2379"}
	}
	if ServerName == "" || Port == 0 {
		return nil, fmt.Errorf("servername or serverport can not be empty")
	}
	if HostIP == "" {
		ip, err := util.LocalIP()
		if err != nil {
			logrus.Errorf("get ip failed,details %s", err.Error())
			return nil, err
		}
		HostIP = ip.String()
	}

	k := &KeepAlive{
		EtcdEndpoint: EtcdEndpoint,
		ServerName:   ServerName,
		Endpoint:     fmt.Sprintf("%s:%d", HostIP, Port),
		TTL:          5,
		Done:         make(chan struct{}),
	}
	if Protocol == "" {
		k.Endpoint = fmt.Sprintf("%s:%d", HostIP, Port)
	} else {
		k.Endpoint = fmt.Sprintf("%s://%s:%d", Protocol, HostIP, Port)
	}
	return k, nil
}

//Start 开始
func (k *KeepAlive) Start() error {
	duration := time.Duration(k.TTL) * time.Second
	timer := time.NewTimer(duration)
	etcdclient, err := client.New(client.Config{
		Endpoints: k.EtcdEndpoint,
	})
	if err != nil {
		return err
	}
	k.etcdClient = etcdclient
	go func() {
		for {
			select {
			case <-k.Done:
				return
			case <-timer.C:
				if k.LID > 0 {
					func() {
						ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
						defer cancel()
						defer timer.Reset(duration)
						_, err := k.etcdClient.KeepAliveOnce(ctx, k.LID)
						if err == nil {
							return
						}
						logrus.Warnf("%s lid[%x] keepAlive err: %s, try to reset...", k.Endpoint, k.LID, err.Error())
						k.LID = 0
					}()
				} else {
					if err := k.reg(); err != nil {
						logrus.Warnf("%s set lid err: %s, try to reset after %d seconds...", k.Endpoint, err.Error(), k.TTL)
					} else {
						logrus.Infof("%s set lid[%x] success", k.Endpoint, k.LID)
					}
					timer.Reset(duration)
				}
			}
		}
	}()
	return nil
}

func (k *KeepAlive) etcdKey() string {
	return fmt.Sprintf("/rainbond/discover/%s", k.ServerName)
}

func (k *KeepAlive) reg() error {
	k.gRPCResolver = &etcdnaming.GRPCResolver{Client: k.etcdClient}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	resp, err := k.etcdClient.Grant(ctx, k.TTL+3)
	if err != nil {
		return err
	}
	if err := k.gRPCResolver.Update(ctx, k.etcdKey(), naming.Update{Op: naming.Add, Addr: k.Endpoint}, clientv3.WithLease(resp.ID)); err != nil {
		return err
	}
	logrus.Infof("Register a %s server endpoint %s to cluster", k.ServerName, k.Endpoint)
	k.LID = resp.ID
	return nil
}

//Stop 结束
func (k *KeepAlive) Stop() {
	k.once.Do(func() {
		close(k.Done)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		if k.gRPCResolver != nil {
			if err := k.gRPCResolver.Update(ctx, k.etcdKey(), naming.Update{Op: naming.Delete, Addr: k.Endpoint}); err != nil {
				logrus.Errorf("cancel %s server endpoint %s from etcd error %s", k.ServerName, k.Endpoint, err.Error())
			} else {
				logrus.Infof("cancel %s server endpoint %s from etcd", k.ServerName, k.Endpoint)
			}
		}
	})
}
