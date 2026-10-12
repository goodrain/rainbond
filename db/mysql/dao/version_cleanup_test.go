package dao

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/goodrain/rainbond/db/model"
	cleanupguard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

func TestVersionWritesWithoutEnrolledCleanupStorage(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "versions.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.LogMode(false)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}, &model.KeyValue{}, &model.VersionInfo{}).Error; err != nil {
		t.Fatal(err)
	}
	versions := &VersionInfoDaoImpl{DB: database}
	record := &model.VersionInfo{ServiceID: "service", BuildVersion: "new", EventID: "event", ImageName: "goodrain.me/team/component:v1"}
	if err := versions.AddModel(record); err != nil {
		t.Fatal("ordinary build cannot create its version", err)
	}
	receipt := model.KeyValue{K: "/rainbond/tarload/owned", V: `{"status":"success","target_images":{"source":"goodrain.me/team/component:v1"}}`}
	if err := database.Create(&receipt).Error; err != nil {
		t.Fatal(err)
	}
	record.FinalStatus = "success"
	if err := versions.UpdateModel(record); err != nil {
		t.Fatal("ordinary build cannot finish its version", err)
	}
	var transferred model.KeyValue
	if err := database.Where("k = ?", receipt.K).First(&transferred).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(transferred.V, "cleanup_version_handoff") {
		t.Fatal("successful version did not take ownership of import references")
	}
	var stored model.VersionInfo
	if err := database.First(&stored, record.ID).Error; err != nil || stored.FinalStatus != "success" || stored.ActivationRevision == "" {
		t.Fatal("version was not durably committed", err)
	}
	if err := database.Delete(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if err := versions.UpdateModel(record); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("late callback recreated retired version", err)
	}
}

