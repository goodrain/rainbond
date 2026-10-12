package controller

import (
	"context"
	"errors"
	"net/http"
	"os"

	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/cleanup/kubeidentity"
	httputil "github.com/goodrain/rainbond/util/http"
)

// NodeJobSelection contains the saved scan identity; paths and executor settings
// are intentionally absent. All supplied locators are checked against Kubernetes.
type NodeJobSelection struct {
	guard.CoordinationRequest
	SourcePod        string `json:"source_pod"`
	SourceUID        string `json:"source_uid"`
	NodeName         string `json:"node_name"`
	NodeUID          string `json:"node_uid"`
	Entry            string `json:"entry"`
	EntryFingerprint string `json:"entry_fingerprint"`
}

func systemNodeJobSettings() kubeidentity.NodeJobSettings {
	return kubeidentity.NodeJobSettings{Region: os.Getenv("REGION_NAME"), Image: os.Getenv("CLEANUP_NODE_EXECUTOR_IMAGE"), Endpoint: os.Getenv("CLEANUP_NODE_CORE_ENDPOINT"), CredentialSecret: os.Getenv("CLEANUP_NODE_CORE_SECRET"), StateClaim: os.Getenv("CLEANUP_NODE_STATE_CLAIM"), ConsoleEnterprise: os.Getenv("CLEANUP_NODE_CONSOLE_ENTERPRISE"), AllowConsoleHTTP: os.Getenv("CLEANUP_NODE_CONSOLE_ALLOW_HTTP") == "true"}
}

// SubmitNodeJob builds an immutable suspended executor for an admitted selection.
func (h *CleanupCoordinationHandler) SubmitNodeJob(w http.ResponseWriter, r *http.Request) {
	var body NodeJobSelection
	if !coordinationDecode(w, r, &body) || !coordinationScopeFromRoute(w, r, &body.CoordinationRequest) {
		return
	}
	if h.gcTarget == nil || h.nodeSettings == nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	client, namespace, _, err := h.gcTarget()
	if err != nil || client == nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	database := h.database()
	binding, err := guard.StorageBinding(database, body.StorageID, body.Generation)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	intent := guard.NodeJobIntent{Namespace: namespace, NodeName: body.NodeName, NodeUID: body.NodeUID, Entry: body.Entry, Fingerprint: body.EntryFingerprint}
	expected, err := guard.ManagedNodeRequest(binding, body.Owner, body.OperationID, body.Fingerprint, intent)
	if err != nil || expected != body.CoordinationRequest {
		coordinationError(w, r, guard.ErrCoordinationChanged)
		return
	}
	existing, err := guard.ReadNodeJobBinding(database, body.CoordinationRequest)
	if err == nil {
		if existing.Namespace != namespace || existing.NodeName != intent.NodeName || existing.NodeUID != intent.NodeUID || existing.Entry != intent.Entry || existing.Fingerprint != intent.Fingerprint {
			coordinationError(w, r, guard.ErrCoordinationChanged)
			return
		}
		if _, err := guard.ReconcileNodeJob(r.Context(), database, client.BatchV1().Jobs(namespace), body.CoordinationRequest); err != nil {
			coordinationError(w, r, err)
			return
		}
	} else {
		if !errors.Is(err, guard.ErrNodeJobNotPrepared) {
			coordinationError(w, r, err)
			return
		}
		template, err := kubeidentity.BuildManagedNodeJob(r.Context(), client, body.SourcePod, body.SourceUID, binding, body.CoordinationRequest, intent, h.nodeSettings())
		if err != nil {
			coordinationError(w, r, guard.ErrCoordinationChanged)
			return
		}
		if _, err := guard.SubmitSuspendedNodeJob(r.Context(), database, client.BatchV1().Jobs(namespace), body.CoordinationRequest, intent, template); err != nil {
			coordinationError(w, r, err)
			return
		}
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol int  `json:"protocol"`
		Recorded bool `json:"recorded"`
	}{1, true})
}

