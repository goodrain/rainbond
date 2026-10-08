package handler

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	gclient "github.com/goodrain/rainbond/mq/client"
	dmodel "github.com/goodrain/rainbond/worker/discover/model"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
)

type cleanupRollbackManager struct {
	db.Manager
	database *gorm.DB
	services *cleanupRollbackServices
}

func (m cleanupRollbackManager) Begin() *gorm.DB                        { return m.database.Begin() }
func (m cleanupRollbackManager) TenantServiceDao() dao.TenantServiceDao { return m.services }

func (m cleanupRollbackManager) VersionInfoDao() dao.VersionInfoDao {
	return &cleanupRollbackVersions{}
}

type cleanupRollbackVersions struct{ dao.VersionInfoDao }

func (*cleanupRollbackVersions) GetVersionByDeployVersion(string, string) (*dbmodel.VersionInfo, error) {
	return &dbmodel.VersionInfo{BuildVersion: "old", FinalStatus: "success"}, nil
}

type cleanupRollbackServices struct {
	dao.TenantServiceDao
	writes int
}

func (s *cleanupRollbackServices) GetServiceByID(string) (*dbmodel.TenantServices, error) {
	return &dbmodel.TenantServices{ServiceID: "service", TenantID: "tenant", DeployVersion: "current"}, nil
}
func (s *cleanupRollbackServices) UpdateModel(dbmodel.Interface) error {
	s.writes++
	return errors.New("unguarded version update")
}

