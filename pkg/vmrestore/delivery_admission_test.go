package vmrestore

import (
	"errors"
	"testing"
)

// capability_id: rainbond.vm-delivery.capture-admission
func TestDeliveryCaptureOwnsMutationUntilReleased(t *testing.T) {
	store, _, scope := fixtureStore(t)
	if err := store.ClaimDeliveryCapture(scope, "delivery-one"); err != nil {
		t.Fatal(err)
	}
	if err := store.ClaimDeliveryCapture(scope, "delivery-one"); err != nil {
		t.Fatal("lost ACK must replay", err)
	}
	if err := store.ClaimDeliveryCapture(scope, "delivery-two"); !errors.Is(err, ErrBusy) {
		t.Fatalf("second capture admitted: %v", err)
	}
	current, err := store.Read(scope, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := current.State.AdmitMutation("power-one", "ordinary-vm-start"); !errors.Is(err, ErrBusy) {
		t.Fatal("capture did not block power", err)
	}
	if err := store.ReleaseDeliveryCapture(scope, "delivery-two"); !errors.Is(err, ErrConflict) {
		t.Fatal("foreign capture released ownership", err)
	}
	if err := store.ReleaseDeliveryCapture(scope, "delivery-one"); err != nil {
		t.Fatal(err)
	}
	if err := store.ReleaseDeliveryCapture(scope, "delivery-one"); err != nil {
		t.Fatal("release must replay", err)
	}
	if err := store.ClaimDeliveryCapture(scope, "delivery-two"); err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.vm-delivery.capture-admission
func TestDeliveryCaptureRejectsInvalidIdentityAndUnavailableStore(t *testing.T) {
	store, database, scope := fixtureStore(t)
	for _, id := range []string{"", "UPPERCASE", "../../foreign"} {
		if err := store.ClaimDeliveryCapture(scope, id); !errors.Is(err, ErrCorrupt) {
			t.Fatal("invalid capture admitted", err)
		}
		if err := store.ReleaseDeliveryCapture(scope, id); !errors.Is(err, ErrCorrupt) {
			t.Fatal("invalid release admitted", err)
		}
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.ClaimDeliveryCapture(scope, "delivery-one"); err == nil {
		t.Fatal("unavailable store admitted a capture")
	}
}
