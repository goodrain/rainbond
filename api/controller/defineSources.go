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

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/middleware"
	api_model "github.com/goodrain/rainbond/api/model"
	httputil "github.com/goodrain/rainbond/util/http"
)

//SetDefineSource SetDefineSource
// swagger:operation POST /v2/tenants/{tenant_name}/sources/{source_alias} v2 setDefineSource
//
// 设置自定义资源
//
// set defineSource
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
//     description: 统一返回格式
func (t *TenantStruct) SetDefineSource(w http.ResponseWriter, r *http.Request) {
	var ss api_model.SetDefineSourcesStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &ss.Body, nil)
	if !ok {
		return
	}
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	//source_alis need legal checking, cant includ "/"
	sourceAlias := chi.URLParam(r, "source_alias")
	ss.Body.SourceSpec.Alias = sourceAlias

	if err := handler.GetSourcesManager().CreateDefineSources(tenantID, &ss); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//UpdateDefineSource UpdateDefineSource
// swagger:operation PUT /v2/tenants/{tenant_name}/sources/{source_alias}/{env_name} v2 updateDefineSource
//
// 更新自定义资源
//
// set defineSource
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
//     description: 统一返回格式
func (t *TenantStruct) UpdateDefineSource(w http.ResponseWriter, r *http.Request) {
	var ss api_model.SetDefineSourcesStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &ss.Body, nil)
	if !ok {
		return
	}
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	//source_alis need legal checking, cant includ "/"
	sourceAlias := chi.URLParam(r, "source_alias")
	envName := chi.URLParam(r, "env_name")
	ss.Body.SourceSpec.Alias = sourceAlias
	ss.Body.SourceSpec.SourceBody.EnvName = envName
	if err := handler.GetSourcesManager().UpdateDefineSources(tenantID, &ss); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//DeleteDefineSource DeleteDefineSource
// swagger:operation DELETE /v2/tenants/{tenant_name}/sources/{source_alias}/{env_name} v2 deleteDefineSource
//
// 设置自定义资源
//
// set defineSource
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
//     description: 统一返回格式
func (t *TenantStruct) DeleteDefineSource(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	//source_alis need legal checking, cant includ "/"
	sourceAlias := chi.URLParam(r, "source_alias")
	envName := chi.URLParam(r, "env_name")
	if err := handler.GetSourcesManager().DeleteDefineSources(tenantID, sourceAlias, envName); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//GetDefineSource GetDefineSource
// swagger:operation GET /v2/tenants/{tenant_name}/sources/{source_alias}/{env_name} v2 getDefineSource
//
// 设置自定义资源
//
// set defineSource
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
//     description: 统一返回格式
func (t *TenantStruct) GetDefineSource(w http.ResponseWriter, r *http.Request) {
	//work for console only.
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	//source_alis need legal checking, cant includ "/"
	sourceAlias := chi.URLParam(r, "source_alias")
	envName := chi.URLParam(r, "env_name")
	ss, err := handler.GetSourcesManager().GetDefineSources(tenantID, sourceAlias, envName)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, ss)
}
