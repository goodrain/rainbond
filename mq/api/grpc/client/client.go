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
	"github.com/goodrain/rainbond/mq/api/grpc/pb"

	clientv3 "github.com/coreos/etcd/clientv3"
	etcdnaming "github.com/coreos/etcd/clientv3/naming"
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

//MQClient mq grpc client
type MQClient struct {
	pb.TaskQueueClient
	ctx    context.Context
	cancel context.CancelFunc
}

//NewMqClient new a mq client
func NewMqClient(etcdendpoints []string, defaultserver string) (*MQClient, error) {
	ctx, cancel := context.WithCancel(context.Background())
	var conn *grpc.ClientConn
	if etcdendpoints != nil && len(defaultserver) > 1 {
		c, err := clientv3.New(clientv3.Config{Endpoints: etcdendpoints, Context: ctx})
		if err != nil {
			return nil, err
		}
		r := &etcdnaming.GRPCResolver{Client: c}
		b := grpc.RoundRobin(r)
		conn, err = grpc.DialContext(ctx, "/rainbond/discover/rainbond_mq", grpc.WithBalancer(b), grpc.WithInsecure())
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		conn, err = grpc.DialContext(ctx, defaultserver, grpc.WithInsecure())
		if err != nil {
			return nil, err
		}
	}
	cli := pb.NewTaskQueueClient(conn)
	client := &MQClient{
		ctx:    ctx,
		cancel: cancel,
	}
	client.TaskQueueClient = cli
	return client, nil
}

//Close mq grpc client must be closed after uesd
func (m *MQClient) Close() {
	m.cancel()
}
