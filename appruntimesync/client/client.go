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

/*
Package client provides app runtime client code

Client code demo:

    //create app status client
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cli, err := client.NewClient(ctx, client.AppRuntimeSyncClientConf{
		EtcdEndpoints: s.Config.EtcdEndpoint,
	})
	if err != nil {
		logrus.Errorf("create app status client error, %v", err)
		return err
	}
*/
package client

import (
	"context"

	"github.com/coreos/etcd/clientv3"

	etcdnaming "github.com/coreos/etcd/clientv3/naming"

	"github.com/goodrain/rainbond/appruntimesync/pb"

	"github.com/Sirupsen/logrus"

	"github.com/goodrain/rainbond/discover"
	"github.com/goodrain/rainbond/discover/config"
	"google.golang.org/grpc"
)

// These are the available operation types.
const (
	RUNNING  string = "running"
	CLOSED          = "closed"
	STARTING        = "starting"
	STOPPING        = "stopping"
	CHECKING        = "checking"
	//运行异常
	ABNORMAL = "abnormal"
	//升级中
	UPGRADE  = "upgrade"
	UNDEPLOY = "undeploy"
	//构建中
	DEPLOYING = "deploying"
	//
	UNKNOW = "unknow"
)

//AppRuntimeSyncClient grpc client
type AppRuntimeSyncClient struct {
	pb.AppRuntimeSyncClient
	AppRuntimeSyncClientConf
	ServerAddress []string
	cc            *grpc.ClientConn
	ctx           context.Context
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
	arsc.ServerAddress = arsc.DefaultServerAddress
	arsc.ctx = ctx
	err := arsc.discover()
	if err != nil {
		return nil, err
	}
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

func (a *AppRuntimeSyncClient) discover() error {
	discover, err := discover.GetDiscover(config.DiscoverConfig{
		EtcdClusterEndpoints: a.AppRuntimeSyncClientConf.EtcdEndpoints,
		Ctx:                  a.ctx,
	})
	if err != nil {
		return err
	}
	discover.AddProject("app_sync_server", a)
	return nil
}

//UpdateEndpoints update endpoints
func (a *AppRuntimeSyncClient) UpdateEndpoints(endpoints ...*config.Endpoint) {
	var newAddress []string
	for _, en := range endpoints {
		if en.URL != "" {
			newAddress = append(newAddress, en.URL)
		}
	}
	if len(newAddress) > 0 {
		a.ServerAddress = newAddress
	}
}

//when watch occurred error,will exec this method
func (a *AppRuntimeSyncClient) Error(err error) {
	logrus.Errorf("discover app runtime sync server address occurred err:%s", err.Error())
}

//SetStatus set app status
func (a *AppRuntimeSyncClient) SetStatus(serviceID, status string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := a.AppRuntimeSyncClient.SetAppStatus(ctx, &pb.StatusMessage{
		Status: map[string]string{serviceID: status},
	})
	return err
}

//GetStatus get status
func (a *AppRuntimeSyncClient) GetStatus(serviceID string) string {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	status, err := a.AppRuntimeSyncClient.GetAppStatus(ctx, &pb.StatusRequest{
		ServiceIds: serviceID,
	})
	if err != nil {
		return "unknow"
	}
	return status.Status[serviceID]
}

//GetAllAppDisk get all service disk
func (a *AppRuntimeSyncClient) GetAllAppDisk() map[string]float64 {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	status, err := a.AppRuntimeSyncClient.GetAppDisk(ctx, &pb.StatusRequest{
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
	status, err := a.AppRuntimeSyncClient.GetAppDisk(ctx, &pb.StatusRequest{
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
	status, err := a.AppRuntimeSyncClient.GetAppStatus(ctx, &pb.StatusRequest{
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
	status, err := a.AppRuntimeSyncClient.GetAppStatus(ctx, &pb.StatusRequest{
		ServiceIds: "",
	})
	if err != nil {
		return nil
	}
	return status.Status
}

//CheckStatus CheckStatus
func (a *AppRuntimeSyncClient) CheckStatus(serviceID string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.AppRuntimeSyncClient.CheckAppStatus(ctx, &pb.StatusRequest{
		ServiceIds: serviceID,
	})
}

//GetNeedBillingStatus get need billing status
func (a *AppRuntimeSyncClient) GetNeedBillingStatus() (map[string]string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	re, err := a.AppRuntimeSyncClient.GetAppStatus(ctx, &pb.StatusRequest{})
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

//IgnoreDelete IgnoreDelete
func (a *AppRuntimeSyncClient) IgnoreDelete(name string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.AppRuntimeSyncClient.IgnoreDeleteEvent(ctx, &pb.Ignore{
		Name: name,
	})
}

//RmIgnoreDelete RmIgnoreDelete
func (a *AppRuntimeSyncClient) RmIgnoreDelete(name string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.AppRuntimeSyncClient.RmIgnoreDeleteEvent(ctx, &pb.Ignore{
		Name: name,
	})
}

//IsClosedStatus  check status
func (a *AppRuntimeSyncClient) IsClosedStatus(curStatus string) bool {
	return curStatus == CLOSED || curStatus == UNDEPLOY || curStatus == DEPLOYING || curStatus == UNKNOW
}
