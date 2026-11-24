package vmrestore

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// capability_id: rainbond.vm-restore.native-platform-convergence
func TestNativeRestoreControlOwnsFenceAndCommitsBindingsAtomically(t *testing.T) {
	f := newRestoreCommitFixture(t)
	f.after = f.intent.Before
	f.intent.ConfigAfterHash, _ = RestoreConfigHash(f.after)
	f.targets[9] = ObjectRef{Kind: "PersistentVolumeClaim", Namespace: f.scope.Namespace, Name: "restored-data-9", UID: "restored-data-9-uid"}
	f.seedBeforeAttributes(t)
	startup := testStartupPolicy("platform")
	source := testStartupPolicy("restored")
	control := NativeRestoreControl{Config: f.after, BeforeStartup: startup, SourceStartup: source,
		SnapshotOperationID: "snapshot-source", NativeProofHash: strings.Repeat("e", 64), RestoreName: "vmr-012345678901234567890123", RequestID: "request-a", Nonce: "attempt-a"}
	prepared, err := f.store.PrepareNativeRestoreDB(f.scope, f.expect, f.intent, control, f.old)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.State.Fence == nil || prepared.State.Fence.Phase != "native-prepared" || prepared.Receipt.NativeRestore == nil {
		t.Fatal("missing durable native owner")
	}
	if _, _, err := f.store.ClaimOrdinaryVM(f.scope, "start", f.intent.CurrentVM); !errors.Is(err, ErrBusy) {
		t.Fatal("ordinary power bypassed native fence", err)
	}
	before := f.databaseImage(t)
	bindings, err := BuildRestoreBindings(f.scope, f.expect.OperationID, f.intent.SourceSnapshot.UID, f.old, f.after, f.targets)
	if err != nil {
		t.Fatal(err)
	}
	commit := f.expectStep(t, NativeRestoreCommitStep)
	if _, err := f.store.CommitNativeRestoreDB(f.scope, commit, RestoreCommitRequest{Config: f.after, Bindings: bindings}); !errors.Is(err, ErrConflict) || f.databaseImage(t) != before {
		t.Fatal("unobserved native completion committed", err)
	}
	nativeRef := ObjectRef{Kind: "VirtualMachineRestore", Namespace: f.scope.Namespace, Name: control.RestoreName, UID: "native-uid"}
	_, err = f.store.SealNativeRestoreObservation(f.scope, f.expectStep(t, "native-observed"), nativeRef, bindings)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := f.store.CommitNativeRestoreDB(f.scope, f.expectStep(t, NativeRestoreCommitStep), RestoreCommitRequest{Config: f.after, Bindings: bindings})
	if err != nil {
		t.Fatal(err)
	}
	if committed.State.BindingGeneration != 1 || !reflect.DeepEqual(committed.State.Bindings, bindings) || committed.Receipt.ConfigCommitHash == "" || committed.State.Fence == nil {
		t.Fatal("native commit did not atomically adopt all disks")
	}
	if _, err := f.store.CommitNativeRestoreDB(f.scope, f.expectStep(t, NativeRestoreCommitStep), RestoreCommitRequest{Config: f.after, Bindings: bindings}); err != nil {
		t.Fatal("identical commit replay failed", err)
	}
	if committed.Receipt.Execution != nil || committed.Receipt.Steps["restore-copy-verified"].Applied {
		t.Fatal("native path fabricated copy qualification/evidence")
	}
	final, err := f.store.FinalizeNativeRestoreDB(f.scope, f.expectStep(t, "native-finalize"))
	if err != nil || final.State.Fence != nil {
		t.Fatal("finalization failed", err)
	}
	// New receipt survives strict persistence; no legacy copy receipt is modified.
	loaded, err := f.store.ReadOnly(f.scope, f.expect.OperationID)
	if err != nil || loaded.Receipt.NativeRestore == nil || !loaded.Receipt.Steps["native-finalize"].Applied {
		t.Fatal("native durable result unreadable", err)
	}
}

// capability_id: rainbond.vm-restore.native-platform-convergence
func TestNativeRestoreControlRejectsChangedIntentAndConfiguration(t *testing.T) {
	f := newRestoreCommitFixture(t)
	f.after = f.intent.Before
	f.intent.ConfigAfterHash, _ = RestoreConfigHash(f.after)
	f.seedBeforeAttributes(t)
	c := NativeRestoreControl{Config: f.after, BeforeStartup: testStartupPolicy("platform"), SourceStartup: testStartupPolicy("restored"), SnapshotOperationID: "snapshot-source", NativeProofHash: strings.Repeat("a", 64), RestoreName: "vmr-012345678901234567890123", RequestID: "request-a", Nonce: "attempt-a"}
	if _, err := f.store.PrepareNativeRestoreDB(f.scope, f.expect, f.intent, c, f.old); err != nil {
		t.Fatal(err)
	}
	bad := c
	bad.Config.Hardware.CPUMilli = 6000
	if _, err := f.store.PrepareNativeRestoreDB(f.scope, f.expect, f.intent, bad, f.old); err == nil {
		t.Fatal("request replay changed sealed config")
	}
	current, _ := f.store.ReadOnly(f.scope, f.expect.OperationID)
	_, err := f.store.Mutate(f.scope, Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration, OperationID: f.expect.OperationID, ManifestHash: f.expect.ManifestHash, RequestHash: strings.Repeat("b", 64), Step: "tamper-native"}, func(now Current) (Mutation, error) {
		now.Receipt.NativeRestore.NativeProofHash = strings.Repeat("b", 64)
		return Mutation{State: now.State, Receipt: now.Receipt}, nil
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatal("immutable native source proof could be rewritten", err)
	}
}
