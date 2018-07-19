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
	apistore "github.com/goodrain/rainbond/entrance/api/store"
	"github.com/emicklei/go-restful"
)

//PodSource 查询应用实例的端口映射情况
type HealthStatus struct {
	apiStoreManager *apistore.Manager
}

//Register 注册
func (h HealthStatus) Register(container *restful.Container) {
	ws := new(restful.WebService)
	ws.Path("/health").
		Doc("Get service health").
		Param(ws.PathParameter("pod_name", "pod name").DataType("string")).
		Consumes(restful.MIME_XML, restful.MIME_JSON).
		Produces(restful.MIME_JSON, restful.MIME_XML) // you can specify this per route as well

	ws.Route(ws.GET("/").To(h.healthCheck)) // on the response
	container.Add(ws)
}

func (h *HealthStatus) healthCheck(request *restful.Request, response *restful.Response) {
	NewSuccessResponse(map[string]string{"status": "health", "info": "entrance service health"}, nil, response)
}
