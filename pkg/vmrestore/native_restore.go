package vmrestore

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kv "kubevirt.io/api/core/v1"
	native "kubevirt.io/api/snapshot/v1beta1"
)

const (
	nativeRestoreRequestAnnotation  = "rainbond.io/vm-native-restore-request"
	nativeRestoreSnapshotAnnotation = "rainbond.io/vm-native-restore-snapshot-uid"
)

// NativeRestoreBackend exposes only the exact KubeVirt resources needed for an
// offline restore. A nil restore means the deterministic object does not exist.
type NativeRestoreBackend interface {
	VirtualMachine(context.Context, Scope) (*kv.VirtualMachine, *kv.VirtualMachineInstance, error)
	Snapshot(context.Context, Scope, string) (*native.VirtualMachineSnapshot, error)
	Restore(context.Context, Scope, string) (*native.VirtualMachineRestore, error)
	CreateRestore(context.Context, Scope, *native.VirtualMachineRestore) (*native.VirtualMachineRestore, error)
}

// NativeRestoreVolumeDTO is the safe volume result exposed to Console.
type NativeRestoreVolumeDTO struct {
	VolumeName         string `json:"volume_name"`
	PVCName            string `json:"pvc_name"`
	VolumeSnapshotName string `json:"volume_snapshot_name"`
}

// NativeRestoreDTO is a durable projection of a KubeVirt restore CR.
type NativeRestoreDTO struct {
	RequiresAdvance bool                     `json:"requires_advance,omitempty"`
	RestoreName     string                   `json:"restore_name"`
	SnapshotName    string                   `json:"snapshot_name"`
	RequestID       string                   `json:"request_id"`
	Phase           string                   `json:"phase"`
	Complete        bool                     `json:"complete"`
	ErrorCode       string                   `json:"error_code,omitempty"`
	Volumes         []NativeRestoreVolumeDTO `json:"volumes"`
}

// NativeRestoreEngine starts and observes one deterministic offline KubeVirt
// restore. It never starts the VM; a restored VM remains halted for inspection.
type NativeRestoreEngine struct {
	backend NativeRestoreBackend
	Managed *ManagedNativeRestore
}

func NewNativeRestoreEngine(backend NativeRestoreBackend) *NativeRestoreEngine {
	return &NativeRestoreEngine{backend: backend}
}

func nativeRestoreName(scope Scope, requestID string) (string, error) {
	if !validScope(scope) || !ValidNativeRestoreRequestID(requestID) {
		return "", ErrSnapshotInvalid
	}
	digest, err := snapshotDigest(struct {
		Scope     Scope  `json:"scope"`
		RequestID string `json:"request_id"`
	}{scope, requestID})
	if err != nil {
		return "", ErrCorrupt
	}
	return "vmr-" + digest[:24], nil
}

func ValidNativeRestoreRequestID(requestID string) bool {
	return idPattern.MatchString(requestID)
}

func expectedNativeRestore(scope Scope, vm *kv.VirtualMachine, snapshot *native.VirtualMachineSnapshot, requestID, name string) *native.VirtualMachineRestore {
	group := kv.GroupVersion.Group
	policy := native.VirtualMachineRestoreFailImmediate
	return &native.VirtualMachineRestore{
		TypeMeta: metav1.TypeMeta{APIVersion: native.SchemeGroupVersion.String(), Kind: "VirtualMachineRestore"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: scope.Namespace,
			Labels:    map[string]string{"tenant_id": scope.TenantID, "service_id": scope.ServiceID},
			Annotations: map[string]string{
				nativeRestoreRequestAnnotation:  requestID,
				nativeRestoreSnapshotAnnotation: string(snapshot.UID),
			},
		},
		Spec: native.VirtualMachineRestoreSpec{
			Target:                     corev1.TypedLocalObjectReference{APIGroup: &group, Kind: "VirtualMachine", Name: vm.Name},
			VirtualMachineSnapshotName: snapshot.Name,
			TargetReadinessPolicy:      &policy,
		},
	}
}

func validateNativeRestoreIdentity(scope Scope, vm *kv.VirtualMachine, snapshot *native.VirtualMachineSnapshot) error {
	if vm == nil || snapshot == nil || vm.Namespace != scope.Namespace || snapshot.Namespace != scope.Namespace || vm.Labels["service_id"] != scope.ServiceID || snapshot.Labels["service_id"] != scope.ServiceID || vm.Labels["tenant_id"] != scope.TenantID || snapshot.Labels["tenant_id"] != scope.TenantID || !validUID(string(vm.UID)) || !validUID(string(snapshot.UID)) || vm.Spec.Template == nil || vm.DeletionTimestamp != nil || snapshot.DeletionTimestamp != nil {
		return ErrConflict
	}
	group := kv.GroupVersion.Group
	if snapshot.Status == nil || snapshot.Status.SourceUID == nil || *snapshot.Status.SourceUID != vm.UID || snapshot.Spec.Source.APIGroup == nil || *snapshot.Spec.Source.APIGroup != group || snapshot.Spec.Source.Kind != "VirtualMachine" || snapshot.Spec.Source.Name != vm.Name {
		return ErrConflict
	}
	return nil
}

