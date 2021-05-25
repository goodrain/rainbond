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

	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/middleware"
	"github.com/goodrain/rainbond/api/model"
	dbmodel "github.com/goodrain/rainbond/db/model"
	rutil "github.com/goodrain/rainbond/util"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
)

//BatchOperation batch operation for tenant
//support operation is : start,build,stop,update
func BatchOperation(w http.ResponseWriter, r *http.Request) {
	var build model.BatchOperationReq
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &build.Body, nil)
	if !ok {
		logrus.Errorf("start batch operation validate request body failure")
		return
	}

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		defer rutil.Elapsed("BatchOperation-" + build.Body.Operation)()
	}

	tenant := r.Context().Value(middleware.ContextKey("tenant")).(*dbmodel.Tenants)

	var res model.BatchOpResult
	var err error
	// TODO: merge the code below
	switch build.Body.Operation {
	case "build":
		var batchOpReqs []model.ComponentOpReq
		for _, build := range build.Body.Builds {
			build := build
			build.TenantName = tenant.Name
			batchOpReqs = append(batchOpReqs, build)
		}
		if len(batchOpReqs) > 1024 {
			batchOpReqs = batchOpReqs[0:1024]
		}
		res, err = handler.GetBatchOperationHandler().Build(r.Context(), tenant, build.Operator, batchOpReqs)
	case "start":
		var batchOpReqs []model.ComponentOpReq
		for _, start := range build.Body.Starts {
			start := start
			batchOpReqs = append(batchOpReqs, start)
		}
		if len(batchOpReqs) > 1024 {
			batchOpReqs = batchOpReqs[0:1024]
		}
		res, err = handler.GetBatchOperationHandler().Start(r.Context(), tenant, build.Operator, batchOpReqs)
	case "stop":
		var batchOpReqs []model.ComponentOpReq
		for _, stop := range build.Body.Stops {
			stop := stop
			batchOpReqs = append(batchOpReqs, stop)
		}
		if len(batchOpReqs) > 1024 {
			batchOpReqs = batchOpReqs[0:1024]
		}
		res, err = handler.GetBatchOperationHandler().Stop(r.Context(), tenant, build.Operator, batchOpReqs)
	case "upgrade":
		var batchOpReqs []model.ComponentOpReq
		for _, upgrade := range build.Body.Upgrades {
			upgrade := upgrade
			batchOpReqs = append(batchOpReqs, upgrade)
		}
		if len(batchOpReqs) > 1024 {
			batchOpReqs = batchOpReqs[0:1024]
		}
		res, err = handler.GetBatchOperationHandler().Upgrade(r.Context(), tenant, build.Operator, batchOpReqs)
	default:
		httputil.ReturnError(r, w, 400, fmt.Sprintf("operation %s do not support batch", build.Body.Operation))
		return
	}

	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}

	// append every create event result to re and then return
	httputil.ReturnSuccess(r, w, map[string]interface{}{
		"batch_result": res,
	})
}
