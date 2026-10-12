package cleanup

import (
	"strings"
	"testing"
	"time"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// capability_id: rainbond.cleanup.gc-job-restore
func TestGCJobRestoreRequiresOriginalBindingAndMeasurements(t *testing.T) {
	db, path := coordinationDB(t)
	storage, err := ProvisionRegistryStorage(db, "owned-volume", "/registry")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.CleanupStorage{}).Where("storage_id = ?", storage.StorageID).Update("mode", "ready").Error; err != nil {
		t.Fatal(err)
	}
	r := CoordinationRequest{StorageID: storage.StorageID, Generation: storage.Generation, OperationID: "restore-job", Owner: "gc", Kind: "gc", Scope: "*", Fingerprint: "confirmed"}
	if _, err := RequestMaintenance(db, r); err != nil {
		t.Fatal(err)
	}
	intent, _, err := PrepareGCJob(db, r, "system", strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	if err := BindGCJob(db, r, intent, "job-uid"); err != nil {
		t.Fatal(err)
	}
	if err := BindGCExecutor(db, r, "job-uid", "job-pod", "pod-uid"); err != nil {
		t.Fatal(err)
	}
	if err := EnterGCJobExecution(db, r, "job-uid", "pod-uid"); err != nil {
		t.Fatal(err)
	}
	fingerprint, _ := storage.Fingerprint()
	before := StorageMeasurement{Protocol: 1, StorageID: storage.StorageID, Generation: storage.Generation, BindingFingerprint: fingerprint, FilesystemID: "same-fs", ObservedAt: time.Now().UTC(), TotalBytes: 1000, FreeBytes: 100, AvailableBytes: 50}
	if err := RecordMaintenanceMeasurement(db, r, "before", before); err != nil {
		t.Fatal(err)
	}
	if err := CompleteMaintenanceWork(db, r, "succeeded"); err != nil {
		t.Fatal(err)
	}
	if err := BeginMaintenanceRestore(db, r); err == nil {
		t.Fatal("legacy endpoint bypassed Job verification")
	}
	if err := BeginGCJobRestore(db, r, "job-uid", "pod-uid"); err == nil {
		t.Fatal("missing after measurement accepted")
	}
	after := before
	after.ObservedAt = before.ObservedAt.Add(time.Second)
	after.FreeBytes = 200
	after.AvailableBytes = 150
	if err := RecordMaintenanceMeasurement(db, r, "after", after); err != nil {
		t.Fatal(err)
	}
	if err := BeginGCJobRestore(db, r, "other-job", "pod-uid"); err == nil {
		t.Fatal("wrong executor accepted")
	}
	if err := BeginGCJobRestore(db, r, "job-uid", "pod-uid"); err != nil {
		t.Fatal(err)
	}
	if err := FinishMaintenanceRestore(db, r, true); err == nil {
		t.Fatal("legacy finish bypassed verification")
	}
	reopened, err := gorm.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := BeginGCJobRestore(reopened, r, "job-uid", "pod-uid"); err != nil {
		t.Fatal("lost begin response not recoverable", err)
	}
	if err := FinishGCJobRestore(reopened, r, "job-uid", "pod-uid"); err != nil {
		t.Fatal(err)
	}
	if err := FinishGCJobRestore(reopened, r, "job-uid", "pod-uid"); err != nil {
		t.Fatal("finish not idempotent", err)
	}
	writer := r
	writer.Kind = "producer"
	writer.OperationID = "new-writer"
	if _, err := AcquireOperation(reopened, writer); err != nil {
		t.Fatal("writes remain blocked", err)
	}
}

// capability_id: rainbond.cleanup.gc-job-progress
func TestGCProgressRejectsAnotherOperationIdentity(t *testing.T) {
	db, _ := coordinationDB(t)
	r := operation("progress", "gc", "*")
	if _, err := RequestMaintenance(db, r); err != nil {
		t.Fatal(err)
	}
	progress, err := ReadGCJobProgress(db, r)
	if err != nil || progress.State != "draining" || progress.OperationID != r.OperationID || progress.JobUID != "" {
		t.Fatal(progress, err)
	}
	changed := r
	changed.Owner = "different"
	if _, err := ReadGCJobProgress(db, changed); err == nil {
		t.Fatal("foreign operation progress disclosed")
	}
}
