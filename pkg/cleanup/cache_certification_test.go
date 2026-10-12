package cleanup

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/goodrain/rainbond/db/model"
)

func cacheBuilderMember(binding StorageRegistration, pod, container string) ParticipantRegistration {
	fingerprint, _ := binding.Fingerprint()
	sum := sha256.Sum256([]byte(pod + "\x00" + container))
	return ParticipantRegistration{StorageID: binding.StorageID, Generation: binding.Generation, Role: "cache-builder", Owner: "cache-builder:" + hex.EncodeToString(sum[:]), PodUID: pod, ContainerID: container, ImageID: "observed-image", BindingFingerprint: fingerprint}
}

func TestCacheCertificationReconcilesOnlyRetiredProducerEpoch(t *testing.T) {
	database, _ := coordinationDB(t)
	database.AutoMigrate(&model.CleanupParticipant{})
	binding, err := ProvisionManagedCacheStorage(database, "retired-cache", "/cache/build")
	if err != nil {
		t.Fatal(err)
	}
	initial := cacheBuilderMember(binding, "old-pod", "old-container")
	if err := RegisterParticipant(database, initial); err != nil {
		t.Fatal(err)
	}
	if _, ready, err := CertifyManagedCacheWriters(database, binding, nil, func() ([]ParticipantRegistration, error) { return []ParticipantRegistration{initial}, nil }); err != nil || !ready {
		t.Fatal("initial cache writer was not certified", ready, err)
	}
	retired := CoordinationRequest{StorageID: binding.StorageID, Generation: binding.Generation, OperationID: "retired-build", Owner: "native-source-builder", Kind: "producer", Scope: "*", Fingerprint: "retired-body"}
	if created, err := AcquireOperation(database, retired); err != nil || !created {
		t.Fatal("retired producer was not admitted", created, err)
	}
	old := time.Now().Add(-time.Hour)
	if err := database.Model(&model.CleanupOperation{}).Where("operation_id = ?", retired.OperationID).Updates(map[string]interface{}{"created_at": old, "updated_at": old}).Error; err != nil {
		t.Fatal(err)
	}
	replacement := cacheBuilderMember(binding, "current-pod", "current-container")
	if err := RegisterParticipant(database, replacement); err != nil {
		t.Fatal(err)
	}
	status, ready, err := CertifyManagedCacheWriters(database, binding, nil, func() ([]ParticipantRegistration, error) { return []ParticipantRegistration{replacement}, nil })
	if err != nil || !ready || status.Mode != "ready" {
		t.Fatal("retired producer epoch was not reconciled", status, ready, err)
	}
	var operation model.CleanupOperation
	if err := database.Where("operation_id = ?", retired.OperationID).First(&operation).Error; err != nil || operation.State != "finished" || operation.Outcome != "reconciled" {
		t.Fatal("retired producer receipt was not preserved", operation, err)
	}
	current := retired
	current.OperationID = "current-build"
	current.Fingerprint = "current-body"
	if created, err := AcquireOperation(database, current); err != nil || !created {
		t.Fatal("current producer was not admitted", created, err)
	}
	if _, ready, err := CertifyManagedCacheWriters(database, binding, nil, func() ([]ParticipantRegistration, error) { return []ParticipantRegistration{replacement}, nil }); err != nil || ready {
		t.Fatal("current producer epoch was reconciled", ready, err)
	}
	operation = model.CleanupOperation{}
	if err := database.Where("operation_id = ?", current.OperationID).First(&operation).Error; err != nil || operation.State != "active" {
		t.Fatal("current producer protection changed", operation, err)
	}
}

// capability_id: rainbond.cleanup.cache-writer-certification
func TestCacheCertificationRequiresMembershipAndQuiescence(t *testing.T) {
	database, _ := coordinationDB(t)
	database.AutoMigrate(&model.CleanupParticipant{})
	binding, err := ProvisionManagedCacheStorage(database, "owned-cache", "/cache/build")
	if err != nil {
		t.Fatal(err)
	}
	member := cacheBuilderMember(binding, "pod", "container")
	inspect := func() ([]ParticipantRegistration, error) { return []ParticipantRegistration{member}, nil }
	if _, ready, err := CertifyManagedCacheWriters(database, binding, nil, inspect); err != nil || ready {
		t.Fatal("unregistered writer certified", err)
	}
	if err := RegisterParticipant(database, member); err != nil {
		t.Fatal(err)
	}
	observed, ready, err := CertifyManagedCacheWriters(database, binding, nil, inspect)
	if err != nil || !ready || observed.Mode != "ready" {
		t.Fatal("complete evidence not enabled", err, observed)
	}
	writer := CoordinationRequest{StorageID: binding.StorageID, Generation: binding.Generation, OperationID: "build", Owner: "builder", Kind: "producer", Scope: "*", Fingerprint: "body"}
	if _, err := AcquireOperation(database, writer); err != nil {
		t.Fatal(err)
	}
	if _, ready, err := CertifyManagedCacheWriters(database, binding, nil, inspect); err != nil || ready {
		t.Fatal("active writer ignored", err)
	}
	if _, _, err := CertifyManagedCacheWriters(database, binding, &writer, inspect); err == nil {
		t.Fatal("writer excluded as cleanup task")
	}
	if err := FinishOperation(database, writer, true); err != nil {
		t.Fatal(err)
	}
	observed, ready, err = CertifyManagedCacheWriters(database, binding, nil, func() ([]ParticipantRegistration, error) { return nil, errors.New("changed source") })
	if err != nil || ready || observed.Mode != "collecting" {
		t.Fatal("lost evidence retained readiness", err, observed)
	}
}
