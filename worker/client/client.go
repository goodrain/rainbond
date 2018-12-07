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

package client

import (
	"context"

	"github.com/goodrain/rainbond/worker/appm/types/v1"

	"github.com/coreos/etcd/clientv3"

	etcdnaming "github.com/coreos/etcd/clientv3/naming"

	"github.com/Sirupsen/logrus"

	"github.com/goodrain/rainbond/worker/server/pb"
	"google.golang.org/grpc"
)

//AppRuntimeSyncClient grpc client
type AppRuntimeSyncClient struct {
	pb.AppRuntimeSyncClient
	AppRuntimeSyncClientConf
	cc  *grpc.ClientConn
	ctx context.Context
}

//AppRuntimeSyncClientConf client conf
type AppRuntimeSyncClientConf struct {
	EtcdEndpoints        []string
	DefaultServerAddress []string
}

//NewClient new client
//ctx must be cancel where client not used
func NewClient(ctx context.Context, conf AppRuntimeSyncClientConf) (*AppRuntimeSyncClient, error) {
	var arsc AppRuntimeSyncClient
	arsc.AppRuntimeSyncClientConf = conf
	arsc.ctx = ctx
	c, err := clientv3.New(clientv3.Config{Endpoints: conf.EtcdEndpoints, Context: ctx})
	if err != nil {
		return nil, err
	}
	r := &etcdnaming.GRPCResolver{Client: c}
	b := grpc.RoundRobin(r)
	arsc.cc, err = grpc.DialContext(ctx, "/rainbond/discover/app_sync_runtime_server", grpc.WithBalancer(b), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	arsc.AppRuntimeSyncClient = pb.NewAppRuntimeSyncClient(arsc.cc)
	return &arsc, nil
}

//when watch occurred error,will exec this method
func (a *AppRuntimeSyncClient) Error(err error) {
	logrus.Errorf("discover app runtime sync server address occurred err:%s", err.Error())
}

//GetStatus get status
func (a *AppRuntimeSyncClient) GetStatus(serviceID string) string {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	status, err := a.AppRuntimeSyncClient.GetAppStatus(ctx, &pb.ServicesRequest{
		ServiceIds: serviceID,
	})
	if err != nil {
		return v1.UNKNOW
	}
	return status.Status[serviceID]
}

//GetAllAppDisk get all service disk
func (a *AppRuntimeSyncClient) GetAllAppDisk() map[string]float64 {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	status, err := a.AppRuntimeSyncClient.GetAppDisk(ctx, &pb.ServicesRequest{
		ServiceIds: "",
	})
	if err != nil {
		return nil
	}
	return status.Disks
}

//GetAppsDisk get define service disk
func (a *AppRuntimeSyncClient) GetAppsDisk(serviceIDs string) map[string]float64 {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	status, err := a.AppRuntimeSyncClient.GetAppDisk(ctx, &pb.ServicesRequest{
		ServiceIds: serviceIDs,
	})
	if err != nil {
		return nil
	}
	return status.Disks
}

//GetStatuss get multiple app status
func (a *AppRuntimeSyncClient) GetStatuss(serviceIDs string) map[string]string {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	status, err := a.AppRuntimeSyncClient.GetAppStatus(ctx, &pb.ServicesRequest{
		ServiceIds: serviceIDs,
	})
	if err != nil {
		return nil
	}
	return status.Status
}

//GetAllStatus get all status
func (a *AppRuntimeSyncClient) GetAllStatus() map[string]string {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	status, err := a.AppRuntimeSyncClient.GetAppStatus(ctx, &pb.ServicesRequest{
		ServiceIds: "",
	})
	if err != nil {
		return nil
	}
	return status.Status
}

//GetNeedBillingStatus get need billing status
func (a *AppRuntimeSyncClient) GetNeedBillingStatus() (map[string]string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	re, err := a.AppRuntimeSyncClient.GetAppStatus(ctx, &pb.ServicesRequest{})
	if err != nil {
		return nil, err
	}
	var res = make(map[string]string)
	for k, v := range re.Status {
		if !a.IsClosedStatus(v) {
			res[k] = v
		}
	}
	return res, nil
}

//GetServiceDeployInfo get service deploy info
func (a *AppRuntimeSyncClient) GetServiceDeployInfo(serviceID string) (*pb.DeployInfo, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	re, err := a.AppRuntimeSyncClient.GetDeployInfo(ctx, &pb.ServiceRequest{
		ServiceId: serviceID,
	})
	if err != nil {
		return nil, err
	}
	return re, nil
}

//IsClosedStatus  check status
func (a *AppRuntimeSyncClient) IsClosedStatus(curStatus string) bool {
	return curStatus == v1.BUILDEFAILURE || curStatus == v1.CLOSED || curStatus == v1.UNDEPLOY || curStatus == v1.BUILDING || curStatus == v1.UNKNOW
}
