package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/db/dameng"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/sirupsen/logrus"

	_ "github.com/go-sql-driver/mysql"
)

type databaseBackend interface {
	validate() error
	open(config.Config) (*gorm.DB, error)
	defaultSchemaAction() schemaAction
	hasTable(*gorm.DB, model.Interface) (bool, error)
	ensureSchema(*Manager) error
}

type databaseBackendFactory func() databaseBackend

var databaseBackendRegistry = map[string]databaseBackendFactory{
	"cockroachdb": func() databaseBackend { return &cockroachDatabaseBackend{} },
	"dm":          func() databaseBackend { return &damengDatabaseBackend{} },
	"mysql":       func() databaseBackend { return &mysqlDatabaseBackend{} },
	"sqlite":      func() databaseBackend { return &sqliteDatabaseBackend{} },
}

func lookupDatabaseBackend(dbType string) (databaseBackend, error) {
	factory, ok := databaseBackendRegistry[strings.ToLower(strings.TrimSpace(dbType))]
	if !ok {
		return nil, fmt.Errorf("DB drivers: %s not supported", dbType)
	}
	return factory(), nil
}

// ValidateConfig rejects unsupported database types and binaries that do not
// contain the selected proprietary driver before the retrying connection loop.
func ValidateConfig(dbConfig config.Config) error {
	backend, err := lookupDatabaseBackend(dbConfig.DBType)
	if err != nil {
		return err
	}
	return backend.validate()
}

type mysqlDatabaseBackend struct{}

func (b *mysqlDatabaseBackend) validate() error { return nil }

