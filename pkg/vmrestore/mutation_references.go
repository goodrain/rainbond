package vmrestore

import (
	"context"
	"reflect"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// VolumeReferencesForMutation rechecks the fixed targets of an already-issued
// disk deletion. It is an internal read, not an admission grant or a replacement
// for ResolveVolume. Missing targets are allowed only by that operation's receipt.
func (r *Resolver) VolumeReferencesForMutation(ctx context.Context, scope Scope, volumeID uint, operationID string) (ReferenceSummary, error) {
	empty := ReferenceSummary{VolumeID: volumeID}
	if ctx == nil || r == nil || r.store == nil || r.store.db == nil || r.core == nil || r.dynamic == nil {
		return empty, ErrReferenceUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	current, err := r.store.Read(scope, operationID)
	if err != nil {
		return empty, err
	}
	binding, err := r.reclaimBinding(ctx, scope, volumeID, operationID, current)
	if err != nil {
		return empty, err
	}
	pvc, err := r.reclaimTargets(ctx, scope, binding)
	if err != nil {
		return empty, err
	}
	resolved := ResolvedVolume{Scope: scope, VolumeID: binding.VolumeID, DiskKey: binding.DiskKey,
		CanonicalName: binding.CanonicalName, Role: binding.Role, Attached: binding.Attached, Mapped: true,
		ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration,
		ActivePVC: &binding.ActivePVC, ActiveDV: binding.ActiveDV, Provisioning: "reclaiming",
		RestoreID: binding.RestoreID, SnapshotUID: binding.SnapshotUID,
		PreviousPVC: binding.PreviousPVC, PreviousDV: binding.PreviousDV, Retained: []ObjectRef{}}
	if binding.PreviousPVC != nil {
		resolved.Retained = append(resolved.Retained, *binding.PreviousPVC)
	}
	if binding.PreviousDV != nil {
		resolved.Retained = append(resolved.Retained, *binding.PreviousDV)
	}
	summary, err := r.volumeReferences(ctx, scope, resolved, current, true, pvc != nil)
	if err != nil {
		summary.CanDelete = false
		return summary, err
	}
	// A same-name replacement during the reference scan must not inherit the
	// missing target's permission. Consumers still use UID preconditions on DELETE.
	if _, err = r.reclaimTargets(ctx, scope, binding); err != nil {
		summary.CanDelete = false
		return summary, err
	}
	if err = r.stable(scope, current.State); err != nil {
		summary.CanDelete = false
		return summary, err
	}
	return summary, nil
}

func (r *Resolver) reclaimBinding(ctx context.Context, scope Scope, volumeID uint, operationID string, current Current) (Binding, error) {
	empty := Binding{}
	a := current.State.Admission
	if current.State.Fence != nil || a == nil || a.Token != operationID || a.OutcomeUnknown || !ownsAdmissionContext(ctx, scope, a.Token) {
		return empty, ErrBusy
	}
	receipt := current.Receipt
	if receipt == nil || a.Action != "disk-delete-"+receipt.ManifestHash {
		return empty, ErrConflict
	}
	prepared, preparedOK := receipt.Steps["prepare"]
	issued, issuedOK := receipt.Steps["issued"]
	if !preparedOK || !issuedOK || !prepared.Applied || !issued.Applied ||
		prepared.RequestHash != receipt.ManifestHash || issued.RequestHash != receipt.ManifestHash ||
		prepared.BindingGeneration != current.State.BindingGeneration || issued.BindingGeneration != current.State.BindingGeneration ||
		issued.ControlRevision <= prepared.ControlRevision || receipt.Steps["complete"].Applied {
		return empty, ErrConflict
	}
	if err := r.validateBindingRecords(scope, current.State); err != nil {
		return empty, err
	}
	volume, err := r.volume(scope, volumeID)
	if err != nil {
		return empty, err
	}
	key := strconv.FormatUint(uint64(volumeID), 10)
	binding, ok := current.State.Bindings[key]
	old, oldOK := receipt.OldBindings[key]
	next, nextOK := receipt.NewBindings[key]
	if !ok || !oldOK || !nextOK || len(receipt.OldBindings) != 1 || len(receipt.NewBindings) != 1 ||
		!reflect.DeepEqual(binding, old) || !reflect.DeepEqual(binding, next) || binding.Role != "data" ||
		volume.IsReadOnly || binding.DiskKey != volume.VolumeName || binding.ActivePVC.Namespace != scope.Namespace {
		return empty, ErrConflict
	}
	resources := []ResourceRef{{ObjectRef: binding.ActivePVC, Purpose: "mutation-active-pvc"}}
	if binding.ActiveDV != nil {
		resources = append(resources, ResourceRef{ObjectRef: *binding.ActiveDV, Purpose: "mutation-active-dv"})
	}
	if !reflect.DeepEqual(resources, receipt.Resources) || !reflect.DeepEqual(resources, prepared.Resources) || !reflect.DeepEqual(resources, issued.Resources) {
		return empty, ErrConflict
	}
	return binding, nil
}

func (r *Resolver) reclaimTargets(ctx context.Context, scope Scope, binding Binding) (*corev1.PersistentVolumeClaim, error) {
	var dv *unstructured.Unstructured
	if ref := binding.ActiveDV; ref != nil {
		object, err := r.dynamic.Resource(dataVolumeResource).Namespace(scope.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, ErrReferenceUnavailable
		}
		if err == nil {
			if !sameObject(*ref, refOf("DataVolume", object)) || object.GetLabels()["service_id"] != scope.ServiceID {
				return nil, ErrConflict
			}
			if tenant := object.GetLabels()["tenant_id"]; tenant != "" && tenant != scope.TenantID {
				return nil, ErrConflict
			}
			claim, found, fieldErr := unstructured.NestedString(object.Object, "status", "claimName")
			if fieldErr != nil || !found || claim != binding.ActivePVC.Name {
				return nil, ErrConflict
			}
			dv = object
		}
	}
	pvc, err := r.core.CoreV1().PersistentVolumeClaims(scope.Namespace).Get(ctx, binding.ActivePVC.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, ErrReferenceUnavailable
	}
	if !sameObject(binding.ActivePVC, refOf("PersistentVolumeClaim", pvc)) {
		return nil, ErrConflict
	}
	if dv != nil {
		if !pvcOwnedBy(scope, pvc, dv) {
			return nil, ErrConflict
		}
	} else {
		// A recorded DV owner is evidence, not a fabricated live DV. Conflicting
		// tenant/service labels or any other DV owner remain invalid.
		if tenant := pvc.Labels["tenant_id"]; tenant != "" && tenant != scope.TenantID {
			return nil, ErrConflict
		}
		if service := pvc.Labels["service_id"]; service != "" && service != scope.ServiceID {
			return nil, ErrConflict
		}
		owned := pvc.Labels["service_id"] == scope.ServiceID
		for _, owner := range pvc.OwnerReferences {
			if owner.Kind != "DataVolume" {
				continue
			}
			if binding.ActiveDV == nil || owner.Name != binding.ActiveDV.Name || string(owner.UID) != binding.ActiveDV.UID {
				return nil, ErrConflict
			}
			owned = true
		}
		if !owned {
			return nil, ErrConflict
		}
	}
	return pvc, nil
}
