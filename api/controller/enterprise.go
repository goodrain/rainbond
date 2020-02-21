package controller

import (
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	httputil "github.com/goodrain/rainbond/util/http"
	"net/http"
)

//GetRunningServices list all running service ids
func GetRunningServices(w http.ResponseWriter, r *http.Request) {
	enterpriseID := chi.URLParam(r, "enterprise_id")
	runningList := handler.GetServiceManager().GetEnterpriseRunningServices(enterpriseID)
	httputil.ReturnNoFomart(r, w, 200, map[string]interface{}{"service_ids": runningList})
}
