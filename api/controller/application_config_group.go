package controller

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/middleware"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/db"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
)

// AddConfigGroup -
func (a *ApplicationStruct) AddConfigGroup(w http.ResponseWriter, r *http.Request) {
	var configReq model.ApplicationConfigGroup
	appID := r.Context().Value(middleware.ContextKey("app_id")).(string)
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &configReq, nil) {
		return
	}

	if !CheckServiceExist(appID, configReq.ServiceIDs) {
		httputil.ReturnBcodeError(r, w, bcode.ErrServiceNotFound)
		return
	}

	// create app ConfigGroups
	resp, err := handler.GetApplicationHandler().AddConfigGroup(appID, &configReq)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, resp)
}

// UpdateConfigGroup -
func (a *ApplicationStruct) UpdateConfigGroup(w http.ResponseWriter, r *http.Request) {
	var updateReq model.UpdateAppConfigGroupReq
	configGroupname := chi.URLParam(r, "config_group_name")
	appID := r.Context().Value(middleware.ContextKey("app_id")).(string)
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &updateReq, nil) {
		return
	}

	if !CheckServiceExist(appID, updateReq.ServiceIDs) {
		httputil.ReturnBcodeError(r, w, bcode.ErrServiceNotFound)
		return
	}

	// update app ConfigGroups
	app, err := handler.GetApplicationHandler().UpdateConfigGroup(appID, configGroupname, &updateReq)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, app)
}

// CheckServiceExist -
func CheckServiceExist(appID string, serviceIDs []string) bool {
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
			logrus.Infof("The serviceID [%s] is not under the application or does not exist", sid)
		}
		return ok
	}
	return false
}

// DeleteConfigGroup -
func (a *ApplicationStruct) DeleteConfigGroup(w http.ResponseWriter, r *http.Request) {
	configGroupname := chi.URLParam(r, "config_group_name")
	appID := r.Context().Value(middleware.ContextKey("app_id")).(string)

	// delete app ConfigGroups
	err := handler.GetApplicationHandler().DeleteConfigGroup(appID, configGroupname)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

// ListConfigGroups -
func (a *ApplicationStruct) ListConfigGroups(w http.ResponseWriter, r *http.Request) {
	appID := r.Context().Value(middleware.ContextKey("app_id")).(string)
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
