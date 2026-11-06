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
	observation, ready, _, _, err := h.certifyRegistryInventory(ctx, binding, self)
	return observation, ready, err
}

func (h *CleanupCoordinationHandler) certifyRegistryInventory(ctx context.Context, binding guard.StorageRegistration, self *guard.CoordinationRequest) (guard.StorageObservation, bool, guard.RegionReferenceInventory, bool, error) {
	if h.inspectRegistryCoverage == nil {
		return guard.StorageObservation{}, false, guard.RegionReferenceInventory{}, false, guard.ErrCoordinationUnavailable
	}
	var references guard.RegionReferenceInventory
	collected := false
	observation, ready, err := guard.CertifyRegistry(h.database(), binding, self, func(records []guard.ParticipantRegistration) (guard.RegistryCoverage, error) {
		coverage, inspectErr := h.inspectRegistryCoverage(ctx, binding, records)
		if inspectErr == nil {
			references = coverage.References
			collected = true
		}
		return coverage, inspectErr
	})
	return observation, ready, references, collected, err
}

func collectRegistryCoverageSources(
	ctx context.Context,
	inspectIngress func(context.Context) ([]guard.ParticipantRegistration, error),
	inspectWriters func(context.Context) ([]guard.ReferenceWriter, error),
	inspectReferences func(context.Context) (guard.RegionReferenceInventory, error),
) ([]guard.ParticipantRegistration, []guard.ReferenceWriter, guard.RegionReferenceInventory, error) {
	type ingressResult struct {
		value []guard.ParticipantRegistration
		err   error
	}
	type writersResult struct {
		value []guard.ReferenceWriter
		err   error
	}
	type referencesResult struct {
		value guard.RegionReferenceInventory
		err   error
	}
	ingressResults := make(chan ingressResult, 1)
	writersResults := make(chan writersResult, 1)
	referencesResults := make(chan referencesResult, 1)
	go func() {
		value, err := inspectIngress(ctx)
		ingressResults <- ingressResult{value: value, err: err}
	}()
	go func() {
		value, err := inspectWriters(ctx)
		writersResults <- writersResult{value: value, err: err}
	}()
	go func() {
		value, err := inspectReferences(ctx)
		referencesResults <- referencesResult{value: value, err: err}
	}()
	ingress := <-ingressResults
	writers := <-writersResults
	references := <-referencesResults
	if ingress.err != nil {
		return nil, nil, guard.RegionReferenceInventory{}, ingress.err
	}
	if writers.err != nil {
		return nil, nil, guard.RegionReferenceInventory{}, writers.err
	}
	if references.err != nil {
		return nil, nil, guard.RegionReferenceInventory{}, references.err
	}
	return ingress.value, writers.value, references.value, nil
}

func (h *CleanupCoordinationHandler) observeRegistryCoverage(ctx context.Context, binding guard.StorageRegistration, records []guard.ParticipantRegistration) (guard.RegistryCoverage, error) {
	if h.gcTarget == nil || h.inspectReferenceWriters == nil || h.clusterReferences == nil {
		return guard.RegistryCoverage{}, guard.ErrCoordinationUnavailable
	}
	client, namespace, service, err := h.gcTarget()
	if err != nil {
		return guard.RegistryCoverage{}, guard.ErrCoordinationUnavailable
	}
	ingress, writers, references, err := collectRegistryCoverageSources(
		ctx,
		func(ctx context.Context) ([]guard.ParticipantRegistration, error) {
			return kubeidentity.InspectRegistryParticipantCoverage(ctx, client, namespace, service, binding, records)
		},
		h.inspectReferenceWriters,
		h.clusterReferences,
	)
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
