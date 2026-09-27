package exector

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

// capability_id: rainbond.cleanup.native-image-build-admission
func TestImageBuildAdmissionBlocksGCAndNeverReplays(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "build.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.LogMode(false)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}).Error; err != nil {
		t.Fatal(err)
	}
	binding, err := guard.ProvisionRegistryStorage(database, "owned-volume", "/registry")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&model.CleanupStorage{}).Where("storage_id = ?", binding.StorageID).Update("mode", "ready").Error; err != nil {
		t.Fatal(err)
	}
	admission, err := admitBuild(database, "image", "owned-task", []byte("owned-input"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admitBuild(database, "image", "owned-task", []byte("owned-input")); err == nil {
		t.Fatal("duplicate native build admitted")
	}
	gc := guard.CoordinationRequest{StorageID: binding.StorageID, Generation: binding.Generation, OperationID: "gc", Owner: "executor", Kind: "gc", Scope: "*", Fingerprint: "confirmation"}
	if _, err := guard.RequestMaintenance(database, gc); err != nil {
		t.Fatal(err)
	}
	if err := guard.EnterMaintenance(database, gc); !errors.Is(err, guard.ErrCoordinationBusy) {
		t.Fatal("GC entered during native build", err)
	}
	if _, err := admitBuild(database, "image", "new-task", []byte("new-input")); err == nil {
		t.Fatal("new build bypassed drain")
	}
	if err := admission.finish(true); err != nil {
		t.Fatal(err)
	}
	if err := guard.EnterMaintenance(database, gc); err != nil {
		t.Fatal(err)
	}
}

func TestFailedImageBuildKeepsAdmissionAcrossRetry(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "failed.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.LogMode(false)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}).Error; err != nil {
		t.Fatal(err)
	}
	binding, err := guard.ProvisionRegistryStorage(database, "owned-volume", "/registry")
	if err != nil {
		t.Fatal(err)
	}
	admission, err := admitBuild(database, "image", "failed-task", []byte("owned-input"))
	if err != nil {
		t.Fatal(err)
	}
	if err := admission.finish(false); err != nil {
		t.Fatal(err)
	}
	if _, err := admitBuild(database, "image", "failed-task", []byte("owned-input")); !errors.Is(err, guard.ErrCoordinationUncertain) {
		t.Fatal("uncertain native work replayed", err)
	}
	var stored model.CleanupOperation
	if err := database.Where("storage_id = ?", binding.StorageID).First(&stored).Error; err != nil || stored.State != "uncertain" {
		t.Fatal("retry released uncertain producer", err)
	}
}

func TestSourceBuildAdmissionUsesSeparateImmutableIdentity(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "source.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.LogMode(false)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := guard.ProvisionRegistryStorage(database, "owned-volume", "/registry"); err != nil {
		t.Fatal(err)
	}
	if _, err := admitBuild(database, "source", "same-task", []byte("same-input")); err != nil {
		t.Fatal(err)
	}
	if _, err := admitBuild(database, "image", "same-task", []byte("same-input")); err != nil {
		t.Fatal("task types collided", err)
	}
	if _, err := admitBuild(database, "image-share", "same-task", []byte("same-input")); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"plugin-image", "plugin-dockerfile", "share-plugin"} {
		if _, err := admitBuild(database, kind, "same-task", []byte("same-input")); err != nil {
			t.Fatal(kind, err)
		}
	}
	if _, err := admitBuild(database, "vm", "same-task", []byte("same-input")); err != nil {
		t.Fatal("VM task admission failed", err)
	}
	if _, err := admitBuild(database, "source", "same-task", []byte("changed-input")); !errors.Is(err, guard.ErrCoordinationChanged) {
		t.Fatal("source task payload changed", err)
	}
}
