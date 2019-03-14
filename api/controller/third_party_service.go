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
	"encoding/json"
	"github.com/Sirupsen/logrus"
	"net/http"

	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/middleware"
	"github.com/goodrain/rainbond/api/model"
	httputil "github.com/goodrain/rainbond/util/http"
)

// ThirdPartyServiceController implements ThirdPartyServicer
type ThirdPartyServiceController struct{}

// Endpoints POST->add endpoints, PUT->update endpoints, DELETE->delete endpoints
func (t *ThirdPartyServiceController) Endpoints(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		t.addEndpoints(w, r)
	case "PUT":
		t.updEndpoints(w, r)
	case "DELETE":
		t.delEndpoints(w, r)
	case "GET":
		t.listEndpoints(w, r)
	}
}

func (t *ThirdPartyServiceController) addEndpoints(w http.ResponseWriter, r *http.Request) {
	var data model.AddEndpiontsReq
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &data, nil) {
		return
	}
	sid := r.Context().Value(middleware.ContextKey("service_id")).(string)
	if err := handler.Get3rdPartySvcHandler().AddEndpoints(sid, &data); err != nil {
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, "success")
}

func (t *ThirdPartyServiceController) updEndpoints(w http.ResponseWriter, r *http.Request) {
	var data model.UpdEndpiontsReq
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &data, nil) {
		return
	}
	if err := handler.Get3rdPartySvcHandler().UpdEndpoints(&data); err != nil {
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, "success")
}

func (t *ThirdPartyServiceController) delEndpoints(w http.ResponseWriter, r *http.Request) {
	var data model.DelEndpiontsReq
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &data, nil) {
		return
	}
	sid := r.Context().Value(middleware.ContextKey("service_id")).(string)
	if err := handler.Get3rdPartySvcHandler().DelEndpoints(data.EpID, sid); err != nil {
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, "success")
}

func (t *ThirdPartyServiceController) listEndpoints(w http.ResponseWriter, r *http.Request) {
	sid := r.Context().Value(middleware.ContextKey("service_id")).(string)
	res, err := handler.Get3rdPartySvcHandler().ListEndpoints(sid)
	if err != nil {
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	b, _ := json.Marshal(res)
	logrus.Debugf("response endpoints: %s", string(b))
	if res == nil || len(res) == 0 {
		httputil.ReturnSuccess(r, w, []*model.EndpointResp{})
		return
	}
	httputil.ReturnSuccess(r, w, res)
}
