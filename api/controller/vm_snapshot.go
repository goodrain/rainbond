package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	httputil "github.com/goodrain/rainbond/util/http"
)

type snapshotHTTPFacade interface {
	PrepareVMSnapshot(context.Context, string, *handler.VMSnapshotRequest) (*handler.VMSnapshotStatus, error)
	ListVMSnapshots(context.Context, string, int, int) (vmrestore.SnapshotListDTO, error)
	GetVMSnapshot(context.Context, string, string) (vmrestore.SnapshotOperationDTO, error)
	VMSnapshotAction(context.Context, string, string, string, vmrestore.SnapshotActionRequest) (vmrestore.SnapshotOperationDTO, error)
}

// VMSnapshotController retains the existing callback seam and response field.
type VMSnapshotController struct {
	service            snapshotHTTPFacade
	createSnapshot     func(string, *handler.VMSnapshotRequest) (*handler.VMSnapshotStatus, error)
	startNativeRestore func(context.Context, string, string, string) (vmrestore.NativeRestoreDTO, error)
	getNativeRestore   func(context.Context, string, string, string) (vmrestore.NativeRestoreDTO, error)
}

var defaultVMSnapshotController = &VMSnapshotController{}

// GetVMSnapshotController returns the service-scoped snapshot HTTP controller.
func GetVMSnapshotController() *VMSnapshotController { return defaultVMSnapshotController }
func snapshotBody(w http.ResponseWriter, r *http.Request, target interface{}) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return vmrestore.ErrSnapshotInvalid
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return vmrestore.ErrSnapshotInvalid
	}
	if err := decoder.Decode(new(interface{})); err != io.EOF {
		return vmrestore.ErrSnapshotInvalid
	}
	body := json.NewDecoder(bytes.NewReader(raw))
	body.DisallowUnknownFields()
	if err := body.Decode(target); err != nil {
		return vmrestore.ErrSnapshotInvalid
	}
	return nil
}
func snapshotServiceID(w http.ResponseWriter, r *http.Request) (string, bool) {
	serviceID, _ := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	tenantID, _ := r.Context().Value(ctxutil.ContextKey("tenant_id")).(string)
	if serviceID == "" || tenantID == "" {
		snapshotHTTPError(w, r, vmrestore.ErrInternalAttribute)
		return "", false
	}
	return serviceID, true
}
func snapshotHTTPError(w http.ResponseWriter, r *http.Request, err error) {
	status, code, retry := http.StatusInternalServerError, "vm_snapshot_internal_error", false
	switch {
	case errors.Is(err, vmrestore.ErrSnapshotInvalid):
		status = http.StatusBadRequest
		code = "vm_snapshot_invalid_request"
	case errors.Is(err, vmrestore.ErrInternalAttribute):
		status = http.StatusForbidden
		code = "vm_snapshot_forbidden"
	case errors.Is(err, vmrestore.ErrNotFound):
		status = http.StatusNotFound
		code = "vm_snapshot_not_found"
	case errors.Is(err, vmrestore.ErrBusy):
		status = http.StatusConflict
		code = "vm_restore_busy"
	case errors.Is(err, vmrestore.ErrConflict) || errors.Is(err, vmrestore.ErrCorrupt):
		status = http.StatusConflict
		code = "vm_restore_conflict"
	case errors.Is(err, vmrestore.ErrSnapshotConfig):
		status = http.StatusUnprocessableEntity
		code = "vm_snapshot_config_unsupported"
	case errors.Is(err, vmrestore.ErrSnapshotStorage):
		status = http.StatusUnprocessableEntity
		code = "vm_snapshot_storage_unsupported"
	case errors.Is(err, vmrestore.ErrSnapshotIncomplete):
		status = http.StatusUnprocessableEntity
		code = "vm_snapshot_incomplete"
	case errors.Is(err, vmrestore.ErrSnapshotPending):
		status = http.StatusConflict
		code = "vm_snapshot_pending"
		retry = true
	case errors.Is(err, vmrestore.ErrReferenceUnavailable):
		status = http.StatusServiceUnavailable
		code = "vm_snapshot_dependency_unavailable"
		retry = true
	case errors.Is(err, vmrestore.ErrRestoreUnknown):
		status = http.StatusServiceUnavailable
		code = "vm_restore_outcome_unknown"
		retry = true
	case errors.Is(err, vmrestore.ErrRestoreCapability):
		status = http.StatusUnprocessableEntity
		code = "vm_restore_provider_not_qualified"
	case errors.Is(err, vmrestore.ErrNativeRestoreConfigurationChanged):
		status = http.StatusUnprocessableEntity
		code = "vm_restore_native_configuration_changed"
	}
	httputil.Return(r, w, status, httputil.ResponseBody{Msg: code, Bean: map[string]interface{}{"error": map[string]interface{}{"code": code, "retryable": retry}}})
}

type nativeRestoreRequest struct {
	RequestID string `json:"request_id"`
}

