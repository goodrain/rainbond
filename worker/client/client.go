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

// 本文件实现了Rainbond平台的应用运行时同步客户端，该客户端通过gRPC与远程服务器通信，用于获取应用的状态、资源信息、第三方服务的端点信息等。
// 文件主要包含以下内容：

// 1. `AppRuntimeSyncClient` 结构体：这是一个gRPC客户端，用于与Rainbond的应用运行时同步服务通信。它封装了与远程服务器交互的各种方法，如获取应用状态、资源信息、第三方服务端点等。

// 2. `NewClient` 函数：该函数用于创建并初始化 `AppRuntimeSyncClient` 实例。它需要传入上下文和gRPC服务器地址。成功创建客户端后，可以通过该实例调用各种远程方法。

// 3. `GetStatus` 和 `GetStatuss` 方法：这些方法用于获取单个或多个服务的状态信息。通过向远程服务器发送请求并接收响应，返回服务的当前状态。

// 4. `GetOperatorWatchData` 方法：该方法用于获取由Operator管理的应用的监控数据。主要用于监控和管理由Kubernetes Operator部署的应用。

// 5. `GetServiceDeployInfo` 方法：用于获取指定服务的部署信息，包括服务的当前部署状态及相关的元数据信息。

// 6. `GetTenantResource` 和 `GetAllTenantResource` 方法：这些方法用于获取租户的资源使用情况。通过这些方法，管理员可以了解每个租户的资源占用情况，如CPU、内存等。

// 7. `ListThirdPartyEndpoints`、`AddThirdPartyEndpoint`、`UpdThirdPartyEndpoint` 和 `DelThirdPartyEndpoint` 方法：
//    这些方法用于管理第三方服务的端点信息，可以添加、更新或删除第三方服务的端点。

// 8. `GetStorageClasses` 方法：该方法用于获取存储类信息，存储类定义了不同类型的存储资源在Kubernetes中的表现和管理方式。

// 9. `GetAppVolumeStatus` 方法：用于获取应用的存储卷状态信息，帮助运维人员监控和管理应用的存储资源。

// 10. `GetAppResources` 方法：用于获取应用的资源使用情况，包括CPU、内存等资源的分配和使用状态。

// 通过这些方法，Rainbond平台的管理员和开发者可以方便地通过gRPC客户端与运行时环境交互，获取所需的应用状态和资源信息，从而实现对应用的实时监控和管理。

package client

import (
	"context"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/keepalive"
	"strings"
	"time"

	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/goodrain/rainbond/worker/server/pb"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// AppRuntimeSyncClient grpc client
type AppRuntimeSyncClient struct {
	pb.AppRuntimeSyncClient
	cc  *grpc.ClientConn
	ctx context.Context
}

// NewClient new client
// ctx must be cancel where client not used
func NewClient(ctx context.Context, grpcServer string) (c *AppRuntimeSyncClient, err error) {
	c = new(AppRuntimeSyncClient)
	c.ctx = ctx
	logrus.Infof("discover app runtime sync server address %s", grpcServer)

	// 定义Keepalive参数
	keepaliveParams := keepalive.ClientParameters{
		Time:                10 * time.Second, // 多长时间内无活动会发送ping
		Timeout:             2 * time.Second,  // ping 之后等待的超时时间
		PermitWithoutStream: true,             // 即使没有活跃的流，是否仍然发送ping
	}

	// 定义重连策略
	retryBackoffConfig := backoff.Config{
		BaseDelay:  1.0 * time.Second, // 初始退避时间
		Multiplier: 1.6,               // 退避乘数
		MaxDelay:   120 * time.Second, // 最大退避时间
	}

	// 使用grpc.Dial添加Keepalive和重连配置
	c.cc, err = grpc.Dial(
		grpcServer,
		grpc.WithInsecure(),
		grpc.WithKeepaliveParams(keepaliveParams),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff:           retryBackoffConfig,
			MinConnectTimeout: 20 * time.Second,
		}),
	)

	if err != nil {
		return nil, err
	}
	c.AppRuntimeSyncClient = pb.NewAppRuntimeSyncClient(c.cc)
	return c, nil
}

// when watch occurred error,will exec this method
func (a *AppRuntimeSyncClient) Error(err error) {
	logrus.Errorf("discover app runtime sync server address occurred err:%s", err.Error())
}

// GetStatus get status
func (a *AppRuntimeSyncClient) GetStatus(serviceID string) string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	status, err := a.AppRuntimeSyncClient.GetAppStatusDeprecated(ctx, &pb.ServicesRequest{
		ServiceIds: serviceID,
	})
	if err != nil {
		return v1.UNKNOW
	}
	return status.Status[serviceID]
}

// GetOperatorWatchData get operator watch data
func (a *AppRuntimeSyncClient) GetOperatorWatchData(appID string) (*pb.OperatorManaged, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	status, err := a.AppRuntimeSyncClient.GetOperatorWatchManagedData(ctx, &pb.AppStatusReq{
		AppId: appID,
	})
	if err != nil {
		return nil, err
	}
	return status, nil
}