// StartNodeJob reconstructs the trusted source before starting the original Job.
func (h *CleanupCoordinationHandler) StartNodeJob(w http.ResponseWriter, r *http.Request) {
	var request guard.CoordinationRequest
	if !coordinationDecode(w, r, &request) || !coordinationScopeFromRoute(w, r, &request) {
		return
	}
	if h.gcTarget == nil || h.nodeSettings == nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	client, namespace, _, err := h.gcTarget()
	if err != nil || client == nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	database := h.database()
	binding, err := guard.ReadNodeJobBinding(database, request)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	if binding.Namespace != namespace {
		coordinationError(w, r, guard.ErrCoordinationChanged)
		return
	}
	job, err := guard.ReconcileNodeJob(r.Context(), database, client.BatchV1().Jobs(namespace), request)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	storage, err := guard.StorageBinding(database, request.StorageID, request.Generation)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	source := job.Spec.Template.Annotations
	current, err := kubeidentity.BuildManagedNodeJob(r.Context(), client, source["rainbond.io/node-source-pod"], source["rainbond.io/node-source-uid"], storage, request, binding.NodeJobIntent, h.nodeSettings())
	if err != nil || !kubeidentity.SameManagedNodeSource(job, current) {
		coordinationError(w, r, guard.ErrCoordinationChanged)
		return
	}
	if _, ready, err := h.certifyManagedNodeWriters(r.Context(), storage, &request); err != nil || !ready {
		if err == nil {
			err = guard.ErrCoordinationBusy
		}
		nodeAPIError(w, r, err)
		return
	}
	if _, err := guard.StartNodeJob(r.Context(), database, client.BatchV1().Jobs(namespace), request); err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol int  `json:"protocol"`
		Recorded bool `json:"recorded"`
	}{1, true})
}

// Recheck immediately before the one-use grant; result recovery must not depend
// on the continued availability of the builder that originally supplied a mount.
func (h *CleanupCoordinationHandler) validateNodeLaunchSource(ctx context.Context, request guard.CoordinationRequest) error {
	if h.gcTarget == nil || h.nodeSettings == nil {
		return guard.ErrCoordinationUnavailable
	}
	client, namespace, _, err := h.gcTarget()
	if err != nil || client == nil {
		return guard.ErrCoordinationUnavailable
	}
	binding, err := guard.ReadNodeJobBinding(h.database(), request)
	if err != nil {
		return err
	}
	if binding.Namespace != namespace {
		return guard.ErrCoordinationChanged
	}
	job, err := guard.ReconcileNodeJob(ctx, h.database(), client.BatchV1().Jobs(namespace), request)
	if err != nil {
		return err
	}
	storage, err := guard.StorageBinding(h.database(), request.StorageID, request.Generation)
	if err != nil {
		return err
	}
	source := job.Spec.Template.Annotations
	current, err := kubeidentity.BuildManagedNodeJob(ctx, client, source["rainbond.io/node-source-pod"], source["rainbond.io/node-source-uid"], storage, request, binding.NodeJobIntent, h.nodeSettings())
	if err != nil || !kubeidentity.SameManagedNodeSource(job, current) {
		return guard.ErrCoordinationChanged
	}
	if _, ready, err := h.certifyManagedNodeWriters(ctx, storage, &request); err != nil || !ready {
		if err != nil {
			return err
		}
		return guard.ErrCoordinationBusy
	}

	return nil
}

// CancelNodeBeforeGrant reconciles an original task that never obtained native
// authority. Core's storage transaction, not caller claims, proves that fact.
func (h *CleanupCoordinationHandler) CancelNodeBeforeGrant(w http.ResponseWriter, r *http.Request) {
	var body NodeJobSelection
	if !coordinationDecode(w, r, &body) || !coordinationScopeFromRoute(w, r, &body.CoordinationRequest) {
		return
	}
	if h.gcTarget == nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	_, namespace, _, err := h.gcTarget()
	if err != nil || namespace == "" {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	intent := guard.NodeJobIntent{Namespace: namespace, NodeName: body.NodeName, NodeUID: body.NodeUID, Entry: body.Entry, Fingerprint: body.EntryFingerprint}
	if err := guard.CancelNodeBeforeGrant(h.database(), body.CoordinationRequest, intent); err != nil {
		nodeAPIError(w, r, err)
		return
	}
	nodeRecorded(w, r)
}
