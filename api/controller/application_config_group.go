package controller

import (
	dbmodel "github.com/goodrain/rainbond/db/model"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/model"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/db"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
)

// AddConfigGroup -
func (a *ApplicationController) AddConfigGroup(w http.ResponseWriter, r *http.Request) {
	var configReq model.ApplicationConfigGroup
	appID := r.Context().Value(ctxutil.ContextKey("app_id")).(string)
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &configReq, nil) {
		return
	}

	checkServiceExist(appID, configReq.ServiceIDs)

	// create app ConfigGroups
	resp, err := handler.GetApplicationHandler().AddConfigGroup(appID, &configReq)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, resp)
}

// UpdateConfigGroup -
func (a *ApplicationController) UpdateConfigGroup(w http.ResponseWriter, r *http.Request) {
	var updateReq model.UpdateAppConfigGroupReq
	configGroupname := chi.URLParam(r, "config_group_name")
	appID := r.Context().Value(ctxutil.ContextKey("app_id")).(string)
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &updateReq, nil) {
		return
	}

	checkServiceExist(appID, updateReq.ServiceIDs)

	// update app ConfigGroups
	app, err := handler.GetApplicationHandler().UpdateConfigGroup(appID, configGroupname, &updateReq)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, app)
}

// checkServiceExist -
func checkServiceExist(appID string, serviceIDs []string) {
	// Get the application bound serviceIDs
	availableServices := db.GetManager().TenantServiceDao().GetServiceIDsByAppID(appID)
	// Judge whether the requested service ID is correct
	set := make(map[string]struct{})
	for _, s := range availableServices {
		set[s.ServiceID] = struct{}{}
	}
	for _, sid := range serviceIDs {
		_, ok := set[sid]
		if !ok {
			logrus.Warningf("The serviceID [%s] is not under the application or does not exist", sid)
		}
	}
}

// DeleteConfigGroup -
func (a *ApplicationController) DeleteConfigGroup(w http.ResponseWriter, r *http.Request) {
	configGroupname := chi.URLParam(r, "config_group_name")
	appID := r.Context().Value(ctxutil.ContextKey("app_id")).(string)

	// delete app ConfigGroups
	err := handler.GetApplicationHandler().DeleteConfigGroup(appID, configGroupname)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

// BatchDeleteConfigGroup -
func (a *ApplicationController) BatchDeleteConfigGroup(w http.ResponseWriter, r *http.Request) {
	configGroupnames := chi.URLParam(r, "config_group_names")
	appID := r.Context().Value(ctxutil.ContextKey("app_id")).(string)

	// batch delete app ConfigGroups
	err := handler.GetApplicationHandler().BatchDeleteConfigGroup(appID, configGroupnames)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

// ListConfigGroups -
func (a *ApplicationController) ListConfigGroups(w http.ResponseWriter, r *http.Request) {
	appID := r.Context().Value(ctxutil.ContextKey("app_id")).(string)
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

	// list app ConfigGroups
	resp, err := handler.GetApplicationHandler().ListConfigGroups(appID, page, pageSize)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, resp)
}

// SyncComponents -
func (a *ApplicationController) SyncComponents(w http.ResponseWriter, r *http.Request) {
	var syncComponentReq model.SyncComponentReq
	app := r.Context().Value(ctxutil.ContextKey("application")).(*dbmodel.Application)
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &syncComponentReq, nil) {
		return
	}
	err := handler.GetApplicationHandler().SyncComponents(app, syncComponentReq.Components, syncComponentReq.DeleteComponentIDs)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

// SyncAppConfigGroups -
func (a *ApplicationController) SyncAppConfigGroups(w http.ResponseWriter, r *http.Request) {
	var syncAppConfigGroupReq model.SyncAppConfigGroup
	app := r.Context().Value(ctxutil.ContextKey("application")).(*dbmodel.Application)
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &syncAppConfigGroupReq, nil) {
		return
	}
	err := handler.GetApplicationHandler().SyncAppConfigGroups(app, syncAppConfigGroupReq.AppConfigGroups)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}
