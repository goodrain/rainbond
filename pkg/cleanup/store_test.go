package cleanup

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
)

func mockRetirementDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	raw, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	database, err := gorm.Open("mysql", raw)
	if err != nil {
		t.Fatal(err)
	}
	database.LogMode(false)
	t.Cleanup(func() { database.Close() })
	return database, mock
}

func TestRetirementTransactionChecksCurrentVersionBeforeDelete(t *testing.T) {
	for _, current := range []string{"current", "old"} {
		t.Run(current, func(t *testing.T) {
			database, mock := mockRetirementDB(t)
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT .*tenant_services.*FOR UPDATE").WithArgs("service", "tenant").WillReturnRows(sqlmock.NewRows([]string{"service_id", "tenant_id", "deploy_version"}).AddRow("service", "tenant", current))
			mock.ExpectQuery("SELECT .*tenant_service_version.*FOR UPDATE").WithArgs("service", "old").WillReturnRows(sqlmock.NewRows([]string{"ID", "service_id", "build_version", "image_name", "delivered_type", "final_status", "event_id"}).AddRow(7, "service", "old", "registry/app:old", "image", "success", "build-event"))
			mock.ExpectQuery("SELECT count.*tenant_services_event").WithArgs("service", "cleanup-retire-buildversion", "complete", "failure", "emptycomplete").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			if current == "current" {
				mock.ExpectExec("DELETE FROM tenant_service_version").WithArgs(uint(7), "service", "old").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			result, err := RetireVersion(database.Begin, "tenant", VersionExpectation{EventID: "build-event", ServiceID: "service", Version: "old", CurrentVersion: current, Image: "registry/app:old"})
			if current == "current" && (err != nil || !result.RecordRetired || result.ImageDeleted || result.ReclaimedBytes != nil) {
				t.Fatal("incorrect retirement receipt", err)
			}
			if current == "old" && !errors.Is(err, ErrVersionProtected) {
				t.Fatal("current version not protected", err)
			}
			if err = mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRollbackCannotSelectMissingRetiredVersion(t *testing.T) {
	database, mock := mockRetirementDB(t)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE .*cleanup_storage").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT .*cleanup_storage.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"storage_id", "generation", "mode"}))
	mock.ExpectQuery("SELECT .*tenant_services.*FOR UPDATE").WithArgs("service", "tenant").WillReturnRows(sqlmock.NewRows([]string{"service_id", "tenant_id", "deploy_version"}).AddRow("service", "tenant", "current"))
	mock.ExpectQuery("SELECT .*tenant_service_version.*FOR UPDATE").WithArgs("service", "old").WillReturnRows(sqlmock.NewRows([]string{"ID"}))
	mock.ExpectRollback()
	if _, err := SelectRollbackVersion(database.Begin, "tenant", "service", "old", "rollback-event"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("retired version selected", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRetirementRejectsActiveOperationUnderLock(t *testing.T) {
	database, mock := mockRetirementDB(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .*tenant_services.*FOR UPDATE").WithArgs("service", "tenant").WillReturnRows(sqlmock.NewRows([]string{"service_id", "tenant_id", "deploy_version"}).AddRow("service", "tenant", "current"))
	mock.ExpectQuery("SELECT .*tenant_service_version.*FOR UPDATE").WithArgs("service", "old").WillReturnRows(sqlmock.NewRows([]string{"ID", "service_id", "build_version", "image_name", "delivered_type", "final_status", "event_id"}).AddRow(7, "service", "old", "registry/app:old", "image", "success", "build-event"))
	mock.ExpectQuery("SELECT count.*tenant_services_event").WithArgs("service", "cleanup-retire-buildversion", "complete", "failure", "emptycomplete").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectRollback()
	_, err := RetireVersion(database.Begin, "tenant", VersionExpectation{EventID: "build-event", ServiceID: "service", Version: "old", CurrentVersion: "current", Image: "registry/app:old"})
	if !errors.Is(err, ErrVersionProtected) {
		t.Fatal("active operation not protected", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRollbackChangesActivationCheckpointAndSelectionTogether(t *testing.T) {
	database, mock := mockRetirementDB(t)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE .*cleanup_storage").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT .*cleanup_storage.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"storage_id", "generation", "mode"}))
	mock.ExpectQuery("SELECT .*tenant_services.*FOR UPDATE").WithArgs("service", "tenant").WillReturnRows(sqlmock.NewRows([]string{"service_id", "tenant_id", "deploy_version"}).AddRow("service", "tenant", "current"))
	mock.ExpectQuery("SELECT .*tenant_service_version.*FOR UPDATE").WithArgs("service", "old").WillReturnRows(sqlmock.NewRows([]string{"ID", "final_status"}).AddRow(7, "success"))
	mock.ExpectExec("UPDATE .*tenant_service_version.*activation_revision").WithArgs("rollback-event", "service", "old").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE .*tenant_services.*deploy_version").WithArgs("old", "service", "tenant").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	previous, err := SelectRollbackVersion(database.Begin, "tenant", "service", "old", "rollback-event")
	if err != nil || previous != "current" {
		t.Fatal(previous, err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFailedRollbackDoesNotOverwriteNewerOperation(t *testing.T) {
	database, mock := mockRetirementDB(t)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE .*cleanup_storage").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT .*cleanup_storage.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"storage_id", "generation", "mode"}))
	mock.ExpectQuery("SELECT .*tenant_services.*FOR UPDATE").WithArgs("service", "tenant").WillReturnRows(sqlmock.NewRows([]string{"service_id", "tenant_id", "deploy_version"}).AddRow("service", "tenant", "newer"))
	mock.ExpectRollback()
	_, err := SelectRollbackVersion(database.Begin, "tenant", "service", "previous", "rollback-undo", "target")
	if !errors.Is(err, ErrStateChanged) {
		t.Fatal("newer operation overwritten", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestActivationCheckpointFailureRollsBackServiceChange(t *testing.T) {
	database, mock := mockRetirementDB(t)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE .*cleanup_storage").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT .*cleanup_storage.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"storage_id", "generation", "mode"}))
	mock.ExpectQuery("SELECT .*tenant_services.*FOR UPDATE").WithArgs("service").WillReturnRows(sqlmock.NewRows([]string{"service_id", "deploy_version"}).AddRow("service", "version"))
	mock.ExpectQuery("SELECT .*tenant_service_version.*FOR UPDATE").WithArgs("service", "version").WillReturnRows(sqlmock.NewRows([]string{"ID"}).AddRow(7))
	mock.ExpectExec("UPDATE tenant_services").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE .*tenant_service_version.*activation_revision").WithArgs(sqlmock.AnyArg(), "service", "version").WillReturnError(errors.New("write failure"))
	mock.ExpectRollback()
	err := TrackServiceActivation(database, "service", "version", func(tx *gorm.DB) error { return tx.Exec("UPDATE tenant_services SET deploy_version='version'").Error })
	if err == nil {
		t.Fatal("checkpoint failure ignored")
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestActivationDoesNotCommitCallerTransaction(t *testing.T) {
	database, mock := mockRetirementDB(t)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE .*cleanup_storage").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT .*cleanup_storage.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"storage_id", "generation", "mode"}))
	tx := database.Begin()
	mock.ExpectQuery("SELECT .*tenant_services.*FOR UPDATE").WithArgs("service").WillReturnRows(sqlmock.NewRows([]string{"service_id", "deploy_version"}).AddRow("service", "version"))
	mock.ExpectQuery("SELECT .*tenant_service_version.*FOR UPDATE").WithArgs("service", "version").WillReturnRows(sqlmock.NewRows([]string{"ID"}).AddRow(7))
	mock.ExpectExec("UPDATE tenant_services").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE .*tenant_service_version.*activation_revision").WithArgs(sqlmock.AnyArg(), "service", "version").WillReturnResult(sqlmock.NewResult(0, 1))
	if err := TrackServiceActivation(tx, "service", "version", func(tx *gorm.DB) error { return tx.Exec("UPDATE tenant_services SET deploy_version='version'").Error }); err != nil {
		t.Fatal(err)
	}
	mock.ExpectRollback()
	tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.cleanup.generic-activation-fence
func TestActivationCannotRestoreRetiredTargetThroughGenericSave(t *testing.T) {
	database, mock := mockRetirementDB(t)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE .*cleanup_storage").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT .*cleanup_storage.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"storage_id", "generation", "mode"}))
	mock.ExpectQuery("SELECT .*tenant_services.*FOR UPDATE").WithArgs("service").WillReturnRows(sqlmock.NewRows([]string{"service_id", "deploy_version"}).AddRow("service", "current"))
	mock.ExpectQuery("SELECT .*tenant_service_version.*FOR UPDATE").WithArgs("service", "retired").WillReturnRows(sqlmock.NewRows([]string{"ID"}))
	mock.ExpectRollback()
	saved := false
	err := TrackServiceActivation(database, "service", "retired", func(tx *gorm.DB) error { saved = true; return nil })
	if !errors.Is(err, gorm.ErrRecordNotFound) || saved {
		t.Fatalf("retired target saved: saved=%v err=%v", saved, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestActivationPermitsNewBuildWithExistingPendingRecord(t *testing.T) {
	database, mock := mockRetirementDB(t)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE .*cleanup_storage").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT .*cleanup_storage.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"storage_id", "generation", "mode"}))
	mock.ExpectQuery("SELECT .*tenant_services.*FOR UPDATE").WithArgs("service").WillReturnRows(sqlmock.NewRows([]string{"service_id", "deploy_version"}).AddRow("service", "current"))
	mock.ExpectQuery("SELECT .*tenant_service_version.*FOR UPDATE").WithArgs("service", "new").WillReturnRows(sqlmock.NewRows([]string{"ID", "final_status"}).AddRow(8, ""))
	mock.ExpectExec("UPDATE tenant_services").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE .*tenant_service_version.*activation_revision").WithArgs(sqlmock.AnyArg(), "service", "new").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	err := TrackServiceActivation(database, "service", "new", func(tx *gorm.DB) error { return tx.Exec("UPDATE tenant_services SET deploy_version='new'").Error })
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestConfigurationSaveKeepsLegacyCurrentTargetWithoutReactivation(t *testing.T) {
	database, mock := mockRetirementDB(t)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE .*cleanup_storage").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT .*cleanup_storage.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"storage_id", "generation", "mode"}))
	mock.ExpectQuery("SELECT .*tenant_services.*FOR UPDATE").WithArgs("service").WillReturnRows(sqlmock.NewRows([]string{"service_id", "deploy_version"}).AddRow("service", "legacy"))
	mock.ExpectQuery("SELECT .*tenant_service_version.*FOR UPDATE").WithArgs("service", "legacy").WillReturnRows(sqlmock.NewRows([]string{"ID"}))
	mock.ExpectExec("UPDATE tenant_services").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE .*tenant_service_version.*activation_revision").WithArgs(sqlmock.AnyArg(), "service", "legacy").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	err := TrackServiceActivation(database, "service", "legacy", func(tx *gorm.DB) error { return tx.Exec("UPDATE tenant_services SET name='renamed'").Error })
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestInspectionAdvertisesGuardedVersionUpdates(t *testing.T) {
	database, mock := mockRetirementDB(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .*tenant_services.*FOR UPDATE").WithArgs("service").WillReturnRows(sqlmock.NewRows([]string{"service_id", "deploy_version"}).AddRow("service", "current"))
	mock.ExpectQuery("SELECT count.*tenant_services_event").WithArgs("service", "cleanup-retire-buildversion", "complete", "failure", "emptycomplete").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT event_id, activation_revision.*tenant_service_version").WithArgs("service").WillReturnRows(sqlmock.NewRows([]string{"event_id", "activation_revision"}).AddRow("event", "revision"))
	mock.ExpectCommit()
	result, err := InspectVersions(database.Begin, "service")
	if err != nil || result.Protocol != 2 || result.CurrentVersion != "current" || result.Checkpoints["event"] != "revision" {
		t.Fatal(result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