// GetStatuss get multiple app status
func (a *AppRuntimeSyncClient) GetStatuss(serviceIDs string) map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	status, err := a.AppRuntimeSyncClient.GetAppStatusDeprecated(ctx, &pb.ServicesRequest{
		ServiceIds: serviceIDs,
	})
	if err != nil {
		logrus.Errorf("get service status failure %s", err.Error())
		re := make(map[string]string, len(serviceIDs))
		for _, id := range strings.Split(serviceIDs, ",") {
			re[id] = v1.UNKNOW
		}
		return re
	}
	return status.Status
}

// GetAllStatus get all status
func (a *AppRuntimeSyncClient) GetAllStatus() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	status, err := a.AppRuntimeSyncClient.GetAppStatusDeprecated(ctx, &pb.ServicesRequest{
		ServiceIds: "",
	})
	if err != nil {
		return nil
	}
	return status.Status
}

// GetNeedBillingStatus get need billing status
func (a *AppRuntimeSyncClient) GetNeedBillingStatus() (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	re, err := a.AppRuntimeSyncClient.GetAppStatusDeprecated(ctx, &pb.ServicesRequest{})
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

// GetServiceDeployInfo get service deploy info
func (a *AppRuntimeSyncClient) GetServiceDeployInfo(serviceID string) (*pb.DeployInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	re, err := a.AppRuntimeSyncClient.GetDeployInfo(ctx, &pb.ServiceRequest{
		ServiceId: serviceID,
	})
	if err != nil {
		return nil, err
	}
	return re, nil
}

// IsClosedStatus  check status
func (a *AppRuntimeSyncClient) IsClosedStatus(curStatus string) bool {
	return curStatus == "" || curStatus == v1.BUILDEFAILURE || curStatus == v1.CLOSED || curStatus == v1.UNDEPLOY || curStatus == v1.BUILDING || curStatus == v1.UNKNOW
}

// GetTenantResource get tenant resource
func (a *AppRuntimeSyncClient) GetTenantResource(tenantID string) (*pb.TenantResource, error) {
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		defer util.Elapsed("[AppRuntimeSyncClient] get tenant resource")()
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	return a.AppRuntimeSyncClient.GetTenantResource(ctx, &pb.TenantRequest{TenantId: tenantID})
}

// GetAllTenantResource get all tenant resource
func (a *AppRuntimeSyncClient) GetAllTenantResource() (*pb.TenantResourceList, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	return a.AppRuntimeSyncClient.GetTenantResources(ctx, &pb.Empty{})
}

// ListThirdPartyEndpoints -
func (a *AppRuntimeSyncClient) ListThirdPartyEndpoints(sid string) (*pb.ThirdPartyEndpoints, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	resp, err := a.AppRuntimeSyncClient.ListThirdPartyEndpoints(ctx, &pb.ServiceRequest{
		ServiceId: sid,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// AddThirdPartyEndpoint -
func (a *AppRuntimeSyncClient) AddThirdPartyEndpoint(req *model.Endpoint) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	_, _ = a.AppRuntimeSyncClient.AddThirdPartyEndpoint(ctx, &pb.AddThirdPartyEndpointsReq{
		Uuid: req.UUID,
		Sid:  req.ServiceID,
		Ip:   req.IP,
		Port: int32(req.Port),
	})
}

// UpdThirdPartyEndpoint -
func (a *AppRuntimeSyncClient) UpdThirdPartyEndpoint(req *model.Endpoint) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	_, _ = a.AppRuntimeSyncClient.UpdThirdPartyEndpoint(ctx, &pb.UpdThirdPartyEndpointsReq{
		Uuid: req.UUID,
		Sid:  req.ServiceID,
		Ip:   req.IP,
		Port: int32(req.Port),
	})
}

// DelThirdPartyEndpoint -
func (a *AppRuntimeSyncClient) DelThirdPartyEndpoint(req *model.Endpoint) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	_, _ = a.AppRuntimeSyncClient.DelThirdPartyEndpoint(ctx, &pb.DelThirdPartyEndpointsReq{
		Uuid: req.UUID,
		Sid:  req.ServiceID,
		Ip:   req.IP,
		Port: int32(req.Port),
	})
}

// GetStorageClasses client GetStorageClasses
func (a *AppRuntimeSyncClient) GetStorageClasses() (storageclasses *pb.StorageClasses, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	return a.AppRuntimeSyncClient.GetStorageClasses(ctx, &pb.Empty{})
}

// GetAppVolumeStatus get app volume status
func (a *AppRuntimeSyncClient) GetAppVolumeStatus(serviceID string) (*pb.ServiceVolumeStatusMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	return a.AppRuntimeSyncClient.GetAppVolumeStatus(ctx, &pb.ServiceRequest{ServiceId: serviceID})
}

// GetAppResources -
func (a *AppRuntimeSyncClient) GetAppResources(appID string) (*pb.AppStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	return a.AppRuntimeSyncClient.GetAppStatus(ctx, &pb.AppStatusReq{AppId: appID})
}
