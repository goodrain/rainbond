package cleanup

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/goodrain/rainbond/db/model"
)

// capability_id: rainbond.cleanup.cache-writer-certification
func TestCacheCertificationRequiresMembershipAndQuiescence(t *testing.T) {
	database, _ := coordinationDB(t)
	database.AutoMigrate(&model.CleanupParticipant{})
	binding, err := ProvisionManagedCacheStorage(database, "owned-cache", "/cache/build")
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, _ := binding.Fingerprint()
	sum := sha256.Sum256([]byte("pod\x00container"))
	member := ParticipantRegistration{StorageID: binding.StorageID, Generation: binding.Generation, Role: "cache-builder", Owner: "cache-builder:" + hex.EncodeToString(sum[:]), PodUID: "pod", ContainerID: "container", ImageID: "observed-image", BindingFingerprint: fingerprint}
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
