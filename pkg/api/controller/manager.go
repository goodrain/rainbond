
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

package controller

import (
	"github.com/goodrain/rainbond/cmd/api/option"
	"github.com/goodrain/rainbond/pkg/api/apiFunc"
	"github.com/goodrain/rainbond/pkg/api/discover"
	"github.com/goodrain/rainbond/pkg/api/proxy"
	"net/http"

	"github.com/Sirupsen/logrus"
)

//V2Manager v2 manager
type V2Manager interface {
	Show(w http.ResponseWriter, r *http.Request)
	Nodes(w http.ResponseWriter, r *http.Request)
	Jobs(w http.ResponseWriter, r *http.Request)
	Apps(w http.ResponseWriter, r *http.Request)
	Entrance(w http.ResponseWriter, r *http.Request)
	TsdbQuery(w http.ResponseWriter, r *http.Request)

	apiFunc.TenantInterface
	apiFunc.ServiceInterface
	apiFunc.LogInterface
	apiFunc.PluginInterface
}

var defaultV2Manager V2Manager

//CreateV2RouterManager 创建manager
func CreateV2RouterManager(conf option.Config) error {
	defaultV2Manager = NewManager(conf)
	return nil
}

//GetManager 获取管理器
func GetManager() V2Manager {
	return defaultV2Manager
}

//NewManager new manager
func NewManager(conf option.Config) *V2Routes {
	var v2r V2Routes
	nodeProxy := proxy.CreateProxy("acp_node", "http", conf.NodeAPI)
	discover.GetEndpointDiscover(conf.EtcdEndpoint).AddProject("acp_node", nodeProxy)
	v2r.AcpNodeStruct.HTTPProxy = nodeProxy
	logrus.Debugf("v2r node api is %v", nodeProxy)

	entranceProxy := proxy.CreateProxy("acp_entrance", "http", conf.EntranceAPI)
	discover.GetEndpointDiscover(conf.EtcdEndpoint).AddProject("acp_entrance", entranceProxy)
	v2r.EntranceStruct.HTTPProxy = entranceProxy
	logrus.Debugf("v2r entrance api is %v", entranceProxy)
	return &v2r
}
