package vmrestore

import (
	"context"
	"errors"
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corefake "k8s.io/client-go/kubernetes/fake"
	kv "kubevirt.io/api/core/v1"
)

// capability_id: rainbond.vm-restore.read-only-production-metadata
func TestRestoreMetadataReadOnlyKeepsPrivateStartupValidation(t *testing.T) {
	f := newRestoreCommitFixture(t)
	vm := &kv.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: f.intent.CurrentVM.Name, Namespace: f.scope.Namespace}, Spec: kv.VirtualMachineSpec{Template: &kv.VirtualMachineInstanceTemplateSpec{}}}
	publication, err := PrepareNewVMStartup(f.scope, vm, VMStartupHints{})
	if err != nil {
		t.Fatal(err)
	}
	vm.UID = "vm-b"
	if err := f.store.SealNewVMStartup(context.Background(), corefake.NewSimpleClientset(), vm, *publication); err != nil {
		t.Fatal(err)
	}
	current, err := f.store.Read(f.scope, "")
	if err != nil {
		t.Fatal(err)
	}
	f.expect.ControlRevision = current.State.ControlRevision
	f.prepare(t)
	f.db.DB().SetMaxOpenConns(1)
	if err := f.db.Exec("PRAGMA query_only=ON").Error; err != nil {
		t.Fatal(err)
	}
	trace := &ledgerSQLTrace{}
	f.db.SetLogger(trace)
	f.db.LogMode(true)
	engine := &RestoreEngine{Store: f.store}
	if _, err := engine.Control(f.scope); err != nil {
		t.Fatal("control required SQL write authority", err)
	}
	if _, err := engine.Status(f.scope, f.expect.OperationID); err != nil {
		t.Fatal("status required SQL write authority", err)
	}
	if len(trace.writes) != 0 {
		t.Fatal("production metadata used write-class SQL", trace.writes)
	}
	f.db.LogMode(false)
	if err := f.db.Exec("PRAGMA query_only=OFF").Error; err != nil {
		t.Fatal(err)
	}
	key, _ := ReceiptKey(publication.OperationID)
	if err := f.db.Where("component_id = ? AND name = ?", f.scope.ServiceID, key).Delete(&dbmodel.ComponentK8sAttributes{}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Status(f.scope, f.expect.OperationID); !errors.Is(err, ErrCorrupt) {
		t.Fatal("readonly projection skipped missing startup proof", err)
	}
}

// capability_id: rainbond.vm-restore.read-only-production-metadata
func TestSnapshotMetadataReadOnlyWorksOnQueryOnlyConnection(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	ready := engineReady(t, engine, scope, backend)
	database := engine.store.db
	database.DB().SetMaxOpenConns(1)
	if err := database.Exec("PRAGMA query_only=ON").Error; err != nil {
		t.Fatal(err)
	}
	trace := &ledgerSQLTrace{}
	database.SetLogger(trace)
	database.LogMode(true)
	dto, err := engine.Get(context.Background(), scope, ready.OperationID)
	if err != nil || !dto.ReadyToUse || len(trace.writes) != 0 {
		t.Fatal("snapshot metadata required write authority", err, trace.writes)
	}
}
