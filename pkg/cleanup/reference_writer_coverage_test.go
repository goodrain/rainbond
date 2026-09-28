package cleanup

import (
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

func TestReferenceWriterCoverageRequiresEveryCurrentRole(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "coverage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.LogMode(false)
	if err := database.AutoMigrate(&model.CleanupReferenceWriter{}).Error; err != nil {
		t.Fatal(err)
	}
	writers := []ReferenceWriter{}
	for role, name := range map[string]string{"api": "rbd-api", "worker": "rbd-worker", "builder": "rbd-chaos", "console": "rbd-app-ui"} {
		writer := ReferenceWriter{Namespace: "system", PodName: name, PodUID: "pod-" + role, ContainerName: name, ContainerID: "containerd://" + role, ImageID: "image-" + role, Role: role, Protocol: ReferenceWriterProtocol}
		writers = append(writers, writer)
	}
	for _, writer := range writers {
		if complete, err := ReferenceWriterCoverageRegistered(database, writers); err != nil || complete {
			t.Fatal("unregistered role certified", complete, err)
		}
		if err := RegisterReferenceWriter(database, writer); err != nil {
			t.Fatal(err)
		}
	}
	if complete, err := ReferenceWriterCoverageRegistered(database, writers); err != nil || !complete {
		t.Fatal(complete, err)
	}
	changed := append([]ReferenceWriter{}, writers...)
	changed[0].ContainerID = "containerd://replacement"
	if complete, err := ReferenceWriterCoverageRegistered(database, changed); err != nil || complete {
		t.Fatal("replacement inherited coverage", complete, err)
	}
	if complete, err := ReferenceWriterCoverageRegistered(database, writers[:3]); err != nil || complete {
		t.Fatal("missing role certified", complete, err)
	}
	if complete, err := ReferenceWriterCoverageRegistered(database, append(writers, writers[0])); err == nil && complete {
		t.Fatal("duplicate instance certified")
	}
}
