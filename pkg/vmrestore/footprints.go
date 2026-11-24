package vmrestore

import (
	"context"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	// BindingGenerationAnnotation stamps the Region generation on a managed VM or restore target.
	BindingGenerationAnnotation = "rainbond.io/vm-binding-generation"
	// RestoreOperationLabel records resource origin, never a replacement for the Region ledger.
	RestoreOperationLabel = "rainbond.io/vm-restore-operation"
	// RestoreVolumeIDAnnotation identifies a restore target's logical Region disk.
	RestoreVolumeIDAnnotation = "rainbond.io/vm-volume-id"
	// RestoreCommitStep seals NewBindings and ConfigCommitHash in the binding-generation transaction.
	RestoreCommitStep = "restore-commit"
)

// MarkRestorePVC is used by the actual target builder before create; no UID is invented.
func MarkRestorePVC(pvc *corev1.PersistentVolumeClaim, scope Scope, volumeID uint, generation uint64, operationID string) error {
	if pvc == nil || pvc.UID != "" || pvc.Namespace != scope.Namespace || !validScope(scope) || volumeID == 0 || generation == 0 || !idPattern.MatchString(operationID) {
		return ErrCorrupt
	}
	if pvc.Labels == nil {
		pvc.Labels = map[string]string{}
	}
	if pvc.Annotations == nil {
		pvc.Annotations = map[string]string{}
	}
	pvc.Labels["service_id"] = scope.ServiceID
	pvc.Labels["tenant_id"] = scope.TenantID
	pvc.Labels[RestoreOperationLabel] = operationID
	pvc.Annotations[BindingGenerationAnnotation] = strconv.FormatUint(generation, 10)
	pvc.Annotations[RestoreVolumeIDAnnotation] = strconv.FormatUint(uint64(volumeID), 10)
	return nil
}

// MarkVMGeneration is used after a Region binding commit by the desired-VM builder.
func MarkVMGeneration(object metav1.Object, scope Scope, generation uint64) error {
	if object == nil || object.GetNamespace() != scope.Namespace || !validScope(scope) {
		return ErrCorrupt
	}
	annotations := object.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[BindingGenerationAnnotation] = strconv.FormatUint(generation, 10)
	object.SetAnnotations(annotations)
	return nil
}
func markerGeneration(object metav1.Object) (uint64, error) {
	raw := object.GetAnnotations()[BindingGenerationAnnotation]
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || strconv.FormatUint(value, 10) != raw {
		return 0, ErrCorrupt
	}
	return value, nil
}
func knownRestoreOperation(state State, receipts []Receipt, operationID string) bool {
	if operationID == "" {
		return false
	}
	for _, binding := range state.Bindings {
		if binding.RestoreID == operationID {
			return true
		}
	}
	for _, receipt := range receipts {
		if receipt.OperationID == operationID {
			return true
		}
	}
	return false
}
func inFlightRestore(state State, receipts []Receipt, operationID string) bool {
	if state.Fence == nil || state.Fence.Kind != "restore" || state.Fence.OperationID != operationID {
		return false
	}
	for _, receipt := range receipts {
		if receipt.OperationID == operationID && receipt.ManifestHash == state.Fence.ManifestHash {
			return true
		}
	}
	return false
}

// committedBindingFootprint is only a loss detector, never a binding source.
// Prepare may materialize OldBindings and plan NewBindings without a commit.
// A successful restore-commit must seal the same request digest in ConfigCommitHash
// and advance the binding generation in that transaction. The producer and this
// reader share RestoreCommitStep; a nonempty plan alone proves no transition.
func committedBindingFootprint(receipt Receipt, volumeID uint) (bool, error) {
	commitStep := RestoreCommitStep
	if receipt.NativeRestore != nil {
		commitStep = NativeRestoreCommitStep
	}
	step, committed := receipt.Steps[commitStep]
	if !committed {
		if receipt.ConfigCommitHash != "" {
			return false, ErrCorrupt
		}
		return false, nil
	}
	if !step.Applied || step.BindingGeneration == 0 || !hashPattern.MatchString(receipt.ConfigCommitHash) || step.RequestHash != receipt.ConfigCommitHash {
		return false, ErrCorrupt
	}
	_, included := receipt.NewBindings[strconv.FormatUint(uint64(volumeID), 10)]
	return included, nil
}

