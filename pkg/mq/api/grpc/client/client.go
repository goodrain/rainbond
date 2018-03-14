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

package client

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/goodrain/rainbond/pkg/mq/api/grpc/pb"

	clientv3 "github.com/coreos/etcd/clientv3"
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/naming"
)

//NewMqClient new a mq client
func NewMqClient(endpoint string) (pb.TaskQueueClient, error) {
	ctx := context.Background()
	//TODO:
	//实现客户端服务发现和负载均衡
	//b := grpc.RoundRobin(newResolver())
	//conn, err := grpc.DialContext(ctx, "http://127.0.0.1:2379", grpc.WithInsecure(), grpc.WithBalancer(b))
	//time.Sleep(time.Second * 4)
	//目前grpc版本实现负载均衡有BUG,暂时不实现
	conn, err := grpc.DialContext(ctx, endpoint, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	return pb.NewTaskQueueClient(conn), nil
}

// resolver is the implementaion of grpc.naming.Resolver
type resolver struct {
	serviceName string // service name to resolve
}

// newResolver return resolver with service name
func newResolver() *resolver {
	return &resolver{serviceName: "rainbond_mq"}
}

// Resolve to resolve the service from etcd, target is the dial address of etcd
// target example: "http://127.0.0.1:2379,http://127.0.0.1:12379,http://127.0.0.1:22379"
func (re *resolver) Resolve(target string) (naming.Watcher, error) {
	logrus.Info("Resolve target %s", target)
	if re.serviceName == "" {
		return nil, errors.New("grpclb: no service name provided")
	}

	// generate etcd client
	client, err := clientv3.New(clientv3.Config{
		Endpoints: strings.Split(target, ","),
	})
	if err != nil {
		return nil, fmt.Errorf("grpclb: creat etcd3 client failed: %s", err.Error())
	}

	// Return watcher
	return &watcher{re: re, client: *client}, nil
}

// watcher is the implementaion of grpc.naming.Watcher
type watcher struct {
	re            *resolver // re: Etcd Resolver
	client        clientv3.Client
	isInitialized bool
}

// Close do nothing
func (w *watcher) Close() {
}

// Next to return the updates
func (w *watcher) Next() ([]*naming.Update, error) {
	// prefix is the etcd prefix/value to watch
	prefix := fmt.Sprintf("/traefik/backends/%s/servers", w.re.serviceName)
	// check if is initialized
	if !w.isInitialized {
		// query addresses from etcd
		resp, err := w.client.Get(context.Background(), prefix, clientv3.WithPrefix())
		w.isInitialized = true
		if err == nil {
			addrs := extractAddrs(resp)
			//if not empty, return the updates or watcher new dir
			if l := len(addrs); l != 0 {
				updates := make([]*naming.Update, l)
				for i := range addrs {
					updates[i] = &naming.Update{Op: naming.Add, Addr: addrs[i]}
				}
				logrus.Info("Servers:", updates)
				return updates, nil
			}
		}
	}

	// generate etcd Watcher
	rch := w.client.Watch(context.Background(), prefix, clientv3.WithPrefix())
	for wresp := range rch {
		for _, ev := range wresp.Events {
			switch ev.Type {
			case mvccpb.PUT:
				return []*naming.Update{{Op: naming.Add, Addr: string(ev.Kv.Value)}}, nil
			case mvccpb.DELETE:
				return []*naming.Update{{Op: naming.Delete, Addr: string(ev.Kv.Value)}}, nil
			}
		}
	}
	return nil, nil
}

func extractAddrs(resp *clientv3.GetResponse) []string {
	addrs := []string{}

	if resp == nil || resp.Kvs == nil {
		return addrs
	}

	for i := range resp.Kvs {
		if v := resp.Kvs[i].Value; v != nil {
			addrs = append(addrs, string(v))
		}
	}

	return addrs
}
