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
	"strconv"

	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"

	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/model"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/db/errors"
	httputil "github.com/goodrain/rainbond/util/http"
)

// AutoscalerRules -
func (t *TenantStruct) AutoscalerRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		t.addAutoscalerRule(w, r)
	case "PUT":
		t.updAutoscalerRule(w, r)
	}
}

func (t *TenantStruct) addAutoscalerRule(w http.ResponseWriter, r *http.Request) {
	var req model.AutoscalerRuleReq
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}

	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	req.ServiceID = serviceID
	if err := handler.GetServiceManager().AddAutoscalerRule(&req); err != nil {
		if err == errors.ErrRecordAlreadyExist {
			httputil.ReturnError(r, w, 400, err.Error())
			return
		}
		logrus.Errorf("add autoscaler rule: %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}

	httputil.ReturnSuccess(r, w, nil)
}

func (t *TenantStruct) updAutoscalerRule(w http.ResponseWriter, r *http.Request) {
	var req model.AutoscalerRuleReq
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}

	if err := handler.GetServiceManager().UpdAutoscalerRule(&req); err != nil {
		if err == errors.ErrRecordAlreadyExist {
			httputil.ReturnError(r, w, 400, err.Error())
			return
		}
		if err == gorm.ErrRecordNotFound {
			httputil.ReturnError(r, w, 404, err.Error())
			return
		}
		logrus.Errorf("update autoscaler rule: %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}

	httputil.ReturnSuccess(r, w, nil)
}

// ScalingRecords -
func (t *TenantStruct) ScalingRecords(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		t.listScalingRecords(w, r)
	}
}

func (t *TenantStruct) listScalingRecords(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		logrus.Warningf("convert '%s(pageStr)' to int: %v", pageStr, err)
	}
	if page <= 0 {
		page = 1
	}

	pageSizeStr := r.URL.Query().Get("page_size")
	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil {
		logrus.Warningf("convert '%s(pageSizeStr)' to int: %v", pageSizeStr, err)
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	records, count, err := handler.GetServiceManager().ListScalingRecords(serviceID, page, pageSize)
	if err != nil {
		logrus.Errorf("list scaling rule: %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}

	httputil.ReturnSuccess(r, w, map[string]interface{}{
		"total": count,
		"data":  records,
	})
}