// capability_id: rainbond.cleanup.version-activation-fence
func TestLegacyRollbackCannotActivateRetiredVersion(t *testing.T) {
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
	services := &cleanupRollbackServices{}
	db.SetTestManager(cleanupRollbackManager{database: database, services: services})
	defer db.SetTestManager(nil)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE .*cleanup_storage").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT .*cleanup_storage.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"storage_id", "generation", "mode"}))
	mock.ExpectQuery("SELECT .*tenant_services.*FOR UPDATE").WithArgs("service", "tenant").WillReturnRows(sqlmock.NewRows([]string{"service_id", "tenant_id", "deploy_version"}).AddRow("service", "tenant", "current"))
	mock.ExpectQuery("SELECT .*tenant_service_version.*FOR UPDATE").WithArgs("service", "old").WillReturnRows(sqlmock.NewRows([]string{"ID"}))
	mock.ExpectRollback()
	err = (&ServiceAction{}).RollBack(&apimodel.RollbackStruct{ServiceID: "service", TenantID: "tenant", DeployVersion: "old", EventID: "rollback-event"})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("retired target was not rejected by the transaction guard: %v", err)
	}
	if services.writes != 0 {
		t.Fatal("legacy rollback bypassed guarded selection")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.cleanup.version-activation-fence
func TestExplicitUpgradeRechecksVersionUnderRetirementLock(t *testing.T) {
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
	services := &cleanupRollbackServices{}
	db.SetTestManager(cleanupRollbackManager{database: database, services: services})
	defer db.SetTestManager(nil)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE .*cleanup_storage").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT .*cleanup_storage.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"storage_id", "generation", "mode"}))
	mock.ExpectQuery("SELECT .*tenant_services.*FOR UPDATE").WithArgs("service", "tenant").WillReturnRows(sqlmock.NewRows([]string{"service_id", "tenant_id", "deploy_version"}).AddRow("service", "tenant", "current"))
	// The earlier lookup was successful, but retirement won before activation.
	mock.ExpectQuery("SELECT .*tenant_service_version.*FOR UPDATE").WithArgs("service", "old").WillReturnRows(sqlmock.NewRows([]string{"ID"}))
	mock.ExpectRollback()
	err = (&OperationHandler{}).upgrade(&apimodel.ComponentUpgradeReq{ComponentOpGeneralReq: apimodel.ComponentOpGeneralReq{ServiceID: "service", EventID: "upgrade-event"}, UpgradeVersion: "old"})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("upgrade activated a retired version: %v", err)
	}
	if services.writes != 0 {
		t.Fatal("upgrade used unguarded full service update")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

type cleanupFailedMQ struct {
	gclient.MQClient
	calls   int
	failure error
	last    gclient.TaskStruct
}

func (m *cleanupFailedMQ) SendBuilderTopic(task gclient.TaskStruct) error {
	m.last = task
	m.calls++
	return m.failure
}

// capability_id: rainbond.cleanup.version-activation-fence
func TestUpgradeQueueFailureCannotOverwriteNewerDeployment(t *testing.T) {
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
	services := &cleanupRollbackServices{}
	db.SetTestManager(cleanupRollbackManager{database: database, services: services})
	defer db.SetTestManager(nil)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE .*cleanup_storage").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT .*cleanup_storage.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"storage_id", "generation", "mode"}))
	mock.ExpectQuery("SELECT .*tenant_services.*FOR UPDATE").WithArgs("service", "tenant").WillReturnRows(sqlmock.NewRows([]string{"service_id", "tenant_id", "deploy_version"}).AddRow("service", "tenant", "current"))
	mock.ExpectQuery("SELECT .*tenant_service_version.*FOR UPDATE").WithArgs("service", "old").WillReturnRows(sqlmock.NewRows([]string{"ID", "final_status"}).AddRow(7, "success"))
	mock.ExpectExec("UPDATE .*tenant_service_version.*activation_revision").WithArgs("upgrade-event", "service", "old").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE .*tenant_services.*deploy_version").WithArgs("old", "service", "tenant").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE .*cleanup_storage").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT .*cleanup_storage.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"storage_id", "generation", "mode"}))
	mock.ExpectQuery("SELECT .*tenant_services.*FOR UPDATE").WithArgs("service", "tenant").WillReturnRows(sqlmock.NewRows([]string{"service_id", "tenant_id", "deploy_version"}).AddRow("service", "tenant", "newer"))
	mock.ExpectRollback()
	failure := errors.New("queue unavailable")
	queue := &cleanupFailedMQ{failure: failure}
	err = (&OperationHandler{mqCli: queue}).upgrade(&apimodel.ComponentUpgradeReq{ComponentOpGeneralReq: apimodel.ComponentOpGeneralReq{ServiceID: "service", EventID: "upgrade-event"}, UpgradeVersion: "old"})
	if !errors.Is(err, failure) || queue.calls != 1 || services.writes != 0 {
		t.Fatal("compensation overwrote another operation", err, queue.calls, services.writes)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.cleanup.version-activation-fence
func TestLegacyRollbackRejectsWrongTenant(t *testing.T) {
	services := &cleanupRollbackServices{}
	db.SetTestManager(cleanupRollbackManager{services: services})
	defer db.SetTestManager(nil)
	if err := (&ServiceAction{}).RollBack(&apimodel.RollbackStruct{ServiceID: "service", TenantID: "other", DeployVersion: "old", EventID: "event"}); err == nil {
		t.Fatal("cross-tenant rollback accepted")
	}
	if services.writes != 0 {
		t.Fatal("cross-tenant request mutated service")
	}
}

// capability_id: rainbond.cleanup.version-activation-fence
func TestExplicitUpgradeEnqueuesValidatedVersion(t *testing.T) {
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
	services := &cleanupRollbackServices{}
	db.SetTestManager(cleanupRollbackManager{database: database, services: services})
	defer db.SetTestManager(nil)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE .*cleanup_storage").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT .*cleanup_storage.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"storage_id", "generation", "mode"}))
	mock.ExpectQuery("SELECT .*tenant_services.*FOR UPDATE").WithArgs("service", "tenant").WillReturnRows(sqlmock.NewRows([]string{"service_id", "tenant_id", "deploy_version"}).AddRow("service", "tenant", "current"))
	mock.ExpectQuery("SELECT .*tenant_service_version.*FOR UPDATE").WithArgs("service", "old").WillReturnRows(sqlmock.NewRows([]string{"ID", "final_status"}).AddRow(7, "success"))
	mock.ExpectExec("UPDATE .*tenant_service_version.*activation_revision").WithArgs("upgrade-event", "service", "old").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE .*tenant_services.*deploy_version").WithArgs("old", "service", "tenant").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	queue := &cleanupFailedMQ{}
	err = (&OperationHandler{mqCli: queue}).upgrade(&apimodel.ComponentUpgradeReq{ComponentOpGeneralReq: apimodel.ComponentOpGeneralReq{ServiceID: "service", EventID: "upgrade-event"}, UpgradeVersion: "old"})
	if err != nil || queue.calls != 1 || services.writes != 0 {
		t.Fatal("validated upgrade failed", err)
	}
	body, ok := queue.last.TaskBody.(*dmodel.RollingUpgradeTaskBody)
	if !ok || body.NewDeployVersion != "old" || body.TenantID != "tenant" || body.ServiceID != "service" {
		t.Fatal("wrong deployment enqueued")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
