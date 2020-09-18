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

	// get current tenant
	tenant := r.Context().Value(middleware.ContextKey("tenant")).(*dbmodel.Tenants)
	appID := r.Context().Value(middleware.ContextKey("app_id")).(string)
	updateAppReq.TenantID = tenant.UUID
	updateAppReq.AppID = appID

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
