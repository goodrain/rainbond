package cleanup

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jinzhu/gorm"
)

// capability_id: rainbond.cleanup.activation-registry-coordination
func TestActivationAndRollbackRejectDeletingImage(t *testing.T) {
	for _, rollback := range []bool{false, true} {
		name := "activation"
		if rollback {
			name = "rollback"
		}
		t.Run(name, func(t *testing.T) {
			database, mock := mockRetirementDB(t)
			mock.ExpectBegin()
			mock.ExpectExec("UPDATE .*cleanup_storage").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery("SELECT .*cleanup_storage.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"storage_id", "generation", "mode"}).AddRow("hub", "one", "ready"))
			mock.ExpectQuery("SELECT .*tenant_services.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"service_id", "tenant_id", "deploy_version"}).AddRow("service", "tenant", "current"))
			mock.ExpectQuery("SELECT .*tenant_service_version.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"service_id", "build_version", "image_name", "final_status"}).AddRow("service", "old", "goodrain.me/team/app:v1", "success"))
			mock.ExpectQuery("SELECT .*cleanup_operations.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"generation", "scope", "kind", "state"}).AddRow("one", "team/app", "delete", "executing"))
			mock.ExpectRollback()
			var err error
			if rollback {
				_, err = SelectRollbackVersion(database.Begin, "tenant", "service", "old", "rollback")
			} else {
				err = TrackServiceActivation(database, "service", "old", func(*gorm.DB) error { t.Error("activation was saved during deletion"); return nil })
			}
			if !errors.Is(err, ErrCoordinationBusy) {
				t.Fatal("deleting image was not protected", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
