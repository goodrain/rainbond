package controller

import (
	"errors"
	"net/http"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/cleanup/kubeidentity"
	httputil "github.com/goodrain/rainbond/util/http"
)

// RestoreGCJob restores this operation's original write admission only after
// verifying native completion, durable measurements and unchanged source state.
// It does not change the Registry deployment or accept a caller's writable flag.
func (h *CleanupCoordinationHandler) RestoreGCJob(w http.ResponseWriter, r *http.Request) {
	var request guard.CoordinationRequest
	if !coordinationDecode(w, r, &request) || !coordinationScopeFromRoute(w, r, &request) {
		return
	}
	database := h.database()
	binding, err := guard.ReadGCJobBinding(database, request)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	state, err := guard.InspectOperation(database, request)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	if state == "finished" {
		// The original verified result survives removal of an old terminal Job. This
		// idempotent acknowledgment never changes a newer maintenance operation.
		if err := guard.FinishGCJobRestore(database, request, binding.JobUID, binding.PodUID); err != nil {
			coordinationError(w, r, err)
			return
		}
	} else {
		if h.gcTarget == nil {
			coordinationError(w, r, guard.ErrCoordinationUnavailable)
			return
		}
		client, namespace, service, err := h.gcTarget()
		if err != nil || client == nil || binding.Namespace != namespace {
			coordinationError(w, r, guard.ErrCoordinationChanged)
			return
		}
		verify := func() error {
			if err := r.Context().Err(); err != nil {
				return err
			}
			job, err := guard.ReconcileGCJob(r.Context(), database, client.BatchV1().Jobs(namespace), request)
			if err != nil {
				return err
			}
			storage, err := guard.StorageBinding(database, request.StorageID, request.Generation)
			if err != nil {
				return err
			}
			current, err := kubeidentity.BuildRegistryGCJob(r.Context(), client, namespace, service, storage, request)
			if err != nil || !kubeidentity.SameRegistryGCSource(job, current) {
				return guard.ErrCoordinationChanged
			}
			if _, err := kubeidentity.InspectTerminatedGCExecutor(r.Context(), client, service, job, binding.PodName, binding.PodUID, storage); err != nil {
				if errors.Is(err, kubeidentity.ErrExecutorRunning) {
					return guard.ErrCoordinationBusy
				}
				return guard.ErrCoordinationChanged
			}
			return nil
		}
		if err := verify(); err != nil {
			coordinationError(w, r, err)
			return
		}
		if err := guard.BeginGCJobRestore(database, request, binding.JobUID, binding.PodUID); err != nil {
			coordinationError(w, r, err)
			return
		}
		if err := verify(); err != nil {
			coordinationError(w, r, err)
			return
		}
		if err := guard.FinishGCJobRestore(database, request, binding.JobUID, binding.PodUID); err != nil {
			coordinationError(w, r, err)
			return
		}
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol int  `json:"protocol"`
		Recorded bool `json:"recorded"`
	}{1, true})
}

// GCJobProgress returns durable task state and actual recorded filesystem data.
func (h *CleanupCoordinationHandler) GCJobProgress(w http.ResponseWriter, r *http.Request) {
	var request guard.CoordinationRequest
	if !coordinationDecode(w, r, &request) || !coordinationScopeFromRoute(w, r, &request) {
		return
	}
	progress, err := guard.ReadGCJobProgress(h.database(), request)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	if progress.State != "finished" && progress.JobUID != "" {
		progress.ExecutorState = "unknown"
		if h.gcTarget != nil {
			client, namespace, _, targetErr := h.gcTarget()
			if targetErr == nil && client != nil && namespace == progress.JobNamespace {
				if job, lookupErr := guard.ReconcileGCJob(r.Context(), h.database(), client.BatchV1().Jobs(namespace), request); lookupErr == nil {
					progress.ExecutorState = gcExecutorState(job)
					if progress.ExecutorState == "failed" || progress.ExecutorState == "succeeded" {
						if latest, readErr := guard.ReadGCJobProgress(h.database(), request); readErr == nil && latest.JobUID == progress.JobUID {
							latest.ExecutorState = progress.ExecutorState
							progress = latest
						}
					}
				}
			}
		}
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol int                 `json:"protocol"`
		GCJob    guard.GCJobProgress `json:"gc_job"`
	}{1, progress})
}

func gcExecutorState(job *batchv1.Job) string {
	if job == nil {
		return "unknown"
	}
	failed, complete := false, false
	for _, condition := range job.Status.Conditions {
		if condition.Status != corev1.ConditionTrue {
			continue
		}
		failed = failed || condition.Type == batchv1.JobFailed
		complete = complete || condition.Type == batchv1.JobComplete
	}
	if failed && complete {
		return "unknown"
	}
	if job.Status.Active > 0 {
		if failed || complete {
			return "unknown"
		}
		return "running"
	}
	if failed {
		return "failed"
	}
	if complete {
		return "succeeded"
	}
	return "waiting"
}

// CancelFailedGCJob ends maintenance only when the original Job has failed and
// no execution permission was granted. The final DB transition checks this
// atomically, so a concurrent late admission cannot be canceled or unlocked.
func (h *CleanupCoordinationHandler) CancelFailedGCJob(w http.ResponseWriter, r *http.Request) {
	var request guard.CoordinationRequest
	if !coordinationDecode(w, r, &request) || !coordinationScopeFromRoute(w, r, &request) {
		return
	}
	database := h.database()
	progress, err := guard.ReadGCJobProgress(database, request)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	if progress.State == "finished" && progress.Outcome == "canceled" {
		h.maintenanceTransitionResult(w, r)
		return
	}
	if progress.State != "draining" || progress.JobUID == "" || h.gcTarget == nil {
		coordinationError(w, r, guard.ErrCoordinationChanged)
		return
	}
	client, namespace, service, err := h.gcTarget()
	if err != nil || client == nil || namespace != progress.JobNamespace {
		coordinationError(w, r, guard.ErrCoordinationChanged)
		return
	}
	job, err := guard.ReconcileGCJob(r.Context(), database, client.BatchV1().Jobs(namespace), request)
	if err != nil || gcExecutorState(job) != "failed" {
		coordinationError(w, r, guard.ErrCoordinationChanged)
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
	if err := guard.CancelMaintenanceDrain(database, request); err != nil {
		coordinationError(w, r, err)
		return
	}
	h.maintenanceTransitionResult(w, r)
}
func (h *CleanupCoordinationHandler) maintenanceTransitionResult(w http.ResponseWriter, r *http.Request) {
	httputil.ReturnSuccess(r, w, struct {
		Protocol int  `json:"protocol"`
		Recorded bool `json:"recorded"`
	}{1, true})
}
