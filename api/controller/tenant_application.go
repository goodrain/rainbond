package controller

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/middleware"
	"github.com/goodrain/rainbond/api/model"
	dbmodel "github.com/goodrain/rainbond/db/model"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/jinzhu/gorm"
)

// TenantAppStruct -
type TenantAppStruct struct{}

// CreateApp -
func (a *TenantAppStruct) CreateApp(w http.ResponseWriter, r *http.Request) {
	var tenantReq model.Application
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &tenantReq, nil) {
		return
	}

	// get current tenant
	tenant := r.Context().Value(middleware.ContextKey("tenant")).(*dbmodel.Tenants)
	tenantReq.TenantID = tenant.UUID

	// create app
	app, err := handler.GetTenantApplicationHandler().CreateApp(&tenantReq)
	if err != nil {
		httputil.ReturnError(r, w, http.StatusInternalServerError, fmt.Sprintf("Create app failed : %v", err))
		return
	}

	httputil.ReturnSuccess(r, w, app)
}

// UpdateApp -
func (a *TenantAppStruct) UpdateApp(w http.ResponseWriter, r *http.Request) {
	var updateAppReq model.UpdateAppRequest
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &updateAppReq, nil) {
		return
	}
	app := r.Context().Value(middleware.ContextKey("application")).(*dbmodel.Application)

	// update app
	app, err := handler.GetTenantApplicationHandler().UpdateApp(app, updateAppReq)
	if err != nil {
		httputil.ReturnError(r, w, http.StatusInternalServerError, fmt.Sprintf("Update app failed : %v", err))
		return
	}

	httputil.ReturnSuccess(r, w, app)
}

// ListApps -
func (a *TenantAppStruct) ListApps(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(chi.URLParam(r, "page"))
	if page == 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(chi.URLParam(r, "pageSize"))
	if pageSize == 0 {
		pageSize = 10
	}

	// get current tenantID
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)

	// List apps
	resp, err := handler.GetTenantApplicationHandler().ListApps(tenantID, page, pageSize)
	if err != nil {
		httputil.ReturnError(r, w, http.StatusInternalServerError, fmt.Sprintf("List apps failure : %v", err))
		return
	}

	httputil.ReturnSuccess(r, w, resp)
}

// ListServices -
func (a *TenantAppStruct) ListServices(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "app_id")
	page, _ := strconv.Atoi(chi.URLParam(r, "page"))
	if page == 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(chi.URLParam(r, "pageSize"))
	if pageSize == 0 {
		pageSize = 10
	}

	// List apps
	resp, err := handler.GetServiceManager().GetServicesByAppID(appID, page, pageSize)
	if err != nil {
		httputil.ReturnError(r, w, http.StatusInternalServerError, fmt.Sprintf("List apps failure : %v", err))
		return
	}

	httputil.ReturnSuccess(r, w, resp)
}

// DeleteApp -
func (a *TenantAppStruct) DeleteApp(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "app_id")

	// Delete application
	err := handler.GetTenantApplicationHandler().DeleteApp(appID)
	if err != nil {
		httputil.ReturnError(r, w, http.StatusInternalServerError, fmt.Sprintf("Delete app failed : %v", err))
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}
