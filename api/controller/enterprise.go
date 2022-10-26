package controller

import (
	typesv1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"net/http"

	"github.com/go-chi/chi"

	"github.com/goodrain/rainbond/api/handler"

	httputil "github.com/goodrain/rainbond/util/http"
)

//GetRunningServices list all running service ids
func GetRunningServices(w http.ResponseWriter, r *http.Request) {
	enterpriseID := chi.URLParam(r, "enterprise_id")
	serviceStatusList, err := handler.GetServiceManager().GetEnterpriseServicesStatus(enterpriseID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	retServices := make([]string, 0, 10)
	for service, status := range serviceStatusList {
		if status == typesv1.RUNNING {
			retServices = append(retServices, service)
		}
	}
	httputil.ReturnNoFomart(r, w, 200, map[string]interface{}{"service_ids": retServices})
}

//GetAbnormalStatus -
func GetAbnormalStatus(w http.ResponseWriter, r *http.Request) {
	enterpriseID := chi.URLParam(r, "enterprise_id")
	serviceStatusList, err := handler.GetServiceManager().GetEnterpriseServicesStatus(enterpriseID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	retServices := make([]string, 0, 10)
	for service, status := range serviceStatusList {
		if status == typesv1.ABNORMAL || status == typesv1.SOMEABNORMAL {
			retServices = append(retServices, service)
		}
	}
	httputil.ReturnNoFomart(r, w, 200, map[string]interface{}{"service_ids": retServices})
}
