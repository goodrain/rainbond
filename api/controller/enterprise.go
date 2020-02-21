package controller

import (
	"github.com/goodrain/rainbond/api/handler"
	apimodel "github.com/goodrain/rainbond/api/model"
	httputil "github.com/goodrain/rainbond/util/http"
	"net/http"
)

//GetRunningServices list all running service ids
func GetRunningServices(w http.ResponseWriter, r *http.Request) {
	req := apimodel.EnterpriseTenantListStruct{}
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req.Body, nil)
	if !ok {
		return
	}
	runningList := handler.GetServiceManager().GetMultiTenantsRunningServices(req.Body.TenantIDs)
	httputil.ReturnNoFomart(r, w, 200, map[string]interface{}{"service_ids": runningList})
}