// RestoreVMSnapshot starts one idempotent offline native restore. The target VM
// must already be stopped and remains stopped after completion.
func (c *VMSnapshotController) RestoreVMSnapshot(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := snapshotServiceID(w, r)
	if !ok {
		return
	}
	var request nativeRestoreRequest
	if err := snapshotBody(w, r, &request); err != nil || !vmrestore.ValidNativeRestoreRequestID(request.RequestID) {
		snapshotHTTPError(w, r, vmrestore.ErrSnapshotInvalid)
		return
	}
	snapshotName := chi.URLParam(r, "name")
	var result vmrestore.NativeRestoreDTO
	var err error
	if c.startNativeRestore != nil {
		result, err = c.startNativeRestore(r.Context(), serviceID, snapshotName, request.RequestID)
	} else {
		result, err = handler.GetServiceManager().StartNativeVMRestore(r.Context(), serviceID, snapshotName, request.RequestID)
	}
	if err != nil {
		snapshotHTTPError(w, r, err)
		return
	}
	httputil.Return(r, w, http.StatusAccepted, httputil.ResponseBody{Bean: result})
}

// GetVMSnapshotRestore observes the deterministic native restore without
// progressing or starting the VM.
func (c *VMSnapshotController) GetVMSnapshotRestore(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := snapshotServiceID(w, r)
	if !ok {
		return
	}
	snapshotName, requestID := chi.URLParam(r, "name"), chi.URLParam(r, "request_id")
	if !vmrestore.ValidNativeRestoreRequestID(requestID) {
		snapshotHTTPError(w, r, vmrestore.ErrSnapshotInvalid)
		return
	}
	var result vmrestore.NativeRestoreDTO
	var err error
	if c.getNativeRestore != nil {
		result, err = c.getNativeRestore(r.Context(), serviceID, snapshotName, requestID)
	} else {
		result, err = handler.GetServiceManager().GetNativeVMRestore(r.Context(), serviceID, snapshotName, requestID)
	}
	if err != nil {
		snapshotHTTPError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, result)
}

// CreateVMSnapshot accepts an asynchronous operation; it does not wait for shutdown.
func (c *VMSnapshotController) CreateVMSnapshot(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := snapshotServiceID(w, r)
	if !ok {
		return
	}
	var request handler.VMSnapshotRequest
	if err := snapshotBody(w, r, &request); err != nil {
		snapshotHTTPError(w, r, err)
		return
	}
	if request.Name == "" {
		snapshotHTTPError(w, r, vmrestore.ErrSnapshotInvalid)
		return
	}
	var status *handler.VMSnapshotStatus
	var err error
	if c.createSnapshot != nil {
		status, err = c.createSnapshot(serviceID, &request)
	} else {
		status, err = c.snapshotFacade().PrepareVMSnapshot(r.Context(), serviceID, &request)
	}
	if err != nil {
		snapshotHTTPError(w, r, err)
		return
	}
	httputil.Return(r, w, http.StatusAccepted, httputil.ResponseBody{Bean: status})
}

// ListVMSnapshots pages safe Region data; Console adds its durable manifest state.
func (c *VMSnapshotController) ListVMSnapshots(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := snapshotServiceID(w, r)
	if !ok {
		return
	}
	page, size := 1, 20
	if raw := r.URL.Query().Get("page"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			snapshotHTTPError(w, r, vmrestore.ErrSnapshotInvalid)
			return
		}
		page = value
	}
	if raw := r.URL.Query().Get("page_size"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			snapshotHTTPError(w, r, vmrestore.ErrSnapshotInvalid)
			return
		}
		size = value
	}
	if page < 1 || size < 1 || size > 100 {
		snapshotHTTPError(w, r, vmrestore.ErrSnapshotInvalid)
		return
	}
	result, err := c.snapshotFacade().ListVMSnapshots(r.Context(), serviceID, page, size)
	if err != nil {
		snapshotHTTPError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, result)
}

// GetVMSnapshot performs only a scoped read.
func (c *VMSnapshotController) GetVMSnapshot(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := snapshotServiceID(w, r)
	if !ok {
		return
	}
	result, err := c.snapshotFacade().GetVMSnapshot(r.Context(), serviceID, chi.URLParam(r, "name"))
	if err != nil {
		snapshotHTTPError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, result)
}

// SnapshotAction is routed only for the explicit action whitelist below.
func (c *VMSnapshotController) SnapshotAction(w http.ResponseWriter, r *http.Request) {
	serviceID, ok := snapshotServiceID(w, r)
	if !ok {
		return
	}
	action := chi.URLParam(r, "snapshot_action")
	if r.Method == http.MethodDelete {
		action = "delete"
	}
	switch action {
	case "halt", "create", "resume", "finalize", "cancel", "delete":
	default:
		snapshotHTTPError(w, r, vmrestore.ErrSnapshotInvalid)
		return
	}
	var request vmrestore.SnapshotActionRequest
	if err := snapshotBody(w, r, &request); err != nil {
		snapshotHTTPError(w, r, err)
		return
	}
	result, err := c.snapshotFacade().VMSnapshotAction(r.Context(), serviceID, chi.URLParam(r, "name"), action, request)
	if err != nil && !(errors.Is(err, vmrestore.ErrSnapshotPending) && result.OperationID != "") {
		snapshotHTTPError(w, r, err)
		return
	}
	httputil.Return(r, w, http.StatusAccepted, httputil.ResponseBody{Bean: result})
}

// CreateVMSnapshot delegates the existing Tenant controller method.
func (t *TenantStruct) CreateVMSnapshot(w http.ResponseWriter, r *http.Request) {
	GetVMSnapshotController().CreateVMSnapshot(w, r)
}

func (c *VMSnapshotController) snapshotFacade() snapshotHTTPFacade {
	if c.service != nil {
		return c.service
	}
	return handler.GetServiceManager()
}