func validateNativeRestoreSource(scope Scope, vm *kv.VirtualMachine, vmi *kv.VirtualMachineInstance, snapshot *native.VirtualMachineSnapshot, restoreName string) error {
	if err := validateNativeRestoreIdentity(scope, vm, snapshot); err != nil {
		return err
	}
	if vmi != nil || vmWantsRunning(vm) || vm.Status.SnapshotInProgress != nil || vm.Status.RestoreInProgress != nil && *vm.Status.RestoreInProgress != restoreName {
		return ErrBusy
	}
	if snapshot.Status == nil || snapshot.Status.ReadyToUse == nil || !*snapshot.Status.ReadyToUse || snapshot.Status.Phase != native.Succeeded || snapshot.Status.SourceUID == nil || *snapshot.Status.SourceUID != vm.UID || snapshot.Status.Error != nil {
		return ErrSnapshotPending
	}
	return nil
}

func sameNativeRestore(actual, expected *native.VirtualMachineRestore) bool {
	if actual == nil || expected == nil || actual.Name != expected.Name || actual.Namespace != expected.Namespace || actual.DeletionTimestamp != nil || actual.Labels["tenant_id"] != expected.Labels["tenant_id"] || actual.Labels["service_id"] != expected.Labels["service_id"] || actual.Annotations[nativeRestoreRequestAnnotation] != expected.Annotations[nativeRestoreRequestAnnotation] || actual.Annotations[nativeRestoreSnapshotAnnotation] != expected.Annotations[nativeRestoreSnapshotAnnotation] || actual.Spec.VirtualMachineSnapshotName != expected.Spec.VirtualMachineSnapshotName || actual.Spec.Target.Kind != expected.Spec.Target.Kind || actual.Spec.Target.Name != expected.Spec.Target.Name || actual.Spec.Target.APIGroup == nil || expected.Spec.Target.APIGroup == nil || *actual.Spec.Target.APIGroup != *expected.Spec.Target.APIGroup || actual.Spec.TargetReadinessPolicy == nil || expected.Spec.TargetReadinessPolicy == nil || *actual.Spec.TargetReadinessPolicy != *expected.Spec.TargetReadinessPolicy || len(actual.Spec.Patches) != 0 {
		return false
	}
	return true
}

func projectNativeRestore(restore *native.VirtualMachineRestore) NativeRestoreDTO {
	result := NativeRestoreDTO{RestoreName: restore.Name, SnapshotName: restore.Spec.VirtualMachineSnapshotName, RequestID: restore.Annotations[nativeRestoreRequestAnnotation], Phase: "pending", Volumes: []NativeRestoreVolumeDTO{}}
	if restore.Status == nil {
		return result
	}
	for _, volume := range restore.Status.Restores {
		result.Volumes = append(result.Volumes, NativeRestoreVolumeDTO{VolumeName: volume.VolumeName, PVCName: volume.PersistentVolumeClaimName, VolumeSnapshotName: volume.VolumeSnapshotName})
	}
	for _, condition := range restore.Status.Conditions {
		if condition.Type == native.ConditionFailure && condition.Status == corev1.ConditionTrue {
			result.Phase, result.ErrorCode = "failed", "vm_restore_native_failed"
			return result
		}
	}
	if restore.Status.Complete != nil && *restore.Status.Complete {
		result.Phase, result.Complete = "succeeded", true
	}
	return result
}

func (e *NativeRestoreEngine) load(ctx context.Context, scope Scope, snapshotName, requestID string, command bool) (*native.VirtualMachineRestore, *native.VirtualMachineRestore, error) {
	if e == nil || e.backend == nil || !validName(snapshotName) {
		return nil, nil, ErrSnapshotInvalid
	}
	name, err := nativeRestoreName(scope, requestID)
	if err != nil {
		return nil, nil, err
	}
	vm, vmi, err := e.backend.VirtualMachine(ctx, scope)
	if err != nil {
		return nil, nil, ErrReferenceUnavailable
	}
	snapshot, err := e.backend.Snapshot(ctx, scope, snapshotName)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, ErrReferenceUnavailable
	}
	if command {
		err = validateNativeRestoreSource(scope, vm, vmi, snapshot, name)
	} else {
		err = validateNativeRestoreIdentity(scope, vm, snapshot)
	}
	if err != nil {
		return nil, nil, err
	}
	expected := expectedNativeRestore(scope, vm, snapshot, requestID, name)
	actual, err := e.backend.Restore(ctx, scope, name)
	if err != nil {
		return nil, nil, ErrReferenceUnavailable
	}
	if actual != nil && !sameNativeRestore(actual, expected) {
		return nil, nil, ErrConflict
	}
	return actual, expected, nil
}

func (e *NativeRestoreEngine) Start(ctx context.Context, scope Scope, snapshotName, requestID string) (NativeRestoreDTO, error) {
	if e != nil && e.Managed != nil {
		return e.Managed.Advance(ctx, scope, snapshotName, requestID)
	}
	actual, expected, err := e.load(ctx, scope, snapshotName, requestID, true)
	if err != nil {
		return NativeRestoreDTO{}, err
	}
	if actual == nil {
		actual, err = e.backend.CreateRestore(ctx, scope, expected)
		if err != nil {
			return NativeRestoreDTO{}, ErrRestoreUnknown
		}
		if !sameNativeRestore(actual, expected) || !validUID(string(actual.UID)) {
			return NativeRestoreDTO{}, ErrConflict
		}
	}
	return projectNativeRestore(actual), nil
}

func (e *NativeRestoreEngine) Status(ctx context.Context, scope Scope, snapshotName, requestID string) (NativeRestoreDTO, error) {
	if e != nil && e.Managed != nil {
		return e.Managed.Status(ctx, scope, snapshotName, requestID)
	}
	actual, _, err := e.load(ctx, scope, snapshotName, requestID, false)
	if err != nil {
		return NativeRestoreDTO{}, err
	}
	if actual == nil {
		return NativeRestoreDTO{}, ErrNotFound
	}
	return projectNativeRestore(actual), nil
}
