package controller

import (
	"net/http"

	"github.com/goodrain/rainbond/api/client/prometheus"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	api_model "github.com/goodrain/rainbond/api/model"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	httputil "github.com/goodrain/rainbond/util/http"
)

//AddServiceMonitors add service monitor
func (t *TenantStruct) AddServiceMonitors(w http.ResponseWriter, r *http.Request) {
	var add api_model.AddServiceMonitorRequestStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &add, nil)
	if !ok {
		return
	}
	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	tenantID := r.Context().Value(ctxutil.ContextKey("tenant_id")).(string)
	tsm, err := handler.GetServiceManager().AddServiceMonitor(tenantID, serviceID, add)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, tsm)
}

//DeleteServiceMonitors delete service monitor
func (t *TenantStruct) DeleteServiceMonitors(w http.ResponseWriter, r *http.Request) {
	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	tenantID := r.Context().Value(ctxutil.ContextKey("tenant_id")).(string)
	name := chi.URLParam(r, "name")
	tsm, err := handler.GetServiceManager().DeleteServiceMonitor(tenantID, serviceID, name)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, tsm)
}

//UpdateServiceMonitors update service monitor
func (t *TenantStruct) UpdateServiceMonitors(w http.ResponseWriter, r *http.Request) {
	var update api_model.UpdateServiceMonitorRequestStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &update, nil)
	if !ok {
		return
	}
	name := chi.URLParam(r, "name")
	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	tenantID := r.Context().Value(ctxutil.ContextKey("tenant_id")).(string)
	tsm, err := handler.GetServiceManager().UpdateServiceMonitor(tenantID, serviceID, name, update)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, tsm)
}

//GetMonitorMetrics get monitor metrics
func GetMonitorMetrics(w http.ResponseWriter, r *http.Request) {
	target := r.FormValue("target")
	var metricMetadatas []prometheus.Metadata
	if target == "tenant" {
		metricMetadatas = handler.GetMonitorHandle().GetTenantMonitorMetrics(r.FormValue("tenant"))
	}
	if target == "app" {
		metricMetadatas = handler.GetMonitorHandle().GetAppMonitorMetrics(r.FormValue("tenant"), r.FormValue("app"))
	}
	if target == "component" {
		metricMetadatas = handler.GetMonitorHandle().GetComponentMonitorMetrics(r.FormValue("tenant"), r.FormValue("component"))
	}
	httputil.ReturnSuccess(r, w, metricMetadatas)
}
