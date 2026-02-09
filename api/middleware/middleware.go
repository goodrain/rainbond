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
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/util"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/api/util/license"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	rutil "github.com/goodrain/rainbond/util"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var pool []string

func init() {
	pool = []string{
		"services_status",
	}
}

// InitTenant 实现中间件
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

// InitService 实现serviceinit中间件
func InitService(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		serviceAlias := chi.URLParam(r, "service_alias")
		if serviceAlias == "" {
			httputil.ReturnError(r, w, 404, "cant find service alias")
			return
		}

		tenantID := r.Context().Value(ctxutil.ContextKey("tenant_id"))

		// 如果是删除服务的请求路径，先执行清理操作
		if r.Method == "DELETE" && r.URL.Path == "/v2/tenants/"+chi.URLParam(r, "tenant_name")+"/services/"+serviceAlias+"/" {
			if err := cleanupKubernetesResources(tenantID.(string), serviceAlias); err != nil {
				logrus.Errorf("cleanup kubernetes resources error: %v", err)
			}
		}

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

// cleanupKubernetesResources 清理与已删除服务相关的 K8s 资源
func cleanupKubernetesResources(tenantID, serviceAlias string) error {
	tenant, err := db.GetManager().TenantDao().GetTenantByUUID(tenantID)
	if err != nil {
		logrus.Errorf("get tenant by id error: %v", err)
		return err
	}

	namespace := tenant.Namespace
	if namespace == "" {
		namespace = tenantID // fallback to tenantID as namespace
	}

	// 清理 ApisixRoute 资源
	if err := cleanupApisixRoutes(namespace, serviceAlias); err != nil {
		logrus.Errorf("cleanup apisix routes error: %v", err)
	}

	// 清理 Service 资源
	if err := cleanupServices(namespace, serviceAlias); err != nil {
		logrus.Errorf("cleanup services error: %v", err)
	}

	logrus.Infof("cleanup kubernetes resources for service %s in namespace %s completed", serviceAlias, namespace)
	return nil
}

// cleanupApisixRoutes 清理 ApisixRoute 资源
func cleanupApisixRoutes(namespace, serviceAlias string) error {
	ctx := context.Background()

	// 直接删除与该组件相关的所有 ApisixRoute
	err := k8s.Default().ApiSixClient.ApisixV2().ApisixRoutes(namespace).DeleteCollection(
		ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{
			LabelSelector: "component_sort=" + serviceAlias,
		},
	)
	if err != nil {
		logrus.Errorf("delete apisix routes for component %s error: %v", serviceAlias, err)
		return err
	}

	logrus.Infof("deleted apisix routes for component %s in namespace %s", serviceAlias, namespace)
	return nil
}

// cleanupServices 清理 Service 资源
func cleanupServices(namespace, serviceAlias string) error {
	ctx := context.Background()

	// 先列出与该组件相关的所有 Service
	serviceList, err := k8s.Default().Clientset.CoreV1().Services(namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: "service_alias=" + serviceAlias,
		},
	)
	if err != nil {
		logrus.Errorf("list services for component %s error: %v", serviceAlias, err)
		return err
	}

	// 逐个删除 Service
	for _, svc := range serviceList.Items {
		if err := k8s.Default().Clientset.CoreV1().Services(namespace).Delete(
			ctx,
			svc.Name,
			metav1.DeleteOptions{},
		); err != nil {
			logrus.Warningf("delete service(%s): %v", svc.GetName(), err)
		} else {
			logrus.Infof("deleted service: %s", svc.GetName())
		}
	}

	logrus.Infof("cleanup services for component %s in namespace %s completed", serviceAlias, namespace)
	return nil
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

// InitPlugin 实现plugin init中间件
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

// SetLog SetLog
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

// Proxy 反向代理中间件
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
		//if strings.HasPrefix(r.RequestURI, "/v2/rules") {
		//	handler.GetMonitorProxy().Proxy(w, r)
		//	return
		//}
		if strings.HasPrefix(r.RequestURI, "/kubernetes/dashboard") {
			proxy := handler.GetKubernetesDashboardProxy()
			r.URL.Path = strings.Replace(r.URL.Path, "/kubernetes/dashboard", "", 1)
			proxy.Proxy(w, r)
			return
		}
		if strings.HasPrefix(r.RequestURI, "/v2/container_disk") {
			handler.GetNodeProxy().Proxy(w, r)
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
func WrapEL(f http.HandlerFunc, target, optType string, synType int, resourceValidation bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := LicenseVerification(r, resourceValidation)
		if err != nil {
			err.Handle(r, w)
			return
		}
		var (
			serviceKind string
		)
		serviceObj := r.Context().Value(ctxutil.ContextKey("service"))
		if serviceObj != nil {
			service := serviceObj.(*dbmodel.TenantServices)
			serviceKind = service.Kind
		}

		if r.Method != "GET" {
			body, err := io.ReadAll(r.Body)
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

			// Check if this is a source code build operation that might need source scanning
			// Only defer event creation for source code builds, not image builds
			skipEventCreation := false
			buildKind := ""
			if kindI, ok := reqData["kind"]; ok {
				buildKind, _ = kindI.(string)
			}
			if optType == "build-service" && buildKind == "build_from_source_code" && shouldDeferBuildEvent() {
				logrus.Infof("Source-scan plugin detected for source code build, deferring build-service event creation until after scan")
				skipEventCreation = true
			}

			if skipEventCreation {
				// Generate event ID but don't create event yet
				eventID := rutil.NewUUID()
				ctx = context.WithValue(r.Context(), ctxutil.ContextKey("event_id"), eventID)
				ctx = context.WithValue(ctx, ctxutil.ContextKey("deferred_event"), true)
				// Store event creation parameters in context for later use
				ctx = context.WithValue(ctx, ctxutil.ContextKey("deferred_event_params"), map[string]interface{}{
					"target":   target,
					"optType":  optType,
					"targetID": targetID,
					"tenantID": tenantID,
					"body":     string(body),
					"operator": operator,
					"synType":  synType,
				})
				rw := &resWriter{origWriter: w}
				f(rw, r.WithContext(ctx))
			} else {
				// Normal flow: create event immediately
				event, err := util.CreateEvent(target, optType, targetID, tenantID, string(body), operator, "", "", synType)
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
}

// shouldDeferBuildEvent checks if build event creation should be deferred due to source scanning
func shouldDeferBuildEvent() bool {
	// Check if source-scan plugin is installed
	ctx := context.Background()
	plugin, err := k8s.Default().RainbondClient.RainbondV1alpha1().RBDPlugins(metav1.NamespaceAll).Get(ctx, "rainbond-sourcescan", metav1.GetOptions{})
	if err != nil {
		logrus.Debugf("Source-scan plugin not found: %v", err)
		return false
	}

	// Check if plugin has a backend configured
	if plugin.Spec.BackendService == "" {
		logrus.Debug("Source-scan plugin found but BackendService is empty")
		return false
	}

	logrus.Infof("Source-scan plugin active with backend: %s, deferring build event creation", plugin.Spec.BackendService)
	return true
}

func debugRequestBody(r *http.Request) {
	if !apiExclude(r) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logrus.Warningf("error reading request body: %v", err)
		}
		logrus.Debugf("method: %s; uri: %s; body: %s", r.Method, r.RequestURI, string(body))

		// set a new body, which will simulate the same data we read
		r.Body = io.NopCloser(bytes.NewBuffer(body))
	}
}

// LicenseCache for caching information
var LicenseCache struct {
	sync.RWMutex
	Data     *license.LicenseResp
	InfoBody string
	ExpireAt time.Time
}

// LicenseVerification verification Information
func LicenseVerification(r *http.Request, resourceValidation bool) *util.APIHandleError {
	if !resourceValidation {
		return nil
	}
	_, err := k8s.Default().RainbondClient.RainbondV1alpha1().RBDPlugins(metav1.NamespaceNone).Get(context.TODO(), "rainbond-enterprise-base", metav1.GetOptions{})
	if err != nil {
		// If the plugin does not exist, it is considered an open source version and returned directly without License verification.
		return nil
	}
	enterpriseID, infoBody, actualCluster, actualNode, actualMemory, err := ParseLicenseParam(r)
	if err != nil {
		return util.CreateAPIHandleError(412, fmt.Errorf("authorize_cluster_lack_of_license"))
	}

	now := time.Now()
	// check whether the cache is valid
	LicenseCache.RLock()
	if LicenseCache.Data != nil && LicenseCache.ExpireAt.After(now) && LicenseCache.InfoBody == infoBody {
		logrus.Debug("Using cached license data")
		LicenseCache.RUnlock()
		return validateLicense(*LicenseCache.Data, actualCluster, actualNode, actualMemory, now)
	}
	LicenseCache.RUnlock()

	logrus.Debug("Fetching new license data")
	lic := license.ReadLicense(enterpriseID, infoBody)
	if lic == nil {
		return util.CreateAPIHandleError(400, fmt.Errorf("invaild license"))
	}

	// Update the cache for 10 minute
	resp := lic.SetResp(actualCluster, actualNode, actualMemory)
	LicenseCache.Lock()
	LicenseCache.Data = resp
	LicenseCache.ExpireAt = now.Add(10 * time.Minute)
	LicenseCache.InfoBody = infoBody
	LicenseCache.Unlock()

	return validateLicense(*resp, actualCluster, actualNode, actualMemory, now)
}

// validateLicense - 执行 License 验证逻辑
func validateLicense(licenseResp license.LicenseResp, actualCluster, actualNode, actualMemory int64, now time.Time) *util.APIHandleError {
	if licenseResp.ExpectCluster != -1 && actualCluster > licenseResp.ExpectCluster {
		return util.CreateAPIHandleError(412, fmt.Errorf("authorize_cluster_lack_of_license"))
	}
	if licenseResp.ExpectNode != -1 && actualNode > licenseResp.ExpectNode {
		return util.CreateAPIHandleError(412, fmt.Errorf("authorize_cluster_lack_of_node"))
	}
	if licenseResp.ExpectMemory != -1 && actualMemory > licenseResp.ExpectMemory {
		return util.CreateAPIHandleError(412, fmt.Errorf("authorize_cluster_lack_of_memory"))
	}
	if licenseResp.EndTime != "" {
		endTime, err := time.Parse("2006-01-02 15:04:05", licenseResp.EndTime)
		if err == nil && endTime.Before(now) {
			return util.CreateAPIHandleError(412, fmt.Errorf("authorize_expiration_of_authorization"))
		}
	}
	return nil
}

func ParseLicenseParam(r *http.Request) (enterpriseID, infoBody string, actualCluster, actualNode, actualMemory int64, err error) {
	enterpriseID = r.Header.Get("enterprise_id")
	infoBody = r.Header.Get("info_body")
	actualClusterStr := r.Header.Get("actual_cluster")
	actualNodeStr := r.Header.Get("actual_node")
	actualMemoryStr := r.Header.Get("actual_memory")
	actualCluster, err = strconv.ParseInt(actualClusterStr, 10, 64)
	if err != nil {
		return
	}
	actualNode, err = strconv.ParseInt(actualNodeStr, 10, 64)
	if err != nil {
		return
	}
	actualMemory, err = strconv.ParseInt(actualMemoryStr, 10, 64)
	if err != nil {
		return
	}
	return
}
