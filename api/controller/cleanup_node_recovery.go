package controller

import (
	"net/http"

	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/cleanup/kubeidentity"
)

// RecoverNodeJob reconstructs only the original receipt helper. It never grants
// another native execution or mounts the cache into the recovery process.
func (h *CleanupCoordinationHandler) RecoverNodeJob(w http.ResponseWriter, r *http.Request) {
	var request guard.CoordinationRequest
	if !coordinationDecode(w, r, &request) || !coordinationScopeFromRoute(w, r, &request) {
		return
	}
	database := h.database()
	progress, err := guard.ReadNodeJobProgress(database, request)
	if err != nil {
		nodeAPIError(w, r, err)
		return
	}
	if progress.State == "finished" {
		nodeRecorded(w, r)
		return
	}
	binding := progress.Execution
	if h.gcTarget == nil || binding.PodUID == "" {
		nodeAPIError(w, r, guard.ErrCoordinationChanged)
		return
	}
	client, namespace, _, err := h.gcTarget()
	if err != nil || client == nil || namespace != binding.Namespace {
		nodeAPIError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	original, err := guard.ReconcileNodeJob(r.Context(), database, client.BatchV1().Jobs(namespace), request)
	if err != nil {
		nodeAPIError(w, r, err)
		return
	}
	storage, err := guard.StorageBinding(database, request.StorageID, request.Generation)
	if err != nil {
		nodeAPIError(w, r, err)
		return
	}
	observed, err := kubeidentity.InspectTerminatedNodeExecutor(r.Context(), client, original, binding.PodName, binding.PodUID, storage, binding)
	if err != nil {
		nodeAPIError(w, r, err)
		return
	}
	if binding.Result != nil {
		if err := guard.FinishNodeExecution(database, request, observed); err != nil {
			nodeAPIError(w, r, err)
			return
		}
		nodeRecorded(w, r)
		return
	}
	template, err := kubeidentity.BuildNodeRecoveryJob(r.Context(), client, original, storage, binding)
	if err != nil {
		nodeAPIError(w, r, err)
		return
	}
	helper, err := guard.SubmitSuspendedNodeRecovery(r.Context(), database, client.BatchV1().Jobs(namespace), request, template)
	if err != nil {
		nodeAPIError(w, r, err)
		return
	}
	latest, err := guard.ReadNodeJobProgress(database, request)
	if err != nil {
		nodeAPIError(w, r, err)
		return
	}
	if latest.State == "finished" {
		nodeRecorded(w, r)
		return
	}
	if helper.Status.Succeeded > 0 || helper.Status.Failed > 0 {
		nodeAPIError(w, r, guard.ErrCoordinationUncertain)
		return
	}
	// Revalidate the original storage immediately before releasing the suspended
	// helper; its template has no native cache mount even if later requests race.
	if err := kubeidentity.InspectNodeJournalVolume(r.Context(), client, original, binding); err != nil {
		nodeAPIError(w, r, err)
		return
	}
	if _, err := guard.StartNodeRecovery(r.Context(), database, client.BatchV1().Jobs(namespace), request); err != nil {
		nodeAPIError(w, r, err)
		return
	}
	nodeRecorded(w, r)
}
