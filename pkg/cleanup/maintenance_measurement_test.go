package cleanup

import (
	"errors"
	"testing"
	"time"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// capability_id: rainbond.cleanup.durable-maintenance-measurements
func TestMaintenanceMeasurementsPersistWithoutGrantingRestore(t *testing.T) {
	database, path := coordinationDB(t)
	binding, err := ProvisionRegistryStorage(database, "owned-volume", "/registry")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&model.CleanupStorage{}).Where("storage_id = ?", binding.StorageID).Update("mode", "ready").Error; err != nil {
		t.Fatal(err)
	}
	r := CoordinationRequest{StorageID: binding.StorageID, Generation: binding.Generation, Owner: "test", OperationID: "manual-gc", Kind: "gc", Scope: "*", Fingerprint: "explicit-confirmation"}
	if _, err := RequestMaintenance(database, r); err != nil {
		t.Fatal(err)
	}
	if err := EnterMaintenance(database, r); err != nil {
		t.Fatal(err)
	}
	fingerprint, _ := binding.Fingerprint()
	before := StorageMeasurement{Protocol: 1, StorageID: binding.StorageID, Generation: binding.Generation, BindingFingerprint: fingerprint, FilesystemID: "owned-fs", ObservedAt: time.Now().UTC(), TotalBytes: 1024, FreeBytes: 512, AvailableBytes: 256}
	if err := RecordMaintenanceMeasurement(database, r, "before", before); err != nil {
		t.Fatal(err)
	}
	if err := RecordMaintenanceMeasurement(database, r, "before", before); err != nil {
		t.Fatal("identical observation retry rejected", err)
	}
	changed := before
	changed.FreeBytes++
	if err := RecordMaintenanceMeasurement(database, r, "before", changed); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("original observation overwritten", err)
	}
	if err := BeginMaintenanceRestore(database, r); err == nil {
		t.Fatal("measurement was treated as GC completion")
	}
	if err := CompleteMaintenanceWork(database, r, "succeeded"); err != nil {
		t.Fatal(err)
	}
	after := before
	after.ObservedAt = after.ObservedAt.Add(time.Second)
	after.AvailableBytes = 384
	foreign := after
	foreign.FilesystemID = "different-volume"
	if err := RecordMaintenanceMeasurement(database, r, "after", foreign); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("measurements from different filesystems were paired", err)
	}
	stale := after
	stale.ObservedAt = before.ObservedAt.Add(-time.Second)
	if err := RecordMaintenanceMeasurement(database, r, "after", stale); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("stale measurement accepted as post-GC evidence", err)
	}
	if err := RecordMaintenanceMeasurement(database, r, "after", after); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := gorm.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	observedBefore, observedAfter, err := MaintenanceMeasurements(reopened, r)
	if err != nil || observedBefore == nil || observedAfter == nil || observedBefore.AvailableBytes != 256 || observedAfter.AvailableBytes != 384 {
		t.Fatal("observations did not survive restart", err)
	}
	state, err := InspectOperation(reopened, r)
	if err != nil || state != "restore_pending" {
		t.Fatal("observation released write protection", state, err)
	}
}
