package dao

// capability_id: rainbond.env-var.noop-update

import (
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

func newTenantServiceEnvVarTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "tenant-service-env-var.db"))
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	db.LogMode(false)

	if err := db.AutoMigrate(&model.TenantServiceEnvVar{}).Error; err != nil {
		db.Close()
		t.Fatalf("auto migrate tenant service env var: %v", err)
	}

	return db
}

func TestTenantServiceEnvVarDaoUpdateModelByAttrNameRenamesEnv(t *testing.T) {
	db := newTenantServiceEnvVarTestDB(t)
	defer db.Close()

	dao := &TenantServiceEnvVarDaoImpl{DB: db}
	oldEnv := &model.TenantServiceEnvVar{
		TenantID:      "tenant-id",
		ServiceID:     "service-id",
		ContainerPort: 0,
		Name:          "original note",
		AttrName:      "OLD_KEY",
		AttrValue:     "old-value",
		IsChange:      true,
		Scope:         "inner",
	}
	if err := dao.AddModel(oldEnv); err != nil {
		t.Fatalf("add env: %v", err)
	}

	err := dao.UpdateModelByAttrName(&model.TenantServiceEnvVar{
		ServiceID: "service-id",
		AttrName:  "NEW_KEY",
		AttrValue: "new-value",
		IsChange:  true,
		Scope:     "outer",
	}, "OLD_KEY")
	if err != nil {
		t.Fatalf("rename env: %v", err)
	}

	if _, err := dao.GetEnv("service-id", "OLD_KEY"); err == nil {
		t.Fatal("expected old env key to be removed")
	}
	renamed, err := dao.GetEnv("service-id", "NEW_KEY")
	if err != nil {
		t.Fatalf("get renamed env: %v", err)
	}
	if renamed.AttrValue != "new-value" {
		t.Fatalf("expected renamed env value new-value, got %q", renamed.AttrValue)
	}
	if renamed.Scope != "outer" {
		t.Fatalf("expected renamed env scope outer, got %q", renamed.Scope)
	}
}

func TestTenantServiceEnvVarDaoUpdateModelByAttrNameNoopSucceeds(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer sqlDB.Close()

	db, err := gorm.Open("mysql", sqlDB)
	if err != nil {
		t.Fatalf("open gorm db: %v", err)
	}
	defer db.Close()
	db.LogMode(false)

	dao := &TenantServiceEnvVarDaoImpl{DB: db}
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT").
		WithArgs("service-id", "LOGGIN_LEVEL").
		WillReturnRows(sqlmock.NewRows([]string{"id", "service_id", "attr_name"}).
			AddRow(1, "service-id", "LOGGIN_LEVEL"))

	err = dao.UpdateModelByAttrName(&model.TenantServiceEnvVar{
		ServiceID: "service-id",
		AttrName:  "LOGGIN_LEVEL",
		AttrValue: "INFO",
		IsChange:  false,
		Scope:     "inner",
	}, "LOGGIN_LEVEL")
	if err != nil {
		t.Fatalf("noop update should succeed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
