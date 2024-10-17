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
	"context"
	"fmt"
	"github.com/goodrain/rainbond/api/middleware"
	"github.com/goodrain/rainbond/db"
	"net/http"

	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/model"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	dbmodel "github.com/goodrain/rainbond/db/model"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
)

// BatchOperation batch operation for tenant
// support operation is : start,build,stop,update
func BatchOperation(w http.ResponseWriter, r *http.Request) {
	var build model.BatchOperationReq
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &build.Body, nil)
	if !ok {
		logrus.Errorf("start batch operation validate request body failure")
		return
	}

	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)

	var batchOpReqs []model.ComponentOpReq
	var f func(ctx context.Context, tenant *dbmodel.Tenants, operator string, batchOpReqs model.BatchOpRequesters) (model.BatchOpResult, error)
	switch build.Body.Operation {
	case "build":
		err := middleware.LicenseVerification(r, true)
		if err != nil {
			err.Handle(r, w)
			return
		}
		for _, build := range build.Body.Builds {
			build.TenantName = tenant.Name
			batchOpReqs = append(batchOpReqs, build)
		}
		checkError := CheckComponentSource(r.Context(), tenant, nil, batchOpReqs)
		if checkError != nil {
			httputil.ReturnResNotEnough(r, w, "", checkError.Error())
			return
		}
		f = handler.GetBatchOperationHandler().Build
	case "start":
		err := middleware.LicenseVerification(r, true)
		if err != nil {
			err.Handle(r, w)
			return
		}
		for _, start := range build.Body.Starts {
			batchOpReqs = append(batchOpReqs, start)
		}
		checkError := CheckComponentSource(r.Context(), tenant, nil, batchOpReqs)
		if checkError != nil {
			httputil.ReturnResNotEnough(r, w, "", checkError.Error())
			return
		}
		f = handler.GetBatchOperationHandler().Start
	case "stop":
		err := middleware.LicenseVerification(r, true)
		if err != nil {
			err.Handle(r, w)
			return
		}
		for _, stop := range build.Body.Stops {
			batchOpReqs = append(batchOpReqs, stop)
		}
		f = handler.GetBatchOperationHandler().Stop
	case "upgrade":
		err := middleware.LicenseVerification(r, true)
		if err != nil {
			err.Handle(r, w)
			return
		}
		for _, upgrade := range build.Body.Upgrades {
			batchOpReqs = append(batchOpReqs, upgrade)
		}
		b := handler.GetBatchOperationHandler()
		f = b.Upgrade
	case "export":
		err := middleware.LicenseVerification(r, true)
		if err != nil {
			err.Handle(r, w)
			return
		}
		for _, build := range build.Body.Builds {
			build.TenantName = tenant.Name
			batchOpReqs = append(batchOpReqs, build)
		}
		b := handler.GetBatchOperationHandler()
		b.DryRun = true
		b.HelmChart = build.Body.HelmChart
		f = b.Build
	default:
		httputil.ReturnError(r, w, 400, fmt.Sprintf("operation %s do not support batch", build.Body.Operation))
		return
	}
	if len(batchOpReqs) > 1024 {
		batchOpReqs = batchOpReqs[0:1024]
	}
	res, err := f(r.Context(), tenant, build.Body.Operator, batchOpReqs)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}

	// append every create event result to re and then return
	httputil.ReturnSuccess(r, w, map[string]interface{}{
		"batch_result": res,
	})
}

func CheckComponentSource(ctx context.Context, tenant *dbmodel.Tenants, componentReq *model.SyncComponentReq, batchOpReqs model.BatchOpRequesters) error {
	// componentReq  Check the application resources installed in the application market
	// batchOpReqs  Check resources for batch operations
	var needMemory, needCPU, needStorage, noMemory, noCPU int
	if batchOpReqs != nil {
		componentIDs := batchOpReqs.ComponentIDs()
		services, err := db.GetManager().TenantServiceDao().GetServiceByIDs(componentIDs)
		if err != nil {
			return err
		}
		for _, service := range services {
			if service.ContainerCPU == 0 {
				noCPU += service.Replicas
			}
			if service.ContainerMemory == 0 {
				noMemory += service.Replicas
			}
			needMemory += service.Replicas * service.ContainerMemory
			needCPU += service.Replicas * service.ContainerCPU
		}
		storages, err := db.GetManager().TenantServiceVolumeDao().ListVolumesByComponentIDs(componentIDs)
		if err != nil {
			return err
		}
		if storages != nil && len(storages) > 0 {
			for _, storage := range storages {
				needStorage += int(storage.VolumeCapacity)
			}
		}
	} else {
		var componentIDs []string
		for _, components := range componentReq.Components {
			if components.ComponentBase.ContainerCPU == 0 {
				noCPU += components.ComponentBase.Replicas
			}
			if components.ComponentBase.ContainerMemory == 0 {
				noMemory += components.ComponentBase.Replicas
			}
			needMemory += components.ComponentBase.Replicas * components.ComponentBase.ContainerMemory
			needCPU += components.ComponentBase.Replicas * components.ComponentBase.ContainerCPU
			componentIDs = append(componentIDs, components.ComponentBase.ComponentID)
		}
		storages, err := db.GetManager().TenantServiceVolumeDao().ListVolumesByComponentIDs(componentIDs)
		if err != nil {
			return err
		}
		if storages != nil && len(storages) > 0 {
			for _, storage := range storages {
				needStorage += int(storage.VolumeCapacity)
			}
		}
	}
	if err := handler.CheckTenantResource(ctx, tenant, needMemory, needCPU, needStorage, noMemory, noCPU); err != nil {
		return err
	}
	return nil
}
