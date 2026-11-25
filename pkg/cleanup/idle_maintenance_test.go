package cleanup

import (
	"errors"
	"testing"

	"github.com/goodrain/rainbond/db/model"
)

// capability_id: rainbond.cleanup.idle-maintenance-preserves-push
func TestIdleMaintenanceLeavesAcceptedBuildWritableUntilPushFinishes(t *testing.T) {
	database, _ := coordinationDB(t)
	build := operation("whole-build", "producer", "*")
	if created, err := AcquireOperation(database, build); err != nil || !created {
		t.Fatal(created, err)
	}
	before, err := InspectStorage(database, build.StorageID, build.Generation)
	if err != nil {
		t.Fatal(err)
	}
	gc := operation("nightly-gc", "gc", "*")
	if created, err := RequestIdleMaintenance(database, gc); created || !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("GC interrupted an admitted whole build", created, err)
	}
	after, err := InspectStorage(database, build.StorageID, build.Generation)
	if err != nil || before != after || after.Mode != "ready" {
		t.Fatal("busy idle check changed write admission", before, after, err)
	}
	for _, id := range []string{"blob-http", "manifest-http"} {
		r := operation(id, "producer", "app/a")
		if created, err := AcquireOperation(database, r); err != nil || !created {
			t.Fatal("accepted build could not continue its Registry requests", created, err)
		}
		if err := FinishOperation(database, r, true); err != nil {
			t.Fatal(err)
		}
	}
	if state, err := InspectOperation(database, build); err != nil || state != "active" {
		t.Fatal("idle maintenance released the whole-build lease", state, err)
	}
	if err := FinishOperation(database, build, true); err != nil {
		t.Fatal(err)
	}
	if created, err := RequestIdleMaintenance(database, gc); err != nil || !created {
		t.Fatal("completed build still blocked GC", created, err)
	}
	if _, err := AcquireOperation(database, operation("late-http", "producer", "app/a")); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("new push entered after idle maintenance won", err)
	}
	if created, err := RequestIdleMaintenance(database, gc); err != nil || created {
		t.Fatal("same-slot retry was not idempotent", created, err)
	}
	changed := gc
	changed.Fingerprint = "changed"
	if _, err := RequestIdleMaintenance(database, changed); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("changed same-slot request accepted", err)
	}
}

// capability_id: rainbond.cleanup.idle-maintenance-preserves-push
func TestIdleMaintenanceDoesNotIgnoreUnfinishedOperations(t *testing.T) {
	for _, state := range []string{"active", "queued", "closing", "uncertain"} {
		t.Run(state, func(t *testing.T) {
			database, _ := coordinationDB(t)
			pending := operation("unfinished", "producer", "app/a")
			if err := database.Create(&model.CleanupOperation{StorageID: pending.StorageID, Generation: pending.Generation, OperationID: pending.OperationID, Owner: pending.Owner, Kind: pending.Kind, Scope: pending.Scope, Fingerprint: pending.Fingerprint, State: state}).Error; err != nil {
				t.Fatal(err)
			}
			if created, err := RequestIdleMaintenance(database, operation("night", "gc", "*")); created || !errors.Is(err, ErrCoordinationBusy) {
				t.Fatal("unfinished operation was ignored", state, created, err)
			}
			observed, err := InspectStorage(database, pending.StorageID, pending.Generation)
			if err != nil || observed.Mode != "ready" {
				t.Fatal("rejected idle maintenance closed writes", observed, err)
			}
			var records int
			if err := database.Model(&model.CleanupOperation{}).Where("kind = ?", "gc").Count(&records).Error; err != nil || records != 0 {
				t.Fatal("rejected idle maintenance persisted a GC operation", records, err)
			}
		})
	}
}

// capability_id: rainbond.cleanup.idle-maintenance-atomic-admission
func TestIdleMaintenanceAndProducerRaceHasOnlyOneAdmission(t *testing.T) {
	for attempt := 0; attempt < 10; attempt++ {
		database, _ := coordinationDB(t)
		database.DB().SetMaxOpenConns(2)
		start := make(chan struct{})
		type admission struct {
			gc      bool
			created bool
			err     error
		}
		results := make(chan admission, 2)
		go func() {
			<-start
			created, err := RequestIdleMaintenance(database, operation("night", "gc", "*"))
			results <- admission{gc: true, created: created, err: err}
		}()
		go func() {
			<-start
			created, err := AcquireOperation(database, operation("build", "producer", "*"))
			results <- admission{created: created, err: err}
		}()
		close(start)
		winners, gcWon := 0, false
		for i := 0; i < 2; i++ {
			result := <-results
			if result.created && result.err == nil {
				winners++
				gcWon = result.gc
			} else if result.created || !errors.Is(result.err, ErrCoordinationBusy) {
				t.Fatal("unexpected race result", result)
			}
		}
		if winners != 1 {
			t.Fatal("GC and producer did not serialize", winners)
		}
		observed, err := InspectStorage(database, "store", "generation1")
		if err != nil || gcWon && observed.Mode != "draining" || !gcWon && observed.Mode != "ready" {
			t.Fatal("admission mode disagrees with race winner", gcWon, observed, err)
		}
	}
}

// capability_id: rainbond.cleanup.idle-maintenance-atomic-admission
func TestConcurrentIdleMaintenanceHasOneGCWinner(t *testing.T) {
	database, _ := coordinationDB(t)
	database.DB().SetMaxOpenConns(2)
	start := make(chan struct{})
	type admission struct {
		created bool
		err     error
	}
	results := make(chan admission, 2)
	for _, id := range []string{"gc-a", "gc-b"} {
		go func(id string) {
			<-start
			created, err := RequestIdleMaintenance(database, operation(id, "gc", "*"))
			results <- admission{created, err}
		}(id)
	}
	close(start)
	winners := 0
	for i := 0; i < 2; i++ {
		result := <-results
		if result.created && result.err == nil {
			winners++
		} else if result.created || !errors.Is(result.err, ErrCoordinationBusy) {
			t.Fatal("unexpected concurrent GC result", result)
		}
	}
	if winners != 1 {
		t.Fatal("concurrent GC admissions", winners)
	}
}
