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

package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/jinzhu/gorm"

	"github.com/goodrain/rainbond/pkg/db"
	"github.com/goodrain/rainbond/pkg/event"

	httputil "github.com/goodrain/rainbond/pkg/util/http"

	"github.com/Sirupsen/logrus"
	"github.com/go-chi/chi"
)

//ContextKey ctx key type
type ContextKey string

//InitTenant 实现中间件
func InitTenant(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		tenantName := chi.URLParam(r, "tenant_name")
		if tenantName == "" {
			httputil.ReturnError(r, w, 404, "cant find tenant")
			return
		}
		tenant, err := db.GetManager().TenantDao().GetTenantIDByName(tenantName)
		if err != nil {
			logrus.Errorf("get tenant by tenantName error, %v", err)
			if err.Error() == gorm.ErrRecordNotFound.Error() {
				httputil.ReturnError(r, w, 404, "cant find tenant")
				return
			}
			httputil.ReturnError(r, w, 500, "get assign tenant uuid failed")
			return
		}
		ctx := context.WithValue(r.Context(), ContextKey("tenant_name"), tenantName)
		ctx = context.WithValue(ctx, ContextKey("tenant_id"), tenant.UUID)
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
		tenantID := r.Context().Value(ContextKey("tenant_id"))
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
		ctx := context.WithValue(r.Context(), ContextKey("service_alias"), serviceAlias)
		ctx = context.WithValue(ctx, ContextKey("service_id"), serviceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

//InitPlugin 实现plugin init中间件
func InitPlugin(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		tenantID := r.Context().Value(ContextKey("tenant_id")).(string)
		if pluginID == "" {
			httputil.ReturnError(r, w, 404, "need plugin id")
			return
		}
		_, err := db.GetManager().TenantPluginDao().GetPluginByID(pluginID,tenantID)
		if err != nil {
			if err.Error() == gorm.ErrRecordNotFound.Error() {
				httputil.ReturnError(r, w, 404, "cant find plugin")
				return
			}
			logrus.Errorf("get plugin error, %v", err)
			httputil.ReturnError(r, w, 500, "get plugin error")
			return
		}
		ctx := context.WithValue(r.Context(), ContextKey("plugin_id"), pluginID)
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
			ctx := context.WithValue(r.Context(), ContextKey("logger"), logger)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	}
	return http.HandlerFunc(fn)
}

//Proxy 反向代理中间件
func Proxy(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.RequestURI, "/v2/nodes") {
			GetNodeProxy().Proxy(w, r)
			return
		}
		if strings.HasPrefix(r.RequestURI, "/v2/builder") {
			GetBuilderProxy().Proxy(w, r)
			return
		}
		if strings.HasPrefix(r.RequestURI, "/v2/tasks") {
			GetNodeProxy().Proxy(w, r)
			return
		}
		if strings.HasPrefix(r.RequestURI, "/v2/tasktemps") {
			GetNodeProxy().Proxy(w, r)
			return
		}
		if strings.HasPrefix(r.RequestURI, "/v2/taskgroups") {
			GetNodeProxy().Proxy(w, r)
			return
		}
		if strings.HasPrefix(r.RequestURI, "/v2/configs") {
			GetNodeProxy().Proxy(w, r)
			return
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

//SetAPI set api 设置api到会话中
/*
func (v2 *V2Routes) SetAPI(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		V1API := v2.V1API
		ctx := context.WithValue(r.Context(), key("v1_api"), V1API)
		logrus.Debugf("set api to ctx, api is %v", V1API)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}
*/
