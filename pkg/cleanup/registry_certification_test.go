package cleanup

import (
	"testing"
	"time"

	"github.com/goodrain/rainbond/db/model"
)

func TestRegistryCertificationReconcilesOnlyRetiredProducerEpoch(t *testing.T) {
	database, _ := coordinationDB(t)
	if err := database.AutoMigrate(&model.CleanupParticipant{}, &model.CleanupReferenceWriter{}, &model.VersionInfo{}, &model.TenantPluginBuildVersion{}, &model.K8sResource{}, &model.KeyValue{}).Error; err != nil {
		t.Fatal(err)
	}
	binding, err := ProvisionRegistryStorage(database, "retired-volume", "/var/lib/registry")
	if err != nil {
		t.Fatal(err)
	}
	retired := CoordinationRequest{StorageID: binding.StorageID, Generation: binding.Generation, OperationID: "retired-build", Owner: "native-image-builder", Kind: "producer", Scope: "*", Fingerprint: "retired-body"}
	if created, err := AcquireOperation(database, retired); err != nil || !created {
		t.Fatal("retired producer was not admitted", created, err)
	}
	old := time.Now().Add(-time.Hour)
	if err := database.Model(&model.CleanupOperation{}).Where("operation_id = ?", retired.OperationID).Updates(map[string]interface{}{"created_at": old, "updated_at": old}).Error; err != nil {
		t.Fatal(err)
	}
	fingerprint, _ := binding.Fingerprint()
	ingress := ParticipantRegistration{StorageID: binding.StorageID, Generation: binding.Generation, Owner: "registry-coordinator:current", Role: "registry-ingress", PodUID: "current-registry", ContainerID: "containerd://current-registry", ImageID: "registry-image", BindingFingerprint: fingerprint}
	if err := RegisterParticipant(database, ingress); err != nil {
		t.Fatal(err)
	}
	writers := []ReferenceWriter{}
	for role, name := range map[string]string{"api": "rbd-api", "worker": "rbd-worker", "builder": "rbd-chaos", "console": "rbd-app-ui"} {
		writer := ReferenceWriter{Namespace: "system", PodName: name, PodUID: "current-" + role, ContainerName: name, ContainerID: "containerd://current-" + role, ImageID: "image-" + role, Role: role, Protocol: ReferenceWriterProtocol}
		if err := RegisterReferenceWriter(database, writer); err != nil {
			t.Fatal(err)
		}
		writers = append(writers, writer)
	}
	complete := false
	inspect := func([]ParticipantRegistration) (RegistryCoverage, error) {
		return RegistryCoverage{Ingress: []ParticipantRegistration{ingress}, Writers: writers, References: RegionReferenceInventory{Complete: complete}}, nil
	}
	if _, ready, err := CertifyRegistry(database, binding, nil, inspect); err != nil || ready {
		t.Fatal("incomplete references reconciled a retired producer", ready, err)
	}
	var operation model.CleanupOperation
	if err := database.Where("operation_id = ?", retired.OperationID).First(&operation).Error; err != nil || operation.State != "active" {
		t.Fatal("incomplete references changed producer protection", operation, err)
	}
	complete = true
	status, ready, err := CertifyRegistry(database, binding, nil, inspect)
	if err != nil || !ready || status.Mode != "ready" {
		t.Fatal("retired producer epoch was not reconciled", status, ready, err)
	}
	operation = model.CleanupOperation{}
	if err := database.Where("operation_id = ?", retired.OperationID).First(&operation).Error; err != nil || operation.State != "finished" || operation.Outcome != "reconciled" {
		t.Fatal("retired producer receipt was not preserved", operation, err)
	}
	current := retired
	current.OperationID = "current-build"
	current.Fingerprint = "current-body"
	if created, err := AcquireOperation(database, current); err != nil || !created {
		t.Fatal("current producer was not admitted", created, err)
	}
	if _, ready, err := CertifyRegistry(database, binding, nil, inspect); err != nil || ready {
		t.Fatal("current producer epoch was reconciled", ready, err)
	}
	operation = model.CleanupOperation{}
	if err := database.Where("operation_id = ?", current.OperationID).First(&operation).Error; err != nil || operation.State != "active" {
		t.Fatal("current producer protection changed", operation, err)
	}
}

func TestRetiredProducerEpochRecognizesOnlyPlatformOwners(t *testing.T) {
	for _, owner := range []string{
		"registry-coordinator:current", "native-source-builder", "native-plugin-dockerfile-builder",
		"api-workload", "helm-upgrade", "api-package-complete",
	} {
		if !reconcilableProducerOwner(owner) {
			t.Fatal("platform producer owner rejected", owner)
		}
	}
	for _, owner := range []string{"", "registry-coordinator:", "native-unknown-builder", "builder", "api-package-unknown", "helm-delete"} {
		if reconcilableProducerOwner(owner) {
			t.Fatal("unknown producer owner accepted", owner)
		}
	}
}

