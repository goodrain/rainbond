package cleanup

import (
	"errors"
	"testing"

	"github.com/goodrain/rainbond/db/model"
)

// capability_id: rainbond.cleanup.single-deletion-attempt
func TestDeletionAttemptIsConsumedOnceAndRemainsProtectedUntilVerified(t *testing.T) {
	database, _ := coordinationDB(t)
	r := operation("selected", "delete", "app/a")
	if _, err := AcquireOperation(database, r); err != nil {
		t.Fatal(err)
	}
	if err := BeginDeletionAttempt(database, r); err != nil {
		t.Fatal(err)
	}
	if err := BeginDeletionAttempt(database, r); !errors.Is(err, ErrCoordinationUncertain) {
		t.Fatal("DELETE attempt granted twice", err)
	}
	if err := FinishOperation(database, r, true); !errors.Is(err, ErrCoordinationUncertain) {
		t.Fatal("in-flight DELETE released", err)
	}
	if err := CompleteDeletionAttempt(database, r, "applied"); err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireOperation(database, operation("writer", "producer", "app/a")); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("post-delete verification scope released early", err)
	}
	if err := FinishOperation(database, r, true); err != nil {
		t.Fatal(err)
	}
	if err := BeginDeletionAttempt(database, r); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("completed DELETE replayed", err)
	}
}
func TestUnknownDeletionAttemptCannotBecomeSuccessByRetry(t *testing.T) {
	database, _ := coordinationDB(t)
	r := operation("selected", "delete", "app/a")
	if _, err := AcquireOperation(database, r); err != nil {
		t.Fatal(err)
	}
	if err := BeginDeletionAttempt(database, r); err != nil {
		t.Fatal(err)
	}
	if err := CompleteDeletionAttempt(database, r, "unknown"); err != nil {
		t.Fatal(err)
	}
	if err := CompleteDeletionAttempt(database, r, "applied"); !errors.Is(err, ErrCoordinationUncertain) {
		t.Fatal("unknown result overwritten", err)
	}
	if err := FinishOperation(database, r, true); !errors.Is(err, ErrCoordinationUncertain) {
		t.Fatal("unknown result released", err)
	}
}

func TestCompletedAttemptCanBeRecordedAfterReadinessWithdrawal(t *testing.T) {
	database, _ := coordinationDB(t)
	r := operation("selected", "delete", "app/a")
	if _, err := AcquireOperation(database, r); err != nil {
		t.Fatal(err)
	}
	if err := BeginDeletionAttempt(database, r); err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&model.CleanupStorage{}).Where("storage_id = ?", r.StorageID).Update("mode", "collecting").Error; err != nil {
		t.Fatal(err)
	}
	if err := CompleteDeletionAttempt(database, r, "applied"); err != nil {
		t.Fatal("observed result lost after readiness withdrawal", err)
	}
	if err := BeginDeletionAttempt(database, r); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("withdrawal still granted execution", err)
	}
	if err := FinishOperation(database, r, true); err != nil {
		t.Fatal("verified existing operation cannot finish", err)
	}
	if _, err := AcquireOperation(database, operation("new-delete", "delete", "app/a")); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("recording an outcome enabled new deletion", err)
	}
}
