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
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"

	httputil "github.com/goodrain/rainbond/util/http"

	"github.com/go-chi/chi"
	"github.com/sirupsen/logrus"

	"github.com/goodrain/rainbond/api/handler"
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/proxy"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
)

//EventLogStruct eventlog struct
type EventLogStruct struct {
	EventlogServerProxy proxy.Proxy
}

//HistoryLogs get service history logs
//proxy
func (e *EventLogStruct) HistoryLogs(w http.ResponseWriter, r *http.Request) {
	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	serviceAlias := r.Context().Value(ctxutil.ContextKey("service_alias")).(string)
	name, _ := handler.GetEventHandler().GetLogInstance(serviceID)
	if name != "" {
		r.URL.Query().Add("host_id", name)
		r = r.WithContext(context.WithValue(r.Context(), proxy.ContextKey("host_id"), name))
	}
	//Replace service alias to service id in path
	r.URL.Path = strings.Replace(r.URL.Path, serviceAlias, serviceID, 1)
	r.URL.Path = strings.Replace(r.URL.Path, "/v2/", "/", 1)
	e.EventlogServerProxy.Proxy(w, r)
}

//LogList GetLogList
func (e *EventLogStruct) LogList(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET  /v2/tenants/{tenant_name}/services/{service_alias}/log-file v2 logList
	//
	// 获取应用日志列表
	//
	// get log list
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
	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	fileList, err := handler.GetEventHandler().GetLogList(GetServiceAliasID(serviceID))
	if err != nil {
		if os.IsNotExist(err) {
			httputil.ReturnError(r, w, 404, err.Error())
			return
		}
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, fileList)
	return
}

//LogFile GetLogFile
func (e *EventLogStruct) LogFile(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /v2/tenants/{tenant_name}/services/{service_alias}/log-file/{file_name} v2 logFile
	//
	// 下载应用指定日志
	//
	// get log file
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

	fileName := chi.URLParam(r, "file_name")
	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	logPath, _, err := handler.GetEventHandler().GetLogFile(GetServiceAliasID(serviceID), fileName)
	if err != nil {
		if os.IsNotExist(err) {
			httputil.ReturnError(r, w, 404, err.Error())
			return
		}
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	http.StripPrefix(fileName, http.FileServer(http.Dir(logPath)))
	//fs.ServeHTTP(w, r)
}

//LogSocket GetLogSocket
func (e *EventLogStruct) LogSocket(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /v2/tenants/{tenant_name}/services/{service_alias}/log-instance v2 logSocket
	//
	// 获取应用日志web-socket实例
	//
	// get log socket
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
	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	value, err := handler.GetEventHandler().GetLogInstance(serviceID)
	if err != nil {
		if strings.Contains(err.Error(), "Key not found") {
			httputil.ReturnError(r, w, 404, err.Error())
			return
		}
		logrus.Errorf("get docker log instance error. %s", err.Error())
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	rc := make(map[string]string)
	rc["host_id"] = value
	httputil.ReturnSuccess(r, w, rc)
	return
}

//LogByAction GetLogByAction
func (e *EventLogStruct) LogByAction(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v2/tenants/{tenant_name}/services/{service_alias}/event-log v2 logByAction
	//
	// 获取指定操作的操作日志
	//
	// get log by level
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
	var elog api_model.LogByLevelStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &elog.Body, nil)
	if !ok {
		return
	}
	dl, err := handler.GetEventHandler().GetLevelLog(elog.Body.EventID, elog.Body.Level)
	if err != nil {
		logrus.Errorf("get event log error, %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, dl.Data)
	return
}

//TenantLogByAction GetTenantLogByAction
// swagger:operation POST /v2/tenants/{tenant_name}/event-log v2 tenantLogByAction
//
// 获取指定操作的操作日志
//
// get tenant log by level
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
func (e *EventLogStruct) TenantLogByAction(w http.ResponseWriter, r *http.Request) {
	var elog api_model.TenantLogByLevelStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &elog.Body, nil)
	if !ok {
		return
	}
	logrus.Info(elog.Body.Level)
	dl, err := handler.GetEventHandler().GetLevelLog(elog.Body.EventID, elog.Body.Level)
	if err != nil {
		logrus.Errorf("get tenant event log error, %v", err)
		httputil.ReturnError(r, w, 200, "success")
		return
	}
	httputil.ReturnSuccess(r, w, dl.Data)
	return
}

//Events get log by target
func (e *EventLogStruct) Events(w http.ResponseWriter, r *http.Request) {
	target := r.FormValue("target")
	targetID := r.FormValue("target-id")
	var page, size int
	var err error
	if page, err = strconv.Atoi(r.FormValue("page")); err != nil || page <= 0 {
		page = 1
	}
	if size, err = strconv.Atoi(r.FormValue("size")); err != nil || size <= 0 {
		size = 10
	}
	logrus.Debugf("get event page param[target:%s id:%s page:%d, page_size:%d]", target, targetID, page, size)
	list, total, err := handler.GetEventHandler().GetEvents(target, targetID, page, size)
	if err != nil {
		logrus.Errorf("get event log error, %v", err)
		httputil.ReturnError(r, w, 500, "get log error")
		return
	}
	// format start and end time
	for i := range list {
		if list[i].EndTime != "" && len(list[i].EndTime) > 20 {
			list[i].EndTime = strings.Replace(list[i].EndTime[0:19]+"+08:00", " ", "T", 1)
		}
	}
	httputil.ReturnList(r, w, total, page, list)
}

//EventLog get event log by eventID
func (e *EventLogStruct) EventLog(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "eventID")
	if strings.TrimSpace(eventID) == "" {
		httputil.ReturnError(r, w, 400, "eventID is request")
		return
	}

	dl, err := handler.GetEventHandler().GetLevelLog(eventID, "debug")
	if err != nil {
		logrus.Errorf("get event log error, %v", err)
		httputil.ReturnError(r, w, 500, "read event log error: "+err.Error())
		return
	}

	httputil.ReturnSuccess(r, w, dl.Data)
	return
}
