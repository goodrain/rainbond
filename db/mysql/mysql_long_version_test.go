package mysql

import (
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

func newLongVersionPatchTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "patch-long-version.db"))
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	db.LogMode(false)

	if err := db.AutoMigrate(&model.EnterpriseLanguageVersion{}).Error; err != nil {
		db.Close()
		t.Fatalf("auto migrate enterprise language version: %v", err)
	}

	return db
}

func TestLongVersionSeedDefaultsKeepSlugVersions(t *testing.T) {
	versions := []*model.EnterpriseLanguageVersion{
		{
			Lang:    "golang",
			Version: "go1.23.12",
		},
		{
			Lang:    "python",
			Version: "python-3.6.15",
		},
	}

	applySeedLongVersionDefaults(versions)

	for _, version := range versions {
		if version.BuildStrategy != model.LongVersionBuildStrategySlug {
			t.Fatalf("expected slug build strategy for %s, got %q", version.Version, version.BuildStrategy)
		}
		if !version.IsAllowed {
			t.Fatalf("expected is_allowed=true for %s", version.Version)
		}
	}
	if versions[0].Version != "go1.23.12" {
		t.Fatalf("expected slug seed version to remain unchanged, got %q", versions[0].Version)
	}
	if versions[1].Version != "python-3.6.15" {
		t.Fatalf("expected slug seed version to remain unchanged, got %q", versions[1].Version)
	}
}

func TestLongVersionBackfillLegacyStrategy(t *testing.T) {
	db := newLongVersionPatchTestDB(t)
	defer db.Close()

	legacy := &model.EnterpriseLanguageVersion{
		Lang:        "python",
		Version:     "python-3.6.15",
		FirstChoice: true,
		FileName:    "Python3.6.15.tar.gz",
		System:      true,
		Show:        true,
	}
	cnb := &model.EnterpriseLanguageVersion{
		Lang:          "python",
		Version:       "3.11",
		BuildStrategy: model.LongVersionBuildStrategyCNB,
		IsAllowed:     false,
		Show:          true,
	}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatalf("create legacy version: %v", err)
	}
	if err := db.Create(cnb).Error; err != nil {
		t.Fatalf("create cnb version: %v", err)
	}

	if err := backfillLegacyLongVersionStrategy(db); err != nil {
		t.Fatalf("backfill legacy strategy: %v", err)
	}

	var gotLegacy model.EnterpriseLanguageVersion
	if err := db.Where("lang = ? AND version = ?", "python", "python-3.6.15").First(&gotLegacy).Error; err != nil {
		t.Fatalf("query legacy version: %v", err)
	}
	if gotLegacy.BuildStrategy != model.LongVersionBuildStrategySlug {
		t.Fatalf("expected legacy build strategy slug, got %q", gotLegacy.BuildStrategy)
	}
	if !gotLegacy.IsAllowed {
		t.Fatal("expected legacy version to backfill is_allowed=true")
	}

	var gotCNB model.EnterpriseLanguageVersion
	if err := db.Where("lang = ? AND version = ?", "python", "3.11").First(&gotCNB).Error; err != nil {
		t.Fatalf("query cnb version: %v", err)
	}
	if gotCNB.BuildStrategy != model.LongVersionBuildStrategyCNB {
		t.Fatalf("expected cnb build strategy to remain cnb, got %q", gotCNB.BuildStrategy)
	}
	if gotCNB.IsAllowed {
		t.Fatal("expected non-legacy cnb version to keep is_allowed=false")
	}
}
