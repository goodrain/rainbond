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
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	httputil "github.com/goodrain/rainbond/util/http"

	"github.com/Sirupsen/logrus"
	"github.com/go-chi/chi"

	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/middleware"
	api_model "github.com/goodrain/rainbond/api/model"
)

//EventLogStruct eventlog struct
type EventLogStruct struct{}

//Logs GetLogs
func (e *EventLogStruct) Logs(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST  /v2/tenants/{tenant_name}/services/{service_alias}/log v2 lastLinesLogs
	//
	// 获取最新指定数量条日志
	//
	// get last x lines logs
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
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	var llines api_model.LastLinesStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &llines.Body, nil)
	if !ok {
		return
	}
	//logrus.Info(llines.Body.Lines)
	if llines.Body.Lines == 0 {
		llines.Body.Lines = 50
	}
	logs, err := handler.GetEventHandler().GetLinesLogs(GetServiceAliasID(serviceID), llines.Body.Lines)
	if err != nil {
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	rc := strings.Split(string(logs), "\n")
	httputil.ReturnSuccess(r, w, rc)
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
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
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
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
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
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
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

//EventsByTarget get log by target
func (e *EventLogStruct) EventsByTarget(w http.ResponseWriter, r *http.Request) {
	target := chi.URLParam(r, "target")
	if strings.TrimSpace(target) == "" {
		httputil.ReturnError(r, w, 400, "target is request")
		return
	}

	targetID := chi.URLParam(r, "targetID")
	if strings.TrimSpace(targetID) == "" {
		httputil.ReturnError(r, w, 400, "targetID is request")
		return
	}

	body, _ := ioutil.ReadAll(r.Body)
	defer r.Body.Close()

	var rbm map[string]int
	if err := json.Unmarshal(body, &rbm); err != nil {
		logrus.Error("parse page param error")
		httputil.ReturnError(r, w, 400, "server parse page param error")
		return
	}

	logrus.Debugf("get event page param[page:%d, page_size:%d]", rbm["page"], rbm["page_size"])

	pageNum := 1
	pageSize := 6
	if rbm["page"] > 0 {
		pageNum = rbm["page"]
	}
	if rbm["page_size"] > 0 {
		pageSize = rbm["page_size"]
	}

	ses, err := handler.GetEventHandler().GetTargetEvents(target, targetID)
	if err != nil {
		logrus.Errorf("get event log error, %v", err)
		httputil.ReturnError(r, w, 500, "get log error")
		return
	}
	re := ses.Paging(pageNum, pageSize)
	httputil.ReturnSuccess(r, w, re)
	return
}

//EventLogByEventID get event log by eventID
func (e *EventLogStruct) EventLogByEventID(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "eventID")
	if strings.TrimSpace(eventID) == "" {
		httputil.ReturnError(r, w, 400, "eventID is request")
		return
	}
	ses, err := handler.GetEventHandler().GetEventLog(eventID)
	if err != nil {
		logrus.Errorf("get event log error, %v", err)
		httputil.ReturnError(r, w, 500, "get log error")
		return
	}
	httputil.ReturnSuccess(r, w, ses)
	return
}
