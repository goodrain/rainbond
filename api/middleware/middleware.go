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
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/util"

	"github.com/jinzhu/gorm"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"

	httputil "github.com/goodrain/rainbond/util/http"

	"github.com/Sirupsen/logrus"
	"github.com/go-chi/chi"
)

//ContextKey ctx key type
type ContextKey string

var pool []string

func init() {
	pool = []string{
		"services_status",
	}
}

//InitTenant 实现中间件
func InitTenant(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if !apiExclude(r) {
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				logrus.Warningf("error reading request body: %v", err)
			} else {
				logrus.Debugf("method: %s; uri: %s; body: %s", r.Method, r.RequestURI, string(body))
			}
			// set a new body, which will simulate the same data we read
			r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		}

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
		ctx := context.WithValue(r.Context(), ContextKey("tenant_name"), tenantName)
		ctx = context.WithValue(ctx, ContextKey("tenant_id"), tenant.UUID)
		ctx = context.WithValue(ctx, ContextKey("tenant"), tenant)

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
		ctx = context.WithValue(ctx, ContextKey("service"), service)
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
			handler.GetNodeProxy().Proxy(w, r)
			return
		}
		if strings.HasPrefix(r.RequestURI, "/v2/cluster") {
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
		if r.Method != "GET" {
			var targetID string
			var ok bool
			if targetID, ok = r.Context().Value(ContextKey("service_id")).(string); !ok {
				httputil.ReturnError(r, w, 400, "操作对象未指定")
				return
			}
			//eventLog check the latest event
			if !canDoEvent(optType, synType, target, targetID) {
				httputil.ReturnError(r, w, 400, "操作过于频繁，请稍后再试")
				return
			}
			// tenantID can not null
			tenantID := r.Context().Value(ContextKey("tenant_id")).(string)
			var ctx context.Context
			// check resource is enough or not
			if err := checkResource(optType, r); err != nil {
				httputil.ReturnError(r, w, 400, err.Error())
				return
			}
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				logrus.Warningf("error reading request body: %v", err)
			} else {
				logrus.Debugf("method: %s; uri: %s; body: %s", r.Method, r.RequestURI, string(body))
			}
			// set a new body, which will simulate the same data we read
			r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
			event, err := createEvent(target, optType, targetID, tenantID, string(body), "system", synType) // TODO username
			if err != nil {
				logrus.Error("create event error : ", err)
				httputil.ReturnError(r, w, 500, "操作失败")
				return
			}
			ctx = context.WithValue(r.Context(), ContextKey("event"), event)
			ctx = context.WithValue(ctx, ContextKey("event_id"), event.EventID)
			rw := &resWriter{origWriter: w}
			f(rw, r.WithContext(ctx))
			if synType == dbmodel.SYNEVENTTYPE || (synType == dbmodel.ASYNEVENTTYPE && rw.statusCode != 200) {
				updateEvent(event.EventID, rw.statusCode)
			}
		}
	}
}

func canDoEvent(optType string, synType int, target, targetID string) bool {
	if synType == dbmodel.SYNEVENTTYPE {
		return true
	}
	event, err := db.GetManager().ServiceEventDao().GetLastASyncEvent(target, targetID)
	if err != nil {
		if err.Error() == gorm.ErrRecordNotFound.Error() {
			return true
		}
		logrus.Error("get event by targetID error:", err)
		return false
	}
	if event == nil || event.FinalStatus != "" {
		return true
	}
	if !checkTimeout(event) {
		return false
	}
	return true
}

func checkTimeout(event *dbmodel.ServiceEvent) bool {
	if event.SynType == dbmodel.ASYNEVENTTYPE {
		if event.FinalStatus == "" {
			startTime := event.StartTime
			start, err := time.ParseInLocation(time.RFC3339, startTime, time.Local)
			if err != nil {
				return false
			}
			var end time.Time
			if event.OptType == "deploy-service" || event.OptType == "create-service" || event.OptType == "build-service" {
				end = start.Add(10 * time.Minute)
			} else {
				end = start.Add(3 * time.Minute)
			}
			if time.Now().After(end) {
				event.FinalStatus = "timeout"
				err = db.GetManager().ServiceEventDao().UpdateModel(event)
				if err != nil {
					logrus.Error("check event timeout error : ", err.Error())
					return false
				}
				return true
			}
			// latest event is still processing on
			return false
		}
	}
	return true
}

func checkResource(optType string, r *http.Request) error {
	if optType == "start-service" || optType == "restart-service" || optType == "deploy-service" || optType == "horizontal-service" || optType == "vertical-service" || optType == "upgrade-service" {
		if publicCloud := os.Getenv("PUBLIC_CLOUD"); publicCloud != "true" {
			tenant := r.Context().Value(ContextKey("tenant")).(*model.Tenants)
			if service, ok := r.Context().Value(ContextKey("service")).(*model.TenantServices); ok {
				return priChargeSverify(tenant, service.ContainerMemory*service.Replicas)
			}
		}
	}
	return nil
}

func priChargeSverify(t *model.Tenants, quantity int) error {
	if t.LimitMemory == 0 {
		clusterStats, err := handler.GetTenantManager().GetAllocatableResources()
		if err != nil {
			return fmt.Errorf("error getting allocatable resources: %v", err)
		}
		availMem := clusterStats.AllMemory - clusterStats.RequestMemory
		if availMem >= int64(quantity) {
			return nil
		}
		return fmt.Errorf("cluster_lack_of_memory")
	}
	tenantStas, err := handler.GetTenantManager().GetTenantResource(t.UUID)
	if err != nil {
		return fmt.Errorf("error getting tenant resource: %v", err)
	}
	// TODO: it should be limit, not request
	availMem := int64(t.LimitMemory) - (tenantStas.MemoryRequest + tenantStas.UnscdMemoryReq)
	if availMem >= int64(quantity) {
		return nil
	}
	return fmt.Errorf("lack_of_memory")
}

func createEvent(target, optType, targetID, tenantID, reqBody, userName string, synType int) (*dbmodel.ServiceEvent, error) {
	event := dbmodel.ServiceEvent{
		EventID:     util.NewUUID(),
		TenantID:    tenantID,
		Target:      target,
		TargetID:    targetID,
		RequestBody: reqBody,
		UserName:    userName,
		StartTime:   time.Now().Format(time.RFC3339),
		SynType:     synType,
		OptType:     optType,
	}
	err := db.GetManager().ServiceEventDao().AddModel(&event)
	return &event, err
}

func updateEvent(eventID string, statusCode int) {
	event, err := db.GetManager().ServiceEventDao().GetEventByEventID(eventID)
	if err != nil && err != gorm.ErrRecordNotFound {
		logrus.Errorf("find event by eventID error : %s", err.Error())
		return
	}
	if err == gorm.ErrRecordNotFound {
		logrus.Errorf("do not found event by eventID %s", eventID)
		return
	}
	event.FinalStatus = "complete"
	event.EndTime = time.Now().Format(time.RFC3339)
	if statusCode == 200 {
		event.Status = "success"
	} else {
		event.Status = "failure"
	}
	err = db.GetManager().ServiceEventDao().UpdateModel(event)
	if err != nil {
		logrus.Errorf("update event status failure %s", err.Error())
		retry := 2
		for retry > 0 {
			if err = db.GetManager().ServiceEventDao().UpdateModel(event); err != nil {
				retry--
			} else {
				break
			}
		}
	}
}
