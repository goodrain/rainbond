package vmrestore

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/google/uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	kv "kubevirt.io/api/core/v1"
	native "kubevirt.io/api/snapshot/v1beta1"
)

// SnapshotFirmwareIdentity records actual source identity, never a guessed UUID.
type SnapshotFirmwareIdentity struct {
	Startup            *RestoreStartupPolicy `json:"startup,omitempty"`
	VM                 ObjectRef             `json:"vm"`
	VMI                *ObjectRef            `json:"vmi,omitempty"`
	VMIResourceVersion string                `json:"vmi_resource_version,omitempty"`
	VMResourceVersion  string                `json:"vm_resource_version"`
	FirmwareUUID       string                `json:"firmware_uuid"`
	Pinned             bool                  `json:"pinned"`
}

func (p SnapshotFirmwareIdentity) validate(scope Scope, source ObjectRef) error {
	if p.Startup != nil && (p.Startup.validate(scope) != nil || p.Startup.FirmwareUUID != p.FirmwareUUID) {
		return ErrCorrupt
	}
	if !sameObject(p.VM, source) || p.VM.Namespace != scope.Namespace || p.VMResourceVersion == "" {
		return ErrCorrupt
	}
	if _, err := uuid.Parse(p.FirmwareUUID); err != nil {
		return ErrCorrupt
	}
	if p.VMI != nil && (p.VMI.Kind != "VirtualMachineInstance" || p.VMI.Namespace != scope.Namespace || !validName(p.VMI.Name) || !validUID(p.VMI.UID) || p.VMIResourceVersion == "") {
		return ErrCorrupt
	}
	return nil
}

func validSnapshotIdentityTransition(old, next *SnapshotFirmwareIdentity) bool {
	if old == nil {
		return true
	}
	if next == nil {
		return false
	}
	a, b := *old, *next
	a.Pinned = false
	b.Pinned = false
	return reflect.DeepEqual(a, b) && (!old.Pinned || next.Pinned)
}

// SnapshotFirmwareMaterializer pins source VMI firmware under the snapshot fence.
// It issues at most one exact-resourceVersion update; uncertainty is observed.
type SnapshotFirmwareMaterializer struct {
	Store        *Store
	Dynamic      dynamic.Interface
	ProveStartup func(context.Context, Scope, *kv.VirtualMachine, string) (RestoreStartupPolicy, error)
}

func (m *SnapshotFirmwareMaterializer) current(scope Scope, op string) (Current, error) {
	if m == nil || m.Store == nil || m.Dynamic == nil {
		return Current{}, ErrReferenceUnavailable
	}
	c, err := m.Store.Read(scope, op)
	if err != nil {
		return c, err
	}
	if c.Receipt == nil || c.Receipt.Snapshot == nil || requireSnapshotOwner(c, op, c.Receipt.ManifestHash) != nil || snapshotCleanupSelected(c.Receipt) {
		return c, ErrBusy
	}
	return c, nil
}

func (m *SnapshotFirmwareMaterializer) source(ctx context.Context, scope Scope, ref ObjectRef) (*kv.VirtualMachine, error) {
	u, err := m.Dynamic.Resource(restoreVMGVR).Namespace(scope.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, ErrReferenceUnavailable
	}
	if !sameObject(ref, refOf("VirtualMachine", u)) || u.GetDeletionTimestamp() != nil || u.GetLabels()["service_id"] != scope.ServiceID || u.GetResourceVersion() == "" {
		return nil, ErrConflict
	}
	if tenant := u.GetLabels()["tenant_id"]; tenant != "" && tenant != scope.TenantID {
		return nil, ErrConflict
	}
	var vm kv.VirtualMachine
	if runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &vm) != nil || vm.Spec.Template == nil {
		return nil, ErrCorrupt
	}
	return &vm, nil
}

