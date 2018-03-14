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

	api_model "github.com/goodrain/rainbond/pkg/api/model"

	"github.com/Sirupsen/logrus"

	"github.com/goodrain/rainbond/pkg/api/handler"
	httputil "github.com/goodrain/rainbond/pkg/util/http"
)

//OpentsdbStruct OpentsdbStruct
type OpentsdbStruct struct{}

//TsdbQuery query data
func (tsdb *OpentsdbStruct) TsdbQuery(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v2/opentsdb/query v2 oentsdbquery
	//
	// 监控数据查询
	//
	// query opentsdb
	//
	// ---
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式
	logrus.Debugf("trans v2 opentsdb query")
	var data api_model.MontiorData
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &(data.Body), nil) {
		return
	}
	//res, err := handler.GetTenantManager().QueryTsdb(&data)
	body, err := handler.GetTenantManager().HTTPTsdb(&data)
	if err != nil {
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	rc := make(map[string]string)
	rc["body"] = string(body)
	httputil.ReturnSuccess(r, w, rc)
}
