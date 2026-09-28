package controller

import (
	"context"

	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/cleanup/kubeidentity"
)

func inspectSystemReferenceWriters(ctx context.Context) ([]guard.ReferenceWriter, error) {
	client, namespace, _, err := systemRegistryInspectionTarget()
	if err != nil {
		return nil, err
	}
	return kubeidentity.InspectReferenceWriterCoverage(ctx, client, namespace)
}

func (h *CleanupCoordinationHandler) requireReferenceWriterCoverage(ctx context.Context) error {
	if h.inspectReferenceWriters == nil {
		return guard.ErrCoordinationUnavailable
	}
	writers, err := h.inspectReferenceWriters(ctx)
	if err != nil {
		return guard.ErrCoordinationUnavailable
	}
	complete, err := guard.ReferenceWriterCoverageRegistered(h.database(), writers)
	if err != nil || !complete {
		return guard.ErrCoordinationUnavailable
	}
	return nil
}
