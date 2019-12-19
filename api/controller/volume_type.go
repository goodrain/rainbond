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
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	api_model "github.com/goodrain/rainbond/api/model"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/jinzhu/gorm"
)

// VolumeOptions list volume option
func VolumeOptions(w http.ResponseWriter, r *http.Request) {
	volumetypeOptions, err := handler.GetVolumeTypeHandler().GetAllVolumeTypes()
	if err != nil {
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, volumetypeOptions)
}

// ListVolumeType list volume type list
func ListVolumeType(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v2/volume-options v2 volumeOptions TODO fanyangyang delete it
	//
	// 查询可用存储驱动模型列表
	//
	// get volume-options
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
	//     description: 统一返回格式
	pageStr := strings.TrimSpace(chi.URLParam(r, "page"))
	pageSizeCul := strings.TrimSpace(chi.URLParam(r, "pageSize"))
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		httputil.ReturnError(r, w, 400, fmt.Sprintf("bad request, %v", err))
		return
	}
	pageSize, err := strconv.Atoi(pageSizeCul)
	if err != nil {
		httputil.ReturnError(r, w, 400, fmt.Sprintf("bad request, %v", err))
		return
	}
	volumetypeOptions, er := handler.GetVolumeTypeHandler().GetAllVolumeTypes()
	volumetypePageOptions, err := handler.GetVolumeTypeHandler().GetAllVolumeTypesByPage(page, pageSize)
	if err != nil || er != nil {
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}

	// httputil.ReturnSuccess(r, w, volumetypeOptions)
	httputil.ReturnSuccess(r, w, map[string]interface{}{
		"data":      volumetypePageOptions,
		"page":      page,
		"page_size": pageSize,
		"count":     len(volumetypeOptions),
	})
}

// VolumeSetVar set volume option
func VolumeSetVar(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v2/volume-options v2 volumeOptions
	//
	// 创建可用存储驱动模型列表
	//
	// get volume-options
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
	//     description: 统一返回格式
	volumeType := api_model.VolumeTypeStruct{}
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &volumeType, nil); !ok {
		return
	}
	err := handler.GetVolumeTypeHandler().SetVolumeType(&volumeType)
	if err != nil {
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

// DeleteVolumeType delete volume option
func DeleteVolumeType(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v2/volume-options v2 volumeOptions
	//
	// 删除可用存储驱动模型
	//
	// get volume-options
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
	//     description: 统一返回格式
	volumeType := chi.URLParam(r, "volume_type")
	err := handler.GetVolumeTypeHandler().DeleteVolumeType(volumeType)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.ReturnError(r, w, 404, "not found")
			return
		}
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

// UpdateVolumeType delete volume option
func UpdateVolumeType(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v2/volume-options v2 volumeOptions
	//
	// 可用更新存储驱动模型
	//
	// get volume-options
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
	//     description: 统一返回格式
	volumeTypeID := chi.URLParam(r, "volume_type")
	volumeType := api_model.VolumeTypeStruct{}
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &volumeType, nil); !ok {
		return
	}
	volume, err := handler.GetVolumeTypeHandler().GetVolumeTypeByType(volumeTypeID)
	if err == nil {
		if volume == nil {
			httputil.ReturnError(r, w, 404, "not found")
			return
		}
		if updateErr := handler.GetVolumeTypeHandler().UpdateVolumeType(volume, &volumeType); updateErr != nil {
			httputil.ReturnError(r, w, 500, err.Error())
		}
		httputil.ReturnSuccess(r, w, nil)
	}
	httputil.ReturnError(r, w, 500, err.Error())
}
