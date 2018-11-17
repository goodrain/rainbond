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

package server

import (
	"context"
	"fmt"
	"net"
	"strings"

	discover "github.com/goodrain/rainbond/discover.v2"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/worker/appm/store"
	"github.com/goodrain/rainbond/worker/server/pb"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

//RuntimeServer app runtime grpc server
type RuntimeServer struct {
	ctx       context.Context
	cancel    context.CancelFunc
	store     store.Storer
	conf      option.Config
	server    *grpc.Server
	hostIP    string
	keepalive *discover.KeepAlive
}

//CreaterRuntimeServer create a runtime grpc server
func CreaterRuntimeServer(conf option.Config, store store.Storer) *RuntimeServer {
	ctx, cancel := context.WithCancel(context.Background())
	rs := &RuntimeServer{
		conf:   conf,
		ctx:    ctx,
		cancel: cancel,
		server: grpc.NewServer(),
		hostIP: conf.HostIP,
		store:  store,
	}
	pb.RegisterAppRuntimeSyncServer(rs.server, rs)
	// Register reflection service on gRPC server.
	reflection.Register(rs.server)
	return rs
}

//Start start runtime server
func (r *RuntimeServer) Start(errchan chan error) {
	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", r.conf.HostIP, r.conf.ServerPort))
		if err != nil {
			logrus.Errorf("failed to listen: %v", err)
			errchan <- err
		}
		if err := r.server.Serve(lis); err != nil {
			errchan <- err
		}
	}()
	if err := r.registServer(); err != nil {
		errchan <- err
	}
}

//GetAppStatus get app service status
func (r *RuntimeServer) GetAppStatus(ctx context.Context, re *pb.StatusRequest) (*pb.StatusMessage, error) {
	status := r.store.GetAppServicesStatus(strings.Split(re.ServiceIds, ","))
	return &pb.StatusMessage{
		Status: status,
	}, nil
}

//GetAppDisk get app service volume disk size
func (r *RuntimeServer) GetAppDisk(ctx context.Context, re *pb.StatusRequest) (*pb.DiskMessage, error) {
	return nil, nil
}

//registServer
//regist sync server to etcd
func (r *RuntimeServer) registServer() error {
	if r.keepalive == nil {
		keepalive, err := discover.CreateKeepAlive(r.conf.EtcdEndPoints, "app_sync_runtime_server", "", r.conf.HostIP, r.conf.ServerPort)
		if err != nil {
			return fmt.Errorf("create app sync server keepalive error,%s", err.Error())
		}
		r.keepalive = keepalive
	}
	return r.keepalive.Start()
}
