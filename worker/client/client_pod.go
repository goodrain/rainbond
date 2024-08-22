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
// 本文件实现了Rainbond平台的客户端功能，通过gRPC与应用运行时同步服务通信，提供对服务的Pod列表、组件的Pod数量、以及单个Pod详细信息的获取接口。
// 该文件中的主要功能包括：

// 1. `GetServicePods` 方法：
//    - 用于获取指定服务的所有Pod列表。
//    - 通过gRPC调用远程服务接口，传入服务ID，获取与该服务相关的所有Pod信息。

// 2. `GetMultiServicePods` 方法：
//    - 用于获取多个服务的Pod列表。
//    - 接收多个服务ID，并通过gRPC接口获取所有相关服务的Pod信息。
//    - 使用了日志记录功能，如果日志级别设置为Debug，则记录操作耗时。

// 3. `GetComponentPodNums` 方法：
//    - 获取指定组件的Pod数量。
//    - 通过传入多个组件ID，调用远程服务获取每个组件的Pod数量，并返回一个包含组件ID和对应Pod数量的映射。
//    - 同样支持耗时日志记录，用于调试和性能分析。

// 4. `GetPodDetail` 方法：
//    - 用于获取指定服务中某个Pod的详细信息。
//    - 通过传入服务ID和Pod名称，调用远程服务获取该Pod的详细状态信息。

// 这些功能为Rainbond平台的应用管理和运维人员提供了便捷的工具，可以通过这些接口实时获取服务的运行状况，监控Pod的数量和状态，及时发现和处理潜在的问题。

package client

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/server/pb"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// GetServicePods get service pods list
func (a *AppRuntimeSyncClient) GetServicePods(serviceID string) (*pb.ServiceAppPodList, error) {
	ctx, cancel := context.WithTimeout(a.ctx, time.Second*5)
	defer cancel()
	return a.AppRuntimeSyncClient.GetAppPods(ctx, &pb.ServiceRequest{ServiceId: serviceID})
}

// GetMultiServicePods get multi service pods list
func (a *AppRuntimeSyncClient) GetMultiServicePods(serviceIDs []string) (*pb.MultiServiceAppPodList, error) {
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		defer util.Elapsed(fmt.Sprintf("[AppRuntimeSyncClient] [GetMultiServicePods] component nums: %d", len(serviceIDs)))()
	}

	ctx, cancel := context.WithTimeout(a.ctx, time.Second*5)
	defer cancel()
	return a.AppRuntimeSyncClient.GetMultiAppPods(ctx, &pb.ServicesRequest{ServiceIds: strings.Join(serviceIDs, ",")})
}

// GetComponentPodNums -
func (a *AppRuntimeSyncClient) GetComponentPodNums(ctx context.Context, componentIDs []string) (map[string]int32, error) {
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		defer util.Elapsed(fmt.Sprintf("[AppRuntimeSyncClient] get component pod nums: %d", len(componentIDs)))()
	}

	res, err := a.AppRuntimeSyncClient.GetComponentPodNums(ctx, &pb.ServicesRequest{ServiceIds: strings.Join(componentIDs, ",")})
	if err != nil {
		return nil, errors.Wrap(err, "get component pod nums")
	}

	return res.PodNums, nil
}

// GetPodDetail -
func (a *AppRuntimeSyncClient) GetPodDetail(sid, name string) (*pb.PodDetail, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	return a.AppRuntimeSyncClient.GetPodDetail(ctx, &pb.GetPodDetailReq{
		Sid:     sid,
		PodName: name,
	})
}
