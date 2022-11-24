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

package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/util"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

var pool []string

func init() {
	pool = []string{
		"services_status",
	}
}

//InitTenant 实现中间件
func InitTenant(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		debugRequestBody(r)

		tenantName := chi.URLParam(r, "tenant_name")
		if tenantName == "" {
			httputil.ReturnError(r, w, 404, "cant find tenant")
			return
		}
		tenant, err := db.GetManager().TenantDao().GetTenantIDByName(tenantName)
		if err != nil {
			logrus.Errorf("get tenant by tenantName error: %s %v", tenantName, err)
			if err.Error() == gorm.ErrRecordNotFound.Error() {
				httputil.ReturnError(r, w, 404, "cant find tenant")
				return
			}
			httputil.ReturnError(r, w, 500, "get assign tenant uuid failed")
			return
		}

		ctx := context.WithValue(r.Context(), ctxutil.ContextKey("tenant_name"), tenantName)
		ctx = context.WithValue(ctx, ctxutil.ContextKey("tenant_id"), tenant.UUID)
		ctx = context.WithValue(ctx, ctxutil.ContextKey("tenant"), tenant)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

//InitService 实现serviceinit中间件
func InitService(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		serviceAlias := chi.URLParam(r, "service_alias")
		if serviceAlias == "" {
			httputil.ReturnError(r, w, 404, "cant find service alias")
			return
		}
		tenantID := r.Context().Value(ctxutil.ContextKey("tenant_id"))
		service, err := db.GetManager().TenantServiceDao().GetServiceByTenantIDAndServiceAlias(tenantID.(string), serviceAlias)
		if err != nil {
			if err.Error() == gorm.ErrRecordNotFound.Error() {
				httputil.ReturnError(r, w, 404, "cant find service")
				return
			}
			logrus.Errorf("get service by tenant & service alias error, %v", err)
			httputil.ReturnError(r, w, 500, "get service id error")
			return
		}
		serviceID := service.ServiceID
		ctx := context.WithValue(r.Context(), ctxutil.ContextKey("service_alias"), serviceAlias)
		ctx = context.WithValue(ctx, ctxutil.ContextKey("service_id"), serviceID)
		ctx = context.WithValue(ctx, ctxutil.ContextKey("service"), service)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

// InitApplication -
func InitApplication(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		appID := chi.URLParam(r, "app_id")
		tenantApp, err := handler.GetApplicationHandler().GetAppByID(appID)
		if err != nil {
			httputil.ReturnBcodeError(r, w, err)
			return
		}

		ctx := context.WithValue(r.Context(), ctxutil.ContextKey("app_id"), tenantApp.AppID)
		ctx = context.WithValue(ctx, ctxutil.ContextKey("application"), tenantApp)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

//InitPlugin 实现plugin init中间件
func InitPlugin(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		debugRequestBody(r)

		pluginID := chi.URLParam(r, "plugin_id")
		tenantID := r.Context().Value(ctxutil.ContextKey("tenant_id")).(string)
		if pluginID == "" {
			httputil.ReturnError(r, w, 404, "need plugin id")
			return
		}
		_, err := db.GetManager().TenantPluginDao().GetPluginByID(pluginID, tenantID)
		if err != nil {
			if err.Error() == gorm.ErrRecordNotFound.Error() {
				httputil.ReturnError(r, w, 404, "cant find plugin")
				return
			}
			logrus.Errorf("get plugin error, %v", err)
			httputil.ReturnError(r, w, 500, "get plugin error")
			return
		}
		ctx := context.WithValue(r.Context(), ctxutil.ContextKey("plugin_id"), pluginID)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

//SetLog SetLog
func SetLog(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		eventID := chi.URLParam(r, "event_id")
		if eventID != "" {
			logger := event.GetManager().GetLogger(eventID)
			ctx := context.WithValue(r.Context(), ctxutil.ContextKey("logger"), logger)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	}
	return http.HandlerFunc(fn)
}

//Proxy 反向代理中间件
func Proxy(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.RequestURI, "/v2/nodes") {
			handler.GetNodeProxy().Proxy(w, r)
			return
		}
		if strings.HasPrefix(r.RequestURI, "/v2/cluster/service-health") {
			handler.GetNodeProxy().Proxy(w, r)
			return
		}
		if strings.HasPrefix(r.RequestURI, "/v2/builder") {
			handler.GetBuilderProxy().Proxy(w, r)
			return
		}
		if strings.HasPrefix(r.RequestURI, "/v2/tasks") {
			handler.GetNodeProxy().Proxy(w, r)
			return
		}
		if strings.HasPrefix(r.RequestURI, "/v2/tasktemps") {
			handler.GetNodeProxy().Proxy(w, r)
			return
		}
		if strings.HasPrefix(r.RequestURI, "/v2/taskgroups") {
			handler.GetNodeProxy().Proxy(w, r)
			return
		}
		if strings.HasPrefix(r.RequestURI, "/v2/configs") {
			handler.GetNodeProxy().Proxy(w, r)
			return
		}
		if strings.HasPrefix(r.RequestURI, "/v2/rules") {
			handler.GetMonitorProxy().Proxy(w, r)
			return
		}
		if strings.HasPrefix(r.RequestURI, "/kubernetes/dashboard") {
			proxy := handler.GetKubernetesDashboardProxy()
			r.URL.Path = strings.Replace(r.URL.Path, "/kubernetes/dashboard", "", 1)
			proxy.Proxy(w, r)
			return
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

func apiExclude(r *http.Request) bool {
	if r.Method == "GET" {
		return true
	}
	for _, item := range pool {
		if strings.Contains(r.RequestURI, item) {
			return true
		}
	}
	return false
}

type resWriter struct {
	origWriter http.ResponseWriter
	statusCode int
}

func (w *resWriter) Header() http.Header {
	return w.origWriter.Header()
}
func (w *resWriter) Write(p []byte) (int, error) {
	return w.origWriter.Write(p)
}
func (w *resWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.origWriter.WriteHeader(statusCode)
}

// WrapEL wrap eventlog, handle event log before and after process
func WrapEL(f http.HandlerFunc, target, optType string, synType int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			serviceKind  string
		)
		serviceObj := r.Context().Value(ctxutil.ContextKey("service"))
		if serviceObj != nil {
			service := serviceObj.(*dbmodel.TenantServices)
			serviceKind = service.Kind
		}

		if r.Method != "GET" {
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				logrus.Warningf("error reading request body: %v", err)
			} else {
				logrus.Debugf("method: %s; uri: %s; body: %s", r.Method, r.RequestURI, string(body))
			}
			// set a new body, which will simulate the same data we read
			r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
			var targetID string
			var ok bool
			if targetID, ok = r.Context().Value(ctxutil.ContextKey("service_id")).(string); !ok {
				var reqDataMap map[string]interface{}
				if err = json.Unmarshal(body, &reqDataMap); err != nil {
					httputil.ReturnError(r, w, 400, "操作对象未指定")
					return
				}

				if targetID, ok = reqDataMap["service_id"].(string); !ok {
					httputil.ReturnError(r, w, 400, "操作对象未指定")
					return
				}
			}
			//eventLog check the latest event

			if !util.CanDoEvent(optType, synType, target, targetID, serviceKind) {
				logrus.Errorf("operation too frequently. uri: %s; target: %s; target id: %s", r.RequestURI, target, targetID)
				httputil.ReturnError(r, w, 409, "操作过于频繁，请稍后再试") // status code 409 conflict
				return
			}

			// handle operator
			var operator string
			var reqData map[string]interface{}
			if err = json.Unmarshal(body, &reqData); err == nil {
				if operatorI := reqData["operator"]; operatorI != nil {
					operator = operatorI.(string)
				}
			}

			// tenantID can not null
			tenantID := r.Context().Value(ctxutil.ContextKey("tenant_id")).(string)
			var ctx context.Context

			event, err := util.CreateEvent(target, optType, targetID, tenantID, string(body), operator, synType)
			if err != nil {
				logrus.Error("create event error : ", err)
				httputil.ReturnError(r, w, 500, "操作失败")
				return
			}
			ctx = context.WithValue(r.Context(), ctxutil.ContextKey("event"), event)
			ctx = context.WithValue(ctx, ctxutil.ContextKey("event_id"), event.EventID)
			rw := &resWriter{origWriter: w}
			f(rw, r.WithContext(ctx))
			if synType == dbmodel.SYNEVENTTYPE || (synType == dbmodel.ASYNEVENTTYPE && rw.statusCode >= 400) { // status code 2XX/3XX all equal to success
				util.UpdateEvent(event.EventID, rw.statusCode)
			}
		}
	}
}

func debugRequestBody(r *http.Request) {
	if !apiExclude(r) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logrus.Warningf("error reading request body: %v", err)
		}
		logrus.Debugf("method: %s; uri: %s; body: %s", r.Method, r.RequestURI, string(body))

		// set a new body, which will simulate the same data we read
		r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	}
}
