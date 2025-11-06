// Copyright (C) 2014-2024 Goodrain Co., Ltd.
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

	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/model"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	dbmodel "github.com/goodrain/rainbond/db/model"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
)

// ServiceGPUController 组件GPU配置控制器
type ServiceGPUController struct {
	serviceGPUHandler handler.ServiceGPUHandler
}

// NewServiceGPUController 创建组件GPU配置控制器
func NewServiceGPUController() *ServiceGPUController {
	return &ServiceGPUController{
		serviceGPUHandler: handler.NewServiceGPUHandler(),
	}
}

// GetServiceGPUConfig 获取组件GPU配置
// GET /v2/tenants/{tenant_name}/services/{service_alias}/gpu-config
func (c *ServiceGPUController) GetServiceGPUConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 从上下文获取服务信息
	service := r.Context().Value(ctxutil.ContextKey("service")).(*dbmodel.TenantServices)
	if service == nil {
		httputil.ReturnError(r, w, 400, "service not found in context")
		return
	}

	// 获取组件GPU配置
	config, err := c.serviceGPUHandler.GetServiceGPUConfig(ctx, service.ServiceID)
	if err != nil {
		logrus.Errorf("Failed to get service GPU config: %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}

	httputil.ReturnSuccess(r, w, config)
}

// SetServiceGPUConfig 设置组件GPU配置
// PUT /v2/tenants/{tenant_name}/services/{service_alias}/gpu-config
func (c *ServiceGPUController) SetServiceGPUConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 从上下文获取租户和服务信息
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)
	service := r.Context().Value(ctxutil.ContextKey("service")).(*dbmodel.TenantServices)
	if tenant == nil || service == nil {
		httputil.ReturnError(r, w, 400, "tenant or service not found in context")
		return
	}

	// 解析请求体
	var req model.ServiceGPUConfigReq
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil); !ok {
		return
	}

	// 设置组件GPU配置
	if err := c.serviceGPUHandler.SetServiceGPUConfig(ctx, tenant.UUID, service.ServiceID, &req); err != nil {
		logrus.Errorf("Failed to set service GPU config: %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}

	httputil.ReturnSuccess(r, w, map[string]interface{}{
		"message": "GPU config set successfully",
	})
}

// DeleteServiceGPUConfig 删除组件GPU配置
// DELETE /v2/tenants/{tenant_name}/services/{service_alias}/gpu-config
func (c *ServiceGPUController) DeleteServiceGPUConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 从上下文获取服务信息
	service := r.Context().Value(ctxutil.ContextKey("service")).(*dbmodel.TenantServices)
	if service == nil {
		httputil.ReturnError(r, w, 400, "service not found in context")
		return
	}

	// 删除组件GPU配置
	if err := c.serviceGPUHandler.DeleteServiceGPUConfig(ctx, service.ServiceID); err != nil {
		logrus.Errorf("Failed to delete service GPU config: %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}

	httputil.ReturnSuccess(r, w, map[string]interface{}{
		"message": "GPU config deleted successfully",
	})
}
