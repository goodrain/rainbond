package cleanup

import (
	"errors"
	"testing"
)

// capability_id: rainbond.cleanup.queued-producer-handoff
func TestQueuedProducerProtectsWaitingWorkAndClaimsExactlyOnce(t *testing.T) {
	database, _ := coordinationDB(t)
	request := operation("queued-native", "producer", "*")
	if created, err := QueueOperation(database, request); err != nil || !created {
		t.Fatal(created, err)
	}
	if _, err := AcquireOperation(database, operation("delete", "delete", "app/a")); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("queue wait gap unprotected", err)
	}
	wrong := request
	wrong.Fingerprint = "different"
	if _, err := ClaimQueuedOperation(database, wrong); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("changed task claimed reservation", err)
	}
	gc := operation("gc", "gc", "*")
	if _, err := RequestMaintenance(database, gc); err != nil {
		t.Fatal(err)
	}
	if claimed, err := ClaimQueuedOperation(database, request); err != nil || !claimed {
		t.Fatal("queued work cannot drain", err)
	}
	if _, err := ClaimQueuedOperation(database, request); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("task claimed twice", err)
	}
	if err := EnterMaintenance(database, gc); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("GC entered active task", err)
	}
	if err := FinishOperation(database, request, true); err != nil {
		t.Fatal(err)
	}
	if err := EnterMaintenance(database, gc); err != nil {
		t.Fatal(err)
	}
}

func TestQueuedReservationCannotBeReleasedAfterClaimOrUncertainty(t *testing.T) {
	database, _ := coordinationDB(t)
	request := operation("waiting", "producer", "*")
	if _, err := QueueOperation(database, request); err != nil {
		t.Fatal(err)
	}
	if err := FinishOperation(database, request, true); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("enqueue completion released queued work", err)
	}
	if err := CancelUnsubmittedOperation(database, request); err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimQueuedOperation(database, request); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("cancelled reservation executed", err)
	}
	request.OperationID = "started"
	if _, err := QueueOperation(database, request); err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimQueuedOperation(database, request); err != nil {
		t.Fatal(err)
	}
	if err := CancelUnsubmittedOperation(database, request); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("active work cancelled as unsubmitted", err)
	}
	if err := FinishOperation(database, request, false); err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimQueuedOperation(database, request); !errors.Is(err, ErrCoordinationUncertain) {
		t.Fatal("uncertain work claimed again", err)
	}
}
