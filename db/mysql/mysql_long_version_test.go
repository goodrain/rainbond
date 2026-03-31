package mysql

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/goodrain/rainbond/db/config"
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

func newMySQLDialectPatchTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock db: %v", err)
	}
	db, err := gorm.Open("mysql", sqlDB)
	if err != nil {
		sqlDB.Close()
		t.Fatalf("open gorm mysql db: %v", err)
	}
	db.LogMode(false)
	t.Cleanup(func() {
		_ = db.Close()
	})
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

func TestCNBSeedVersionsUseCNBStrategy(t *testing.T) {
	versions := cnbSeedLanguageVersions()
	if len(versions) == 0 {
		t.Fatal("expected cnb seed versions to be populated")
	}

	foundJava := false
	foundNode := false
	foundGoDefault := false
	foundGoLatest := false
	foundDotnetDefault := false
	foundDotnetLatest := false
	foundPHPDefault := false
	foundPHPLatest := false
	for _, version := range versions {
		if version.BuildStrategy != model.LongVersionBuildStrategyCNB {
			t.Fatalf("expected cnb build strategy for %s-%s, got %q", version.Lang, version.Version, version.BuildStrategy)
		}
		if !version.IsAllowed {
			t.Fatalf("expected cnb version %s-%s to be allowed", version.Lang, version.Version)
		}
		if version.Lang == "openJDK" && version.Version == "17" {
			foundJava = true
		}
		if version.Lang == "node" && version.Version == "24.13.0" {
			foundNode = true
		}
		if version.Lang == "golang" && version.Version == "1.25" && version.FirstChoice {
			foundGoDefault = true
		}
		if version.Lang == "golang" && version.Version == "1.26" {
			foundGoLatest = true
		}
		if version.Lang == "dotnet" && version.Version == "8.0" && version.FirstChoice {
			foundDotnetDefault = true
		}
		if version.Lang == "dotnet" && version.Version == "10.0" {
			foundDotnetLatest = true
		}
		if version.Lang == "php" && version.Version == "8.4" && version.FirstChoice {
			foundPHPDefault = true
		}
		if version.Lang == "php" && version.Version == "8.5" {
			foundPHPLatest = true
		}
	}
	if !foundJava {
		t.Fatal("expected Java CNB seed version 17")
	}
	if !foundNode {
		t.Fatal("expected Node CNB seed version 24.13.0")
	}
	if !foundGoDefault {
		t.Fatal("expected Golang CNB default seed version 1.25")
	}
	if !foundGoLatest {
		t.Fatal("expected Golang CNB seed version 1.26")
	}
	if !foundDotnetDefault {
		t.Fatal("expected Dotnet CNB default seed version 8.0")
	}
	if !foundDotnetLatest {
		t.Fatal("expected Dotnet CNB seed version 10.0")
	}
	if !foundPHPDefault {
		t.Fatal("expected PHP CNB default seed version 8.4")
	}
	if !foundPHPLatest {
		t.Fatal("expected PHP CNB seed version 8.5")
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

func TestDeduplicateLanguageVersionsRemovesHistoricalDuplicates(t *testing.T) {
	db := newLongVersionPatchTestDB(t)
	defer db.Close()

	versions := []*model.EnterpriseLanguageVersion{
		{
			Lang:          "openJDK",
			Version:       "1.8",
			BuildStrategy: model.LongVersionBuildStrategySlug,
			FirstChoice:   false,
			Show:          true,
			System:        true,
			IsAllowed:     true,
			FileName:      "OpenJDK1.8.tar.gz",
		},
		{
			Lang:          "openJDK",
			Version:       "1.8",
			BuildStrategy: "",
			FirstChoice:   true,
			Show:          true,
			System:        true,
			IsAllowed:     true,
			FileName:      "OpenJDK1.8.tar.gz",
		},
		{
			Lang:          "openJDK",
			Version:       "1.8",
			BuildStrategy: model.LongVersionBuildStrategySlug,
			FirstChoice:   false,
			Show:          true,
			System:        true,
			IsAllowed:     true,
			FileName:      "OpenJDK1.8.tar.gz",
		},
		{
			Lang:          "openJDK",
			Version:       "1.8",
			BuildStrategy: model.LongVersionBuildStrategySlug,
			FirstChoice:   false,
			Show:          true,
			System:        true,
			IsAllowed:     true,
			FileName:      "OpenJDK1.8.tar.gz",
		},
	}
	for _, version := range versions {
		if err := db.Create(version).Error; err != nil {
			t.Fatalf("create duplicate version: %v", err)
		}
	}

	if err := backfillLegacyLongVersionStrategy(db); err != nil {
		t.Fatalf("backfill legacy strategy before dedup: %v", err)
	}
	if err := deduplicateLanguageVersions(db); err != nil {
		t.Fatalf("deduplicate language versions: %v", err)
	}

	var rows []model.EnterpriseLanguageVersion
	if err := db.Where("lang = ? AND version = ?", "openJDK", "1.8").Find(&rows).Error; err != nil {
		t.Fatalf("query deduplicated rows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 historical row after deduplication, got %d", len(rows))
	}
	if rows[0].BuildStrategy != model.LongVersionBuildStrategySlug {
		t.Fatalf("expected deduplicated row to use slug strategy, got %q", rows[0].BuildStrategy)
	}
	if !rows[0].FirstChoice {
		t.Fatal("expected deduplicated row to preserve first_choice=true")
	}

	manager := &Manager{
		db:     db,
		config: config.Config{DBType: "sqlite"},
	}
	if err := manager.patchLanguageVersionUniqueIndex(); err != nil {
		t.Fatalf("patch unique index after deduplication: %v", err)
	}
	if err := db.Create(&model.EnterpriseLanguageVersion{
		Lang:          "openJDK",
		Version:       "1.8",
		BuildStrategy: model.LongVersionBuildStrategySlug,
		Show:          true,
		IsAllowed:     true,
	}).Error; err == nil {
		t.Fatal("expected unique index to reject duplicate openJDK slug row")
	}
}

func TestLongVersionDeduplicationOrderClausesQuoteReservedColumnsForMySQL(t *testing.T) {
	db := newMySQLDialectPatchTestDB(t)

	clauses := longVersionDeduplicationOrderClauses(db)

	joined := strings.Join(clauses, ",")
	if strings.Contains(joined, "system DESC") && !strings.Contains(joined, "`system` DESC") {
		t.Fatalf("expected quoted system order clause, got %q", joined)
	}
	if !strings.Contains(joined, "`system` DESC") {
		t.Fatalf("expected mysql deduplication order clause to quote system column, got %q", joined)
	}
}
