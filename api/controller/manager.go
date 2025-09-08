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

package controller

import (
	"github.com/goodrain/rainbond/pkg/component/mq"
	"net/http"

	"github.com/goodrain/rainbond/api/api"
	"github.com/goodrain/rainbond/api/proxy"
	"github.com/goodrain/rainbond/worker/client"
)

// V2Manager v2 manager
type V2Manager interface {
	Show(w http.ResponseWriter, r *http.Request)
	Health(w http.ResponseWriter, r *http.Request)
	AlertManagerWebHook(w http.ResponseWriter, r *http.Request)
	Version(w http.ResponseWriter, r *http.Request)
	api.ClusterInterface
	api.NodesInterface
	api.TenantInterface
	api.ServiceInterface
	api.LogInterface
	api.PluginInterface
	api.RulesInterface
	api.AppInterface
	api.LongVersionInterface
	api.Gatewayer
	api.ThirdPartyServicer
	api.Labeler
	api.AppRestoreInterface
	api.PodInterface
	api.ApplicationInterface
	api.RegistryAuthSecretInterface
	api.HelmInterface
	api.RegistryInterface
	api.GatewayInterface
	api.KubeBlocksInterface
}

var defaultV2Manager V2Manager

// CreateV2RouterManager 创建manager
func CreateV2RouterManager(statusCli *client.AppRuntimeSyncClient) (err error) {
	defaultV2Manager, err = NewAPIManager(statusCli)
	return err
}

// GetManager 获取管理器
func GetManager() V2Manager {
	return defaultV2Manager
}

// NewAPIManager new manager
func NewAPIManager(statusCli *client.AppRuntimeSyncClient) (*V2Routes, error) {
	var v2r V2Routes
	v2r.TenantStruct.StatusCli = statusCli
	v2r.TenantStruct.MQClient = mq.Default().MqClient
	v2r.GatewayStruct.MQClient = mq.Default().MqClient
	eventServerProxy := proxy.CreateProxy("eventlog", "http", []string{"local=>rbd-eventlog:6363"})
	v2r.EventLogStruct.EventlogServerProxy = eventServerProxy
	return &v2r, nil
}
