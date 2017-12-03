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
	api_model "github.com/goodrain/rainbond/pkg/api/model"
	httputil "github.com/goodrain/rainbond/pkg/util/http"
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

//GetToken GetToken
// swagger:operation POST /cloud/auth cloud getToken
//
// 获取token
//
// get token
//
// ---
// produces:
// - application/json
// - application/xml
// parameters:
// - name: license
//   in: form
//   description: license
//   required: true
//   type: string
//   format: string
//
// Responses:
//   '200':
func (c *CloudManager) GetToken(w http.ResponseWriter, r *http.Request) {
	var gt api_model.GetUserToken
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &gt, nil); !ok {
		return
	}
	ti, err := handler.GetCloudManager().TokenDispatcher(&gt)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, ti)
}
