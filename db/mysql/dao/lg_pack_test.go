package dao

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

func newLongVersionTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "long-version.db"))
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

func newMySQLDialectLongVersionTestDB(t *testing.T) *gorm.DB {
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

func mustCreateVersion(t *testing.T, db *gorm.DB, version *model.EnterpriseLanguageVersion) {
	t.Helper()
	if err := db.Create(version).Error; err != nil {
		t.Fatalf("create language version: %v", err)
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func TestLongVersionDaoUsesSlugStrategyByDefault(t *testing.T) {
	db := newLongVersionTestDB(t)
	defer db.Close()

	mustCreateVersion(t, db, &model.EnterpriseLanguageVersion{
		Lang:          "python",
		Version:       "python-3.11",
		BuildStrategy: model.LongVersionBuildStrategySlug,
		Show:          true,
		IsAllowed:     true,
	})
	mustCreateVersion(t, db, &model.EnterpriseLanguageVersion{
		Lang:          "python",
		Version:       "python-3.11",
		BuildStrategy: model.LongVersionBuildStrategyCNB,
		Show:          true,
		IsAllowed:     true,
	})

	dao := &LongVersionDaoImpl{DB: db}

	versions, err := dao.ListVersionByLanguage("python", "")
	if err != nil {
		t.Fatalf("list version by language: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 slug version, got %d", len(versions))
	}
	if versions[0].BuildStrategy != model.LongVersionBuildStrategySlug {
		t.Fatalf("expected slug strategy, got %q", versions[0].BuildStrategy)
	}

	version, err := dao.GetVersionByLanguageAndVersion("python", "python-3.11")
	if err != nil {
		t.Fatalf("get version by language and version: %v", err)
	}
	if version.BuildStrategy != model.LongVersionBuildStrategySlug {
		t.Fatalf("expected slug strategy, got %q", version.BuildStrategy)
	}
}

func TestLongVersionDaoListVersionByLanguageAndStrategy(t *testing.T) {
	db := newLongVersionTestDB(t)
	defer db.Close()

	mustCreateVersion(t, db, &model.EnterpriseLanguageVersion{
		Lang:          "python",
		Version:       "3.11",
		BuildStrategy: model.LongVersionBuildStrategyCNB,
		Show:          true,
		IsAllowed:     true,
	})
	mustCreateVersion(t, db, &model.EnterpriseLanguageVersion{
		Lang:          "python",
		Version:       "3.12",
		BuildStrategy: model.LongVersionBuildStrategyCNB,
		Show:          false,
		IsAllowed:     true,
	})

	dao := &LongVersionDaoImpl{DB: db}

	versions, err := dao.ListVersionByLanguageAndStrategy("python", "true", model.LongVersionBuildStrategyCNB)
	if err != nil {
		t.Fatalf("list version by language and strategy: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 visible cnb version, got %d", len(versions))
	}
	if versions[0].Version != "3.11" {
		t.Fatalf("expected visible version 3.11, got %q", versions[0].Version)
	}
}

func TestLongVersionDaoListVersionByLanguageAndStrategyDeduplicatesHistoricalRows(t *testing.T) {
	db := newLongVersionTestDB(t)
	defer db.Close()

	for i := 0; i < 4; i++ {
		mustCreateVersion(t, db, &model.EnterpriseLanguageVersion{
			Lang:          "openJDK",
			Version:       "1.8",
			BuildStrategy: model.LongVersionBuildStrategySlug,
			FirstChoice:   i == 3,
			Show:          true,
			IsAllowed:     true,
		})
	}

	dao := &LongVersionDaoImpl{DB: db}

	versions, err := dao.ListVersionByLanguageAndStrategy("openJDK", "true", model.LongVersionBuildStrategySlug)
	if err != nil {
		t.Fatalf("list deduplicated versions by language and strategy: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 deduplicated slug version, got %d", len(versions))
	}
	if versions[0].Version != "1.8" {
		t.Fatalf("expected slug version 1.8, got %q", versions[0].Version)
	}
	if !versions[0].FirstChoice {
		t.Fatal("expected deduplicated version to keep first_choice=true")
	}
}

func TestLongVersionOrderClausesQuoteReservedColumnsForMySQL(t *testing.T) {
	db := newMySQLDialectLongVersionTestDB(t)

	clauses := longVersionOrderClauses(db)

	joined := strings.Join(clauses, ",")
	if strings.Contains(joined, "system DESC") && !strings.Contains(joined, "`system` DESC") {
		t.Fatalf("expected quoted system order clause, got %q", joined)
	}
	if !strings.Contains(joined, "`system` DESC") {
		t.Fatalf("expected mysql order clause to quote system column, got %q", joined)
	}
}

func TestLongVersionDaoCreateLangVersionAllowsSameVersionAcrossStrategies(t *testing.T) {
	db := newLongVersionTestDB(t)
	defer db.Close()

	mustCreateVersion(t, db, &model.EnterpriseLanguageVersion{
		Lang:          "python",
		Version:       "3.11",
		BuildStrategy: model.LongVersionBuildStrategySlug,
		Show:          true,
		IsAllowed:     true,
	})

	dao := &LongVersionDaoImpl{DB: db}

	created, err := dao.CreateLangVersion("python", "3.11", "event-cnb", "Python3.11.tar.gz", model.LongVersionBuildStrategyCNB, true, true)
	if err != nil {
		t.Fatalf("create cnb language version: %v", err)
	}
	if created.BuildStrategy != model.LongVersionBuildStrategyCNB {
		t.Fatalf("expected created strategy cnb, got %q", created.BuildStrategy)
	}
	if !created.IsAllowed {
		t.Fatal("expected created version to default is_allowed=true")
	}

	var count int
	if err := db.Model(&model.EnterpriseLanguageVersion{}).Where("lang = ? AND version = ?", "python", "3.11").Count(&count).Error; err != nil {
		t.Fatalf("count created rows: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected duplicate lang/version across strategies to be allowed, got %d rows", count)
	}
}

func TestLongVersionDaoDefaultLangVersionScopesByStrategy(t *testing.T) {
	db := newLongVersionTestDB(t)
	defer db.Close()

	mustCreateVersion(t, db, &model.EnterpriseLanguageVersion{
		Lang:          "python",
		Version:       "3.10",
		BuildStrategy: model.LongVersionBuildStrategySlug,
		FirstChoice:   true,
		Show:          true,
		IsAllowed:     true,
	})
	mustCreateVersion(t, db, &model.EnterpriseLanguageVersion{
		Lang:          "python",
		Version:       "3.11",
		BuildStrategy: model.LongVersionBuildStrategyCNB,
		FirstChoice:   true,
		Show:          true,
		IsAllowed:     true,
	})
	mustCreateVersion(t, db, &model.EnterpriseLanguageVersion{
		Lang:          "python",
		Version:       "3.12",
		BuildStrategy: model.LongVersionBuildStrategyCNB,
		FirstChoice:   false,
		Show:          false,
		IsAllowed:     true,
	})

	dao := &LongVersionDaoImpl{DB: db}

	updated, err := dao.DefaultLangVersion("python", "3.12", model.LongVersionBuildStrategyCNB, true, true, boolPtr(false))
	if err != nil {
		t.Fatalf("set default lang version: %v", err)
	}
	if updated.BuildStrategy != model.LongVersionBuildStrategyCNB {
		t.Fatalf("expected updated strategy cnb, got %q", updated.BuildStrategy)
	}
	if updated.IsAllowed {
		t.Fatal("expected update to set is_allowed=false")
	}

	cnbDefault, err := dao.GetDefaultVersionByLanguageAndStrategy("python", model.LongVersionBuildStrategyCNB)
	if err != nil {
		t.Fatalf("get cnb default version: %v", err)
	}
	if cnbDefault.Version != "3.12" {
		t.Fatalf("expected cnb default version 3.12, got %q", cnbDefault.Version)
	}

	slugDefault, err := dao.GetDefaultVersionByLanguageAndStrategy("python", model.LongVersionBuildStrategySlug)
	if err != nil {
		t.Fatalf("get slug default version: %v", err)
	}
	if slugDefault.Version != "3.10" {
		t.Fatalf("expected slug default version 3.10 to remain unchanged, got %q", slugDefault.Version)
	}
}
