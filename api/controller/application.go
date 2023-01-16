package controller

import (
	"fmt"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util/bcode"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
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
	if tenantReq.AppType == model.AppTypeHelm {
		if tenantReq.AppStoreName == "" {
			httputil.ReturnBcodeError(r, w, bcode.NewBadRequest("the field 'app_tore_name' is required"))
			return
		}
		if tenantReq.AppTemplateName == "" {
			httputil.ReturnBcodeError(r, w, bcode.NewBadRequest("the field 'app_template_name' is required"))
			return
		}
		if tenantReq.AppName == "" {
			httputil.ReturnBcodeError(r, w, bcode.NewBadRequest("the field 'helm_app_name' is required"))
			return
		}
		if tenantReq.Version == "" {
			httputil.ReturnBcodeError(r, w, bcode.NewBadRequest("the field 'version' is required"))
			return
		}
		tenantReq.K8sApp = tenantReq.AppTemplateName
	}
	if tenantReq.K8sApp != "" {
		if len(k8svalidation.IsQualifiedName(tenantReq.K8sApp)) > 0 {
			httputil.ReturnBcodeError(r, w, bcode.ErrInvaildK8sApp)
			return
		}
	}
	// get current tenant
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)
	tenantReq.TenantID = tenant.UUID
	// create app
	app, err := handler.GetApplicationHandler().CreateApp(r.Context(), &tenantReq)
	if err != nil {
		logrus.Errorf("create app: %+v", err)
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
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)
	respList, err := handler.GetApplicationHandler().BatchCreateApp(r.Context(), &apps, tenant.UUID)
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
	app := r.Context().Value(ctxutil.ContextKey("application")).(*dbmodel.Application)
	if updateAppReq.K8sApp != "" && len(k8svalidation.IsQualifiedName(updateAppReq.K8sApp)) > 0 {
		httputil.ReturnBcodeError(r, w, bcode.ErrInvaildK8sApp)
		return
	}
	if updateAppReq.K8sApp == "" {
		updateAppReq.K8sApp = fmt.Sprintf("app-%s", app.AppID[:8])
		if app.K8sApp != "" {
			updateAppReq.K8sApp = app.K8sApp
		}
	}
	// update app
	app, err := handler.GetApplicationHandler().UpdateApp(r.Context(), app, updateAppReq)
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
	tenantID := r.Context().Value(ctxutil.ContextKey("tenant_id")).(string)

	// List apps
	resp, err := handler.GetApplicationHandler().ListApps(tenantID, appName, page, pageSize)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}

	httputil.ReturnSuccess(r, w, resp)
}

// ListComponents -
func (a *ApplicationController) ListComponents(w http.ResponseWriter, r *http.Request) {
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
	app := r.Context().Value(ctxutil.ContextKey("application")).(*dbmodel.Application)

	var req model.EtcdCleanReq
	if httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil) {
		logrus.Debugf("delete app etcd keys : %+v", req.Keys)
		handler.GetEtcdHandler().CleanAllServiceData(req.Keys)
	}
	// Delete application
	err := handler.GetApplicationHandler().DeleteApp(r.Context(), app)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

// BatchUpdateComponentPorts update component ports in batch.
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

	appID := r.Context().Value(ctxutil.ContextKey("app_id")).(string)

	if err := handler.GetApplicationHandler().BatchUpdateComponentPorts(appID, appPorts); err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}

	httputil.ReturnSuccess(r, w, nil)
}

// GetAppStatus returns the status of the application.
func (a *ApplicationController) GetAppStatus(w http.ResponseWriter, r *http.Request) {
	app := r.Context().Value(ctxutil.ContextKey("application")).(*dbmodel.Application)

	res, err := handler.GetApplicationHandler().GetStatus(r.Context(), app)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}

	httputil.ReturnSuccess(r, w, res)
}

// Install installs the application.
func (a *ApplicationController) Install(w http.ResponseWriter, r *http.Request) {
	app := r.Context().Value(ctxutil.ContextKey("application")).(*dbmodel.Application)

	var installAppReq model.InstallAppReq
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &installAppReq, nil) {
		return
	}

	if err := handler.GetApplicationHandler().Install(r.Context(), app, installAppReq.Overrides); err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
}

// ListServices returns the list fo the application.
func (a *ApplicationController) ListServices(w http.ResponseWriter, r *http.Request) {
	app := r.Context().Value(ctxutil.ContextKey("application")).(*dbmodel.Application)

	services, err := handler.GetApplicationHandler().ListServices(r.Context(), app)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}

	httputil.ReturnSuccess(r, w, services)
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

// ListHelmAppReleases returns the list of helm releases.
func (a *ApplicationController) ListHelmAppReleases(w http.ResponseWriter, r *http.Request) {
	app := r.Context().Value(ctxutil.ContextKey("application")).(*dbmodel.Application)

	releases, err := handler.GetApplicationHandler().ListHelmAppReleases(r.Context(), app)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}

	httputil.ReturnSuccess(r, w, releases)
}

// ListAppStatuses returns the status of the applications.
func (a *ApplicationController) ListAppStatuses(w http.ResponseWriter, r *http.Request) {
	var req model.AppStatusesReq
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil) {
		return
	}
	res, err := handler.GetApplicationHandler().ListAppStatuses(r.Context(), req.AppIDs)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, res)
}

// ListGovernanceMode list governance mode.
func (a *ApplicationController) ListGovernanceMode(w http.ResponseWriter, r *http.Request) {
	governance, err := handler.GetApplicationHandler().ListGovernanceMode()
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, governance)
}

// CheckGovernanceMode check governance mode.
func (a *ApplicationController) CheckGovernanceMode(w http.ResponseWriter, r *http.Request) {
	governanceMode := r.URL.Query().Get("governance_mode")
	err := handler.GetApplicationHandler().CheckGovernanceMode(r.Context(), governanceMode)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

// CreateGovernanceModeCR create governance mode cr.
func (a *ApplicationController) CreateGovernanceModeCR(w http.ResponseWriter, r *http.Request) {
	app := r.Context().Value(ctxutil.ContextKey("application")).(*dbmodel.Application)
	var req model.CreateUpdateGovernanceModeReq
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil) {
		return
	}
	content, err := handler.GetApplicationHandler().CreateServiceMeshCR(app, req.Provisioner)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, content)
}

// UpdateGovernanceModeCR update governance mode cr
func (a *ApplicationController) UpdateGovernanceModeCR(w http.ResponseWriter, r *http.Request) {
	app := r.Context().Value(ctxutil.ContextKey("application")).(*dbmodel.Application)
	var req model.CreateUpdateGovernanceModeReq
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil) {
		return
	}
	content, err := handler.GetApplicationHandler().UpdateServiceMeshCR(app, req.Provisioner)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, content)
}

// DeleteGovernanceModeCR delete governance mode cr
func (a *ApplicationController) DeleteGovernanceModeCR(w http.ResponseWriter, r *http.Request) {
	app := r.Context().Value(ctxutil.ContextKey("application")).(*dbmodel.Application)
	err := handler.GetApplicationHandler().DeleteServiceMeshCR(app)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

// ChangeVolumes Since the component name supports modification, the storage directory of stateful components will change.
// This interface is used to modify the original directory name to the storage directory that will actually be used.
func (a *ApplicationController) ChangeVolumes(w http.ResponseWriter, r *http.Request) {
	app := r.Context().Value(ctxutil.ContextKey("application")).(*dbmodel.Application)
	err := handler.GetApplicationHandler().ChangeVolumes(app)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}
