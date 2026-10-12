package controller

import (
	"context"

	guard "github.com/goodrain/rainbond/pkg/cleanup"
)

func (h *CleanupCoordinationHandler) certifyPackageWriters(ctx context.Context, binding guard.StorageRegistration, self *guard.CoordinationRequest) (guard.StorageObservation, bool, error) {
	return guard.CertifyManagedPackageWriters(h.database(), binding, self, func() ([]guard.ReferenceWriter, error) {
		if ctx.Err() != nil || h.inspectPackageWriters == nil {
			return nil, guard.ErrCoordinationUnavailable
		}
		return h.inspectPackageWriters(ctx)
	})
}

func (h *CleanupCoordinationHandler) certifyManagedNodeWriters(ctx context.Context, binding guard.StorageRegistration, self *guard.CoordinationRequest) (guard.StorageObservation, bool, error) {
	kind, err := guard.ManagedNodeStorageKind(binding)
	if err != nil {
		return guard.StorageObservation{}, false, err
	}
	if kind == "cache" {
		return h.certifyCacheWriters(ctx, binding, self)
	}
	return h.certifyPackageWriters(ctx, binding, self)
}
