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

	"github.com/sirupsen/logrus"

	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/middleware"
	"github.com/goodrain/rainbond/api/util"

	"github.com/goodrain/rainbond/api/model"
	dbmodel "github.com/goodrain/rainbond/db/model"
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
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)

	// create event for each operation
	eventRe := createBatchEvents(&build, tenantID, build.Operator)

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

	// append every create event result to re and then return
	re.BatchResult = append(re.BatchResult, eventRe.BatchResult...)
	httputil.ReturnSuccess(r, w, re)
}

func createBatchEvents(build *model.BeatchOperationRequestStruct, tenantID, operator string) (re handler.BatchOperationResult) {
	for i := range build.Body.BuildInfos {
		event, err := util.CreateEvent(dbmodel.TargetTypeService, "build-service", build.Body.BuildInfos[i].ServiceID, tenantID, "", operator, dbmodel.ASYNEVENTTYPE)
		if err != nil {
			re.BatchResult = append(re.BatchResult, handler.OperationResult{ErrMsg: "create event failure", ServiceID: build.Body.BuildInfos[i].ServiceID})
			continue
		}
		build.Body.BuildInfos[i].EventID = event.EventID

	}
	for i := range build.Body.StartInfos {
		event, err := util.CreateEvent(dbmodel.TargetTypeService, "start-service", build.Body.StartInfos[i].ServiceID, tenantID, "", operator, dbmodel.ASYNEVENTTYPE)
		if err != nil {
			re.BatchResult = append(re.BatchResult, handler.OperationResult{ErrMsg: "create event failure", ServiceID: build.Body.StartInfos[i].ServiceID})
			continue
		}
		build.Body.StartInfos[i].EventID = event.EventID
	}
	for i := range build.Body.StopInfos {
		event, err := util.CreateEvent(dbmodel.TargetTypeService, "stop-service", build.Body.StopInfos[i].ServiceID, tenantID, "", operator, dbmodel.ASYNEVENTTYPE)
		if err != nil {
			re.BatchResult = append(re.BatchResult, handler.OperationResult{ErrMsg: "create event failure", ServiceID: build.Body.StopInfos[i].ServiceID})
			continue
		}
		build.Body.StopInfos[i].EventID = event.EventID
	}
	for i := range build.Body.UpgradeInfos {
		event, err := util.CreateEvent(dbmodel.TargetTypeService, "upgrade-service", build.Body.UpgradeInfos[i].ServiceID, tenantID, "", operator, dbmodel.ASYNEVENTTYPE)
		if err != nil {
			re.BatchResult = append(re.BatchResult, handler.OperationResult{ErrMsg: "create event failure", ServiceID: build.Body.UpgradeInfos[i].ServiceID})
			continue
		}
		build.Body.UpgradeInfos[i].EventID = event.EventID
	}

	return
}
