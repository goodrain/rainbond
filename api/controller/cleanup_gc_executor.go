package controller

import (
	"context"
	"errors"
	"net/http"
	"time"

	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/cleanup/kubeidentity"
	httputil "github.com/goodrain/rainbond/util/http"
)

func waitForGCExecutor(ctx context.Context, interval, timeout time.Duration, inspect func() error) error {
	if inspect == nil || interval <= 0 || timeout <= 0 {
		return guard.ErrCoordinationChanged
	}
	bounded, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		err := inspect()
		if !errors.Is(err, kubeidentity.ErrExecutorStarting) {
			return err
		}
		timer := time.NewTimer(interval)
		select {
		case <-bounded.Done():
			timer.Stop()
			return bounded.Err()
		case <-timer.C:
		}
	}
}

// EnterGCJob is internal authenticated admission for the original executor.
// All runtime facts are fetched from Kubernetes; the body carries locators only.
func (h *CleanupCoordinationHandler) EnterGCJob(w http.ResponseWriter, r *http.Request) {
	var body struct {
		guard.CoordinationRequest
		guard.GCExecutorLocator
	}
	if !coordinationDecode(w, r, &body) || !coordinationScopeFromRoute(w, r, &body.CoordinationRequest) {
		return
	}
	if !body.GCExecutorLocator.Valid() {
		httputil.ReturnError(r, w, 400, "INVALID_GC_EXECUTOR")
		return
	}
	if err := r.Context().Err(); err != nil {
		coordinationError(w, r, err)
		return
	}
	if h.gcTarget == nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	database := h.database()
	binding, err := guard.ReadGCJobBinding(database, body.CoordinationRequest)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	client, namespace, service, err := h.gcTarget()
	if err != nil || client == nil || binding.Namespace != namespace {
		coordinationError(w, r, guard.ErrCoordinationChanged)
		return
	}
	job, err := guard.ReconcileGCJob(r.Context(), database, client.BatchV1().Jobs(namespace), body.CoordinationRequest)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	storage, err := guard.StorageBinding(database, body.StorageID, body.Generation)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	current, err := kubeidentity.BuildRegistryGCJob(r.Context(), client, namespace, service, storage, body.CoordinationRequest)
	if err != nil || !kubeidentity.SameRegistryGCSource(job, current) {
		coordinationError(w, r, guard.ErrCoordinationChanged)
		return
	}
	var observed kubeidentity.RegistryMountObservation
	err = waitForGCExecutor(r.Context(), 100*time.Millisecond, 5*time.Second, func() error {
		var inspectErr error
		observed, inspectErr = kubeidentity.InspectGCExecutor(r.Context(), client, service, job, body.Pod, body.PodUID, storage)
		return inspectErr
	})
	if err != nil {
		coordinationError(w, r, guard.ErrCoordinationChanged)
		return
	}
	if err := guard.BindGCExecutor(database, body.CoordinationRequest, string(job.UID), body.Pod, observed.PodUID); err != nil {
		coordinationError(w, r, err)
		return
	}
	if err := r.Context().Err(); err != nil {
		coordinationError(w, r, err)
		return
	}
	if err := guard.EnterGCJobExecution(database, body.CoordinationRequest, string(job.UID), observed.PodUID); err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol int  `json:"protocol"`
		Recorded bool `json:"recorded"`
	}{1, true})
}
