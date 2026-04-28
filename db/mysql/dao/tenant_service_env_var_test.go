package dao

import (
	"path/filepath"
	"testing"

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
