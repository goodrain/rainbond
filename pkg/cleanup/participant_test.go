package cleanup

import (
	"errors"
	"testing"

	"github.com/goodrain/rainbond/db/model"
)

func TestParticipantRegistrationCannotReplaceInstanceOrEnableCleanup(t *testing.T) {
	database, _ := coordinationDB(t)
	if err := database.AutoMigrate(&model.CleanupParticipant{}).Error; err != nil {
		t.Fatal(err)
	}
	binding, err := ProvisionRegistryStorage(database, "volume", "/var/lib/registry")
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, _ := binding.Fingerprint()
	participant := ParticipantRegistration{StorageID: binding.StorageID, Generation: binding.Generation, Owner: "registry-instance", Role: "registry-ingress", PodUID: "pod", ContainerID: "containerd://instance", ImageID: "image-sha", BindingFingerprint: fingerprint}
	if err := RegisterParticipant(database, participant); err != nil {
		t.Fatal(err)
	}
	if err := RegisterParticipant(database, participant); err != nil {
		t.Fatal(err)
	}
	changed := participant
	changed.ContainerID = "containerd://replacement"
	if err := RegisterParticipant(database, changed); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("replaced instance inherited registration", err)
	}
	observed, err := InspectStorage(database, binding.StorageID, binding.Generation)
	if err != nil || observed.Mode != "collecting" {
		t.Fatal("participant self-registration enabled deletion", err)
	}
}

// capability_id: rainbond.cleanup.participant-replacement
func TestParticipantReplacementWithdrawsReadinessWithoutLosingDeletion(t *testing.T) {
	database, _ := coordinationDB(t)
	if err := database.AutoMigrate(&model.CleanupParticipant{}).Error; err != nil {
		t.Fatal(err)
	}
	binding, err := ProvisionRegistryStorage(database, "replacement-volume", "/var/lib/registry")
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, _ := binding.Fingerprint()
	participant := ParticipantRegistration{StorageID: binding.StorageID, Generation: binding.Generation, Owner: "first-instance", Role: "registry-ingress", PodUID: "pod", ContainerID: "containerd://first", ImageID: "image-sha", BindingFingerprint: fingerprint}
	if err := RegisterParticipant(database, participant); err != nil {
		t.Fatal(err)
	}
	// Only this test fixture establishes readiness; enrollment must never do it.
	if err := database.Model(&model.CleanupStorage{}).Where("storage_id = ?", binding.StorageID).Update("mode", "ready").Error; err != nil {
		t.Fatal(err)
	}
	request := operation("selected", "delete", "app/a")
	request.StorageID, request.Generation = binding.StorageID, binding.Generation
	if _, err := AcquireOperation(database, request); err != nil {
		t.Fatal(err)
	}
	if err := BeginDeletionAttempt(database, request); err != nil {
		t.Fatal(err)
	}
	participant.Owner, participant.ContainerID = "replacement-instance", "containerd://replacement"
	if err := RegisterParticipant(database, participant); err != nil {
		t.Fatal(err)
	}
	observed, err := InspectStorage(database, binding.StorageID, binding.Generation)
	if err != nil || observed.Mode != "collecting" {
		t.Fatal("replacement did not withdraw deletion readiness", observed, err)
	}
	if err := CompleteDeletionAttempt(database, request, "applied"); err != nil {
		t.Fatal("replacement lost the in-flight deletion receipt", err)
	}
	writer := request
	writer.OperationID, writer.Kind, writer.Target = "writer", "producer", ""
	if _, err := AcquireOperation(database, writer); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("replacement released the original deletion protection", err)
	}
	if err := FinishOperation(database, request, true); err != nil {
		t.Fatal(err)
	}
	request.OperationID = "another-delete"
	if _, err := AcquireOperation(database, request); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("finishing the original deletion restored readiness", err)
	}
}
