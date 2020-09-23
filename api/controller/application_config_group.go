package controller

import (
	"net/http"

	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/middleware"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/db"
	httputil "github.com/goodrain/rainbond/util/http"
)

// AddConfigGroup -
func (a *ApplicationStruct) AddConfigGroup(w http.ResponseWriter, r *http.Request) {
	var configReq model.ApplicationConfigGroup
	appID := r.Context().Value(middleware.ContextKey("app_id")).(string)
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &configReq, nil) {
		return
	}

	// Get the application bound serviceIDs
	var availableServiceIDs []string
	availableServices := db.GetManager().TenantServiceDao().GetServiceIDsByAppID(appID)
	for _, s := range availableServices {
		availableServiceIDs = append(availableServiceIDs, s.ServiceID)
	}
	// Judge whether the requested service ID is correct
	for _, sid := range configReq.ServiceIDs {
		if !MapKeyInStringSlice(availableServiceIDs, sid) {
			httputil.ReturnBcodeError(r, w, bcode.ErrServiceNotFound)
			return
		}
	}

	// create app ConfigGroups
	app, err := handler.GetApplicationHandler().AddConfigGroup(appID, &configReq)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, app)
}

// MapKeyInStringSlice -
func MapKeyInStringSlice(source []string, needle string) bool {
	set := make(map[string]struct{})
	for _, s := range source {
		set[s] = struct{}{}
	}
	_, ok := set[needle]
	return ok
}
