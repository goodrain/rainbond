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
	"github.com/goodrain/rainbond/entrance/api/model"
	"context"
	"fmt"
	"time"

	apistore "github.com/goodrain/rainbond/entrance/api/store"

	restful "github.com/emicklei/go-restful"
)

//PodSource 查询应用实例的端口映射情况
type PodSource struct {
	apiStoreManager *apistore.Manager
}

//Register 注册
func (u PodSource) Register(container *restful.Container) {
	ws := new(restful.WebService)
	ws.Path("/pods/{pod_name}").
		Doc("Get pod some info").
		Param(ws.PathParameter("pod_name", "pod name").DataType("string")).
		Consumes(restful.MIME_XML, restful.MIME_JSON).
		Produces(restful.MIME_JSON, restful.MIME_XML) // you can specify this per route as well

	ws.Route(ws.GET("/ports/{port}/hostport").To(u.findPodHostPort).
		// docs
		Doc("get a host port for pod port").
		Operation("findPodHostPort").
		Param(ws.PathParameter("port", "container port").DataType("string")).
		Writes(ResponseType{
			Body: ResponseBody{
				Bean: model.Domain{},
			},
		})) // on the response
	container.Add(ws)
}

func (u *PodSource) findPodHostPort(request *restful.Request, response *restful.Response) {
	podName := request.PathParameter("pod_name")
	port := request.PathParameter("port")
	if podName == "" || port == "" {
		NewFaliResponse(410, "pod name or port can not be empty", "", response)
		return
	}
	ctx, cancel := context.WithTimeout(request.Request.Context(), time.Second*3)
	defer cancel()
	res, err := u.apiStoreManager.GetV3Client().Get(ctx, fmt.Sprintf("/store/pods/%s/ports/%s/mapport", podName, port))
	if err != nil {
		NewFaliResponse(500, "get host port info from store error", "", response)
		return
	}
	if res.Count == 0 {
		NewFaliResponse(404, "host port info is not exist for this pod ", "", response)
		return
	}
	hostPort := string(res.Kvs[0].Value)
	NewSuccessResponse(map[string]string{"host_port": hostPort, "container_port": port, "pod_name": podName}, nil, response)
}
