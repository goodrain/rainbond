package cleanup

import (
	"errors"
	"testing"

	"github.com/goodrain/rainbond/db/model"
)

// capability_id: rainbond.cleanup.manual-gc-maintenance
func TestMaintenanceDrainsWritersAndRestoresOnlyAfterConfirmation(t *testing.T) {
	database, _ := coordinationDB(t)
	writer := operation("writer", "producer", "app/a")
	if _, err := AcquireOperation(database, writer); err != nil {
		t.Fatal(err)
	}
	gc := operation("manual-gc", "gc", "*")
	if created, err := RequestMaintenance(database, gc); err != nil || !created {
		t.Fatal(created, err)
	}
	if _, err := AcquireOperation(database, operation("new-writer", "producer", "app/b")); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("maintenance admitted new writer", err)
	}
	if err := EnterMaintenance(database, gc); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("GC entered before draining writer", err)
	}
	if err := FinishOperation(database, writer, true); err != nil {
		t.Fatal(err)
	}
	if err := EnterMaintenance(database, gc); err != nil {
		t.Fatal(err)
	}
	if err := FinishMaintenanceRestore(database, gc, true); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("restored before GC and native restore finished", err)
	}
	if err := CompleteMaintenanceWork(database, gc, "succeeded"); err != nil {
		t.Fatal(err)
	}
	if err := BeginMaintenanceRestore(database, gc); err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireOperation(database, operation("still-blocked", "producer", "app/b")); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("native restore pending but writes admitted", err)
	}
	if err := FinishMaintenanceRestore(database, gc, true); err != nil {
		t.Fatal(err)
	}
	if created, err := AcquireOperation(database, operation("new-write", "producer", "app/b")); err != nil || !created {
		t.Fatal("writes not restored", created, err)
	}
	if err := FinishMaintenanceRestore(database, gc, true); err != nil {
		t.Fatal("idempotent completion failed", err)
	}
}
func TestUncertainGCDoesNotAutomaticallyRestoreWrites(t *testing.T) {
	database, _ := coordinationDB(t)
	gc := operation("manual-gc", "gc", "*")
	if _, err := AcquireOperation(database, gc); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("GC bypassed maintenance entry")
	}
	if _, err := RequestMaintenance(database, gc); err != nil {
		t.Fatal(err)
	}
	if err := EnterMaintenance(database, gc); err != nil {
		t.Fatal(err)
	}
	if err := CompleteMaintenanceWork(database, gc, "unknown"); err != nil {
		t.Fatal(err)
	}
	if err := BeginMaintenanceRestore(database, gc); !errors.Is(err, ErrCoordinationUncertain) {
		t.Fatal("unresolved GC restored writes", err)
	}
	if err := FinishOperation(database, gc, true); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("generic completion bypassed GC restoration", err)
	}
	if _, err := AcquireOperation(database, operation("new-write", "producer", "app/b")); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("uncertain GC lost global exclusion", err)
	}
	var store model.CleanupStorage
	if err := database.First(&store).Error; err != nil || store.Mode != "recovery_required" || store.MaintenanceOperationID != gc.OperationID {
		t.Fatal(store, err)
	}
}
func TestOnlyMaintenanceOwnerCanCancelBeforeGC(t *testing.T) {
	database, _ := coordinationDB(t)
	gc := operation("manual-gc", "gc", "*")
	if _, err := RequestMaintenance(database, gc); err != nil {
		t.Fatal(err)
	}
	other := gc
	other.Owner = "other"
	if err := CancelMaintenanceDrain(database, other); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("foreign owner canceled maintenance", err)
	}
	if err := CancelMaintenanceDrain(database, gc); err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireOperation(database, operation("writer", "producer", "app/a")); err != nil {
		t.Fatal(err)
	}
}
