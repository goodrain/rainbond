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
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/pquerna/ffjson/ffjson"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/pkg/db"
	"github.com/goodrain/rainbond/pkg/db/model"

	"github.com/goodrain/rainbond/pkg/api/middleware"
	httputil "github.com/goodrain/rainbond/pkg/util/http"
)

//ChargesVerifyController service charges verify
// swagger:operation POST /v2/tenants/{tenant_name}/chargesverify v2 chargesverify
//
// 应用扩大资源申请接口，公有云云市验证，私有云不验证
//
// service charges verify
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
func ChargesVerifyController(w http.ResponseWriter, r *http.Request) {

	if publicCloud := os.Getenv("PUBLIC_CLOUD"); publicCloud != "true" {

		httputil.ReturnSuccess(r, w, nil)
	}
	tenant := r.Context().Value(middleware.ContextKey("tenant")).(*model.Tenants)
	if tenant.EID == "" {
		eid := r.FormValue("eid")
		if eid == "" {
			httputil.ReturnError(r, w, 400, "enterprice id can not found")
			return
		}
		tenant.EID = eid
		db.GetManager().TenantDao().UpdateModel(tenant)
	}
	cloudAPI := os.Getenv("CLOUD_API")
	if cloudAPI == "" {
		cloudAPI = "http://api.goodrain.com"
	}
	quantity := r.FormValue("quantity")
	if quantity == "" {
		httputil.ReturnError(r, w, 400, "quantity  can not found")
		return
	}
	reason := r.FormValue("reason")
	api := fmt.Sprintf("%s/openapi/v1/enterprises/%s/memory-apply?quantity=%s&tid=%s&reason=%s", cloudAPI, tenant.EID, quantity, tenant.UUID, reason)
	req, err := http.NewRequest("GET", api, nil)
	if err != nil {
		logrus.Error("create request cloud api error", err.Error())
		httputil.ReturnError(r, w, 400, "create request cloud api error")
		return
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		logrus.Error("create request cloud api error", err.Error())
		httputil.ReturnError(r, w, 400, "create request cloud api error")
		return
	}
	if res.StatusCode == 200 {
		httputil.ReturnSuccess(r, w, nil)
		return
	}
	if res.Body != nil {
		defer res.Body.Close()
		rebody, _ := ioutil.ReadAll(res.Body)
		var re = make(map[string]interface{})
		if err := ffjson.Unmarshal(rebody, &re); err == nil {
			if msg, ok := re["msg"]; ok {
				httputil.ReturnError(r, w, res.StatusCode, msg.(string))
				return
			}
		}
	}
	httputil.ReturnError(r, w, res.StatusCode, "none")
	return
}
