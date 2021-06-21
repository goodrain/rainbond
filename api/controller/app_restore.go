package controller

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/model"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
)

// AppRestoreController is an implementation of AppRestoreInterface
type AppRestoreController struct {
}

// RestoreEnvs restores environment variables. delete the existing environment
// variables first, then create the ones in the request body.
func (a *AppRestoreController) RestoreEnvs(w http.ResponseWriter, r *http.Request) {
	var req model.RestoreEnvsReq
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}

	tenantID := r.Context().Value(ctxutil.ContextKey("tenant_id")).(string)
	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	err := handler.GetAppRestoreHandler().RestoreEnvs(tenantID, serviceID, &req)
	if err != nil {
		format := "Service ID: %s; failed to restore envs: %v"
		logrus.Errorf(format, serviceID, err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf(format, serviceID, err))
		return
	}

	httputil.ReturnSuccess(r, w, "ok")
	return
}

// RestorePorts restores service ports. delete the existing ports first,
// then create the ones in the request body.
func (a *AppRestoreController) RestorePorts(w http.ResponseWriter, r *http.Request) {
	var req model.RestorePortsReq
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil) {
		return
	}

	tenantID := r.Context().Value(ctxutil.ContextKey("tenant_id")).(string)
	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	err := handler.GetAppRestoreHandler().RestorePorts(tenantID, serviceID, &req)
	if err != nil {
		format := "Service ID: %s; failed to restore ports: %v"
		logrus.Errorf(format, serviceID, err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf(format, serviceID, err))
		return
	}

	httputil.ReturnSuccess(r, w, "ok")
	return
}

// RestoreVolumes restores service volumes. delete the existing volumes first,
// then create the ones in the request body.
func (a *AppRestoreController) RestoreVolumes(w http.ResponseWriter, r *http.Request) {
	var req model.RestoreVolumesReq
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil) {
		return
	}

	tenantID := r.Context().Value(ctxutil.ContextKey("tenant_id")).(string)
	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	err := handler.GetAppRestoreHandler().RestoreVolumes(tenantID, serviceID, &req)
	if err != nil {
		format := "Service ID: %s; failed to restore volumes: %v"
		logrus.Errorf(format, serviceID, err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf(format, serviceID, err))
		return
	}

	httputil.ReturnSuccess(r, w, "ok")
	return
}

// RestoreProbe restores service probe. delete the existing probe first,
// then create the one in the request body.
func (a *AppRestoreController) RestoreProbe(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		format := "error reading request body: %v"
		httputil.ReturnError(r, w, 500, fmt.Sprintf(format, err))
	}
	// set a new body, which will simulate the same data we read
	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))

	var probeReq *model.ServiceProbe
	if string(body) != "" {
		var req model.ServiceProbe
		if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil) {
			return
		}
		probeReq = &req
	} else {
		probeReq = nil
	}

	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	if err := handler.GetAppRestoreHandler().RestoreProbe(serviceID, probeReq); err != nil {
		format := "Service ID: %s; failed to restore volumes: %v"
		logrus.Errorf(format, serviceID, err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf(format, serviceID, err))
		return
	}

	httputil.ReturnSuccess(r, w, "ok")
	return
}

// RestoreDeps restores service dependencies. delete the existing dependencies first,
// then create the ones in the request body.
func (a *AppRestoreController) RestoreDeps(w http.ResponseWriter, r *http.Request) {
	var req model.RestoreDepsReq
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil) {
		return
	}

	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	tenantID := r.Context().Value(ctxutil.ContextKey("tenant_id")).(string)
	err := handler.GetAppRestoreHandler().RestoreDeps(tenantID, serviceID, &req)
	if err != nil {
		format := "Service ID: %s; failed to restore service dependencies: %v"
		logrus.Errorf(format, serviceID, err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf(format, serviceID, err))
		return
	}

	httputil.ReturnSuccess(r, w, "ok")
	return
}

// RestoreDepVols restores service dependent volumes. delete the existing
// dependent volumes first, then create the ones in the request body.
func (a *AppRestoreController) RestoreDepVols(w http.ResponseWriter, r *http.Request) {
	var req model.RestoreDepVolsReq
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil); !ok {
		return
	}

	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	tenantID := r.Context().Value(ctxutil.ContextKey("tenant_id")).(string)
	err := handler.GetAppRestoreHandler().RestoreDepVols(tenantID, serviceID, &req)
	if err != nil {
		format := "Service ID: %s; failed to restore volume dependencies: %v"
		logrus.Errorf(format, serviceID, err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf(format, serviceID, err))
		return
	}

	httputil.ReturnSuccess(r, w, nil)
}

// RestorePlugins restores service plugins. delete the existing
// service plugins first, then create the ones in the request body.
func (a *AppRestoreController) RestorePlugins(w http.ResponseWriter, r *http.Request) {
	var req model.RestorePluginsReq
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil); !ok {
		return
	}

	tenantID := r.Context().Value(ctxutil.ContextKey("tenant_id")).(string)
	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	if err := handler.GetAppRestoreHandler().RestorePlugins(tenantID, serviceID, &req); err != nil {
		format := "Service ID: %s; failed to restore plugins: %v"
		logrus.Errorf(format, serviceID, err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf(format, serviceID, err))
	}
	httputil.ReturnSuccess(r, w, nil)
}