// capability_id: rainbond.cleanup.version-reference-coordination
func TestVersionCreationCannotRaceManifestDeletion(t *testing.T) {
	for _, update := range []bool{false, true} {
		t.Run(fmt.Sprintf("update=%v", update), func(t *testing.T) {
			raw, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			database, err := gorm.Open("mysql", raw)
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			database.LogMode(false)
			mock.ExpectBegin()
			mock.ExpectExec("UPDATE .*cleanup_storage").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery("SELECT .*cleanup_storage.*FOR UPDATE$").WillReturnRows(sqlmock.NewRows([]string{"storage_id", "generation", "mode"}).AddRow("hub", "one", "ready"))
			mock.ExpectQuery("SELECT .*cleanup_operations.*FOR UPDATE$").WillReturnRows(sqlmock.NewRows([]string{"storage_id", "generation", "kind", "scope", "state"}).AddRow("hub", "one", "delete", "team/component", "executing"))
			mock.ExpectRollback()
			record := &model.VersionInfo{Model: model.Model{ID: 7}, ServiceID: "service", BuildVersion: "new", ImageName: "goodrain.me/team/component:latest"}
			if update {
				err = (&VersionInfoDaoImpl{DB: database}).UpdateModel(record)
			} else {
				err = (&VersionInfoDaoImpl{DB: database}).AddModel(record)
			}
			if !errors.Is(err, cleanupguard.ErrCoordinationBusy) {
				t.Fatal("version was not rejected by deletion coordination", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// capability_id: rainbond.cleanup.version-update-no-resurrection
func TestVersionUpdateNeverRecreatesRetiredRecordsOrResetsActivation(t *testing.T) {
	for _, scenario := range []struct {
		name                string
		affected, remaining int64
	}{{"changed", 1, 1}, {"unchanged", 0, 1}, {"retired", 0, 0}} {
		t.Run(scenario.name, func(t *testing.T) {
			raw, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expected, actual string) error {
				if strings.HasPrefix(actual, "UPDATE") && strings.Contains(actual, "tenant_service_version") {
					if strings.Contains(actual, "activation_revision") {
						return fmt.Errorf("stale callback overwrites activation checkpoint")
					}
					if !strings.Contains(actual, "`cmd`") {
						return fmt.Errorf("zero-value command update was lost")
					}
				}
				if matched, err := regexp.MatchString(expected, actual); err != nil || !matched {
					return fmt.Errorf("unexpected SQL: %s", actual)
				}
				return nil
			})))
			if err != nil {
				t.Fatal(err)
			}
			database, err := gorm.Open("mysql", raw)
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			database.LogMode(false)
			mock.ExpectBegin()
			mock.ExpectExec("UPDATE .*cleanup_storage").WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectQuery("SELECT .*cleanup_storage.*FOR UPDATE$").WillReturnRows(sqlmock.NewRows([]string{"storage_id", "generation", "mode"}))
			mock.ExpectExec("UPDATE .*tenant_service_version.* WHERE .*ID.*service_id.*build_version.*event_id").WillReturnResult(sqlmock.NewResult(0, scenario.affected))
			if scenario.affected == 0 {
				mock.ExpectQuery("SELECT count.*tenant_service_version").WithArgs(uint(7), "service", "old", "event").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(scenario.remaining))
			}
			if scenario.remaining == 0 {
				mock.ExpectRollback()
			} else {
				mock.ExpectCommit()
			}
			record := &model.VersionInfo{Model: model.Model{ID: 7}, ServiceID: "service", BuildVersion: "old", EventID: "event", ActivationRevision: "stale", Cmd: ""}
			err = (&VersionInfoDaoImpl{DB: database}).UpdateModel(record)
			if scenario.remaining == 0 && err != gorm.ErrRecordNotFound {
				t.Fatalf("retired record not rejected: %v", err)
			}
			if scenario.remaining > 0 && err != nil {
				t.Fatal(err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAdmittedBuildVersionPersistsDuringGCDrain(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "admitted.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.LogMode(false)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}, &model.KeyValue{}, &model.VersionInfo{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.CleanupStorage{StorageID: "owned", Generation: "one", Mode: "ready"}).Error; err != nil {
		t.Fatal(err)
	}
	record := &model.VersionInfo{ServiceID: "service", BuildVersion: "new", EventID: "event", ImageName: "goodrain.me/team/component:v1"}
	if err := (&VersionInfoDaoImpl{DB: database}).AddModel(record); err != nil {
		t.Fatal(err)
	}
	producer := cleanupguard.CoordinationRequest{StorageID: "owned", Generation: "one", OperationID: "build", Owner: "builder", Kind: "producer", Scope: "team/component", Fingerprint: "original-build"}
	if _, err := cleanupguard.AcquireOperation(database, producer); err != nil {
		t.Fatal(err)
	}
	gc := cleanupguard.CoordinationRequest{StorageID: "owned", Generation: "one", OperationID: "gc", Owner: "cleanup", Kind: "gc", Scope: "*", Fingerprint: "confirmed-gc"}
	if _, err := cleanupguard.RequestMaintenance(database, gc); err != nil {
		t.Fatal(err)
	}
	record.FinalStatus = "success"
	var escaped *gorm.DB
	if err := cleanupguard.WithProducerReferenceMutation(database, []cleanupguard.CoordinationRequest{producer}, []string{"team/component"}, func(tx *gorm.DB) error {
		escaped = tx
		return (&VersionInfoDaoImpl{DB: tx}).UpdateModel(record)
	}); err != nil {
		t.Fatal("real version DAO rejected admitted completion", err)
	}
	var saved model.VersionInfo
	if err := database.First(&saved, record.ID).Error; err != nil || saved.FinalStatus != "success" {
		t.Fatal("version result not committed", err)
	}
	if err := (&VersionInfoDaoImpl{DB: database}).UpdateModel(record); !errors.Is(err, cleanupguard.ErrCoordinationBusy) {
		t.Fatal("admission leaked onto shared connection", err)
	}
	if err := (&VersionInfoDaoImpl{DB: escaped}).UpdateModel(record); err == nil {
		t.Fatal("completed transaction could be reused")
	}
	if err := cleanupguard.EnterMaintenance(database, gc); !errors.Is(err, cleanupguard.ErrCoordinationBusy) {
		t.Fatal("version persistence prematurely released producer", err)
	}
}

// capability_id: rainbond.cleanup.plugin-version-reference-coordination
func TestPluginVersionWritesRespectDeletionAndNeverResurrect(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "plugins.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.LogMode(false)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}, &model.TenantPluginBuildVersion{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.CleanupStorage{StorageID: "owned", Generation: "one", Mode: "ready"}).Error; err != nil {
		t.Fatal(err)
	}
	versions := &PluginBuildVersionDaoImpl{DB: database}
	record := &model.TenantPluginBuildVersion{PluginID: "plugin", VersionID: "v1", DeployVersion: "build", BuildLocalImage: "goodrain.me/plugin/local:latest"}
	if err := versions.AddModel(record); err != nil {
		t.Fatal(err)
	}
	selected := cleanupguard.CoordinationRequest{StorageID: "owned", Generation: "one", OperationID: "delete", Owner: "cleanup", Kind: "delete", Scope: "plugin/local", Target: "sha256:" + strings.Repeat("a", 64), Fingerprint: "selected"}
	if _, err := cleanupguard.AcquireOperation(database, selected); err != nil {
		t.Fatal(err)
	}
	record.Status = "complete"
	if err := versions.UpdateModel(record); !errors.Is(err, cleanupguard.ErrCoordinationBusy) {
		t.Fatal("plugin reference changed during deletion", err)
	}
	if err := cleanupguard.FinishOperation(database, selected, true); err != nil {
		t.Fatal(err)
	}
	if err := database.Delete(record).Error; err != nil {
		t.Fatal(err)
	}
	if err := versions.UpdateModel(record); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("late plugin callback recreated deleted version", err)
	}
}
