package handler

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/require"
)

// capability_id: rainbond.rollback.selection-failure-diagnostics
func TestRollbackDatabaseFailureIsNotReportedAsMissingVersion(t *testing.T) {
	raw, mock, err := sqlmock.New()
	require.NoError(t, err)
	database, err := gorm.Open("mysql", raw)
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close() })
	database.LogMode(false)
	previous := db.GetManager()
	db.SetTestManager(cleanupRollbackManager{database: database, services: &cleanupRollbackServices{}})
	t.Cleanup(func() { db.SetTestManager(previous) })
	mock.ExpectBegin().WillReturnError(errors.New("fixture database unavailable"))
	queue := &cleanupFailedMQ{}
	result := (&OperationHandler{mqCli: queue}).RollBack(apimodel.RollbackInfoRequestStruct{
		ServiceID: "service", RollBackVersion: "old", EventID: "event",
	})
	require.Equal(t, "failure", result.Status)
	require.Equal(t, "failed to select rollback version", result.ErrMsg)
	require.Zero(t, queue.calls, "failed selection must not submit a worker task")
	require.NoError(t, mock.ExpectationsWereMet())
}

func expectRollbackSelection(mock sqlmock.Sqlmock, current, target, event string) {
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE .*cleanup_storage").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT .*cleanup_storage.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"storage_id", "generation", "mode"}))
	mock.ExpectQuery("SELECT .*tenant_services.*FOR UPDATE").WithArgs("service", "tenant").WillReturnRows(
		sqlmock.NewRows([]string{"service_id", "tenant_id", "deploy_version"}).AddRow("service", "tenant", current))
	mock.ExpectQuery("SELECT .*tenant_service_version.*FOR UPDATE").WithArgs("service", target).WillReturnRows(
		sqlmock.NewRows([]string{"ID", "final_status"}).AddRow(7, "success"))
	mock.ExpectExec("UPDATE .*tenant_service_version.*activation_revision").WithArgs(event, "service", target).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE .*tenant_services.*deploy_version").WithArgs(target, "service", "tenant").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
}

func TestRollbackQueuesOnceAndCompensatesOnlyAfterQueueFailure(t *testing.T) {
	for _, failQueue := range []bool{false, true} {
		name := "success"
		if failQueue {
			name = "queue failure"
		}
		t.Run(name, func(t *testing.T) {
			raw, mock, err := sqlmock.New()
			require.NoError(t, err)
			database, err := gorm.Open("mysql", raw)
			require.NoError(t, err)
			t.Cleanup(func() { _ = database.Close() })
			database.LogMode(false)
			previous := db.GetManager()
			db.SetTestManager(cleanupRollbackManager{database: database, services: &cleanupRollbackServices{}})
			t.Cleanup(func() { db.SetTestManager(previous) })
			expectRollbackSelection(mock, "current", "old", "event")
			queue := &cleanupFailedMQ{}
			if failQueue {
				queue.failure = errors.New("fixture queue unavailable")
				expectRollbackSelection(mock, "old", "current", "event-undo")
			}
			result := (&OperationHandler{mqCli: queue}).RollBack(apimodel.RollbackInfoRequestStruct{
				ServiceID: "service", RollBackVersion: "old", EventID: "event",
			})
			require.Equal(t, 1, queue.calls)
			if failQueue {
				require.Equal(t, "failure", result.Status)
				require.Contains(t, result.ErrMsg, "rollback message failure")
			} else {
				require.Equal(t, "success", result.Status)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRollbackUnavailableTargetsStayDistinctFromDatabaseFailures(t *testing.T) {
	for _, reason := range []string{"current version changed", "missing target", "failed target"} {
		t.Run(reason, func(t *testing.T) {
			raw, mock, err := sqlmock.New()
			require.NoError(t, err)
			database, err := gorm.Open("mysql", raw)
			require.NoError(t, err)
			t.Cleanup(func() { _ = database.Close() })
			database.LogMode(false)
			previous := db.GetManager()
			db.SetTestManager(cleanupRollbackManager{database: database, services: &cleanupRollbackServices{}})
			t.Cleanup(func() { db.SetTestManager(previous) })
			mock.ExpectBegin()
			mock.ExpectExec("UPDATE .*cleanup_storage").WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectQuery("SELECT .*cleanup_storage.*FOR UPDATE").WillReturnRows(
				sqlmock.NewRows([]string{"storage_id", "generation", "mode"}))
			current := "current"
			if reason == "current version changed" {
				current = "newer"
			}
			mock.ExpectQuery("SELECT .*tenant_services.*FOR UPDATE").WithArgs("service", "tenant").WillReturnRows(
				sqlmock.NewRows([]string{"service_id", "tenant_id", "deploy_version"}).AddRow("service", "tenant", current))
			if reason != "current version changed" {
				rows := sqlmock.NewRows([]string{"ID", "final_status"})
				if reason == "failed target" {
					rows.AddRow(7, "failure")
				}
				mock.ExpectQuery("SELECT .*tenant_service_version.*FOR UPDATE").WithArgs("service", "old").WillReturnRows(rows)
			}
			mock.ExpectRollback()
			queue := &cleanupFailedMQ{}
			result := (&OperationHandler{mqCli: queue}).RollBack(apimodel.RollbackInfoRequestStruct{
				ServiceID: "service", RollBackVersion: "old", EventID: "event",
			})
			require.Equal(t, "failure", result.Status)
			require.Equal(t, "rollback version is no longer available", result.ErrMsg)
			require.Zero(t, queue.calls)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
