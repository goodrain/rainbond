package controller

import (
	"context"
	"reflect"

	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/cleanup/kubeidentity"
)

// certifyRegistry derives coverage from the configured system Registry and
// current platform instances. No deployment facts come from caller claims.
func (h *CleanupCoordinationHandler) certifyRegistry(ctx context.Context, binding guard.StorageRegistration, self *guard.CoordinationRequest) (guard.StorageObservation, bool, error) {
	if h.inspectRegistryCoverage == nil {
		return guard.StorageObservation{}, false, guard.ErrCoordinationUnavailable
	}
	return guard.CertifyRegistry(h.database(), binding, self, func(records []guard.ParticipantRegistration) (guard.RegistryCoverage, error) {
		return h.inspectRegistryCoverage(ctx, binding, records)
	})
}

func (h *CleanupCoordinationHandler) observeRegistryCoverage(ctx context.Context, binding guard.StorageRegistration, records []guard.ParticipantRegistration) (guard.RegistryCoverage, error) {
	if h.gcTarget == nil || h.inspectReferenceWriters == nil || h.clusterReferences == nil {
		return guard.RegistryCoverage{}, guard.ErrCoordinationUnavailable
	}
	client, namespace, service, err := h.gcTarget()
	if err != nil {
		return guard.RegistryCoverage{}, guard.ErrCoordinationUnavailable
	}
	ingress, err := kubeidentity.InspectRegistryParticipantCoverage(ctx, client, namespace, service, binding, records)
	if err != nil {
		return guard.RegistryCoverage{}, err
	}
	writers, err := h.inspectReferenceWriters(ctx)
	if err != nil {
		return guard.RegistryCoverage{}, err
	}
	references, err := h.clusterReferences(ctx)
	if err != nil {
		return guard.RegistryCoverage{}, err
	}
	// Recheck ingress after the other remote sources, under the same DB storage
	// lock. The inspector requires exact identities from the original records.
	ingress, err = kubeidentity.InspectRegistryParticipantCoverage(ctx, client, namespace, service, binding, ingress)
	if err != nil {
		return guard.RegistryCoverage{}, err
	}
	currentWriters, err := h.inspectReferenceWriters(ctx)
	if err != nil || !reflect.DeepEqual(writers, currentWriters) {
		return guard.RegistryCoverage{}, guard.ErrCoordinationChanged
	}
	if err := ctx.Err(); err != nil {
		return guard.RegistryCoverage{}, err
	}
	return guard.RegistryCoverage{Ingress: ingress, Writers: writers, References: references}, nil
}