func (b *mysqlDatabaseBackend) open(dbConfig config.Config) (*gorm.DB, error) {
	logrus.Info("mysql db driver create")
	db, err := gorm.Open("mysql", dbConfig.MysqlConnectionInfo+"?charset=utf8mb4&parseTime=True&loc=Local")
	if err != nil {
		return nil, err
	}
	for {
		if err := db.DB().Ping(); err != nil {
			logrus.Errorf("failed to connect to database: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}
		logrus.Info("database connection successful")
		break
	}
	configureConnectionPool(db.DB())
	return db, nil
}

func (b *mysqlDatabaseBackend) defaultSchemaAction() schemaAction { return schemaMigrate }

func (b *mysqlDatabaseBackend) hasTable(db *gorm.DB, md model.Interface) (bool, error) {
	return db.HasTable(md), nil
}

func (b *mysqlDatabaseBackend) ensureSchema(manager *Manager) error {
	return ensureStandardSchema(manager, "ENGINE=InnoDB charset=utf8mb4", patchMySQLSchema)
}

type damengDatabaseBackend struct {
	schema string
}

func (b *damengDatabaseBackend) validate() error {
	if !dameng.DriverBuilt() {
		return dameng.ErrDriverNotBuilt
	}
	return nil
}

func (b *damengDatabaseBackend) open(dbConfig config.Config) (*gorm.DB, error) {
	connectionInfo, err := dameng.NormalizeDSN(dbConfig.MysqlConnectionInfo)
	if err != nil {
		return nil, errors.New("invalid Dameng connection information")
	}
	b.schema, err = dameng.SchemaName(connectionInfo)
	if err != nil {
		return nil, errors.New("invalid Dameng connection information")
	}
	db, err := gorm.Open("dm", connectionInfo)
	if err != nil {
		return nil, damengOpenError(err)
	}
	for {
		if err := db.DB().Ping(); err != nil {
			logrus.Error("failed to connect to Dameng database")
			time.Sleep(2 * time.Second)
			continue
		}
		logrus.Info("Dameng database connection successful")
		break
	}
	configureConnectionPool(db.DB())
	return db, nil
}

func (b *damengDatabaseBackend) defaultSchemaAction() schemaAction { return schemaVerify }

const damengTableCatalogQuery = `SELECT COUNT(*) FROM SYS.SYSOBJECTS SCHEMAS, SYS.SYSOBJECTS TABLES
WHERE SCHEMAS.ID = TABLES.SCHID
  AND SCHEMAS.TYPE$ = 'SCH'
  AND SCHEMAS.NAME = ?
  AND TABLES.TYPE$ = 'SCHOBJ'
  AND TABLES.SUBTYPE$ = 'UTAB'
  AND TABLES.NAME = ?`

func (b *damengDatabaseBackend) hasTable(db *gorm.DB, md model.Interface) (bool, error) {
	if b.schema == "" {
		return db.HasTable(md), nil
	}
	var count int
	if err := db.Raw(damengTableCatalogQuery, b.schema, md.TableName()).Row().Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (b *damengDatabaseBackend) ensureSchema(manager *Manager) error {
	for _, md := range manager.models {
		exists, err := b.hasTable(manager.db, md)
		if err != nil {
			return fmt.Errorf("check Dameng table %q: %w", md.TableName(), err)
		}
		if exists {
			continue
		}
		if err := manager.db.CreateTable(md).Error; err != nil {
			return fmt.Errorf("initialize Dameng table %q: %w", md.TableName(), err)
		}
		logrus.Infof("auto create table %s to Dameng success", md.TableName())
	}
	return patchDamengSchema(manager)
}

type cockroachDatabaseBackend struct{}

func (b *cockroachDatabaseBackend) validate() error { return nil }

func (b *cockroachDatabaseBackend) open(dbConfig config.Config) (*gorm.DB, error) {
	return gorm.Open("postgres", dbConfig.MysqlConnectionInfo)
}

func (b *cockroachDatabaseBackend) defaultSchemaAction() schemaAction { return schemaMigrate }

func (b *cockroachDatabaseBackend) hasTable(db *gorm.DB, md model.Interface) (bool, error) {
	return db.HasTable(md), nil
}

func (b *cockroachDatabaseBackend) ensureSchema(manager *Manager) error {
	return ensureStandardSchema(manager, "", patchGenericSchema)
}

type sqliteDatabaseBackend struct{}

func (b *sqliteDatabaseBackend) validate() error { return nil }

func (b *sqliteDatabaseBackend) open(config.Config) (*gorm.DB, error) {
	if _, err := os.Stat("/db"); err != nil && !os.IsExist(err) {
		if err := os.MkdirAll("/db", 0777); err != nil {
			return nil, err
		}
	}
	db, err := gorm.Open("sqlite3", "/db/region.sqlite3")
	if err != nil {
		return nil, err
	}
	db.Exec("PRAGMA journal_mode = WAL")
	return db, nil
}

func (b *sqliteDatabaseBackend) defaultSchemaAction() schemaAction { return schemaMigrate }

func (b *sqliteDatabaseBackend) hasTable(db *gorm.DB, md model.Interface) (bool, error) {
	return db.HasTable(md), nil
}

func (b *sqliteDatabaseBackend) ensureSchema(manager *Manager) error {
	return ensureStandardSchema(manager, "", patchSQLiteSchema)
}

func ensureStandardSchema(manager *Manager, tableOptions string, patch func(*Manager) error) error {
	for _, md := range manager.models {
		exists, err := manager.backend.hasTable(manager.db, md)
		if err != nil {
			logrus.Errorf("check table %s error: %v", md.TableName(), err)
			continue
		}
		if !exists {
			db := manager.db
			if tableOptions != "" {
				db = db.Set("gorm:table_options", tableOptions)
			}
			if err := db.CreateTable(md).Error; err != nil {
				logrus.Errorf("auto create table %s to db error.%s", md.TableName(), err.Error())
			} else {
				logrus.Infof("auto create table %s to db success", md.TableName())
			}
			continue
		}
		if err := manager.db.AutoMigrate(md).Error; err != nil {
			logrus.Errorf("auto Migrate table %s to db error.%s", md.TableName(), err.Error())
		}
	}
	return patch(manager)
}

func patchCommonSchema(manager *Manager, patchIndex func(*Manager) error) error {
	count := -1
	if err := backfillLegacyLongVersionStrategy(manager.db); err != nil {
		logrus.Errorf("backfill legacy language versions error: %s", err.Error())
	}
	if err := deduplicateLanguageVersions(manager.db); err != nil {
		logrus.Errorf("deduplicate enterprise_language_version rows error: %s", err.Error())
	}
	if patchIndex != nil {
		if err := patchIndex(manager); err != nil {
			logrus.Errorf("patch enterprise_language_version unique index error: %s", err.Error())
		}
	}
	manager.db.Model(&model.EnterpriseLanguageVersion{}).Count(&count)
	if count == 0 {
		manager.initLanguageVersion()
	} else {
		manager.updateLanguageVersions()
	}
	return nil
}

func patchGenericSchema(manager *Manager) error {
	return patchCommonSchema(manager, nil)
}

func patchSQLiteSchema(manager *Manager) error {
	return patchCommonSchema(manager, patchSQLiteLanguageVersionUniqueIndex)
}

func patchSQLiteLanguageVersionUniqueIndex(manager *Manager) error {
	if err := manager.db.Exec("DROP INDEX IF EXISTS lang_version_unique;").Error; err != nil {
		return err
	}
	return manager.db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS lang_version_unique ON enterprise_language_version(lang, version, build_strategy);").Error
}

func patchMySQLSchema(manager *Manager) error {
	if err := patchCommonSchema(manager, patchMySQLLanguageVersionUniqueIndex); err != nil {
		return err
	}
	statements := []struct {
		name string
		sql  string
		args []interface{}
	}{
		{name: "tenant_services_envs", sql: "alter table tenant_services_envs modify column attr_value text;"},
		{name: "tenant_services_event", sql: "alter table tenant_services_event modify column request_body varchar(1024);"},
		{name: "gateway_tcp_rule", sql: "update gateway_tcp_rule set ip=? where ip=?", args: []interface{}{"0.0.0.0", ""}},
		{name: "tenant_services_volume", sql: "alter table tenant_services_volume modify column volume_type varchar(64);"},
		{name: "tenants", sql: "update tenants set namespace=uuid where namespace is NULL;"},
		{name: "applications", sql: "update applications set k8s_app=concat('app-',LEFT(app_id,8)) where k8s_app is NULL;"},
		{name: "tenant_services", sql: "update tenant_services set k8s_component_name=service_alias where k8s_component_name is NULL;"},
		{name: "tenant_services_probe", sql: "alter table tenant_services_probe modify column cmd text;"},
		{name: "app_config_group_item", sql: "alter table app_config_group_item modify column item_value text;"},
		{name: "applications", sql: "alter table applications modify column governance_mode varchar(255) DEFAULT 'KUBERNETES_NATIVE_SERVICE';"},
		{name: "tenant_services_volume_type", sql: "alter table tenant_services_volume_type modify column storage_class_detail text;"},
	}
	for _, statement := range statements {
		if err := manager.db.Exec(statement.sql, statement.args...).Error; err != nil {
			logrus.Errorf("patch table %s error: %s", statement.name, err.Error())
		}
	}
	return nil
}

func patchMySQLLanguageVersionUniqueIndex(manager *Manager) error {
	var columns sql.NullString
	row := manager.db.Raw("SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?", "enterprise_language_version", "lang_version_unique").Row()
	if err := row.Scan(&columns); err != nil {
		return err
	}
	if columns.Valid && columns.String == "lang,version,build_strategy" {
		return nil
	}
	if columns.Valid && columns.String != "" {
		if err := manager.db.Exec("alter table enterprise_language_version drop index lang_version_unique;").Error; err != nil {
			return err
		}
	}
	return manager.db.Exec("alter table enterprise_language_version add unique index lang_version_unique (lang, version, build_strategy);").Error
}

func patchDamengSchema(manager *Manager) error {
	if err := backfillLegacyLongVersionStrategy(manager.db); err != nil {
		return fmt.Errorf("backfill legacy language versions: %w", err)
	}
	if err := deduplicateLanguageVersions(manager.db); err != nil {
		return fmt.Errorf("deduplicate language versions: %w", err)
	}
	if err := syncDamengLanguageVersions(manager); err != nil {
		return fmt.Errorf("seed language versions: %w", err)
	}
	return nil
}

func syncDamengLanguageVersions(manager *Manager) error {
	for _, version := range allSeedLanguageVersions() {
		var existing model.EnterpriseLanguageVersion
		err := manager.db.Where("lang = ? AND version = ? AND build_strategy = ?", version.Lang, version.Version, version.BuildStrategy).First(&existing).Error
		if err != nil {
			if !gorm.IsRecordNotFoundError(err) {
				return err
			}
			if err := manager.db.Create(version).Error; err != nil {
				return err
			}
			continue
		}
		if !version.System {
			continue
		}
		existing.FirstChoice = version.FirstChoice
		existing.FileName = version.FileName
		existing.Show = version.Show
		existing.System = version.System
		existing.BuildStrategy = version.BuildStrategy
		existing.IsAllowed = version.IsAllowed
		if err := manager.db.Save(&existing).Error; err != nil {
			return err
		}
	}
	return retireMissingSystemCNBVersions(manager.db, cnbSeedLanguageVersions())
}

var damengDSNCredentialsPattern = regexp.MustCompile(`(?i)(dm://[^:/\s]+:)[^@\s]*@`)

type damengOpenFailure struct {
	cause error
}

func (e damengOpenFailure) Error() string {
	return "unable to open Dameng database: " + damengDSNCredentialsPattern.ReplaceAllString(e.cause.Error(), "${1}<redacted>@")
}

func (e damengOpenFailure) Unwrap() error {
	return e.cause
}

func damengOpenError(err error) error {
	return damengOpenFailure{cause: err}
}
