package controller

import (
	"context"

	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/cleanup/kubeidentity"
)

func (h *CleanupCoordinationHandler) certifyCacheWriters(ctx context.Context, binding guard.StorageRegistration, self *guard.CoordinationRequest) (guard.StorageObservation, bool, error) {
	return guard.CertifyManagedCacheWriters(h.database(), binding, self, func() ([]guard.ParticipantRegistration, error) {
		if ctx.Err() != nil || h.gcTarget == nil {
			return nil, guard.ErrCoordinationUnavailable
		}
		client, namespace, _, err := h.gcTarget()
		if err != nil || client == nil {
			return nil, guard.ErrCoordinationUnavailable
		}
		return kubeidentity.InspectCacheWriterCoverage(ctx, client, namespace, binding)
	})
}
