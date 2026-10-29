package cleanup

import (
	"errors"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"path/filepath"
	"testing"
)

// capability_id: rainbond.cleanup.sqlite-version-store
func TestVersionStoreSupportsSQLite(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.TenantServices{}, &model.VersionInfo{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Exec("CREATE TABLE tenant_services_event (service_id TEXT, opt_type TEXT, final_status TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.TenantServices{TenantID: "tenant", ServiceID: "service", DeployVersion: "current"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.VersionInfo{ServiceID: "service", BuildVersion: "old", ImageName: "registry/app:old", DeliveredType: "image", FinalStatus: "success", EventID: "build"}).Error; err != nil {
		t.Fatal(err)
	}
	expected := VersionExpectation{ServiceID: "service", Version: "old", Image: "registry/app:old", EventID: "build", CurrentVersion: "current"}
	t.Run("inspect", func(t *testing.T) {
		result, err := InspectVersions(database.Begin, "service")
		if err != nil {
			t.Fatal(err)
		}
		if result.CurrentVersion != "current" {
			t.Fatal(result)
		}
	})
	t.Run("retirement inspection", func(t *testing.T) {
		result, err := InspectVersionRetirement(database.Begin, "tenant", expected)
		if err != nil || result != "present" {
			t.Fatal(result, err)
		}
	})
	t.Run("select and protect current", func(t *testing.T) {
		previous, err := SelectRollbackVersion(database.Begin, "tenant", "service", "old", "activate", "current")
		if err != nil || previous != "current" {
			t.Fatal(previous, err)
		}
		result, err := InspectVersions(database.Begin, "service")
		if err != nil || result.CurrentVersion != "old" || result.Checkpoints["build"] != "activate" {
			t.Fatal(result, err)
		}
		expected.ActivationRevision = "activate"
		expected.CurrentVersion = "old"
		_, err = RetireVersion(database.Begin, "tenant", expected)
		if !errors.Is(err, ErrVersionProtected) {
			t.Fatalf("current version must remain protected: %v", err)
		}
		_, err = SelectRollbackVersion(database.Begin, "tenant", "service", "", "restore", "old")
		if err != nil {
			t.Fatal(err)
		}
		if err := database.Model(&model.TenantServices{}).Where("service_id = ?", "service").Update("deploy_version", "current").Error; err != nil {
			t.Fatal(err)
		}
		expected.CurrentVersion = "current"
	})
	t.Run("retire", func(t *testing.T) {
		result, err := RetireVersion(database.Begin, "tenant", expected)
		if err != nil || !result.RecordRetired {
			t.Fatal(result, err)
		}
		state, err := InspectVersionRetirement(database.Begin, "tenant", expected)
		if err != nil || state != "absent" {
			t.Fatal(state, err)
		}
	})
}

func TestSQLiteVersionStoreSerializesTransactions(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "locks.db")+"?_busy_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.AutoMigrate(&model.TenantServices{}).Error; err != nil {
		t.Fatal(err)
	}
	first := database.Begin()
	defer first.Rollback()
	if err := lockSQLiteVersionStore(first); err != nil {
		t.Fatal(err)
	}
	second := database.Begin()
	defer second.Rollback()
	if err := lockSQLiteVersionStore(second); err == nil {
		t.Fatal("second transaction acquired write lock before first finished")
	}
	if err := first.Rollback().Error; err != nil {
		t.Fatal(err)
	}
	if err := lockSQLiteVersionStore(second); err != nil {
		t.Fatalf("lock not released after rollback: %v", err)
	}
}
