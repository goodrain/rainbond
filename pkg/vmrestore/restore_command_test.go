package vmrestore

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
)

// capability_id: rainbond.vm-restore.rpc-command-contract
func TestRestoreControlIsReadOnlyBeforePrepareAndEpochStaysImmutable(t *testing.T) {
	f := newRestoreCommitFixture(t)
	e := &RestoreEngine{Store: f.store}
	d, err := e.Control(f.scope)
	if err != nil || d.Fenced || d.AdmissionInProgress || d.ControlRevision != 0 {
		t.Fatal(d, err)
	}
	var count int
	if err := f.db.Model(&dbmodel.ComponentK8sAttributes{}).Where("component_id = ?", f.scope.ServiceID).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("control read allocated state", count, err)
	}
	f, e, r := engineFixture(t)
	before, err := e.Status(f.scope, f.expect.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.OperationEpoch) != 64 || before.Source.SnapshotUID != f.intent.SourceSnapshot.UID || before.TargetVMUID != f.intent.CurrentVM.UID || !before.FenceOwned || before.FenceCleared || before.Complete {
		t.Fatal("incomplete source/epoch/fence projection")
	}
	r.observation.Halted = true
	r.observation.WritersClear = true
	if _, err := e.RuntimeStep(context.Background(), f.scope, f.expect.OperationID, "halt"); err != nil {
		t.Fatal(err)
	}
	after, _ := e.Status(f.scope, f.expect.OperationID)
	if after.OperationEpoch != before.OperationEpoch || after.ControlRevision <= before.ControlRevision {
		t.Fatal("control progress changed immutable operation epoch")
	}
	control, _ := e.Control(f.scope)
	if !control.Fenced || control.ActiveOperationID != f.expect.OperationID {
		t.Fatal("active fence missing from control")
	}
}

// capability_id: rainbond.vm-restore.rpc-command-contract
func TestRestoreCommandsRejectMissingOrChangedEpochBeforeSideEffect(t *testing.T) {
	f, e, r := engineFixture(t)
	d, _ := e.Status(f.scope, f.expect.OperationID)
	for _, epoch := range []string{"", strings.Repeat("f", 64)} {
		_, err := e.Command(context.Background(), f.scope, f.expect.OperationID, "halt", RestoreCommand{ExpectedOperationEpoch: epoch, ExpectedControlRevision: d.ControlRevision, ExpectedBindingGeneration: d.BindingGeneration})
		if !errors.Is(err, ErrConflict) || r.applies != 0 {
			t.Fatal("missing/foreign epoch authorized mutation", err)
		}
	}
	after, _ := e.Status(f.scope, f.expect.OperationID)
	if after.ControlRevision != d.ControlRevision {
		t.Fatal("rejected epoch wrote state")
	}
}

// capability_id: rainbond.vm-restore.rpc-command-contract
func TestRestoreReadProvidesSealedAAccessModeFullBAndOriginalPowerIntent(t *testing.T) {
	f, e, source := enginePrepareFixture(t)
	source.value.Config.Volumes[0].AccessMode = "ReadWriteOnce"
	source.value.Intent.ConfigAfterHash, _ = RestoreConfigHash(source.value.Config)
	source.value.Frame.Disks[0].TargetAccessMode = "ReadWriteOnce"
	source.value.Frame.FrameHash, _ = RestoreResourceFrameHash(source.value.Frame)
	if err := f.db.Create(&dbmodel.ComponentK8sAttributes{TenantID: f.scope.TenantID, ComponentID: f.scope.ServiceID, Name: "vm_ssh_username", AttributeValue: "B_USER_SENTINEL"}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := e.Prepare(context.Background(), f.scope, f.expect.OperationID, f.intent.SourceSnapshot.Name, f.intent.PowerAfter); err != nil {
		t.Fatal(err)
	}
	d, err := e.Status(f.scope, f.expect.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if !d.SourceConfigReady || d.ConfigAfter == nil || d.ConfigAfter.Volumes[0].AccessMode != "ReadWriteOnce" || d.ConfigBefore.Volumes[0].AccessMode != "ReadWriteMany" || len(d.ConfigBefore.Volumes) != 3 || len(d.ConfigAfter.Volumes) != 2 {
		t.Fatal("A and B metadata were guessed or incompletely projected")
	}
	if !d.WasRunning || !d.ResumeRequired || d.PowerAfter != "previous" || d.SourceBindingGeneration != 0 {
		t.Fatal("sealed pre-restore power/generation lost")
	}
	if hash, _ := RestoreConfigHash(d.ConfigBefore); hash != d.ConfigBeforeHash {
		t.Fatal("B integrity seal mismatch")
	}
	if hash, _ := RestoreConfigHash(*d.ConfigAfter); hash != d.ConfigAfterHash {
		t.Fatal("A integrity seal mismatch")
	}
	raw, _ := json.Marshal(d)
	if strings.Contains(string(raw), "B_USER_SENTINEL") || strings.Contains(string(raw), "vm_ssh_username") || strings.Contains(string(raw), "userdata") {
		t.Fatal("safe before/after projection exposed initialization input")
	}
	before, _ := e.Store.Read(f.scope, f.expect.OperationID)
	if _, err := e.Status(f.scope, f.expect.OperationID); err != nil {
		t.Fatal(err)
	}
	after, _ := e.Store.Read(f.scope, f.expect.OperationID)
	if before.State.ControlRevision != after.State.ControlRevision {
		t.Fatal("safe read progressed restore")
	}
}

// capability_id: rainbond.vm-restore.rpc-command-contract
func TestHistoricalReceiptWithoutSealedAConfigDoesNotInventIt(t *testing.T) {
	f, e, _ := engineFixture(t)
	d, err := e.Status(f.scope, f.expect.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if d.SourceConfigReady || d.ConfigAfter != nil || len(d.ConfigBefore.Volumes) != 3 {
		t.Fatal("historical missing A was filled from current B")
	}
}
