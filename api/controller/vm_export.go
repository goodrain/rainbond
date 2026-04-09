package controller

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	dbmodel "github.com/goodrain/rainbond/db/model"
	httputil "github.com/goodrain/rainbond/util/http"
)

type VMExportController struct {
	startExport     func(serviceID, exportID string, req *handler.VMExportRequest) (*handler.VMExportStatus, error)
	getExportStatus func(serviceID, exportID string) (*handler.VMExportStatus, error)
}

var defaultVMExportController = &VMExportController{}

func GetVMExportController() *VMExportController {
	return defaultVMExportController
}

func (c *VMExportController) StartVMExport(w http.ResponseWriter, r *http.Request) {
	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	sEvent := r.Context().Value(ctxutil.ContextKey("event")).(*dbmodel.ServiceEvent)
	var reqBody handler.VMExportRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		httputil.ReturnError(r, w, http.StatusBadRequest, "invalid request body")
		return
	}
	start := c.startExport
	if start == nil {
		start = handler.GetServiceManager().StartVMExport
	}
	status, err := start(serviceID, sEvent.EventID, &reqBody)
	if err != nil {
		if errors.Is(err, handler.ErrServiceNotClosed) {
			httputil.ReturnError(r, w, http.StatusConflict, err.Error())
			return
		}
		httputil.ReturnError(r, w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, status)
}

func (c *VMExportController) GetVMExportStatus(w http.ResponseWriter, r *http.Request) {
	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	exportID := chi.URLParam(r, "export_id")
	getStatus := c.getExportStatus
	if getStatus == nil {
		getStatus = handler.GetServiceManager().GetVMExportStatus
	}
	status, err := getStatus(serviceID, exportID)
	if err != nil {
		httputil.ReturnError(r, w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, status)
}

func (t *TenantStruct) StartVMExport(w http.ResponseWriter, r *http.Request) {
	GetVMExportController().StartVMExport(w, r)
}

func (t *TenantStruct) GetVMExportStatus(w http.ResponseWriter, r *http.Request) {
	GetVMExportController().GetVMExportStatus(w, r)
}
