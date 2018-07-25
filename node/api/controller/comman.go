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

	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/core/config"
	"github.com/goodrain/rainbond/node/core/service"
	"github.com/goodrain/rainbond/node/kubecache"
	"github.com/goodrain/rainbond/node/masterserver"
)

var datacenterConfig *config.DataCenterConfig
var taskService *service.TaskService
var prometheusService *service.PrometheusService
var taskTempService *service.TaskTempService
var taskGroupService *service.TaskGroupService
var appService *service.AppService
var nodeService *service.NodeService
var discoverService *service.DiscoverAction
var kubecli kubecache.KubeClient

//Init 初始化
func Init(c *option.Conf, ms *masterserver.MasterServer, kube kubecache.KubeClient) {
	if ms != nil {
		prometheusService = service.CreatePrometheusService(c, ms)
		taskService = service.CreateTaskService(c, ms)
		taskTempService = service.CreateTaskTempService(c)
		taskGroupService = service.CreateTaskGroupService(c, ms)
		datacenterConfig = config.GetDataCenterConfig()
		nodeService = service.CreateNodeService(c, ms.Cluster, kube)
	}
	appService = service.CreateAppService(c)
	discoverService = service.CreateDiscoverActionManager(c, kube)
	kubecli = kube
}

//Exist 退出
func Exist(i interface{}) {
	if datacenterConfig != nil {
		datacenterConfig.Stop()
	}
}

//Ping Ping
func Ping(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
}
