package controller

import (
	"net/http"

	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/middleware"
	"github.com/goodrain/rainbond/api/model"
	httputil "github.com/goodrain/rainbond/util/http"
)

// AddConfigGroup -
func (a *ApplicationStruct) AddConfigGroup(w http.ResponseWriter, r *http.Request) {
	var configReq model.ApplicationConfigGroup
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &configReq, nil) {
		return
	}

	appID := r.Context().Value(middleware.ContextKey("app_id")).(string)
	// create app ConfigGroups
	app, err := handler.GetApplicationHandler().AddConfigGroup(appID, &configReq)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}

	httputil.ReturnSuccess(r, w, app)
}
