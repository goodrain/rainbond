package cleanup

import (
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// capability_id: rainbond.cleanup.upload-writer-coverage
func TestUploadDeletionRequiresEveryCurrentAPIWriterProtocol(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "writers.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.AutoMigrate(&model.CleanupReferenceWriter{}).Error; err != nil {
		t.Fatal(err)
	}
	first := ReferenceWriter{Namespace: "system", PodName: "api-a", PodUID: "a", ContainerName: "rbd-api", ContainerID: "containerd://a", ImageID: "registry@sha256:actual", Role: "api", Protocol: ReferenceWriterProtocol}
	second := first
	second.PodName = "api-b"
	second.PodUID = "b"
	second.ContainerID = "containerd://b"
	for _, w := range []ReferenceWriter{first, second} {
		if err := RegisterReferenceWriter(database, w); err != nil {
			t.Fatal(err)
		}
	}
	if ok, err := UploadWriterCoverageRegistered(database, []ReferenceWriter{first, second}); err != nil || ok {
		t.Fatal("old reference protocol granted upload deletion", err)
	}
	for _, w := range []ReferenceWriter{first, second} {
		w.Protocol = UploadWriterProtocol
		if err := RegisterReferenceWriter(database, w); err != nil {
			t.Fatal(err)
		}
	}
	if ok, err := UploadWriterCoverageRegistered(database, []ReferenceWriter{first, second}); err != nil || !ok {
		t.Fatal("upgraded API coverage rejected", err)
	}
	second.ContainerID = "containerd://restarted"
	if ok, err := UploadWriterCoverageRegistered(database, []ReferenceWriter{first, second}); err != nil || ok {
		t.Fatal("old process proof authorized restarted API", err)
	}
	if ok, err := ReferenceWriterRegistered(database, first); err != nil || !ok {
		t.Fatal("upload protocol replaced registry evidence", err)
	}
	wrong := first
	wrong.Role = "worker"
	wrong.ContainerName = "rbd-worker"
	wrong.Protocol = UploadWriterProtocol
	if err := RegisterReferenceWriter(database, wrong); err == nil {
		t.Fatal("non-API claimed upload protocol")
	}
	if ok, err := UploadWriterCoverageRegistered(database, nil); err != nil || ok {
		t.Fatal("empty API set treated as complete", err)
	}
}
