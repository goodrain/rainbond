package vmrestore

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	kv "kubevirt.io/api/core/v1"
	native "kubevirt.io/api/snapshot/v1beta1"
)

type nativeRestoreBackendFixture struct {
	vm      *kv.VirtualMachine
	vmi     *kv.VirtualMachineInstance
	snap    *native.VirtualMachineSnapshot
	restore *native.VirtualMachineRestore
	creates int
}

func newNativeRestoreBackendFixture() *nativeRestoreBackendFixture {
	ready := true
	sourceUID := types.UID("vm-uid")
	runStrategy := kv.RunStrategyHalted
	group := kv.GroupVersion.Group
	return &nativeRestoreBackendFixture{
		vm:   &kv.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "vm", Namespace: "tenant", UID: sourceUID, Labels: map[string]string{"service_id": "service", "tenant_id": "tenant-id"}}, Spec: kv.VirtualMachineSpec{RunStrategy: &runStrategy, Template: &kv.VirtualMachineInstanceTemplateSpec{}}},
		snap: &native.VirtualMachineSnapshot{ObjectMeta: metav1.ObjectMeta{Name: "snapshot", Namespace: "tenant", UID: "snapshot-uid", Labels: map[string]string{"service_id": "service", "tenant_id": "tenant-id"}}, Spec: native.VirtualMachineSnapshotSpec{Source: corev1.TypedLocalObjectReference{APIGroup: &group, Kind: "VirtualMachine", Name: "vm"}}, Status: &native.VirtualMachineSnapshotStatus{SourceUID: &sourceUID, Phase: native.Succeeded, ReadyToUse: &ready}},
	}
}

func (f *nativeRestoreBackendFixture) VirtualMachine(context.Context, Scope) (*kv.VirtualMachine, *kv.VirtualMachineInstance, error) {
	var vmi *kv.VirtualMachineInstance
	if f.vmi != nil {
		vmi = f.vmi.DeepCopy()
	}
	return f.vm.DeepCopy(), vmi, nil
}
func (f *nativeRestoreBackendFixture) Snapshot(context.Context, Scope, string) (*native.VirtualMachineSnapshot, error) {
	return f.snap.DeepCopy(), nil
}
func (f *nativeRestoreBackendFixture) Restore(context.Context, Scope, string) (*native.VirtualMachineRestore, error) {
	if f.restore == nil {
		return nil, nil
	}
	return f.restore.DeepCopy(), nil
}
func (f *nativeRestoreBackendFixture) CreateRestore(_ context.Context, _ Scope, restore *native.VirtualMachineRestore) (*native.VirtualMachineRestore, error) {
	f.creates++
	f.restore = restore.DeepCopy()
	f.restore.UID = "restore-uid"
	return f.restore.DeepCopy(), nil
}

// capability_id: rainbond.vm-restore.native-longhorn
func TestNativeRestoreRequiresStoppedOwnedReadySnapshot(t *testing.T) {
	for _, scenario := range []string{"running", "unready", "foreign-snapshot", "ready"} {
		t.Run(scenario, func(t *testing.T) {
			backend := newNativeRestoreBackendFixture()
			switch scenario {
			case "running":
				backend.vmi = &kv.VirtualMachineInstance{ObjectMeta: metav1.ObjectMeta{Name: "vm", Namespace: "tenant", UID: "vmi-uid"}, Status: kv.VirtualMachineInstanceStatus{Phase: kv.Running}}
			case "unready":
				ready := false
				backend.snap.Status.ReadyToUse = &ready
			case "foreign-snapshot":
				backend.snap.Labels["service_id"] = "other"
			}
			engine := NewNativeRestoreEngine(backend)
			result, err := engine.Start(context.Background(), Scope{TenantID: "tenant-id", ServiceID: "service", Namespace: "tenant"}, "snapshot", "request-a")
			if scenario == "ready" {
				if err != nil || result.Phase != "pending" || result.RestoreName == "" || backend.creates != 1 {
					t.Fatalf("ready restore did not start: %#v %v", result, err)
				}
			} else if err == nil || backend.creates != 0 {
				t.Fatalf("invalid restore started: %s %#v", scenario, result)
			}
		})
	}
}

// capability_id: rainbond.vm-restore.native-longhorn
func TestNativeRestoreIsIdempotentAndProjectsCompletion(t *testing.T) {
	backend := newNativeRestoreBackendFixture()
	engine := NewNativeRestoreEngine(backend)
	scope := Scope{TenantID: "tenant-id", ServiceID: "service", Namespace: "tenant"}
	first, err := engine.Start(context.Background(), scope, "snapshot", "request-a")
	if err != nil {
		t.Fatal(err)
	}
	second, err := engine.Start(context.Background(), scope, "snapshot", "request-a")
	if err != nil || second.RestoreName != first.RestoreName || backend.creates != 1 {
		t.Fatal("idempotent start created another restore", err)
	}
	complete := true
	backend.restore.Status = &native.VirtualMachineRestoreStatus{Complete: &complete, Restores: []native.VolumeRestore{{VolumeName: "root", PersistentVolumeClaimName: "restored-root", VolumeSnapshotName: "root-snapshot"}}}
	status, err := engine.Status(context.Background(), scope, "snapshot", "request-a")
	if err != nil || status.Phase != "succeeded" || !status.Complete || len(status.Volumes) != 1 || status.Volumes[0].PVCName != "restored-root" {
		t.Fatalf("completed restore was not projected: %#v %v", status, err)
	}
}

// capability_id: rainbond.vm-restore.native-longhorn
func TestNativeRestoreRejectsConflictingExistingObject(t *testing.T) {
	backend := newNativeRestoreBackendFixture()
	engine := NewNativeRestoreEngine(backend)
	scope := Scope{TenantID: "tenant-id", ServiceID: "service", Namespace: "tenant"}
	started, err := engine.Start(context.Background(), scope, "snapshot", "request-a")
	if err != nil {
		t.Fatal(err)
	}
	backend.restore.Spec.VirtualMachineSnapshotName = "other-snapshot"
	if _, err := engine.Status(context.Background(), scope, "snapshot", "request-a"); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflicting restore object accepted for %s: %v", started.RestoreName, err)
	}
}
