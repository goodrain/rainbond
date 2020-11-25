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

package handler

import (
	"github.com/goodrain/rainbond/api/discover"
	"github.com/goodrain/rainbond/api/proxy"
	"github.com/goodrain/rainbond/cmd/api/option"
)

var nodeProxy proxy.Proxy
var builderProxy proxy.Proxy
var prometheusProxy proxy.Proxy
var monitorProxy proxy.Proxy
var kubernetesDashboard proxy.Proxy

//InitProxy 初始化
func InitProxy(conf option.Config) {
	if nodeProxy == nil {
		nodeProxy = proxy.CreateProxy("acp_node", "http", conf.NodeAPI)
		discover.GetEndpointDiscover().AddProject("acp_node", nodeProxy)
	}
	if builderProxy == nil {
		builderProxy = proxy.CreateProxy("builder", "http", conf.BuilderAPI)
	}
	if prometheusProxy == nil {
		prometheusProxy = proxy.CreateProxy("prometheus", "http", []string{conf.PrometheusEndpoint})
	}
	if monitorProxy == nil {
		monitorProxy = proxy.CreateProxy("monitor", "http", []string{"127.0.0.1:3329"})
		discover.GetEndpointDiscover().AddProject("monitor", monitorProxy)
	}
	if kubernetesDashboard == nil {
		kubernetesDashboard = proxy.CreateProxy("kubernetesdashboard", "http", []string{conf.KuberentesDashboardAPI})
	}
}

//GetNodeProxy GetNodeProxy
func GetNodeProxy() proxy.Proxy {
	return nodeProxy
}

//GetBuilderProxy GetNodeProxy
func GetBuilderProxy() proxy.Proxy {
	return builderProxy
}

//GetPrometheusProxy GetPrometheusProxy
func GetPrometheusProxy() proxy.Proxy {
	return prometheusProxy
}

//GetMonitorProxy GetMonitorProxy
func GetMonitorProxy() proxy.Proxy {
	return monitorProxy
}

// GetKubernetesDashboardProxy returns the kubernetes dashboard proxy.
func GetKubernetesDashboardProxy() proxy.Proxy {
	return kubernetesDashboard
}
