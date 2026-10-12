//go:build linux || darwin

package registryproxy

import (
	"context"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
)

type gcAPIRecorder struct {
	client      *coordination.CoordinationClient
	binding     coordination.StorageRegistration
	request     coordination.CoordinationRequest
	fingerprint string
	executor    *coordination.GCExecutorLocator
}

// NewGCRecorder binds executor callbacks to one immutable storage and operation.
// The launcher remains responsible for verifying process and ingress identity.
func NewGCRecorder(client *coordination.CoordinationClient, binding coordination.StorageRegistration, request coordination.CoordinationRequest) (GCExecutionRecorder, error) {
	if client == nil {
		return nil, coordination.ErrCoordinationUnavailable
	}
	if _, _, err := gcReceiptKey(binding, request); err != nil {
		return nil, err
	}
	fingerprint, _ := binding.Fingerprint()
	return &gcAPIRecorder{client: client, binding: binding, request: request, fingerprint: fingerprint}, nil
}

func (r *gcAPIRecorder) valid(value StorageMeasurement) bool {
	return coordination.IsValidStorageMeasurement(value) && value.StorageID == r.binding.StorageID && value.Generation == r.binding.Generation && value.BindingFingerprint == r.fingerprint
}

func (r *gcAPIRecorder) BeginGC(ctx context.Context, before StorageMeasurement) error {
	if !r.valid(before) {
		return ErrStorageIdentity
	}
	var err error
	if r.executor != nil {
		err = r.client.EnterGCJob(ctx, r.request, *r.executor)
	} else {
		err = r.client.EnterMaintenance(ctx, r.request)
	}
	if err != nil {
		return err
	}
	// A lost response leaves maintenance protected; never reacquire admission.
	return r.client.RecordMaintenanceMeasurement(ctx, r.request, "before", before)
}

func (r *gcAPIRecorder) CompleteGC(ctx context.Context, outcome string) error {
	if outcome != "succeeded" && outcome != "failed" && outcome != "unknown" {
		return coordination.ErrCoordinationChanged
	}
	return r.client.CompleteMaintenanceWork(ctx, r.request, outcome)
}

func (r *gcAPIRecorder) ObserveGC(ctx context.Context, after StorageMeasurement) error {
	if !r.valid(after) {
		return ErrStorageIdentity
	}
	return r.client.RecordMaintenanceMeasurement(ctx, r.request, "after", after)
}

// NewGCJobRecorder uses control-plane-verified Kubernetes execution admission.
// Unlike the legacy recorder, it cannot enter through the generic GC endpoint.
func NewGCJobRecorder(client *coordination.CoordinationClient, binding coordination.StorageRegistration, request coordination.CoordinationRequest, executor coordination.GCExecutorLocator) (GCExecutionRecorder, error) {
	if !executor.Valid() {
		return nil, coordination.ErrCoordinationChanged
	}
	recorder, err := NewGCRecorder(client, binding, request)
	if err != nil {
		return nil, err
	}
	concrete := recorder.(*gcAPIRecorder)
	concrete.executor = &executor
	return concrete, nil
}
