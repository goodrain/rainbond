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
	api_model "github.com/goodrain/rainbond/api/model"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	httputil "github.com/goodrain/rainbond/util/http"
)

// LoadTarImage 开始解析tar包镜像
func LoadTarImage(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v2/tenants/{tenant_name}/image/load v2 loadTarImage
	//
	// 开始解析tar包镜像
	//
	// load tar image
	//
	// ---
	// consumes:
	// - application/json
	//
	// produces:
	// - application/json
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式
	var req api_model.LoadTarImageReq
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}

	tenantID := r.Context().Value(ctxutil.ContextKey("tenant_id")).(string)

	tarHandler := handler.GetTarImageHandle()
	if tarHandler == nil {
		httputil.ReturnError(r, w, 503, "tar image service is not available")
		return
	}

	res, errS := tarHandler.LoadTarImage(tenantID, req)
	if errS != nil {
		errS.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, res)
}

// GetTarLoadResult 查询tar包解析结果
func GetTarLoadResult(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /v2/tenants/{tenant_name}/image/load/{load_id} v2 getTarLoadResult
	//
	// 查询tar包解析结果
	//
	// get tar load result
	//
	// ---
	// produces:
	// - application/json
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式
	loadID := chi.URLParam(r, "load_id")

	tarHandler := handler.GetTarImageHandle()
	if tarHandler == nil {
		httputil.ReturnError(r, w, 503, "tar image service is not available")
		return
	}

	res, errS := tarHandler.GetTarLoadResult(loadID)
	if errS != nil {
		errS.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, res)
}

// ImportTarImages 确认导入镜像到镜像仓库
func ImportTarImages(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v2/tenants/{tenant_name}/image/import v2 importTarImages
	//
	// 确认导入镜像到镜像仓库
	//
	// import tar images
	//
	// ---
	// consumes:
	// - application/json
	//
	// produces:
	// - application/json
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式
	var req api_model.ImportTarImagesReq
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}

	tenantID := r.Context().Value(ctxutil.ContextKey("tenant_id")).(string)
	tenantName := r.Context().Value(ctxutil.ContextKey("tenant_name")).(string)

	tarHandler := handler.GetTarImageHandle()
	if tarHandler == nil {
		httputil.ReturnError(r, w, 503, "tar image service is not available")
		return
	}

	res, errS := tarHandler.ImportTarImages(tenantID, tenantName, req)
	if errS != nil {
		errS.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, res)
}
