package cleanup

import (
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

func TestReferenceWriterEvidenceIsIndependentAndImmutable(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "writers.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.LogMode(false)
	// No storage exists yet. Registration must not invent one or mark one ready.
	if err := database.AutoMigrate(&model.CleanupReferenceWriter{}).Error; err != nil {
		t.Fatal(err)
	}
	writer := ReferenceWriter{Namespace: "system", PodName: "api-pod", PodUID: "pod-uid", ContainerName: "rbd-api", ContainerID: "containerd://original", ImageID: "repo@sha256:original", Role: "api", Protocol: ReferenceWriterProtocol}
	if known, err := ReferenceWriterRegistered(database, writer); err != nil || known {
		t.Fatal(known, err)
	}
	if err := RegisterReferenceWriter(database, writer); err != nil {
		t.Fatal(err)
	}
	if err := RegisterReferenceWriter(database, writer); err != nil {
		t.Fatal("identical retry failed", err)
	}
	if known, err := ReferenceWriterRegistered(database, writer); err != nil || !known {
		t.Fatal("missing original proof", known, err)
	}
	changed := writer
	changed.ImageID = "repo@sha256:changed"
	if err := RegisterReferenceWriter(database, changed); err == nil {
		t.Fatal("same runtime identity overwritten")
	}
	changed = writer
	changed.ContainerID = "containerd://restarted"
	if known, err := ReferenceWriterRegistered(database, changed); err != nil || known {
		t.Fatal("restarted container inherited proof", known, err)
	}
	changed = writer
	changed.Protocol = "unknown"
	if err := RegisterReferenceWriter(database, changed); err == nil {
		t.Fatal("unsupported protocol accepted")
	}
	if database.HasTable(&model.CleanupStorage{}) {
		t.Fatal("runtime registration created storage")
	}
}
