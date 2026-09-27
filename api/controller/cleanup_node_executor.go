package controller

import (
	"context"
	"errors"
	"net/http"

	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/cleanup/kubeidentity"
	httputil "github.com/goodrain/rainbond/util/http"
)

func (h *CleanupCoordinationHandler) inspectNodeExecution(ctx context.Context, request guard.CoordinationRequest, locator guard.NodeExecutorLocator, terminal bool) (guard.NodeExecutorIdentity, error) {
	denied := guard.NodeExecutorIdentity{}
	if err := ctx.Err(); err != nil {
		return denied, err
	}
	if h.gcTarget == nil {
		return denied, guard.ErrCoordinationUnavailable
	}
	database := h.database()
	execution, err := guard.ReadNodeJobBinding(database, request)
	if err != nil {
		return denied, err
	}
	client, namespace, _, err := h.gcTarget()
	if err != nil || client == nil || execution.Namespace != namespace {
		return denied, guard.ErrCoordinationChanged
	}
	job, err := guard.ReconcileNodeJob(ctx, database, client.BatchV1().Jobs(namespace), request)
	if err != nil {
		return denied, err
	}
	storage, err := guard.StorageBinding(database, request.StorageID, request.Generation)
	if err != nil {
		return denied, err
	}
	var observed guard.NodeExecutorIdentity
	if terminal {
		observed, err = kubeidentity.InspectTerminatedNodeExecutor(ctx, client, job, locator.Pod, locator.PodUID, storage, execution)
	} else {
		observed, err = kubeidentity.InspectNodeExecutor(ctx, client, job, locator.Pod, locator.PodUID, storage, execution)
	}
	if contextErr := ctx.Err(); contextErr != nil {
		return denied, contextErr
	}
	return observed, err
}
func nodeAPIError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, guard.ErrNodeJobNotPrepared) {
		httputil.ReturnError(r, w, 404, "NODE_JOB_NOT_PREPARED")
		return
	}
	if errors.Is(err, kubeidentity.ErrExecutorRunning) {
		err = guard.ErrCoordinationBusy
	}
	if errors.Is(err, kubeidentity.ErrBinding) {
		err = guard.ErrCoordinationChanged
	}
	coordinationError(w, r, err)
}
func nodeRecorded(w http.ResponseWriter, r *http.Request) {
	httputil.ReturnSuccess(r, w, struct {
		Protocol int  `json:"protocol"`
		Recorded bool `json:"recorded"`
	}{1, true})
}

// EnterNodeJob admits only a Core-bound node executor using live Kubernetes facts.
// This route is always protected by CleanupIdentity; caller runtime assertions fail decoding.
func (h *CleanupCoordinationHandler) EnterNodeJob(w http.ResponseWriter, r *http.Request) {
	var body struct {
		guard.CoordinationRequest
		guard.NodeExecutorLocator
	}
	if !coordinationDecode(w, r, &body) || !coordinationScopeFromRoute(w, r, &body.CoordinationRequest) {
		return
	}
	if !body.NodeExecutorLocator.Valid() {
		httputil.ReturnError(r, w, 400, "INVALID_NODE_EXECUTOR")
		return
	}
	if err := h.validateNodeLaunchSource(r.Context(), body.CoordinationRequest); err != nil {
		nodeAPIError(w, r, err)
		return
	}
	observed, err := h.inspectNodeExecution(r.Context(), body.CoordinationRequest, body.NodeExecutorLocator, false)
	if err != nil {
		nodeAPIError(w, r, err)
		return
	}
	if err := guard.EnterNodeExecution(h.database(), body.CoordinationRequest, observed); err != nil {
		nodeAPIError(w, r, err)
		return
	}
	nodeRecorded(w, r)
}

// RecordNodeJobResult stores effects from the original live or terminated executor.
// It never releases deletion protection or accepts caller termination assertions.
func (h *CleanupCoordinationHandler) RecordNodeJobResult(w http.ResponseWriter, r *http.Request) {
	var body struct {
		guard.CoordinationRequest
		guard.NodeExecutorLocator
		Result guard.NodeExecutionResult `json:"result"`
	}
	if !coordinationDecode(w, r, &body) || !coordinationScopeFromRoute(w, r, &body.CoordinationRequest) {
		return
	}
	if !body.NodeExecutorLocator.Valid() {
		httputil.ReturnError(r, w, 400, "INVALID_NODE_EXECUTOR")
		return
	}
	recorded, err := guard.NodeResultRecorded(h.database(), body.CoordinationRequest, body.NodeExecutorLocator, body.Result)
	if err != nil {
		nodeAPIError(w, r, err)
		return
	}
	if recorded {
		nodeRecorded(w, r)
		return
	}
	observed, err := h.inspectNodeExecution(r.Context(), body.CoordinationRequest, body.NodeExecutorLocator, false)
	if errors.Is(err, kubeidentity.ErrBinding) {
		observed, err = h.inspectNodeExecution(r.Context(), body.CoordinationRequest, body.NodeExecutorLocator, true)
	}
	if err != nil {
		nodeAPIError(w, r, err)
		return
	}
	if err := guard.RecordNodeResult(h.database(), body.CoordinationRequest, observed, body.Result); err != nil {
		nodeAPIError(w, r, err)
		return
	}
	nodeRecorded(w, r)
}

// FinishNodeJob checks original container termination before releasing its scope.
func (h *CleanupCoordinationHandler) FinishNodeJob(w http.ResponseWriter, r *http.Request) {
	var body struct {
		guard.CoordinationRequest
		guard.NodeExecutorLocator
	}
	if !coordinationDecode(w, r, &body) || !coordinationScopeFromRoute(w, r, &body.CoordinationRequest) {
		return
	}
	if !body.NodeExecutorLocator.Valid() {
		httputil.ReturnError(r, w, 400, "INVALID_NODE_EXECUTOR")
		return
	}
	progress, err := guard.ReadNodeJobProgress(h.database(), body.CoordinationRequest)
	if err != nil {
		nodeAPIError(w, r, err)
		return
	}
	if progress.State == "finished" {
		if progress.Execution.PodName != body.Pod || progress.Execution.PodUID != body.PodUID {
			nodeAPIError(w, r, guard.ErrCoordinationChanged)
			return
		}
		nodeRecorded(w, r)
		return
	}
	observed, err := h.inspectNodeExecution(r.Context(), body.CoordinationRequest, body.NodeExecutorLocator, true)
	if err != nil {
		nodeAPIError(w, r, err)
		return
	}
	if err := guard.FinishNodeExecution(h.database(), body.CoordinationRequest, observed); err != nil {
		nodeAPIError(w, r, err)
		return
	}
	nodeRecorded(w, r)
}

// NodeJobProgress reads a durable task receipt without mutating its native Job.
func (h *CleanupCoordinationHandler) NodeJobProgress(w http.ResponseWriter, r *http.Request) {
	var request guard.CoordinationRequest
	if !coordinationDecode(w, r, &request) || !coordinationScopeFromRoute(w, r, &request) {
		return
	}
	result, err := guard.ReadNodeJobProgress(h.database(), request)
	if err != nil {
		nodeAPIError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol int                   `json:"protocol"`
		NodeJob  guard.NodeJobProgress `json:"node_job"`
	}{1, result})
}
