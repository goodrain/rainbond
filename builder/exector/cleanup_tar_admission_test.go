package exector

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	pb "github.com/goodrain/rainbond/mq/api/grpc/pb"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	"github.com/goodrain/rainbond/db/model"
	mysqldao "github.com/goodrain/rainbond/db/mysql/dao"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
)

// capability_id: rainbond.cleanup.tar-image-admission
func TestTarImportAdmissionCoversWorkAndResultPersistence(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "tar.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.DB().SetMaxOpenConns(1)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}, &model.KeyValue{}).Error; err != nil {
		t.Fatal(err)
	}
	binding, err := guard.ProvisionRegistryStorage(database, "owned-volume", "/registry")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&model.CleanupStorage{}).Where("storage_id = ?", binding.StorageID).Update("mode", "ready").Error; err != nil {
		t.Fatal(err)
	}
	gc := guard.CoordinationRequest{StorageID: binding.StorageID, Generation: binding.Generation, Owner: "gc", OperationID: "gc", Kind: "gc", Scope: "*", Fingerprint: "confirmed"}
	calls := 0
	run := func(a *nativeBuildAdmission) bool {
		calls++
		if _, err := guard.RequestMaintenance(database, gc); err != nil {
			t.Fatal(err)
		}
		if err := guard.EnterMaintenance(database, gc); !errors.Is(err, guard.ErrCoordinationBusy) {
			t.Fatal("GC entered during import", err)
		}
		if err := a.saveTarImageResult("owned-load", `{"status":"success"}`); err != nil {
			t.Fatal("drain blocked already admitted result", err)
		}
		return true
	}
	if err := runTarImageWithAdmission(database, "owned-load", []byte("original"), run); err != nil {
		t.Fatal(err)
	}
	if err := guard.EnterMaintenance(database, gc); err != nil {
		t.Fatal(err)
	}
	if err := runTarImageWithAdmission(database, "owned-load", []byte("original"), run); err == nil || calls != 1 {
		t.Fatal("import replayed", err)
	}
	if err := runTarImageWithAdmission(database, "new-load", []byte("new"), run); err == nil || calls != 1 {
		t.Fatal("maintenance bypassed", err)
	}
	var result model.KeyValue
	if err := database.Where("k = ?", "/rainbond/tarload/owned-load").First(&result).Error; err != nil {
		t.Fatal(err)
	}
}

func TestTarImportUnknownOutcomeRetainsProtection(t *testing.T) {
	for _, panics := range []bool{false, true} {
		database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "tar.db"))
		if err != nil {
			t.Fatal(err)
		}
		defer database.Close()
		if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := guard.ProvisionRegistryStorage(database, "owned-volume", "/registry"); err != nil {
			t.Fatal(err)
		}
		func() {
			defer func() {
				if value := recover(); panics && value == nil {
					t.Error("test panic swallowed")
				}
			}()
			err := runTarImageWithAdmission(database, "failed-load", []byte("original"), func(*nativeBuildAdmission) bool {
				if panics {
					panic("interrupted")
				}
				return false
			})
			if err != nil {
				t.Fatal(err)
			}
		}()
		if err := runTarImageWithAdmission(database, "failed-load", []byte("original"), func(*nativeBuildAdmission) bool { t.Fatal("unknown import replayed"); return true }); !errors.Is(err, guard.ErrCoordinationUncertain) {
			t.Fatal(err)
		}
	}
}

func TestTarImportInvalidIdentityStopsBeforeFilesystemOrDatabase(t *testing.T) {
	for _, id := range []string{"", "../other", "valid/../../other", "not-a-uuid"} {
		raw, _ := json.Marshal(TarImageLoadTaskBody{LoadID: id})
		// No DB, event manager or image client is initialized in this test.
		new(exectorManager).loadTarImage(&pb.TaskMessage{TaskBody: raw})
	}
}

type tarImportManager struct {
	db.Manager
	database *gorm.DB
}

func (m tarImportManager) DB() *gorm.DB { return m.database }
func (m tarImportManager) KeyValueDao() dbdao.KeyValueDao {
	return &mysqldao.KeyValueImpl{DB: m.database}
}

// capability_id: rainbond.cleanup.tar-image-entry
func TestTarImportEntryReportsMaintenanceWithoutStartingNativeWork(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "entry.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
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
	request := guard.CoordinationRequest{StorageID: binding.StorageID, Generation: binding.Generation, OperationID: "gc", Owner: "test", Kind: "gc", Scope: "*", Fingerprint: "confirmed"}
	if _, err := guard.RequestMaintenance(database, request); err != nil {
		t.Fatal(err)
	}
	db.SetTestManager(tarImportManager{database: database})
	defer db.SetTestManager(nil)
	id := "00000000-0000-4000-8000-000000000001"
	raw, _ := json.Marshal(TarImageLoadTaskBody{LoadID: id, TarFilePath: "owned.tar"})
	// Native image client, storage downloader and event logger are intentionally
	// absent: reaching any of them would fail this test.
	new(exectorManager).loadTarImage(&pb.TaskMessage{TaskBody: raw})
	result, err := db.GetManager().KeyValueDao().Get("/rainbond/tarload/" + id)
	if err != nil || result == nil {
		t.Fatal("missing terminal rejection", err)
	}
	var decoded apimodel.TarLoadResult
	if json.Unmarshal([]byte(result.V), &decoded) != nil || decoded.Status != "failure" || len(decoded.TargetImages) != 0 {
		t.Fatal("incorrect rejection receipt")
	}
	new(exectorManager).loadTarImage(&pb.TaskMessage{TaskBody: raw})
	again, _ := db.GetManager().KeyValueDao().Get("/rainbond/tarload/" + id)
	if again.V != result.V {
		t.Fatal("redelivery changed existing result")
	}
}

// capability_id: rainbond.cleanup.tar-image-redelivery
func TestTarImportRedeliveryDoesNotPublishFailureOverActiveWork(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "active.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}, &model.KeyValue{}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := guard.ProvisionRegistryStorage(database, "owned", "/registry"); err != nil {
		t.Fatal(err)
	}
	db.SetTestManager(tarImportManager{database: database})
	defer db.SetTestManager(nil)
	id := "00000000-0000-4000-8000-000000000002"
	raw, _ := json.Marshal(TarImageLoadTaskBody{LoadID: id, TarFilePath: "owned.tar"})
	admission, err := admitBuild(database, "tar-image", id, raw)
	if err != nil {
		t.Fatal(err)
	}
	new(exectorManager).loadTarImage(&pb.TaskMessage{TaskBody: raw})
	result, err := db.GetManager().KeyValueDao().Get("/rainbond/tarload/" + id)
	if err != nil || result != nil {
		t.Fatal("duplicate delivery replaced the running import with failure", err)
	}
	if err := admission.saveTarImageResult(id, `{"status":"success"}`); err != nil {
		t.Fatal(err)
	}
}
