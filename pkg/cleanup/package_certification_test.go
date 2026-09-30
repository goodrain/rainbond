package cleanup

import (
	"encoding/hex"
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

func packageCertificationFixture(t *testing.T) (*gorm.DB, StorageRegistration, []ReferenceWriter) {
	t.Helper()
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "package-certification.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}, &model.CleanupReferenceWriter{}, &model.PackageUploadUse{}).Error; err != nil {
		t.Fatal(err)
	}
	binding, err := ProvisionManagedPackageStorage(database, "package-volume", "upload_events")
	if err != nil {
		t.Fatal(err)
	}
	writers := []ReferenceWriter{}
	for _, role := range []string{"api", "worker", "builder", "console"} {
		name := map[string]string{"api": "rbd-api", "worker": "rbd-worker", "builder": "rbd-chaos", "console": "rbd-app-ui"}[role]
		writer := ReferenceWriter{Namespace: "rbd-system", PodName: name + "-one", PodUID: "pod-" + role, ContainerName: name, ContainerID: "containerd://" + role, ImageID: "image-" + role, Role: role, Protocol: ReferenceWriterProtocol}
		if err := RegisterReferenceWriter(database, writer); err != nil {
			t.Fatal(err)
		}
		writers = append(writers, writer)
	}
	upload := writers[0]
	upload.Protocol = UploadWriterProtocol
	if err := RegisterReferenceWriter(database, upload); err != nil {
		t.Fatal(err)
	}
	return database, binding, writers
}

// capability_id: rainbond.cleanup.package-writer-readiness
func TestManagedPackageReadinessRequiresCurrentWritersAndNoUnfinishedUse(t *testing.T) {
	database, binding, writers := packageCertificationFixture(t)
	inspect := func() ([]ReferenceWriter, error) { return writers, nil }
	observed, ready, err := CertifyManagedPackageWriters(database, binding, nil, inspect)
	if err != nil || !ready || observed.Mode != "ready" {
		t.Fatal("complete package writers did not enable readiness", observed, ready, err)
	}

	use := model.PackageUploadUse{OperationID: "upload-use", SessionID: "session", EventID: "event", Action: "chunk", State: "uncertain"}
	if err := database.Create(&use).Error; err != nil {
		t.Fatal(err)
	}
	observed, ready, err = CertifyManagedPackageWriters(database, binding, nil, inspect)
	if err != nil || ready || observed.Mode != "collecting" {
		t.Fatal("unfinished upload use retained package readiness", observed, ready, err)
	}
	if err := database.Model(&model.PackageUploadUse{}).Where("operation_id = ?", use.OperationID).Update("state", "finished").Error; err != nil {
		t.Fatal(err)
	}
	missingUploadProof := append([]ReferenceWriter(nil), writers...)
	missingUploadProof[0].ContainerID = "containerd://replacement"
	if _, ready, err = CertifyManagedPackageWriters(database, binding, nil, func() ([]ReferenceWriter, error) { return missingUploadProof, nil }); err != nil || ready {
		t.Fatal("unregistered replacement writer became ready", ready, err)
	}
}

func TestManagedPackageReadinessExemptsOnlyOriginalDeletion(t *testing.T) {
	database, binding, writers := packageCertificationFixture(t)
	target := hex.EncodeToString(make([]byte, 32))
	request := CoordinationRequest{StorageID: binding.StorageID, Generation: binding.Generation, Owner: "disk-cleanup-task", OperationID: "selected-delete", Kind: "delete", Scope: "event", Fingerprint: "scan", Target: target}
	if _, _, err := CertifyManagedPackageWriters(database, binding, nil, func() ([]ReferenceWriter, error) { return writers, nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireOperation(database, request); err != nil {
		t.Fatal(err)
	}
	if _, ready, err := CertifyManagedPackageWriters(database, binding, &request, func() ([]ReferenceWriter, error) { return writers, nil }); err != nil || !ready {
		t.Fatal("original deletion was not revalidated", ready, err)
	}
	other := request
	other.OperationID = "other-delete"
	if _, ready, err := CertifyManagedPackageWriters(database, binding, &other, func() ([]ReferenceWriter, error) { return writers, nil }); err != nil || ready {
		t.Fatal("unrecorded deletion was exempted", ready, err)
	}
}