// A post-commit VM footprint pointing at a receipt's selected disk also forbids
// fallback. This corroborates lost identity even for a generic Store actor;
// neither that receipt nor the VM is adopted as the current mapping.
func vmSelectsReceiptBinding(vm unstructured.Unstructured, volumeID uint, receipts []Receipt) bool {
	operationID := vm.GetLabels()[RestoreOperationLabel]
	for _, receipt := range receipts {
		if operationID == "" || receipt.OperationID != operationID {
			continue
		}
		binding, exists := receipt.NewBindings[strconv.FormatUint(uint64(volumeID), 10)]
		if !exists {
			continue
		}
		selected := ResolvedVolume{ActivePVC: &binding.ActivePVC, ActiveDV: binding.ActiveDV}
		for _, value := range vmVolumes(vm.Object, false) {
			volume := asMap(value)
			if volume["name"] == binding.CanonicalName && volumeUses(volume, selected) {
				return true
			}
		}
	}
	return false
}

// fallbackFootprints only detects ledger loss. It never derives a PVC binding
// from annotations and never adopts an otherwise unknown restored resource.
func (r *Resolver) fallbackFootprints(ctx context.Context, scope Scope, state State, volumeID uint, receipts []Receipt) error {
	for _, receipt := range receipts {
		committed, err := committedBindingFootprint(receipt, volumeID)
		if err != nil {
			return err
		}
		if committed {
			return ErrCorrupt
		}
	}
	vms, err := r.objects(ctx, virtualMachineResource, scope.Namespace, false)
	if err != nil {
		return err
	}
	for _, vm := range vms {
		if vm.GetLabels()["service_id"] != scope.ServiceID {
			continue
		}
		generation, err := markerGeneration(&vm)
		if err != nil {
			return err
		}
		if generation > state.BindingGeneration {
			return ErrCorrupt
		}
		if generation > 0 && vmSelectsReceiptBinding(vm, volumeID, receipts) {
			return ErrCorrupt
		}
		operationID := vm.GetLabels()[RestoreOperationLabel]
		if operationID != "" && !knownRestoreOperation(state, receipts, operationID) && !inFlightRestore(state, receipts, operationID) {
			return ErrCorrupt
		}
	}
	claims, err := r.core.CoreV1().PersistentVolumeClaims(scope.Namespace).List(ctx, metav1.ListOptions{LabelSelector: "service_id=" + scope.ServiceID})
	if err != nil {
		return ErrReferenceUnavailable
	}
	for _, claim := range claims.Items {
		generation, err := markerGeneration(&claim)
		if err != nil {
			return err
		}
		operationID := claim.Labels[RestoreOperationLabel]
		if generation == 0 && operationID == "" {
			continue
		}
		if claim.Namespace != scope.Namespace || !validUID(string(claim.UID)) {
			return ErrConflict
		}
		if inFlightRestore(state, receipts, operationID) {
			if generation > 0 && generation != state.BindingGeneration && (state.BindingGeneration == ^uint64(0) || generation != state.BindingGeneration+1) {
				return ErrCorrupt
			}
			continue
		}
		if state.BindingGeneration == 0 || !knownRestoreOperation(state, receipts, operationID) {
			return ErrCorrupt
		}
		raw := claim.Annotations[RestoreVolumeIDAnnotation]
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || strconv.FormatUint(id, 10) != raw || id == 0 {
			return ErrCorrupt
		}
		if uint64(volumeID) == id {
			return ErrCorrupt
		}
	}
	return nil
}
