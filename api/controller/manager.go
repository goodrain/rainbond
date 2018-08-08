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
	"net/http"

	"github.com/goodrain/rainbond/cmd/api/option"
	"github.com/goodrain/rainbond/api/apiFunc"
	"github.com/goodrain/rainbond/api/discover"
	"github.com/goodrain/rainbond/api/proxy"
	"github.com/goodrain/rainbond/appruntimesync/client"

	"github.com/Sirupsen/logrus"
)

//V2Manager v2 manager
type V2Manager interface {
	Show(w http.ResponseWriter, r *http.Request)
	Nodes(w http.ResponseWriter, r *http.Request)
	Jobs(w http.ResponseWriter, r *http.Request)
	Apps(w http.ResponseWriter, r *http.Request)
	Entrance(w http.ResponseWriter, r *http.Request)
	Health(w http.ResponseWriter, r *http.Request)
	AlertManagerWebHook(w http.ResponseWriter, r *http.Request)

	apiFunc.TenantInterface
	apiFunc.ServiceInterface
	apiFunc.LogInterface
	apiFunc.PluginInterface
	apiFunc.RulesInterface
	apiFunc.SourcesInterface
	apiFunc.AppInterface
}

var defaultV2Manager V2Manager

//CreateV2RouterManager 创建manager
func CreateV2RouterManager(conf option.Config, statusCli *client.AppRuntimeSyncClient) error {
	defaultV2Manager = NewManager(conf, statusCli)
	return nil
}

//GetManager 获取管理器
func GetManager() V2Manager {
	return defaultV2Manager
}

//NewManager new manager
func NewManager(conf option.Config, statusCli *client.AppRuntimeSyncClient) *V2Routes {
	var v2r V2Routes
	v2r.TenantStruct.StatusCli = statusCli
	nodeProxy := proxy.CreateProxy("acp_node", "http", conf.NodeAPI)
	discover.GetEndpointDiscover(conf.EtcdEndpoint).AddProject("acp_node", nodeProxy)
	v2r.AcpNodeStruct.HTTPProxy = nodeProxy
	logrus.Debugf("create  node api proxy success")

	entranceProxy := proxy.CreateProxy("acp_entrance", "http", conf.EntranceAPI)
	discover.GetEndpointDiscover(conf.EtcdEndpoint).AddProject("acp_entrance", entranceProxy)
	v2r.EntranceStruct.HTTPProxy = entranceProxy
	logrus.Debugf("create  entrance api proxy success")
	return &v2r
}
