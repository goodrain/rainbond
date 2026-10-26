package dao

import (
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

func newServiceProbeTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "service-probe.db"))
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	db.LogMode(false)
	if err := db.AutoMigrate(&model.TenantServiceProbe{}).Error; err != nil {
		db.Close()
		t.Fatalf("auto migrate service probe: %v", err)
	}
	return db
}

// capability_id: rainbond.service-probe.reconcile-duplicate-mode
func TestServiceProbeDaoAddModelReconcilesExistingMode(t *testing.T) {
	db := newServiceProbeTestDB(t)
	defer db.Close()

	dao := &ServiceProbeDaoImpl{DB: db}
	enabled := 1
	existing := &model.TenantServiceProbe{
		ServiceID: "service-1",
		ProbeID:   "old-probe-id-0000000000000000001",
		Mode:      "readiness",
		Scheme:    "tcp",
		Port:      80,
		IsUsed:    &enabled,
	}
	if err := dao.AddModel(existing); err != nil {
		t.Fatalf("add initial probe: %v", err)
	}
	originalID := existing.ID

	replacement := &model.TenantServiceProbe{
		ServiceID: "service-1",
		ProbeID:   "new-probe-id-0000000000000000001",
		Mode:      "readiness",
		Scheme:    "http",
		Path:      "/ready",
		Port:      8080,
		IsUsed:    &enabled,
	}
	if err := dao.AddModel(replacement); err != nil {
		t.Fatalf("reconcile duplicate mode: %v", err)
	}

	var probes []model.TenantServiceProbe
	if err := db.Find(&probes).Error; err != nil {
		t.Fatalf("list probes: %v", err)
	}
	if len(probes) != 1 {
		t.Fatalf("expected one reconciled probe, got %d", len(probes))
	}
	if probes[0].ID != originalID {
		t.Fatalf("expected database id %d to be preserved, got %d", originalID, probes[0].ID)
	}
	if probes[0].ProbeID != replacement.ProbeID || probes[0].Port != 8080 || probes[0].Path != "/ready" {
		t.Fatalf("expected replacement probe fields, got %#v", probes[0])
	}
}
