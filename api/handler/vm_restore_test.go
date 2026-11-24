package handler

import (
	"context"
	"errors"
	"strings"
	"testing"

	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/pkg/vmrestore"
)

func restoreFacadeFixture(t *testing.T) (*ServiceAction, *vmrestore.RestoreEngine, vmrestore.Scope) {
	t.Helper()
	a, _, database, scope := snapshotFacadeFixture(t)
	store := vmrestore.NewStore(database)
	config := vmrestore.RestoreConfig{Hardware: vmrestore.SnapshotConfig{CPUMilli: 1000, MemoryMiB: 1024, BootMode: "bios", DiskLayout: []vmrestore.SnapshotLayout{{VolumeID: 7, DiskKey: "disk", CanonicalName: "manual7", Role: "root", Device: "disk", Bus: "virtio", BootOrder: 1}}}, Volumes: []vmrestore.RestoreVolumeConfig{{VolumeID: 7, DiskKey: "disk", CapacityGiB: 1, StorageClass: "cephfs-vm-snapshot", AccessMode: "ReadWriteMany"}}}
	hash, err := vmrestore.RestoreConfigHash(config)
	if err != nil {
		t.Fatal(err)
	}
	manifest := strings.Repeat("a", 64)
	_, err = store.Mutate(scope, vmrestore.Expectation{OperationID: "restore-op", ManifestHash: manifest, RequestHash: manifest, Step: "restore-prepare"}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		r := vmrestore.NewReceipt(scope, "restore-op", manifest)
		r.Restore = &vmrestore.RestoreIntent{CurrentVM: vmrestore.ObjectRef{Kind: "VirtualMachine", Namespace: scope.Namespace, Name: "vm", UID: "vm-uid"}, SourceSnapshot: vmrestore.ObjectRef{Kind: "VirtualMachineSnapshot", Namespace: scope.Namespace, Name: "snapshot", UID: "snapshot-uid"}, SourceContent: vmrestore.ObjectRef{Kind: "VirtualMachineSnapshotContent", Namespace: scope.Namespace, Name: "content", UID: "content-uid"}, SourceVMUID: "source-vm", SourceCoreHash: manifest, SourceDocumentHash: manifest, Before: config, ConfigAfterHash: hash, PowerAfter: "stopped"}
		c.State.Fence = &vmrestore.Fence{Kind: "restore", Phase: "prepared", OperationID: "restore-op", ManifestHash: manifest, TargetVMUID: "vm-uid"}
		return vmrestore.Mutation{State: c.State, Receipt: &r}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	e := &vmrestore.RestoreEngine{Store: store}
	a.restoreEngineFactory = func(context.Context, string) (*vmrestore.RestoreEngine, vmrestore.Scope, error) { return e, scope, nil }
	return a, e, scope
}

// capability_id: rainbond.vm-restore.rpc-handler
func TestRestoreFacadeStatusIsScopedReadOnlyAndIndependentOfRuntime(t *testing.T) {
	a, e, scope := restoreFacadeFixture(t)
	before, _ := e.Store.Read(scope, "restore-op")
	for i := 0; i < 2; i++ {
		d, err := a.GetVMRestore(context.Background(), scope.ServiceID, "restore-op")
		if err != nil || d.Complete || d.Phase != "prepared" || d.ConfigCommitted {
			t.Fatal(d, err)
		}
	}
	after, _ := e.Store.Read(scope, "restore-op")
	if after.State.ControlRevision != before.State.ControlRevision {
		t.Fatal("GET progressed restore")
	}
	ctx := context.WithValue(context.Background(), ctxutil.ContextKey("tenant_id"), "foreign")
	if _, err := a.GetVMRestore(ctx, scope.ServiceID, "restore-op"); !errors.Is(err, vmrestore.ErrInternalAttribute) {
		t.Fatal(err)
	}
	if _, err := a.GetVMRestore(context.Background(), "another-service", "restore-op"); !errors.Is(err, vmrestore.ErrInternalAttribute) {
		t.Fatal(err)
	}
}

// capability_id: rainbond.vm-restore.rpc-handler
func TestRestoreFacadeCommandUsesCurrentRevisionAndSealedReceipt(t *testing.T) {
	a, e, scope := restoreFacadeFixture(t)
	if _, err := a.VMRestoreCommand(context.Background(), scope.ServiceID, "restore-op", "cancel", vmrestore.RestoreCommand{}); !errors.Is(err, vmrestore.ErrConflict) {
		t.Fatal("stale command accepted", err)
	}
	c, _ := e.Store.Read(scope, "restore-op")
	status, _ := a.GetVMRestore(context.Background(), scope.ServiceID, "restore-op")
	d, err := a.VMRestoreCommand(context.Background(), scope.ServiceID, "restore-op", "cancel", vmrestore.RestoreCommand{ExpectedOperationEpoch: status.OperationEpoch, ExpectedControlRevision: c.State.ControlRevision, ExpectedBindingGeneration: c.State.BindingGeneration})
	if err != nil || !d.Applied || d.Complete {
		t.Fatal(d, err)
	}
	c, _ = e.Store.Read(scope, "restore-op")
	if !c.Receipt.Steps["restore-cancel-requested"].Applied || c.State.Fence == nil {
		t.Fatal("cancel command did not keep ownership")
	}
}

// capability_id: rainbond.vm-restore.rpc-handler
func TestDefaultRestoreReadSurvivesUnavailableKubernetesWithoutProgress(t *testing.T) {
	a, e, scope := restoreFacadeFixture(t)
	a.restoreEngineFactory = nil
	control, err := a.GetVMRestoreControl(context.Background(), scope.ServiceID)
	if err != nil || !control.Fenced || control.ActiveOperationID != "restore-op" {
		t.Fatal(control, err)
	}
	before, _ := e.Store.Read(scope, "restore-op")
	d, err := a.GetVMRestore(context.Background(), scope.ServiceID, "restore-op")
	if err != nil || d.Phase != "prepared" || d.Complete {
		t.Fatal(d, err)
	}
	_, err = a.VMRestoreCommand(context.Background(), scope.ServiceID, "restore-op", "halt", vmrestore.RestoreCommand{ExpectedOperationEpoch: d.OperationEpoch, ExpectedControlRevision: before.State.ControlRevision})
	if !errors.Is(err, vmrestore.ErrReferenceUnavailable) {
		t.Fatal("missing runtime was not an explicit dependency error", err)
	}
	after, _ := e.Store.Read(scope, "restore-op")
	if after.State.ControlRevision != before.State.ControlRevision {
		t.Fatal("unavailable runtime progressed operation")
	}
}
