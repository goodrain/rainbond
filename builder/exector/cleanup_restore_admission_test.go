package exector

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
)

// capability_id: rainbond.cleanup.restore-producer-transaction
func TestRestoreMetadataRetainsAdmissionAndRollsBackFailure(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "restore.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.DB().SetMaxOpenConns(1)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}, &model.KeyValue{}).Error; err != nil {
		t.Fatal(err)
	}
	binding, err := guard.ProvisionRegistryStorage(database, "owned", "/registry")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&model.CleanupStorage{}).Where("storage_id = ?", binding.StorageID).Update("mode", "ready").Error; err != nil {
		t.Fatal(err)
	}
	admission, err := admitBuild(database, "backup_apps_restore", "owned-restore", []byte("original"))
	if err != nil {
		t.Fatal(err)
	}
	gc := guard.CoordinationRequest{StorageID: binding.StorageID, Generation: binding.Generation, OperationID: "gc", Owner: "test", Kind: "gc", Scope: "*", Fingerprint: "confirmed"}
	if _, err := guard.RequestMaintenance(database, gc); err != nil {
		t.Fatal(err)
	}
	db.SetTestManager(tarImportManager{database: database})
	defer db.SetTestManager(nil)
	restore := &BackupAPPRestore{RestoreID: "owned"}
	restore.setCleanupAdmission(admission)
	if err := restore.restoreMetadata(&AppSnapshot{}); err != nil {
		t.Fatal("restore entry lost admission", err)
	}
	if err := restore.withMetadataWrite(func(tx *gorm.DB) error {
		return guard.WithReferenceMutation(tx, nil, func(bound *gorm.DB) error {
			return bound.Create(&model.KeyValue{K: "restored-reference", V: "original"}).Error
		})
	}); err != nil {
		t.Fatal("admitted restore lost its transaction context", err)
	}
	failure := errors.New("owned failure")
	if err := restore.withMetadataWrite(func(tx *gorm.DB) error {
		if err := tx.Create(&model.KeyValue{K: "partial", V: "not-committed"}).Error; err != nil {
			return err
		}
		return failure
	}); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	var count int
	database.Model(&model.KeyValue{}).Where("k = ?", "partial").Count(&count)
	if count != 0 {
		t.Fatal("partial restore metadata committed")
	}
	if err := restore.saveResult("success", ""); err != nil {
		t.Fatal(err)
	}
	database.Callback().Create().Before("gorm:create").Register("owned-reject-result", func(scope *gorm.Scope) {
		if value, ok := scope.Value.(*model.KeyValue); ok && value.K == "/rainbond/backup_restore/owned" {
			scope.Err(failure)
		}
	})
	if err := restore.saveResult("failed", "ignored"); !errors.Is(err, failure) {
		t.Fatal("lost result reported success", err)
	}
	saved, err := db.GetManager().KeyValueDao().Get("/rainbond/backup_restore/owned")
	if err != nil || saved == nil || !containsRestoreSuccess(saved.V) {
		t.Fatal("previous receipt removed by failed replacement", err)
	}
}
func containsRestoreSuccess(value string) bool { return strings.Contains(value, `"status":"success"`) }

func TestRestoreReceiptFailureDoesNotRollbackAppliedMetadata(t *testing.T) {
	// No data access is initialized: invoking the ordinary cleanup callback would
	// panic instead of silently deleting an already applied restore.
	restore := &BackupAPPRestore{Logger: event.NewLogger("owned", make(chan []byte, 20)), serviceChange: map[string]*Info{"owned": {ServiceID: "owned-service"}}}
	restore.ErrorCallBack(&restoreResultPersistenceError{cause: errors.New("owned failure")})
}
