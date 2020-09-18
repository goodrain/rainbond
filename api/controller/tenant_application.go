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

// ListAppResponse -
type ListAppResponse struct {
	Page     int                    `json:"page"`
	PageSize int                    `json:"pageSize"`
	Total    int64                  `json:"total"`
	Apps     []*dbmodel.Application `json:"apps"`
}

// ListServiceResponse -
type ListServiceResponse struct {
	Page     int                       `json:"page"`
	PageSize int                       `json:"pageSize"`
	Total    int64                     `json:"total"`
	Services []*dbmodel.TenantServices `json:"services"`
}

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
	var updateAppReq model.Application
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &updateAppReq, nil) {
		return
	}
	appID := chi.URLParam(r, "app_id")
	tenantApp, err := handler.GetTenantApplicationHandler().GetAppByID(appID)
	if err != nil {
		if err.Error() == gorm.ErrRecordNotFound.Error() {
			httputil.ReturnError(r, w, 404, "can't find application")
			return
		}
		httputil.ReturnError(r, w, 500, "get assign tenant application failed")
		return
	}
	// get current tenant
	tenant := r.Context().Value(middleware.ContextKey("tenant")).(*dbmodel.Tenants)
	updateAppReq.TenantID = tenant.UUID
	updateAppReq.AppID = tenantApp.AppID

	// create app
	app, err := handler.GetTenantApplicationHandler().UpdateApp(&updateAppReq)
	if err != nil {
		httputil.ReturnError(r, w, http.StatusInternalServerError, fmt.Sprintf("Create app failed : %v", err))
		return
	}

	httputil.ReturnSuccess(r, w, app)
}

// ListApps -
func (a *TenantAppStruct) ListApps(w http.ResponseWriter, r *http.Request) {
	var resp ListAppResponse
	page, _ := strconv.Atoi(chi.URLParam(r, "page"))
	if page == 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(chi.URLParam(r, "pageSize"))
	if pageSize == 0 {
		pageSize = 10
	}

	// get current tenantID
	tenant := r.Context().Value(middleware.ContextKey("tenant")).(*dbmodel.Tenants)

	// List apps
	apps, total, err := handler.GetTenantApplicationHandler().ListApps(tenant.UUID, page, pageSize)
	if err != nil {
		httputil.ReturnError(r, w, http.StatusInternalServerError, fmt.Sprintf("List apps failure : %v", err))
		return
	}
	if apps != nil {
		resp.Apps = apps
	} else {
		resp.Apps = make([]*dbmodel.Application, 0)
	}

	resp.Page = page
	resp.Total = total
	resp.PageSize = pageSize
	httputil.ReturnSuccess(r, w, resp)
}

// ListServices -
func (a *TenantAppStruct) ListServices(w http.ResponseWriter, r *http.Request) {
	var resp ListServiceResponse
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
	services, total, err := handler.GetServiceManager().GetServicesByAppID(appID, page, pageSize)
	if err != nil {
		httputil.ReturnError(r, w, http.StatusInternalServerError, fmt.Sprintf("List apps failure : %v", err))
		return
	}
	if services != nil {
		resp.Services = services
	} else {
		resp.Services = make([]*dbmodel.TenantServices, 0)
	}

	resp.Page = page
	resp.Total = total
	resp.PageSize = pageSize
	httputil.ReturnSuccess(r, w, resp)
}

// DeleteApp -
func (a *TenantAppStruct) DeleteApp(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "app_id")

	// Get the number of services under the application
	_, total, err := handler.GetServiceManager().GetServicesByAppID(appID, 1, 10)
	if err != nil {
		if err.Error() != gorm.ErrRecordNotFound.Error() {
			httputil.ReturnError(r, w, http.StatusInternalServerError, fmt.Sprintf("Delete app failed : %v", err))
			return
		}
	}
	if total != 0 {
		httputil.ReturnError(r, w, http.StatusFound, "Failed to delete the app because it has bound services")
		return
	}

	// Delete application
	err = handler.GetTenantApplicationHandler().DeleteApp(appID)
	if err != nil {
		httputil.ReturnError(r, w, http.StatusInternalServerError, fmt.Sprintf("Delete app failed : %v", err))
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}
