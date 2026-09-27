package cleanup

import (
	"context"

	"k8s.io/apimachinery/pkg/util/validation"
)

// GCExecutorLocator contains identity locators only, never claimed runtime facts.
type GCExecutorLocator struct {
	Pod    string `json:"pod"`
	PodUID string `json:"pod_uid"`
}

// Valid rejects incomplete locators before contacting the execution gate.
func (l GCExecutorLocator) Valid() bool {
	return l.Pod != "" && len(validation.IsDNS1123Subdomain(l.Pod)) == 0 && coordinationIdentity.MatchString(l.PodUID)
}

// EnterGCJob requests one admission after Core verifies the actual bound Job,
// Pod and volume. It never retries a lost execution grant.
func (c *CoordinationClient) EnterGCJob(ctx context.Context, r CoordinationRequest, executor GCExecutorLocator) error {
	if r.Kind != "gc" || !executor.Valid() {
		return ErrCoordinationChanged
	}
	return c.record(ctx, r, "maintenance/enter-job", struct {
		CoordinationRequest
		GCExecutorLocator
	}{r, executor})
}

// SubmitGCJob asks Core to derive and persist a suspended executor Job.
func (c *CoordinationClient) SubmitGCJob(ctx context.Context, r CoordinationRequest) error {
	if r.Kind != "gc" {
		return ErrCoordinationChanged
	}
	return c.record(ctx, r, "maintenance/job", r)
}

// StartGCJob requests startup of the original Job; native admission is separate.
func (c *CoordinationClient) StartGCJob(ctx context.Context, r CoordinationRequest) error {
	if r.Kind != "gc" {
		return ErrCoordinationChanged
	}
	return c.record(ctx, r, "maintenance/job/start", r)
}

// RestoreGCJob requests verified restoration, never an unconditional write flag.
func (c *CoordinationClient) RestoreGCJob(ctx context.Context, r CoordinationRequest) error {
	if r.Kind != "gc" {
		return ErrCoordinationChanged
	}
	return c.record(ctx, r, "maintenance/job/restore", r)
}

// GCJobProgress reads the original task receipt without replaying any mutation.
func (c *CoordinationClient) GCJobProgress(ctx context.Context, r CoordinationRequest) (GCJobProgress, error) {
	if r.Kind != "gc" {
		return GCJobProgress{}, ErrCoordinationChanged
	}
	response, err := c.call(ctx, r, "maintenance/job/status", r)
	if err != nil {
		return GCJobProgress{}, err
	}
	progress := response.Bean.GCJob
	if progress == nil || progress.StorageID != r.StorageID || progress.Generation != r.Generation || progress.OperationID != r.OperationID {
		return GCJobProgress{}, ErrCoordinationChanged
	}
	return *progress, nil
}

// CancelFailedGCJob cannot release a task that consumed an execution permission.
func (c *CoordinationClient) CancelFailedGCJob(ctx context.Context, r CoordinationRequest) error {
	if r.Kind != "gc" {
		return ErrCoordinationChanged
	}
	return c.record(ctx, r, "maintenance/job/cancel-failed", r)
}
