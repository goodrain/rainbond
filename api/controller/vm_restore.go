package controller

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	"github.com/goodrain/rainbond/pkg/vmrestore/copyio"
	"github.com/goodrain/rainbond/pkg/vmrestore/restoreplan"
	httputil "github.com/goodrain/rainbond/util/http"
)

type restoreHTTPFacade interface {
	GetVMRestoreControl(context.Context, string) (vmrestore.RestoreControlDTO, error)
	GetVMRestore(context.Context, string, string) (vmrestore.RestoreOperationDTO, error)
	VMRestoreCommand(context.Context, string, string, string, vmrestore.RestoreCommand) (vmrestore.RestoreOperationDTO, error)
}

// GetVMRestoreControl supplies the scoped initial CAS without allocating work.
func (c *VMRestoreController) GetVMRestoreControl(w http.ResponseWriter, r *http.Request) {
	id, ok := snapshotServiceID(w, r)
	if !ok {
		return
	}
	d, err := c.facade().GetVMRestoreControl(r.Context(), id)
	if err != nil {
		restoreHTTPError(w, r, err)
		return
	}
	httputil.Return(r, w, http.StatusOK, httputil.ResponseBody{Bean: d})
}

// VMRestoreController exposes bounded, scoped Region receipt commands.
type VMRestoreController struct{ service restoreHTTPFacade }

var defaultVMRestoreController = &VMRestoreController{}

// GetVMRestoreController returns the standard restore controller.
func GetVMRestoreController() *VMRestoreController { return defaultVMRestoreController }
func (c *VMRestoreController) facade() restoreHTTPFacade {
	if c.service != nil {
		return c.service
	}
	return handler.GetServiceManager()
}

func restoreHTTPError(w http.ResponseWriter, r *http.Request, err error) {
	status, code, retry := http.StatusInternalServerError, "vm_restore_internal_error", false
	switch {
	case errors.Is(err, vmrestore.ErrInternalAttribute):
		status, code = http.StatusForbidden, "vm_restore_forbidden"
	case errors.Is(err, vmrestore.ErrNotFound):
		status, code = http.StatusNotFound, "vm_restore_not_found"
	case errors.Is(err, vmrestore.ErrSnapshotInvalid):
		status, code = http.StatusBadRequest, "vm_restore_invalid_request"
	case errors.Is(err, vmrestore.ErrRestoreCapability) || errors.Is(err, restoreplan.ErrCapability):
		status, code = http.StatusUnprocessableEntity, "vm_restore_provider_not_qualified"
	case errors.Is(err, restoreplan.ErrCapacity):
		status, code = http.StatusUnprocessableEntity, "vm_restore_capacity_insufficient"
	case errors.Is(err, vmrestore.ErrRestoreUnknown):
		status, code, retry = http.StatusAccepted, "vm_restore_outcome_unknown", true
	case errors.Is(err, vmrestore.ErrRestorePending):
		status, code, retry = http.StatusAccepted, "vm_restore_pending", true
	case errors.Is(err, vmrestore.ErrReferenceUnavailable):
		status, code, retry = http.StatusServiceUnavailable, "vm_restore_dependency_unavailable", true
	case errors.Is(err, vmrestore.ErrBusy) || errors.Is(err, vmrestore.ErrConflict) || errors.Is(err, vmrestore.ErrCorrupt) || errors.Is(err, copyio.ErrDigest) || errors.Is(err, restoreplan.ErrResultIdentity) || errors.Is(err, restoreplan.ErrResultIncomplete):
		status, code = http.StatusConflict, "vm_restore_conflict"
	}
	httputil.Return(r, w, status, httputil.ResponseBody{Msg: code, Bean: map[string]interface{}{"error_code": code, "retryable": retry, "complete": false}})
}

// Existing power routes preserve the bounded restore/admission status codes.
func writeRestorePowerError(w http.ResponseWriter, r *http.Request, err error) bool {
	var code *bcode.Code
	if errors.As(err, &code) {
		switch code.Message {
		case "vm_power_pending":
			err = vmrestore.ErrRestorePending
		case "vm_restore_busy":
			err = vmrestore.ErrBusy
		}
	}
	for _, domain := range []error{vmrestore.ErrRestorePending, vmrestore.ErrRestoreUnknown, vmrestore.ErrBusy, vmrestore.ErrConflict, vmrestore.ErrCorrupt, vmrestore.ErrRestoreCapability, vmrestore.ErrReferenceUnavailable, vmrestore.ErrInternalAttribute} {
		if errors.Is(err, domain) {
			restoreHTTPError(w, r, err)
			return true
		}
	}
	return false
}

// GetVMRestore is read-only; GET never retries a side effect or releases a fence.
func (c *VMRestoreController) GetVMRestore(w http.ResponseWriter, r *http.Request) {
	id, ok := snapshotServiceID(w, r)
	if !ok {
		return
	}
	d, err := c.facade().GetVMRestore(r.Context(), id, chi.URLParam(r, "operation_id"))
	if err != nil {
		restoreHTTPError(w, r, err)
		return
	}
	httputil.Return(r, w, http.StatusOK, httputil.ResponseBody{Bean: d})
}

// VMRestoreCommand accepts no arbitrary resources or client readiness assertion.
func (c *VMRestoreController) VMRestoreCommand(w http.ResponseWriter, r *http.Request) {
	id, ok := snapshotServiceID(w, r)
	if !ok {
		return
	}
	var request vmrestore.RestoreCommand
	if snapshotBody(w, r, &request) != nil {
		restoreHTTPError(w, r, vmrestore.ErrSnapshotInvalid)
		return
	}
	d, err := c.facade().VMRestoreCommand(r.Context(), id, chi.URLParam(r, "operation_id"), chi.URLParam(r, "restore_action"), request)
	if err != nil {
		restoreHTTPError(w, r, err)
		return
	}
	httputil.Return(r, w, http.StatusAccepted, httputil.ResponseBody{Bean: d})
}
