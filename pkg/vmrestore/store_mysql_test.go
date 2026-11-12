package vmrestore

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
)

// capability_id: rainbond.vm-restore.atomic-state-store
func TestMySQLUsesCurrentLockingReadsBeforeMutation(t *testing.T) {
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
	scope := Scope{TenantID: "tenant", ServiceID: "service", Namespace: "team"}
	expected := expectation()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .*tenant_services.*ORDER BY `ID` FOR UPDATE").WithArgs(scope.ServiceID).WillReturnRows(sqlmock.NewRows([]string{"ID", "tenant_id", "service_id", "extend_method"}).AddRow(1, scope.TenantID, scope.ServiceID, string(dbmodel.ServiceTypeVM)))
	mock.ExpectQuery("SELECT .*tenants.*uuid").WithArgs(scope.TenantID).WillReturnRows(sqlmock.NewRows([]string{"uuid", "namespace"}).AddRow(scope.TenantID, scope.Namespace))
	mock.ExpectQuery("SELECT .*component_k8s_attributes.*ORDER BY name, ID FOR UPDATE").WithArgs(scope.ServiceID, StateKey, LegacyBindingsKey, ReceiptPrefix+expected.OperationID).WillReturnRows(sqlmock.NewRows([]string{"ID", "tenant_id", "component_id", "name", "attribute_value"}))
	mock.ExpectQuery("SELECT ID, tenant_id, component_id, name FROM .*component_k8s_attributes.*FOR UPDATE").WithArgs(scope.ServiceID).WillReturnRows(sqlmock.NewRows([]string{"ID", "tenant_id", "component_id", "name"}))
	mock.ExpectRollback()
	_, err = NewStore(database).Mutate(scope, expected, func(Current) (Mutation, error) { return Mutation{}, ErrBusy })
	if !errors.Is(err, ErrBusy) {
		t.Fatal("callback rejection was not preserved")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
