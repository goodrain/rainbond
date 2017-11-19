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

	"github.com/goodrain/rainbond/pkg/api/handler"
	"github.com/goodrain/rainbond/pkg/api/middleware"
	api_model "github.com/goodrain/rainbond/pkg/api/model"
	httputil "github.com/goodrain/rainbond/pkg/util/http"
)

//SetDefineSource SetDefineSource
// swagger:operation POST /v2/tenant/{tenant_name}/sources/{source-type} v2 SetDefineSource
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
	sourceAlias := r.Context().Value(middleware.ContextKey("source_alias")).(string)
	ss.Body.SourceSpec.Alias = sourceAlias

	if err := handler.GetSourcesManager().CreateDefineSources(tenantID, &ss); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}
