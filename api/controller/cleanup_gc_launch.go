package controller

import (
	"errors"
	"net/http"

	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/cleanup/kubeidentity"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/jinzhu/gorm"
)

// SubmitGCJob derives an immutable suspended Job from the installed coordinator.
// It never accepts an image, volume, credentials or Pod specification in JSON.
func (h *CleanupCoordinationHandler) SubmitGCJob(w http.ResponseWriter, r *http.Request) {
	var request guard.CoordinationRequest
	if !coordinationDecode(w, r, &request) || !coordinationScopeFromRoute(w, r, &request) {
		return
	}
	if h.gcTarget == nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	client, namespace, service, err := h.gcTarget()
	if err != nil || client == nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	database := h.database()
	existing, err := guard.ReadGCJobBinding(database, request)
	if err == nil {
		if existing.Namespace != namespace {
			coordinationError(w, r, guard.ErrCoordinationChanged)
			return
		}
		if _, err := guard.ReconcileGCJob(r.Context(), database, client.BatchV1().Jobs(namespace), request); err != nil {
			coordinationError(w, r, err)
			return
		}
	} else {
		if !errors.Is(err, guard.ErrGCJobNotPrepared) && !gorm.IsRecordNotFoundError(err) {
			coordinationError(w, r, err)
			return
		}
		binding, err := guard.StorageBinding(database, request.StorageID, request.Generation)
		if err != nil {
			coordinationError(w, r, err)
			return
		}
		template, err := kubeidentity.BuildRegistryGCJob(r.Context(), client, namespace, service, binding, request)
		if err != nil {
			coordinationError(w, r, guard.ErrCoordinationChanged)
			return
		}
		if _, err := guard.RequestMaintenance(database, request); err != nil {
			coordinationError(w, r, err)
			return
		}
		if _, err := guard.SubmitSuspendedGCJob(r.Context(), database, client.BatchV1().Jobs(namespace), request, template); err != nil {
			coordinationError(w, r, err)
			return
		}
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol int  `json:"protocol"`
		Recorded bool `json:"recorded"`
	}{1, true})
}

// StartGCJob rechecks the current Registry before unsuspending the recorded Job.
// Execution remains guarded by the original operation and verified Pod identity.
func (h *CleanupCoordinationHandler) StartGCJob(w http.ResponseWriter, r *http.Request) {
	var request guard.CoordinationRequest
	if !coordinationDecode(w, r, &request) || !coordinationScopeFromRoute(w, r, &request) {
		return
	}
	if h.gcTarget == nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	client, namespace, service, err := h.gcTarget()
	if err != nil || client == nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	database := h.database()
	binding, err := guard.ReadGCJobBinding(database, request)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	if binding.Namespace != namespace {
		coordinationError(w, r, guard.ErrCoordinationChanged)
		return
	}
	job, err := guard.ReconcileGCJob(r.Context(), database, client.BatchV1().Jobs(namespace), request)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	storage, err := guard.StorageBinding(database, request.StorageID, request.Generation)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	current, err := kubeidentity.BuildRegistryGCJob(r.Context(), client, namespace, service, storage, request)
	if err != nil || !kubeidentity.SameRegistryGCSource(job, current) {
		coordinationError(w, r, guard.ErrCoordinationChanged)
		return
	}
	if _, err := guard.StartGCJob(r.Context(), database, client.BatchV1().Jobs(namespace), request); err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol int  `json:"protocol"`
		Recorded bool `json:"recorded"`
	}{1, true})
}
