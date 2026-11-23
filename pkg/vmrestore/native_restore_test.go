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
	errors  map[string]error
}

// capability_id: rainbond.vm-restore.native-status-read
func TestNativeRestoreStatusSurvivesPowerAndReadinessChanges(t *testing.T) {
	for _, scenario := range []string{"VMI running", "running intent", "snapshot unready", "other operation in progress"} {
		t.Run(scenario, func(t *testing.T) {
			backend := newNativeRestoreBackendFixture()
			engine := NewNativeRestoreEngine(backend)
			scope := Scope{TenantID: "tenant-id", ServiceID: "service", Namespace: "tenant"}
			if _, err := engine.Start(context.Background(), scope, "snapshot", "request-a"); err != nil {
				t.Fatal(err)
			}
			complete := true
			backend.restore.Status = &native.VirtualMachineRestoreStatus{Complete: &complete}
			switch scenario {
			case "VMI running":
				backend.vmi = &kv.VirtualMachineInstance{ObjectMeta: metav1.ObjectMeta{Name: "vm", Namespace: "tenant", UID: "vmi-uid"}}
			case "running intent":
				running := kv.RunStrategyAlways
				backend.vm.Spec.RunStrategy = &running
			case "snapshot unready":
				ready := false
				backend.snap.Status.ReadyToUse = &ready
			case "other operation in progress":
				other := "other-operation"
				backend.vm.Status.SnapshotInProgress = &other
			}
			got, err := engine.Status(context.Background(), scope, "snapshot", "request-a")
			if err != nil || !got.Complete || got.Phase != "succeeded" || backend.creates != 1 {
				t.Fatalf("durable status gated on current command preconditions: %+v %v", got, err)
			}
			if _, err := engine.Start(context.Background(), scope, "snapshot", "request-b"); err == nil || backend.creates != 1 {
				t.Fatal("status fix relaxed mutation preconditions", err)
			}
		})
	}
	for _, scenario := range []string{"VM tenant", "snapshot service", "source UID", "snapshot UID", "target name", "restore request", "restore patches"} {
		t.Run("identity/"+scenario, func(t *testing.T) {
			backend := newNativeRestoreBackendFixture()
			engine := NewNativeRestoreEngine(backend)
			scope := Scope{TenantID: "tenant-id", ServiceID: "service", Namespace: "tenant"}
			if _, err := engine.Start(context.Background(), scope, "snapshot", "request-a"); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "VM tenant":
				backend.vm.Labels["tenant_id"] = "foreign"
			case "snapshot service":
				backend.snap.Labels["service_id"] = "foreign"
			case "source UID":
				uid := types.UID("replacement-vm")
				backend.snap.Status.SourceUID = &uid
			case "snapshot UID":
				backend.snap.UID = "replacement-snapshot"
			case "target name":
				backend.restore.Spec.Target.Name = "other-vm"
			case "restore request":
				backend.restore.Annotations[nativeRestoreRequestAnnotation] = "request-b"
			case "restore patches":
				backend.restore.Spec.Patches = []string{`{"op":"remove","path":"/spec"}`}
			}
			if _, err := engine.Status(context.Background(), scope, "snapshot", "request-a"); !errors.Is(err, ErrConflict) || backend.creates != 1 {
				t.Fatal("status accepted conflicting identity or mutated resources", err)
			}
		})
	}
	for _, tt := range []struct {
		name, dependency string
		failure, want    error
	}{
		{"VM unreadable", "vm", errors.New("denied"), ErrReferenceUnavailable},
		{"snapshot unreadable", "snapshot", errors.New("denied"), ErrReferenceUnavailable},
		{"snapshot missing", "snapshot", ErrNotFound, ErrNotFound},
		{"restore unreadable", "restore", errors.New("denied"), ErrReferenceUnavailable},
		{"restore absent", "", nil, ErrNotFound},
	} {
		t.Run(tt.name, func(t *testing.T) {
			backend := newNativeRestoreBackendFixture()
			backend.errors = map[string]error{tt.dependency: tt.failure}
			engine := NewNativeRestoreEngine(backend)
			if _, err := engine.Status(context.Background(), Scope{TenantID: "tenant-id", ServiceID: "service", Namespace: "tenant"}, "snapshot", "request-a"); !errors.Is(err, tt.want) || backend.creates != 0 {
				t.Fatalf("status dependency failure: %v; want %v", err, tt.want)
			}
		})
	}
	t.Run("invalid request never creates", func(t *testing.T) {
		backend := newNativeRestoreBackendFixture()
		engine := NewNativeRestoreEngine(backend)
		scope := Scope{TenantID: "tenant-id", ServiceID: "service", Namespace: "tenant"}
		for _, request := range []struct{ snapshot, id string }{{"../bad", "request-a"}, {"snapshot", "bad/id"}} {
			if _, err := engine.Status(context.Background(), scope, request.snapshot, request.id); !errors.Is(err, ErrSnapshotInvalid) || backend.creates != 0 {
				t.Fatal("invalid status scope reached a mutation", err)
			}
		}
	})
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
	if err := f.errors["vm"]; err != nil {
		return nil, nil, err
	}
	var vmi *kv.VirtualMachineInstance
	if f.vmi != nil {
		vmi = f.vmi.DeepCopy()
	}
	return f.vm.DeepCopy(), vmi, nil
}
func (f *nativeRestoreBackendFixture) Snapshot(context.Context, Scope, string) (*native.VirtualMachineSnapshot, error) {
	if err := f.errors["snapshot"]; err != nil {
		return nil, err
	}
	return f.snap.DeepCopy(), nil
}
func (f *nativeRestoreBackendFixture) Restore(context.Context, Scope, string) (*native.VirtualMachineRestore, error) {
	if err := f.errors["restore"]; err != nil {
		return nil, err
	}
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
