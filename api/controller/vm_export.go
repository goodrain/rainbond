package controller

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	httputil "github.com/goodrain/rainbond/util/http"
)

type VMExportController struct {
	createExport func(serviceID string, req *handler.VMExportRequest) (*handler.VMExportStatus, error)
	getExport    func(serviceID, exportName string) (*handler.VMExportStatus, error)
}

var defaultVMExportController = &VMExportController{}

func GetVMExportController() *VMExportController {
	return defaultVMExportController
}

func (c *VMExportController) CreateVMExport(w http.ResponseWriter, r *http.Request) {
	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	var reqBody handler.VMExportRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		httputil.ReturnError(r, w, http.StatusBadRequest, "invalid request body")
		return
	}
	if reqBody.Name == "" {
		httputil.ReturnError(r, w, http.StatusBadRequest, "export name is required")
		return
	}
	createExport := c.createExport
	if createExport == nil {
		createExport = handler.GetServiceManager().CreateVMExport
	}
	status, err := createExport(serviceID, &reqBody)
	if err != nil {
		httputil.ReturnError(r, w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, status)
}

func (c *VMExportController) GetVMExport(w http.ResponseWriter, r *http.Request) {
	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	exportName := chi.URLParam(r, "name")
	if exportName == "" {
		httputil.ReturnError(r, w, http.StatusBadRequest, "export name is required")
		return
	}
	getExport := c.getExport
	if getExport == nil {
		getExport = handler.GetServiceManager().GetVMExport
	}
	status, err := getExport(serviceID, exportName)
	if err != nil {
		httputil.ReturnError(r, w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, status)
}

func (t *TenantStruct) CreateVMExport(w http.ResponseWriter, r *http.Request) {
	GetVMExportController().CreateVMExport(w, r)
}

func (t *TenantStruct) GetVMExport(w http.ResponseWriter, r *http.Request) {
	GetVMExportController().GetVMExport(w, r)
}
