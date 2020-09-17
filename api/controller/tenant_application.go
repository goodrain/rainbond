package controller

import (
	"fmt"
	"net/http"

	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/middleware"
	dbmodel "github.com/goodrain/rainbond/db/model"
	httputil "github.com/goodrain/rainbond/util/http"
)

// TenantAppStruct -
type TenantAppStruct struct{}

// CreateApplication -
func (a *TenantAppStruct) CreateApplication(w http.ResponseWriter, r *http.Request) {
	// get current tenant
	tenant := r.Context().Value(middleware.ContextKey("tenant")).(*dbmodel.Tenants)

	// create app
	app, err := handler.GetTenantApplicationHandler().CreateApplication(tenant.UUID)
	if err != nil {
		httputil.ReturnError(r, w, http.StatusInternalServerError, fmt.Sprintf("Create app failed : %v", err))
		return
	}

	httputil.ReturnSuccess(r, w, app)
}
