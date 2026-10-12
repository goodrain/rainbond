//go:build linux || darwin

package registryproxy

import (
	"context"
	"errors"
	"testing"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	"golang.org/x/sys/unix"
)

// capability_id: rainbond.cleanup.durable-gc-receipt
func TestGCReceiptIsBoundImmutableAndDoesNotPermitReplay(t *testing.T) {
	root := t.TempDir()
	binding := coordination.StorageRegistration{StorageID: "owned", Generation: "one", VolumeUID: "volume", RootPath: "/registry"}
	r := coordination.CoordinationRequest{StorageID: "owned", Generation: "one", OperationID: "gc", Owner: "owner", Kind: "gc", Scope: "*", Fingerprint: "confirmation"}
	if err := InitializeStorageIdentity(root, binding); err != nil {
		t.Fatal(err)
	}
	fd, err := openIdentityRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)
	before, err := measureStorageFD(fd, binding)
	if err != nil {
		t.Fatal(err)
	}
	journal, err := reserveGCReceipt(fd, binding, r, before)
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	if _, err := ReadGCReceipt(root, binding, r); !errors.Is(err, coordination.ErrCoordinationUncertain) {
		t.Fatal("admission alone was treated as completed work", err)
	}
	if _, err := reserveGCReceipt(fd, binding, r, before); !errors.Is(err, coordination.ErrCoordinationUncertain) {
		t.Fatal("same operation admitted twice", err)
	}
	if err := journal.Complete("succeeded", &before); err != nil {
		t.Fatal(err)
	}
	if err := journal.Complete("failed", &before); err == nil {
		t.Fatal("terminal receipt overwritten")
	}
	receipt, err := ReadGCReceipt(root, binding, r)
	if err != nil || receipt.Outcome != "succeeded" {
		t.Fatal("stored receipt unavailable", err)
	}
	r.Fingerprint = "another-confirmation"
	if _, err := ReadGCReceipt(root, binding, r); err == nil {
		t.Fatal("receipt reused for another confirmation")
	}
}

type receiptRecoveryRecorder struct{ calls int }

func (r *receiptRecoveryRecorder) BeginGC(context.Context, StorageMeasurement) error {
	r.calls++
	return nil
}
func (r *receiptRecoveryRecorder) CompleteGC(context.Context, string) error { r.calls++; return nil }
func (r *receiptRecoveryRecorder) ObserveGC(context.Context, StorageMeasurement) error {
	r.calls++
	return nil
}

func TestUnknownGCReceiptCannotBeTurnedIntoCompletion(t *testing.T) {
	root := t.TempDir()
	binding := coordination.StorageRegistration{StorageID: "owned", Generation: "one", VolumeUID: "volume", RootPath: "/registry"}
	r := coordination.CoordinationRequest{StorageID: "owned", Generation: "one", OperationID: "gc", Owner: "owner", Kind: "gc", Scope: "*", Fingerprint: "confirmation"}
	if err := InitializeStorageIdentity(root, binding); err != nil {
		t.Fatal(err)
	}
	fd, err := openIdentityRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)
	before, err := measureStorageFD(fd, binding)
	if err != nil {
		t.Fatal(err)
	}
	journal, err := reserveGCReceipt(fd, binding, r, before)
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	if err := journal.Complete("unknown", nil); err != nil {
		t.Fatal(err)
	}
	recorder := &receiptRecoveryRecorder{}
	if err := RecoverGCReceipt(context.Background(), root, binding, r, recorder); !errors.Is(err, coordination.ErrCoordinationUncertain) || recorder.calls != 0 {
		t.Fatal("unknown execution was acknowledged or retried", err, recorder.calls)
	}
}
