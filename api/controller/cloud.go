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
	httputil "github.com/goodrain/rainbond/util/http"
)

//CloudManager cloud manager
type CloudManager struct{}

var defaultCloudManager *CloudManager

//GetCloudRouterManager get cloud Manager
func GetCloudRouterManager() *CloudManager {
	if defaultCloudManager != nil {
		return defaultCloudManager
	}
	defaultCloudManager = &CloudManager{}
	return defaultCloudManager
}

//Show Show
func (c *CloudManager) Show(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("cloud urls"))
}

//CreateToken CreateToken
// swagger:operation POST /cloud/auth cloud createToken
//
// 产生token
//
// create token
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
func (c *CloudManager) CreateToken(w http.ResponseWriter, r *http.Request) {
	var gt api_model.GetUserToken
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &gt.Body, nil); !ok {
		return
	}
	ti, err := handler.GetCloudManager().TokenDispatcher(&gt)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, ti)
}

//GetTokenInfo GetTokenInfo
// swagger:operation GET /cloud/auth/{eid} cloud getTokenInfo
//
// 获取tokeninfo
//
// get token info
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
func (c *CloudManager) GetTokenInfo(w http.ResponseWriter, r *http.Request) {
	eid := chi.URLParam(r, "eid")
	ti, err := handler.GetCloudManager().GetTokenInfo(eid)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, ti)
}

//UpdateToken UpdateToken
// swagger:operation PUT /cloud/auth/{eid} cloud updateToken
//
// 更新token
//
// update token info
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
func (c *CloudManager) UpdateToken(w http.ResponseWriter, r *http.Request) {
	var ut api_model.UpdateToken
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &ut.Body, nil); !ok {
		return
	}
	eid := chi.URLParam(r, "eid")
	err := handler.GetCloudManager().UpdateTokenTime(eid, ut.Body.ValidityPeriod)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//GetAPIManager GetAPIManager
// swagger:operation GET /cloud/api/manager cloud GetAPIManager
//
// 获取api管理
//
// get api manager
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
func (c *CloudManager) GetAPIManager(w http.ResponseWriter, r *http.Request) {
	apiMap := handler.GetTokenIdenHandler().GetAPIManager()
	httputil.ReturnSuccess(r, w, apiMap)
}

//AddAPIManager AddAPIManager
// swagger:operation POST /cloud/api/manager cloud addAPIManager
//
// 添加api管理
//
// get api manager
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
func (c *CloudManager) AddAPIManager(w http.ResponseWriter, r *http.Request) {
	var am api_model.APIManager
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &am.Body, nil); !ok {
		return
	}
	err := handler.GetTokenIdenHandler().AddAPIManager(&am)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//DeleteAPIManager DeleteAPIManager
// swagger:operation DELETE /cloud/api/manager cloud deleteAPIManager
//
// 删除api管理
//
// delete api manager
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
func (c *CloudManager) DeleteAPIManager(w http.ResponseWriter, r *http.Request) {
	var am api_model.APIManager
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &am.Body, nil); !ok {
		return
	}
	err := handler.GetTokenIdenHandler().DeleteAPIManager(&am)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}
