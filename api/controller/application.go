package controller

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/middleware"
	"github.com/goodrain/rainbond/api/model"
	dbmodel "github.com/goodrain/rainbond/db/model"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
)

// ApplicationController -
type ApplicationController struct{}

// CreateApp -
func (a *ApplicationController) CreateApp(w http.ResponseWriter, r *http.Request) {
	var tenantReq model.Application
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &tenantReq, nil) {
		return
	}

	// get current tenant
	tenant := r.Context().Value(middleware.ContextKey("tenant")).(*dbmodel.Tenants)
	tenantReq.TenantID = tenant.UUID

	// create app
	app, err := handler.GetApplicationHandler().CreateApp(&tenantReq)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}

	httputil.ReturnSuccess(r, w, app)
}

// BatchCreateApp -
func (a *ApplicationController) BatchCreateApp(w http.ResponseWriter, r *http.Request) {
	var apps model.CreateAppRequest
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &apps, nil) {
		return
	}

	// get current tenant
	tenant := r.Context().Value(middleware.ContextKey("tenant")).(*dbmodel.Tenants)
	respList, err := handler.GetApplicationHandler().BatchCreateApp(&apps, tenant.UUID)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, respList)
}

// UpdateApp -
func (a *ApplicationController) UpdateApp(w http.ResponseWriter, r *http.Request) {
	var updateAppReq model.UpdateAppRequest
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &updateAppReq, nil) {
		return
	}
	app := r.Context().Value(middleware.ContextKey("application")).(*dbmodel.Application)

	// update app
	app, err := handler.GetApplicationHandler().UpdateApp(app, updateAppReq)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}

	httputil.ReturnSuccess(r, w, app)
}

// ListApps -
func (a *ApplicationController) ListApps(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	appName := query.Get("app_name")
	pageQuery := query.Get("page")
	pageSizeQuery := query.Get("pageSize")

	page, _ := strconv.Atoi(pageQuery)
	if page == 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(pageSizeQuery)
	if pageSize == 0 {
		pageSize = 10
	}

	// get current tenantID
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)

	// List apps
	resp, err := handler.GetApplicationHandler().ListApps(tenantID, appName, page, pageSize)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}

	httputil.ReturnSuccess(r, w, resp)
}

// ListServices -
func (a *ApplicationController) ListServices(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "app_id")
	query := r.URL.Query()
	pageQuery := query.Get("page")
	pageSizeQuery := query.Get("pageSize")

	page, _ := strconv.Atoi(pageQuery)
	if page == 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(pageSizeQuery)
	if pageSize == 0 {
		pageSize = 10
	}

	// List services
	resp, err := handler.GetServiceManager().GetServicesByAppID(appID, page, pageSize)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}

	httputil.ReturnSuccess(r, w, resp)
}

// DeleteApp -
func (a *ApplicationController) DeleteApp(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "app_id")

	var req model.EtcdCleanReq
	if httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil) {
		logrus.Debugf("delete app etcd keys : %+v", req.Keys)
		handler.GetEtcdHandler().CleanAllServiceData(req.Keys)
	}
	// Delete application
	err := handler.GetApplicationHandler().DeleteApp(appID)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

// BatchUpdateComponentPorts -
func (a *ApplicationController) BatchUpdateComponentPorts(w http.ResponseWriter, r *http.Request) {
	var appPorts []*model.AppPort
	if err := httputil.ReadEntity(r, &appPorts); err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	for _, port := range appPorts {
		if err := httputil.ValidateStruct(port); err != nil {
			httputil.ReturnBcodeError(r, w, err)
			return
		}
	}

	appID := r.Context().Value(middleware.ContextKey("app_id")).(string)

	if err := handler.GetApplicationHandler().BatchUpdateComponentPorts(appID, appPorts); err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}

	httputil.ReturnSuccess(r, w, nil)
}

// GetAppStatus -
func (a *ApplicationController) GetAppStatus(w http.ResponseWriter, r *http.Request) {
	appID := r.Context().Value(middleware.ContextKey("app_id")).(string)

	res, err := handler.GetApplicationHandler().GetStatus(appID)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}

	httputil.ReturnSuccess(r, w, res)
}

// BatchBindService -
func (a *ApplicationController) BatchBindService(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "app_id")
	var bindServiceReq model.BindServiceRequest
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &bindServiceReq, nil) {
		return
	}

	// bind service
	err := handler.GetApplicationHandler().BatchBindService(appID, bindServiceReq)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}