// capability_id: rainbond.cleanup.registry-certification
func TestRegistryCertificationRequiresCompleteEvidence(t *testing.T) {
	database, _ := coordinationDB(t)
	if err := database.AutoMigrate(&model.CleanupParticipant{}, &model.CleanupReferenceWriter{}, &model.VersionInfo{}, &model.TenantPluginBuildVersion{}, &model.K8sResource{}, &model.KeyValue{}).Error; err != nil {
		t.Fatal(err)
	}
	binding, err := ProvisionRegistryStorage(database, "owned-volume", "/var/lib/registry")
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, _ := binding.Fingerprint()
	ingress := ParticipantRegistration{StorageID: binding.StorageID, Generation: binding.Generation, Owner: "proxy:owned", Role: "registry-ingress", PodUID: "proxy-pod", ContainerID: "containerd://proxy", ImageID: "proxy-image", BindingFingerprint: fingerprint}
	writers := []ReferenceWriter{}
	for role, name := range map[string]string{"api": "rbd-api", "worker": "rbd-worker", "builder": "rbd-chaos", "console": "rbd-app-ui"} {
		writers = append(writers, ReferenceWriter{Namespace: "system", PodName: name, PodUID: "pod-" + role, ContainerName: name, ContainerID: "containerd://" + role, ImageID: "image-" + role, Role: role, Protocol: ReferenceWriterProtocol})
	}
	complete := true
	inspect := func(records []ParticipantRegistration) (RegistryCoverage, error) {
		return RegistryCoverage{Ingress: []ParticipantRegistration{ingress}, Writers: writers, References: RegionReferenceInventory{Complete: complete}}, nil
	}
	if _, ready, err := CertifyRegistry(database, binding, nil, inspect); err != nil || ready {
		t.Fatal("unregistered ingress accepted", ready, err)
	}
	if err := RegisterParticipant(database, ingress); err != nil {
		t.Fatal(err)
	}
	if _, ready, err := CertifyRegistry(database, binding, nil, inspect); err != nil || ready {
		t.Fatal("unregistered writers accepted", ready, err)
	}
	for _, writer := range writers {
		if err := RegisterReferenceWriter(database, writer); err != nil {
			t.Fatal(err)
		}
	}
	status, ready, err := CertifyRegistry(database, binding, nil, inspect)
	if err != nil || !ready || status.Mode != "ready" {
		t.Fatal("complete evidence rejected", status, ready, err)
	}
	producer := CoordinationRequest{StorageID: binding.StorageID, Generation: binding.Generation, OperationID: "build", Owner: "builder", Kind: "producer", Scope: "*", Fingerprint: "body"}
	if _, err := AcquireOperation(database, producer); err != nil {
		t.Fatal(err)
	}
	if _, ready, err := CertifyRegistry(database, binding, nil, inspect); err != nil || ready {
		t.Fatal("active producer ignored", ready, err)
	}
	if _, _, err := CertifyRegistry(database, binding, &producer, inspect); err == nil {
		t.Fatal("producer exempted as deletion")
	}
	if err := FinishOperation(database, producer, true); err != nil {
		t.Fatal(err)
	}
	complete = false
	status, ready, err = CertifyRegistry(database, binding, nil, inspect)
	if err != nil || ready || status.Mode != "collecting" {
		t.Fatal("lost reference coverage retained readiness", status, ready, err)
	}
	complete = true
	originalID := writers[0].ContainerID
	writers[0].ContainerID = "containerd://replacement"
	if _, ready, err := CertifyRegistry(database, binding, nil, inspect); err != nil || ready {
		t.Fatal("replaced writer inherited coverage", ready, err)
	}

	writers[0].ContainerID = originalID
	if _, ready, err := CertifyRegistry(database, binding, nil, inspect); err != nil || !ready {
		t.Fatal(ready, err)
	}
	deletion := CoordinationRequest{StorageID: binding.StorageID, Generation: binding.Generation, OperationID: "selected-delete", Owner: "plugin", Kind: "delete", Scope: "app", Target: "sha256:owned", Fingerprint: "selected"}
	if _, err := AcquireOperation(database, deletion); err != nil {
		t.Fatal(err)
	}
	if _, ready, err := CertifyRegistry(database, binding, nil, inspect); err != nil || ready {
		t.Fatal("other active deletion ignored", ready, err)
	}
	if _, ready, err := CertifyRegistry(database, binding, &deletion, inspect); err != nil || !ready {
		t.Fatal("original deletion cannot revalidate", ready, err)
	}
	foreign := deletion
	foreign.Owner = "other"
	if _, _, err := CertifyRegistry(database, binding, &foreign, inspect); err == nil {
		t.Fatal("foreign original operation accepted")
	}
	if err := FinishOperation(database, deletion, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := CertifyRegistry(database, binding, &deletion, inspect); err == nil {
		t.Fatal("uncertain deletion treated as active")
	}
	if _, ready, err := CertifyRegistry(database, binding, nil, inspect); err != nil || ready {
		t.Fatal("uncertain operation ignored", ready, err)
	}
}
