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
	"net/http"
	"strings"

	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/middleware"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	httputil "github.com/goodrain/rainbond/util/http"
)

//Pods get some service pods
// swagger:operation GET /v2/tenants/{tenant_name}/pods v2/tenants pods
//
// 获取一些应用的Pod信息
//
// get some service pods
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: get some service pods
func Pods(w http.ResponseWriter, r *http.Request) {
	serviceIDs := strings.Split(r.FormValue("service_ids"), ",")
	if serviceIDs == nil || len(serviceIDs) == 0 {
		tenant := r.Context().Value(middleware.ContextKey("tenant")).(*model.Tenants)
		services, _ := db.GetManager().TenantServiceDao().GetServicesByTenantID(tenant.UUID)
		for _, s := range services {
			serviceIDs = append(serviceIDs, s.ServiceID)
		}
	}
	var allpods []*handler.K8sPodInfo
	for _, serviceID := range serviceIDs {
		pods, _ := handler.GetServiceManager().GetPods(serviceID)
		allpods = append(allpods, pods...)
	}
	httputil.ReturnSuccess(r, w, allpods)
}
