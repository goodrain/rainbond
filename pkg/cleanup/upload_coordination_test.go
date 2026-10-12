package cleanup

import (
	"errors"
	"testing"
)

// capability_id: rainbond.cleanup.registry-upload-lifecycle
func TestUploadSessionRemainsProtectedBetweenParts(t *testing.T) {
	database, _ := coordinationDB(t)
	upload := operation("upload", "producer", "app/a")
	if _, err := AcquireOperation(database, upload); err != nil {
		t.Fatal(err)
	}
	if err := BindRegistryUpload(database, upload, "app/a", "upload-id"); err != nil {
		t.Fatal(err)
	}
	found, err := LookupRegistryUpload(database, "store", "generation1", "app/a", "upload-id")
	if err != nil || found != upload {
		t.Fatal("upload identity lost", found, err)
	}
	if err := FinishOperation(database, upload, true); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("ordinary HTTP completion released entire upload", err)
	}
	part := operation("part-1", "producer", "app/a")
	if created, err := AcquireUploadRequest(database, upload, part, false); err != nil || !created {
		t.Fatal(created, err)
	}
	finish := operation("complete", "producer", "app/a")
	if _, err := AcquireUploadRequest(database, upload, finish, true); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("upload completed with part still active", err)
	}
	if err := FinishUploadRequest(database, upload, part, true); err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireOperation(database, operation("delete", "delete", "app/a")); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("gap between parts exposed upload to cleanup", err)
	}
	gc := operation("gc", "gc", "*")
	if _, err := RequestMaintenance(database, gc); err != nil {
		t.Fatal(err)
	}
	// Already accepted uploads may drain, while brand-new writers stay blocked.
	if _, err := AcquireUploadRequest(database, upload, finish, true); err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireUploadRequest(database, upload, operation("late-part", "producer", "app/a"), false); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("late part entered closing upload", err)
	}
	if err := FinishUploadRequest(database, upload, finish, true); err != nil {
		t.Fatal(err)
	}
	if err := EnterMaintenance(database, gc); err != nil {
		t.Fatal("completed upload did not drain", err)
	}
}
func TestUnknownUploadRequestDoesNotReleaseParent(t *testing.T) {
	database, _ := coordinationDB(t)
	parent := operation("upload", "producer", "app/a")
	if _, err := AcquireOperation(database, parent); err != nil {
		t.Fatal(err)
	}
	if err := BindRegistryUpload(database, parent, "app/a", "upload-id"); err != nil {
		t.Fatal(err)
	}
	part := operation("part", "producer", "app/a")
	if _, err := AcquireUploadRequest(database, parent, part, false); err != nil {
		t.Fatal(err)
	}
	if err := FinishUploadRequest(database, parent, part, false); err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireUploadRequest(database, parent, operation("retry", "producer", "app/a"), false); !errors.Is(err, ErrCoordinationUncertain) {
		t.Fatal("unknown upload silently resumed", err)
	}
	if err := FinishUploadRequest(database, parent, part, true); !errors.Is(err, ErrCoordinationUncertain) {
		t.Fatal("unknown upload silently released", err)
	}
	if _, err := LookupRegistryUpload(database, "store", "generation1", "other/repository", "upload-id"); err == nil {
		t.Fatal("cross-repository upload identity accepted")
	}
}

func TestRejectedUploadCompletionReopensWithoutReleasingSession(t *testing.T) {
	database, _ := coordinationDB(t)
	parent := operation("upload", "producer", "app/a")
	if _, err := AcquireOperation(database, parent); err != nil {
		t.Fatal(err)
	}
	if err := BindRegistryUpload(database, parent, "app/a", "upload-id"); err != nil {
		t.Fatal(err)
	}
	closing := operation("complete", "producer", "app/a")
	if _, err := AcquireUploadRequest(database, parent, closing, true); err != nil {
		t.Fatal(err)
	}
	if err := RecordUploadRequest(database, parent, closing, "rejected"); err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireOperation(database, operation("delete", "delete", "app/a")); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("rejection released upload", err)
	}
	if _, err := AcquireUploadRequest(database, parent, operation("next-part", "producer", "app/a"), false); err != nil {
		t.Fatal("confirmed rejection stranded upload", err)
	}
}
