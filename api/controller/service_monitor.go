package controller

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"strings"

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

//UploadPackage upload package
func (t *TenantStruct) UploadPackage(w http.ResponseWriter, r *http.Request) {
	eventID := strings.TrimSpace(chi.URLParam(r, "eventID"))
	switch r.Method {
	case "POST":
		if eventID == "" {
			httputil.ReturnError(r, w, 400, "Failed to parse eventID.")
			return
		}
		logrus.Debug("Start receive upload file: ", eventID)
		reader, header, err := r.FormFile("packageTarFile")
		if err != nil {
			logrus.Errorf("Failed to parse upload file: %s", err.Error())
			httputil.ReturnError(r, w, 501, "Failed to parse upload file.")
			return
		}
		defer reader.Close()

		dirName := fmt.Sprintf("/grdata/package_build/temp/events/%s", eventID)
		os.MkdirAll(dirName, 0755)

		fileName := fmt.Sprintf("%s/%s", dirName, header.Filename)
		file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			logrus.Errorf("Failed to open file: %s", err.Error())
			httputil.ReturnError(r, w, 502, "Failed to open file: "+err.Error())
		}
		defer file.Close()

		logrus.Debug("Start write file to: ", fileName)
		if _, err := io.Copy(file, reader); err != nil {
			logrus.Errorf("Failed to write file：%s", err.Error())
			httputil.ReturnError(r, w, 503, "Failed to write file: "+err.Error())
		}

		logrus.Debug("successful write file to: ", fileName)
		origin := r.Header.Get("Origin")
		w.Header().Add("Access-Control-Allow-Origin", origin)
		w.Header().Add("Access-Control-Allow-Methods", "POST,OPTIONS")
		w.Header().Add("Access-Control-Allow-Credentials", "true")
		w.Header().Add("Access-Control-Allow-Headers", "x-requested-with,Content-Type,X-Custom-Header")
		httputil.ReturnSuccess(r, w, nil)

	case "OPTIONS":
		origin := r.Header.Get("Origin")
		w.Header().Add("Access-Control-Allow-Origin", origin)
		w.Header().Add("Access-Control-Allow-Methods", "POST,OPTIONS")
		w.Header().Add("Access-Control-Allow-Credentials", "true")
		w.Header().Add("Access-Control-Allow-Headers", "x-requested-with,Content-Type,X-Custom-Header")
		httputil.ReturnSuccess(r, w, nil)
	}
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
