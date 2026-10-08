package cleanup

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

func coordinationDB(t *testing.T) (*gorm.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "coordination.db")
	db, err := gorm.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	db.LogMode(false)
	db.DB().SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if err = db.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&model.CleanupStorage{StorageID: "store", Generation: "generation1", Mode: "ready"}).Error; err != nil {
		t.Fatal(err)
	}
	return db, path
}
func operation(id, kind, scope string) CoordinationRequest {
	r := CoordinationRequest{StorageID: "store", Generation: "generation1", Owner: "owner", OperationID: id, Kind: kind, Scope: scope, Fingerprint: "request-binding"}
	if kind == "delete" {
		r.Target = "original-manifest"
	}
	return r
}

// capability_id: rainbond.cleanup.durable-coordination
func TestPersistentCoordinationConflictsAndRecovery(t *testing.T) {
	db, path := coordinationDB(t)
	writer := operation("write-a", "producer", "app/a")
	if created, err := AcquireOperation(db, writer); err != nil || !created {
		t.Fatal(created, err)
	}
	if created, err := AcquireOperation(db, writer); err != nil || created {
		t.Fatal("retry granted execution twice", created, err)
	}
	if _, err := AcquireOperation(db, operation("delete-a", "delete", "app/a")); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("delete raced writer", err)
	}
	if created, err := AcquireOperation(db, operation("delete-b", "delete", "app/b")); err != nil || !created {
		t.Fatal("unrelated scope blocked", err)
	}
	if _, err := AcquireOperation(db, operation("unknown-scope", "producer", "*")); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("global writer ignored selected deletion", err)
	}
	if err := FinishOperation(db, writer, false); err != nil {
		t.Fatal(err)
	}
	if err := FinishOperation(db, writer, true); !errors.Is(err, ErrCoordinationUncertain) {
		t.Fatal("uncertain operation automatically released", err)
	}
	// Close and reopen the on-disk database: protection must not live in the
	// coordinator process or expire merely because a connection was recreated.
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := gorm.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	reopened.LogMode(false)
	t.Cleanup(func() { reopened.Close() })
	db = reopened
	if _, err := AcquireOperation(db, operation("later-delete-a", "delete", "app/a")); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("restart lost unresolved writer", err)
	}
	changed := writer
	changed.Generation = "generation2"
	if _, err := AcquireOperation(db, changed); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("old storage identity accepted", err)
	}
}
func TestCoordinationOwnerAndMaintenanceBoundaries(t *testing.T) {
	db, _ := coordinationDB(t)
	req := operation("delete-a", "delete", "app/a")
	if _, err := AcquireOperation(db, req); err != nil {
		t.Fatal(err)
	}
	other := req
	other.Owner = "other"
	if err := FinishOperation(db, other, true); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("other owner released lock", err)
	}
	other = req
	other.Fingerprint = "changed"
	if _, err := AcquireOperation(db, other); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("idempotency request changed", err)
	}
	if err := FinishOperation(db, req, true); err != nil {
		t.Fatal(err)
	}
	if created, err := AcquireOperation(db, req); err != nil || created {
		t.Fatal("completed operation replayed", created, err)
	}
	if err := db.Model(&model.CleanupStorage{}).Where("storage_id = ?", "store").Update("mode", "draining").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireOperation(db, operation("write-b", "producer", "app/b")); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("new writer admitted during maintenance", err)
	}
}

func TestCoordinationPersistenceFailureRollsBackAdmission(t *testing.T) {
	database, _ := coordinationDB(t)
	if err := database.Exec(`CREATE TRIGGER reject_revision BEFORE UPDATE OF revision ON cleanup_storage WHEN NEW.revision <> OLD.revision BEGIN SELECT RAISE(ABORT, 'injected persistence failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if created, err := AcquireOperation(database, operation("write-a", "producer", "app/a")); err == nil || created {
		t.Fatal("failed persistence admitted operation", created, err)
	}
	var count int
	if err := database.Model(&model.CleanupOperation{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("partial transaction survived", count, err)
	}
}
func TestCoordinationNoAutomaticExpiryOrUnknownModeAdmission(t *testing.T) {
	database, _ := coordinationDB(t)
	req := operation("write-a", "producer", "app/a")
	if _, err := AcquireOperation(database, req); err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&model.CleanupOperation{}).Where("operation_id = ?", req.OperationID).Update("updated_at", "2000-01-01 00:00:00").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireOperation(database, operation("delete-a", "delete", "app/a")); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("old operation silently expired", err)
	}
	if err := database.Model(&model.CleanupStorage{}).Where("storage_id = ?", "store").Update("mode", "unverified").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireOperation(database, operation("write-b", "producer", "app/b")); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("unverified storage accepted writer", err)
	}
}

func TestConcurrentDeletionAdmissionHasOneWinner(t *testing.T) {
	database, _ := coordinationDB(t)
	database.DB().SetMaxOpenConns(2)
	start := make(chan struct{})
	results := make(chan bool, 2)
	for _, id := range []string{"delete-1", "delete-2"} {
		go func(id string) {
			<-start
			created, err := AcquireOperation(database, operation(id, "delete", "app/a"))
			results <- created && err == nil
		}(id)
	}
	close(start)
	winners := 0
	for i := 0; i < 2; i++ {
		if <-results {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("concurrent cleanup admissions=%d, want exactly one", winners)
	}
}

func TestCoordinationDeletionTargetCannotChangeAfterAdmission(t *testing.T) {
	database, _ := coordinationDB(t)
	r := operation("delete", "delete", "app/a")
	if _, err := AcquireOperation(database, r); err != nil {
		t.Fatal(err)
	}
	r.Target = "unselected-manifest"
	if _, err := AcquireOperation(database, r); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("target changed under same operation", err)
	}
	if err := FinishOperation(database, r, true); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("different target released operation", err)
	}
}
