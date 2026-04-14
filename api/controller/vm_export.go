package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	httputil "github.com/goodrain/rainbond/util/http"
)

type VMExportController struct {
	startExport     func(serviceID, exportID string, req *handler.VMExportRequest) (*handler.VMExportStatus, error)
	getExportStatus func(serviceID, exportID string) (*handler.VMExportStatus, error)
	persistExport   func(serviceID, exportID string, req *handler.VMExportPersistRequest) (*handler.VMExportPersistStatus, error)
	restorePlan     func(req *handler.VMAssetRestorePlanRequest) (*handler.VMAssetRestorePlan, error)
	setEventStatus  func(ctx context.Context, status dbmodel.EventStatus) error
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
	if reqBody.SourceKind == "snapshot" && reqBody.SnapshotName == "" {
		httputil.ReturnError(r, w, http.StatusBadRequest, "snapshot_name is required for snapshot exports")
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
	setEventStatus := c.setEventStatus
	if setEventStatus == nil {
		setEventStatus = db.GetManager().ServiceEventDao().SetEventStatus
	}
	if err := setEventStatus(r.Context(), dbmodel.EventStatusSuccess); err != nil {
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

func (c *VMExportController) PersistVMExport(w http.ResponseWriter, r *http.Request) {
	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	exportID := chi.URLParam(r, "export_id")
	var reqBody handler.VMExportPersistRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		httputil.ReturnError(r, w, http.StatusBadRequest, "invalid request body")
		return
	}
	persist := c.persistExport
	if persist == nil {
		persist = handler.GetServiceManager().PersistVMExport
	}
	status, err := persist(serviceID, exportID, &reqBody)
	if err != nil {
		httputil.ReturnError(r, w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, status)
}

func (c *VMExportController) BuildVMAssetRestorePlan(w http.ResponseWriter, r *http.Request) {
	var reqBody handler.VMAssetRestorePlanRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		httputil.ReturnError(r, w, http.StatusBadRequest, "invalid request body")
		return
	}
	build := c.restorePlan
	if build == nil {
		build = handler.GetServiceManager().BuildVMAssetRestorePlan
	}
	plan, err := build(&reqBody)
	if err != nil {
		httputil.ReturnError(r, w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, plan)
}

func (t *TenantStruct) StartVMExport(w http.ResponseWriter, r *http.Request) {
	GetVMExportController().StartVMExport(w, r)
}

func (t *TenantStruct) GetVMExportStatus(w http.ResponseWriter, r *http.Request) {
	GetVMExportController().GetVMExportStatus(w, r)
}

func (t *TenantStruct) PersistVMExport(w http.ResponseWriter, r *http.Request) {
	GetVMExportController().PersistVMExport(w, r)
}

func (t *TenantStruct) BuildVMAssetRestorePlan(w http.ResponseWriter, r *http.Request) {
	GetVMExportController().BuildVMAssetRestorePlan(w, r)
}
