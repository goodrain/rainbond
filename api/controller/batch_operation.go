// RAINBOND, Application Management Platform
// Copyright (C) 2014-2019 Goodrain Co., Ltd.

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
	"fmt"
	"net/http"

	"github.com/Sirupsen/logrus"

	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/middleware"

	"github.com/goodrain/rainbond/api/model"
	httputil "github.com/goodrain/rainbond/util/http"
)

//BatchOperation batch operation for tenant
//support operation is : start,build,stop,update
func BatchOperation(w http.ResponseWriter, r *http.Request) {
	var build model.BeatchOperationRequestStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &build.Body, nil)
	if !ok {
		logrus.Errorf("start batch operation validate request body failure")
		return
	}
	tenantName := r.Context().Value(middleware.ContextKey("tenant_name")).(string)
	var re handler.BatchOperationResult
	switch build.Body.Operation {
	case "build":
		for i := range build.Body.BuildInfos {
			build.Body.BuildInfos[i].TenantName = tenantName
		}
		re = handler.GetBatchOperationHandler().Build(build.Body.BuildInfos)
	case "start":
		re = handler.GetBatchOperationHandler().Start(build.Body.StartInfos)
	case "stop":
		re = handler.GetBatchOperationHandler().Stop(build.Body.StopInfos)
	case "upgrade":
		re = handler.GetBatchOperationHandler().Upgrade(build.Body.UpgradeInfos)
	default:
		httputil.ReturnError(r, w, 400, fmt.Sprintf("operation %s do not support batch", build.Body.Operation))
		return
	}
	httputil.ReturnSuccess(r, w, re)
}