// Pin never generates a new guest identity or mutates the running VMI.
func (m *SnapshotFirmwareMaterializer) Pin(ctx context.Context, scope Scope, op string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	c, err := m.current(scope, op)
	if err != nil {
		return err
	}
	vm, err := m.source(ctx, scope, c.Receipt.Snapshot.SourceVM)
	if err != nil {
		return err
	}
	proof := c.Receipt.SnapshotIdentity
	engine := NewSnapshotEngine(m.Store, nil)
	if proof == nil {
		identity := ""
		if fw := vm.Spec.Template.Spec.Domain.Firmware; fw != nil {
			identity = string(fw.UUID)
		}
		var instanceRef *ObjectRef
		instanceVersion := ""
		instance, err := m.Dynamic.Resource(restoreVMIGVR).Namespace(scope.Namespace).Get(ctx, vm.Name, metav1.GetOptions{})
		if err == nil {
			var vmi kv.VirtualMachineInstance
			if runtime.DefaultUnstructuredConverter.FromUnstructured(instance.Object, &vmi) != nil {
				return ErrCorrupt
			}
			owned := false
			for _, owner := range vmi.OwnerReferences {
				owned = owned || (owner.Kind == "VirtualMachine" && owner.UID == vm.UID)
			}
			if !owned || !validUID(string(vmi.UID)) || vmi.DeletionTimestamp != nil || vmi.Status.Phase != kv.Running || vmi.Spec.Domain.Firmware == nil {
				return ErrConflict
			}
			actual := string(vmi.Spec.Domain.Firmware.UUID)
			if identity != "" && identity != actual {
				return ErrConflict
			}
			identity = actual
			ref := refOf("VirtualMachineInstance", &vmi)
			instanceRef = &ref
			instanceVersion = vmi.ResourceVersion
		} else if !apierrors.IsNotFound(err) {
			return ErrReferenceUnavailable
		}
		proof = &SnapshotFirmwareIdentity{VM: c.Receipt.Snapshot.SourceVM, VMI: instanceRef, VMIResourceVersion: instanceVersion, VMResourceVersion: vm.ResourceVersion, FirmwareUUID: identity}
		if m.ProveStartup != nil {
			policy, err := m.ProveStartup(ctx, scope, vm, identity)
			if err != nil {
				return err
			}
			proof.Startup = &policy
		}
		if proof.validate(scope, c.Receipt.Snapshot.SourceVM) != nil {
			return ErrRestoreCapability
		}
		hash, _ := snapshotDigest(proof)
		c, err = engine.mutate(scope, op, "snapshot-identity-intent", hash, func(c Current) (Mutation, error) {
			if c.Receipt.SnapshotIdentity != nil {
				return Mutation{}, ErrConflict
			}
			c.Receipt.SnapshotIdentity = proof
			return Mutation{State: c.State, Receipt: c.Receipt}, nil
		})
		if err != nil {
			return err
		}
		proof = c.Receipt.SnapshotIdentity
	}
	if proof.validate(scope, c.Receipt.Snapshot.SourceVM) != nil {
		return ErrCorrupt
	}
	if proof.VMI != nil {
		instance, err := m.Dynamic.Resource(restoreVMIGVR).Namespace(scope.Namespace).Get(ctx, proof.VMI.Name, metav1.GetOptions{})
		if err == nil {
			if !sameObject(*proof.VMI, refOf("VirtualMachineInstance", instance)) || instance.GetDeletionTimestamp() != nil || textAt(instance.Object, "spec", "domain", "firmware", "uuid") != proof.FirmwareUUID {
				return ErrConflict
			}
		} else if !apierrors.IsNotFound(err) {
			return ErrReferenceUnavailable
		} else if !proof.Pinned && vmWantsRunning(vm) {
			return ErrRestorePending
		}
	}
	if fw := vm.Spec.Template.Spec.Domain.Firmware; fw != nil && string(fw.UUID) == proof.FirmwareUUID {
		if proof.Pinned {
			return nil
		}
		hash, _ := snapshotDigest(proof.FirmwareUUID)
		_, err := engine.mutate(scope, op, "snapshot-identity-pinned", hash, func(c Current) (Mutation, error) {
			c.Receipt.SnapshotIdentity.Pinned = true
			return Mutation{State: c.State, Receipt: c.Receipt}, nil
		})
		return err
	}
	if proof.Pinned {
		return ErrConflict
	}
	if c.Receipt.Steps["snapshot-identity-issued"].Applied {
		return ErrRestoreUnknown
	}
	if vm.ResourceVersion != proof.VMResourceVersion {
		return ErrConflict
	}
	if proof.VMI != nil {
		instance, err := m.Dynamic.Resource(restoreVMIGVR).Namespace(scope.Namespace).Get(ctx, proof.VMI.Name, metav1.GetOptions{})
		if err != nil {
			return ErrReferenceUnavailable
		}
		if !sameObject(*proof.VMI, refOf("VirtualMachineInstance", instance)) || instance.GetDeletionTimestamp() != nil || textAt(instance.Object, "spec", "domain", "firmware", "uuid") != proof.FirmwareUUID {
			return ErrConflict
		}
		owned := false
		for _, owner := range instance.GetOwnerReferences() {
			owned = owned || (owner.Kind == "VirtualMachine" && owner.UID == types.UID(proof.VM.UID))
		}
		if !owned || textAt(instance.Object, "status", "phase") != string(kv.Running) {
			return ErrConflict
		}
	}
	hash, _ := snapshotDigest(proof)
	issued, err := engine.mutate(scope, op, "snapshot-identity-issued", hash, func(c Current) (Mutation, error) { return Mutation{State: c.State, Receipt: c.Receipt}, nil })
	if err != nil {
		return err
	}
	if issued.Replayed {
		return ErrRestoreUnknown
	}
	if _, err := m.current(scope, op); err != nil {
		return err
	}
	latest, err := m.source(ctx, scope, proof.VM)
	if err != nil {
		return err
	}
	if latest.ResourceVersion != proof.VMResourceVersion {
		return ErrConflict
	}
	wanted := latest.DeepCopy()
	if wanted.Spec.Template.Spec.Domain.Firmware == nil {
		wanted.Spec.Template.Spec.Domain.Firmware = &kv.Firmware{}
	}
	wanted.Spec.Template.Spec.Domain.Firmware.UUID = types.UID(proof.FirmwareUUID)
	if _, err := UpdateObservedRuntimeVM(ctx, m.Dynamic, latest, wanted, func() error {
		current, err := m.current(scope, op)
		if err != nil {
			return err
		}
		if current.State.ControlRevision != issued.State.ControlRevision || current.State.BindingGeneration != issued.State.BindingGeneration {
			return ErrConflict
		}
		return nil
	}); err != nil {
		if errors.Is(err, ErrConflict) || errors.Is(err, ErrCorrupt) || errors.Is(err, ErrRestoreCapability) || errors.Is(err, ErrReferenceUnavailable) {
			return err
		}
		return ErrRestoreUnknown
	}
	return ErrRestorePending
}

func checkSnapshotFirmware(identity *SnapshotFirmwareIdentity, content *native.VirtualMachineSnapshotContent) error {
	if identity == nil {
		return nil
	}
	if !identity.Pinned || content == nil || content.Spec.Source.VirtualMachine == nil {
		return ErrSnapshotPending
	}
	vm := content.Spec.Source.VirtualMachine
	if string(vm.UID) != identity.VM.UID || vm.Spec.Template == nil || vm.Spec.Template.Spec.Domain.Firmware == nil || string(vm.Spec.Template.Spec.Domain.Firmware.UUID) != identity.FirmwareUUID {
		return ErrConflict
	}
	if identity.Startup != nil {
		return checkStartupDevices(*identity.Startup, vm.Spec.Template.Spec)
	}
	return nil
}
