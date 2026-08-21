// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package mysql

import (
	"fmt"
	"strings"
	"sync"

	"github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/db/portable"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

// Manager db manager
type Manager struct {
	db      *gorm.DB
	config  config.Config
	backend databaseBackend
	initOne sync.Once
	initErr error
	models  []model.Interface
}

type schemaAction string

const (
	schemaMigrate schemaAction = "migrate"
	schemaVerify  schemaAction = "verify"
)

type cnbSeedVersion struct {
	Version string
	Default bool
}

// CreateManager create manager
func CreateManager(dbConfig config.Config) (*Manager, error) {
	backend, err := lookupDatabaseBackend(dbConfig.DBType)
	if err != nil {
		return nil, err
	}
	if err := backend.validate(); err != nil {
		return nil, err
	}
	db, err := backend.open(dbConfig)
	if err != nil {
		return nil, err
	}
	if dbConfig.ShowSQL {
		db = db.Debug()
	}
	logrus.Info("db init success")
	manager := &Manager{
		db:      db,
		config:  dbConfig,
		backend: backend,
		initOne: sync.Once{},
	}

	db.SetLogger(manager)
	logrus.Info("register table model")
	manager.RegisterTableModel()
	if manager.schemaAction() == schemaVerify {
		logrus.Info("verify database schema")
		if err := manager.VerifySchema(); err != nil {
			_ = manager.CloseManager()
			return nil, err
		}
	} else {
		logrus.Info("check table")
		if err := manager.CheckTable(); err != nil {
			_ = manager.CloseManager()
			return nil, err
		}
	}
	logrus.Debug("database manager created")
	return manager, nil
}

// CloseManager 关闭管理器
func (m *Manager) CloseManager() error {
	return m.db.Close()
}

// Begin begin a transaction
func (m *Manager) Begin() *gorm.DB {
	return m.db.Begin()
}

// DB returns the db.
func (m *Manager) DB() *gorm.DB {
	return m.db
}

// EnsureEndTransactionFunc -
func (m *Manager) EnsureEndTransactionFunc() func(tx *gorm.DB) {
	return func(tx *gorm.DB) {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}
}

// Print Print
func (m *Manager) Print(v ...interface{}) {
	logrus.Info(v...)
}

// RegisterTableModel register table model
func (m *Manager) RegisterTableModel() {
	m.models = append(m.models, &model.Tenants{})
	m.models = append(m.models, &model.TenantServices{})
	m.models = append(m.models, &model.TenantServicesPort{})
	m.models = append(m.models, &model.TenantServiceRelation{})
	m.models = append(m.models, &model.TenantServiceEnvVar{})
	m.models = append(m.models, &model.TenantServiceMountRelation{})
	m.models = append(m.models, &model.TenantServiceVolume{})
	m.models = append(m.models, &model.TenantServiceLable{})
	m.models = append(m.models, &model.TenantServiceProbe{})
	m.models = append(m.models, &model.LicenseInfo{})
	m.models = append(m.models, &model.TenantServicesDelete{})
	m.models = append(m.models, &model.TenantServiceLBMappingPort{})
	m.models = append(m.models, &model.TenantPlugin{})
	m.models = append(m.models, &model.TenantPluginBuildVersion{})
	m.models = append(m.models, &model.TenantServicePluginRelation{})
	m.models = append(m.models, &model.TenantPluginVersionEnv{})
	m.models = append(m.models, &model.TenantPluginVersionDiscoverConfig{})
	m.models = append(m.models, &model.CodeCheckResult{})
	m.models = append(m.models, &model.ServiceEvent{})
	m.models = append(m.models, &model.VersionInfo{})
	m.models = append(m.models, &model.TenantServicesStreamPluginPort{})
	m.models = append(m.models, &model.RegionProcotols{})
	m.models = append(m.models, &model.LocalScheduler{})
	m.models = append(m.models, &model.NotificationEvent{})
	m.models = append(m.models, &model.AppStatus{})
	m.models = append(m.models, &model.AppBackup{})
	m.models = append(m.models, &model.UploadSession{})
	m.models = append(m.models, &model.ServiceSourceConfig{})
	m.models = append(m.models, &model.Application{})
	m.models = append(m.models, &model.ApplicationConfigGroup{})
	m.models = append(m.models, &model.ConfigGroupService{})
	m.models = append(m.models, &model.ConfigGroupItem{})
	// gateway
	m.models = append(m.models, &model.Certificate{})
	m.models = append(m.models, &model.RuleExtension{})
	m.models = append(m.models, &model.HTTPRule{})
	m.models = append(m.models, &model.HTTPRuleRewrite{})
	m.models = append(m.models, &model.TCPRule{})
	m.models = append(m.models, &model.TenantServiceConfigFile{})
	m.models = append(m.models, &model.Endpoint{})
	m.models = append(m.models, &model.ThirdPartySvcDiscoveryCfg{})
	m.models = append(m.models, &model.GwRuleConfig{})

	// volumeType
	m.models = append(m.models, &model.TenantServiceVolumeType{})
	// pod autoscaler
	m.models = append(m.models, &model.TenantServiceAutoscalerRules{})
	m.models = append(m.models, &model.TenantServiceAutoscalerRuleMetrics{})
	m.models = append(m.models, &model.TenantServiceScalingRecords{})
	m.models = append(m.models, &model.TenantServiceMonitor{})
	m.models = append(m.models, &model.ComponentK8sAttributes{})
	m.models = append(m.models, &model.K8sResource{})
	m.models = append(m.models, &model.KeyValue{})
	m.models = append(m.models, &model.EnterpriseLanguageVersion{})
	m.models = append(m.models, &model.EnterpriseOverScore{})
}

func (m *Manager) schemaAction() schemaAction {
	switch strings.ToLower(strings.TrimSpace(m.config.SchemaMode)) {
	case string(schemaMigrate):
		return schemaMigrate
	case string(schemaVerify):
		return schemaVerify
	}
	if err := m.ensureBackend(); err != nil {
		return schemaMigrate
	}
	return m.backend.defaultSchemaAction()
}

func (m *Manager) ensureBackend() error {
	if m.backend != nil {
		return nil
	}
	backend, err := lookupDatabaseBackend(m.config.DBType)
	if err != nil {
		return err
	}
	m.backend = backend
	return nil
}

// VerifySchema checks that all registered tables exist without changing the schema.
func (m *Manager) VerifySchema() error {
	for _, md := range m.models {
		exists, err := m.hasTable(md)
		if err != nil {
			return fmt.Errorf("check required database table %q: %w", md.TableName(), err)
		}
		if !exists {
			return fmt.Errorf("required database table %q is missing", md.TableName())
		}
	}
	return nil
}

// CheckTable creates or migrates database tables according to the configured database dialect.
func (m *Manager) CheckTable() error {
	m.initOne.Do(func() {
		if err := m.ensureBackend(); err != nil {
			m.initErr = err
			return
		}
		m.initErr = m.backend.ensureSchema(m)
	})
	return m.initErr
}
func (m *Manager) hasTable(md model.Interface) (bool, error) {
	if err := m.ensureBackend(); err != nil {
		return false, err
	}
	return m.backend.hasTable(m.db, md)
}

func (m *Manager) initLanguageVersion() {
	versions := allSeedLanguageVersions()
	var objects []interface{}
	for _, version := range versions {
		objects = append(objects, *version)
	}
	if err := portable.BulkUpsert(m.db, objects, 2000, "lang", "version", "build_strategy"); err != nil {
		logrus.Errorf("create K8sResource groups in batch failure: %v", err)
	}
}

func (m *Manager) updateLanguageVersions() {
	versions := allSeedLanguageVersions()
	cnbVersions := cnbSeedLanguageVersions()
	var objects []interface{}
	for _, version := range versions {
		// Only update system versions
		if version.System {
			objects = append(objects, *version)
		}
	}
	if len(objects) > 0 {
		if err := portable.BulkUpsert(m.db, objects, 2000, "lang", "version", "build_strategy"); err != nil {
			logrus.Errorf("update language versions in batch failure: %v", err)
		} else {
			logrus.Info("successfully updated language versions")
		}
	}
	if err := retireMissingSystemCNBVersions(m.db, cnbVersions); err != nil {
		logrus.Errorf("retire obsolete cnb language versions failure: %v", err)
	}
}

func applySeedLongVersionDefaults(versions []*model.EnterpriseLanguageVersion) {
	for _, version := range versions {
		if version == nil {
			continue
		}
		version.BuildStrategy = model.LongVersionBuildStrategySlug
		version.IsAllowed = true
	}
}

func slugSeedLanguageVersions() []*model.EnterpriseLanguageVersion {
	var versions []*model.EnterpriseLanguageVersion
	versions = append(versions, GolangInitVersion...)
	versions = append(versions, NodeInitVersion...)
	versions = append(versions, WebCompilerInitVersion...)
	versions = append(versions, OpenJDKInitVersion...)
	versions = append(versions, MavenInitVersion...)
	versions = append(versions, PythonInitVersion...)
	versions = append(versions, NetRuntimeInitVersion...)
	versions = append(versions, NetCompilerInitVersion...)
	versions = append(versions, PHPInitVersion...)
	versions = append(versions, WebRuntimeInitVersion...)
	applySeedLongVersionDefaults(versions)
	return versions
}

func cnbSeedLanguageVersions() []*model.EnterpriseLanguageVersion {
	var versions []*model.EnterpriseLanguageVersion
	versions = append(versions, buildStrategySeedVersions("openJDK", model.LongVersionBuildStrategyCNB, []cnbSeedVersion{
		{Version: "8", Default: false},
		{Version: "11", Default: false},
		{Version: "17", Default: true},
		{Version: "21", Default: false},
		{Version: "25", Default: false},
	})...)
	versions = append(versions, buildStrategySeedVersions("node", model.LongVersionBuildStrategyCNB, []cnbSeedVersion{
		{Version: "18.20.7", Default: false},
		{Version: "18.20.8", Default: false},
		{Version: "20.19.6", Default: false},
		{Version: "20.20.0", Default: false},
		{Version: "22.21.1", Default: false},
		{Version: "22.22.0", Default: false},
		{Version: "24.12.0", Default: false},
		{Version: "24.13.0", Default: true},
	})...)
	versions = append(versions, buildStrategySeedVersions("python", model.LongVersionBuildStrategyCNB, []cnbSeedVersion{
		{Version: "3.10", Default: false},
		{Version: "3.11", Default: false},
		{Version: "3.12", Default: false},
		{Version: "3.13", Default: false},
		{Version: "3.14", Default: true},
	})...)
	versions = append(versions, buildStrategySeedVersions("golang", model.LongVersionBuildStrategyCNB, []cnbSeedVersion{
		{Version: "1.24", Default: false},
		{Version: "1.25", Default: true},
	})...)
	versions = append(versions, buildStrategySeedVersions("dotnet", model.LongVersionBuildStrategyCNB, []cnbSeedVersion{
		{Version: "8.0", Default: true},
		{Version: "9.0", Default: false},
		{Version: "10.0", Default: false},
	})...)
	versions = append(versions, buildStrategySeedVersions("php", model.LongVersionBuildStrategyCNB, []cnbSeedVersion{
		{Version: "8.1", Default: false},
		{Version: "8.2", Default: false},
		{Version: "8.3", Default: true},
	})...)
	return versions
}

func allSeedLanguageVersions() []*model.EnterpriseLanguageVersion {
	versions := slugSeedLanguageVersions()
	versions = append(versions, cnbSeedLanguageVersions()...)
	return versions
}

func buildStrategySeedVersions(lang, buildStrategy string, versions []cnbSeedVersion) []*model.EnterpriseLanguageVersion {
	var records []*model.EnterpriseLanguageVersion
	for _, version := range versions {
		records = append(records, &model.EnterpriseLanguageVersion{
			Lang:          lang,
			Version:       version.Version,
			BuildStrategy: buildStrategy,
			FirstChoice:   version.Default,
			System:        true,
			Show:          true,
			IsAllowed:     true,
		})
	}
	return records
}

func deduplicateLanguageVersions(db *gorm.DB) error {
	var versions []model.EnterpriseLanguageVersion
	query := db
	for _, clause := range longVersionDeduplicationOrderClauses(db) {
		query = query.Order(clause)
	}
	if err := query.Find(&versions).Error; err != nil {
		return err
	}
	if len(versions) < 2 {
		return nil
	}

	type keeperState struct {
		version *model.EnterpriseLanguageVersion
		dirty   bool
	}

	keepers := make(map[string]*keeperState, len(versions))
	var deleteIDs []uint
	for i := range versions {
		version := versions[i]
		version.BuildStrategy = normalizeSeedLongVersionBuildStrategy(version.BuildStrategy)
		key := buildSeedLongVersionDedupKey(version.Lang, version.Version, version.BuildStrategy)

		if existing, ok := keepers[key]; ok {
			if mergeStoredLanguageVersion(existing.version, &version) {
				existing.dirty = true
			}
			deleteIDs = append(deleteIDs, version.ID)
			continue
		}

		keep := version
		keepers[key] = &keeperState{version: &keep}
	}

	if len(deleteIDs) == 0 {
		return nil
	}

	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	for _, keeper := range keepers {
		if !keeper.dirty {
			continue
		}
		if err := tx.Save(keeper.version).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	if err := tx.Where("ID IN (?)", deleteIDs).Delete(&model.EnterpriseLanguageVersion{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func retireMissingSystemCNBVersions(db *gorm.DB, seedVersions []*model.EnterpriseLanguageVersion) error {
	activeVersions := make(map[string]struct{}, len(seedVersions))
	for _, version := range seedVersions {
		if version == nil {
			continue
		}
		activeVersions[buildSeedLongVersionDedupKey(version.Lang, version.Version, model.LongVersionBuildStrategyCNB)] = struct{}{}
	}

	var stored []model.EnterpriseLanguageVersion
	scope := db.NewScope(&model.EnterpriseLanguageVersion{})
	if err := db.Where("build_strategy = ? AND "+scope.Quote("system")+" = ?", model.LongVersionBuildStrategyCNB, true).Find(&stored).Error; err != nil {
		return err
	}

	for i := range stored {
		version := stored[i]
		key := buildSeedLongVersionDedupKey(version.Lang, version.Version, model.LongVersionBuildStrategyCNB)
		if _, ok := activeVersions[key]; ok {
			continue
		}
		if !version.Show && !version.IsAllowed && !version.FirstChoice {
			continue
		}
		version.Show = false
		version.IsAllowed = false
		version.FirstChoice = false
		if err := db.Save(&version).Error; err != nil {
			return err
		}
	}
	return nil
}

func normalizeSeedLongVersionBuildStrategy(buildStrategy string) string {
	if buildStrategy == "" {
		return model.LongVersionBuildStrategySlug
	}
	return buildStrategy
}

func longVersionDeduplicationOrderClauses(db *gorm.DB) []string {
	scope := db.NewScope(&model.EnterpriseLanguageVersion{})
	return []string{
		scope.Quote("lang") + " ASC",
		scope.Quote("version") + " ASC",
		scope.Quote("build_strategy") + " ASC",
		scope.Quote("first_choice") + " DESC",
		scope.Quote("is_show") + " DESC",
		scope.Quote("is_allowed") + " DESC",
		scope.Quote("system") + " DESC",
		scope.Quote("ID") + " ASC",
	}
}

func buildSeedLongVersionDedupKey(lang, version, buildStrategy string) string {
	return lang + "\x00" + version + "\x00" + normalizeSeedLongVersionBuildStrategy(buildStrategy)
}

func mergeStoredLanguageVersion(target, source *model.EnterpriseLanguageVersion) bool {
	changed := false

	if normalized := normalizeSeedLongVersionBuildStrategy(target.BuildStrategy); target.BuildStrategy != normalized {
		target.BuildStrategy = normalized
		changed = true
	}
	if source.FirstChoice && !target.FirstChoice {
		target.FirstChoice = true
		changed = true
	}
	if source.Show && !target.Show {
		target.Show = true
		changed = true
	}
	if source.System && !target.System {
		target.System = true
		changed = true
	}
	if source.IsAllowed && !target.IsAllowed {
		target.IsAllowed = true
		changed = true
	}
	if target.EventID == "" && source.EventID != "" {
		target.EventID = source.EventID
		changed = true
	}
	if target.FileName == "" && source.FileName != "" {
		target.FileName = source.FileName
		changed = true
	}
	return changed
}

func backfillLegacyLongVersionStrategy(db *gorm.DB) error {
	return db.Model(&model.EnterpriseLanguageVersion{}).
		Where("build_strategy IS NULL OR build_strategy = ''").
		Updates(map[string]interface{}{
			"build_strategy": model.LongVersionBuildStrategySlug,
			"is_allowed":     true,
		}).Error
}

// GolangInitVersion -
var GolangInitVersion = []*model.EnterpriseLanguageVersion{
	{
		Lang:        "golang",
		Version:     "go1.25.1",
		FirstChoice: false,
		System:      true,
		FileName:    "go1.25.1.tar.gz",
		Show:        true,
	}, {
		Lang:        "golang",
		Version:     "go1.24.7",
		FirstChoice: false,
		System:      true,
		FileName:    "go1.24.7.tar.gz",
		Show:        true,
	}, {
		Lang:        "golang",
		Version:     "go1.23.12",
		FirstChoice: false,
		System:      true,
		FileName:    "go1.23.12.tar.gz",
		Show:        true,
	}, {
		Lang:        "golang",
		Version:     "go1.22.12",
		FirstChoice: false,
		System:      true,
		FileName:    "go1.22.12.tar.gz",
		Show:        true,
	}, {
		Lang:        "golang",
		Version:     "go1.21.13",
		FirstChoice: false,
		System:      true,
		FileName:    "go1.21.13.tar.gz",
		Show:        true,
	}, {
		Lang:        "golang",
		Version:     "go1.20.4",
		FirstChoice: true,
		System:      true,
		FileName:    "go1.20.4.tar.gz",
		Show:        true,
	}, {
		Lang:        "golang",
		Version:     "go1.19.9",
		FirstChoice: false,
		System:      true,
		FileName:    "go1.19.9.tar.gz",
		Show:        true,
	}, {
		Lang:        "golang",
		Version:     "go1.18.10",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "go1.18.10.tar.gz",
	}, {
		Lang:        "golang",
		Version:     "go1.17.13",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "go1.17.13.tar.gz",
	}, {
		Lang:        "golang",
		Version:     "go1.16.15",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "go1.16.15.tar.gz",
	}, {
		Lang:        "golang",
		Version:     "go1.15.15",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "go1.15.15.tar.gz",
	}, {
		Lang:        "golang",
		Version:     "go1.14.15",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "go1.14.15.tar.gz",
	}, {
		Lang:        "golang",
		Version:     "go1.13.15",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "go1.13.15.tar.gz",
	}, {
		Lang:        "golang",
		Version:     "go1.12.17",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "go1.12.17.tar.gz",
	},
}

// OpenJDKInitVersion -
var OpenJDKInitVersion = []*model.EnterpriseLanguageVersion{
	{
		Lang:        "openJDK",
		Version:     "25",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "OpenJDK25.tar.gz",
	}, {
		Lang:        "openJDK",
		Version:     "21",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "OpenJDK21.tar.gz",
	}, {
		Lang:        "openJDK",
		Version:     "17",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "OpenJDK17.tar.gz",
	}, {
		Lang:        "openJDK",
		Version:     "16",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "OpenJDK16.tar.gz",
	}, {
		Lang:        "openJDK",
		Version:     "15",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "OpenJDK15.tar.gz",
	}, {
		Lang:        "openJDK",
		Version:     "14",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "OpenJDK14.tar.gz",
	}, {
		Lang:        "openJDK",
		Version:     "13",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "OpenJDK13.tar.gz",
	}, {
		Lang:        "openJDK",
		Version:     "12",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "OpenJDK12.tar.gz",
	}, {
		Lang:        "openJDK",
		Version:     "11",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "OpenJDK11.tar.gz",
	}, {
		Lang:        "openJDK",
		Version:     "10",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "OpenJDK10.tar.gz",
	}, {
		Lang:        "openJDK",
		Version:     "1.9",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "OpenJDK1.9.tar.gz",
	},
	{
		Lang:        "openJDK",
		Version:     "1.8",
		FirstChoice: true,
		Show:        true,
		System:      true,
		FileName:    "OpenJDK1.8.tar.gz",
	},
}

// PythonInitVersion -
var PythonInitVersion = []*model.EnterpriseLanguageVersion{
	{
		Lang:        "python",
		Version:     "python-3.9.16",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Python3.9.16.tar.gz",
	}, {
		Lang:        "python",
		Version:     "python-3.8.16",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Python3.8.16.tar.gz",
	}, {
		Lang:        "python",
		Version:     "python-3.7.16",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Python3.7.16.tar.gz",
	}, {
		Lang:        "python",
		Version:     "python-3.6.15",
		FirstChoice: true,
		Show:        true,
		System:      true,
		FileName:    "Python3.6.15.tar.gz",
	}, {
		Lang:        "python",
		Version:     "python-3.5.6",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Python3.5.6.tar.gz",
	}, {
		Lang:        "python",
		Version:     "python-2.7.18",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Python2.7.18.tar.gz",
	},
}

// MavenInitVersion -
var MavenInitVersion = []*model.EnterpriseLanguageVersion{
	{
		Lang:        "maven",
		Version:     "3.9.1",
		FirstChoice: true,
		Show:        true,
		System:      true,
		FileName:    "Maven3.9.1.tar.gz",
	}, {
		Lang:        "maven",
		Version:     "3.8.8",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Maven3.8.8.tar.gz",
	}, {
		Lang:        "maven",
		Version:     "3.6.3",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Maven3.6.3.tar.gz",
	}, {
		Lang:        "maven",
		Version:     "3.5.4",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Maven3.5.4.tar.gz",
	}, {
		Lang:        "maven",
		Version:     "3.3.9",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Maven3.3.9.tar.gz",
	}, {
		Lang:        "maven",
		Version:     "3.2.5",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Maven3.2.5.tar.gz",
	}, {
		Lang:        "maven",
		Version:     "3.1.1",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Maven3.1.1.tar.gz",
	},
}

// PHPInitVersion -
var PHPInitVersion = []*model.EnterpriseLanguageVersion{
	{
		Lang:        "php",
		Version:     "8.2.5",
		FirstChoice: true,
		Show:        true,
		System:      true,
		FileName:    "php8.2.5.tar.gz",
	}, {
		Lang:        "php",
		Version:     "8.1.18",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "php8.1.18.tar.gz",
	},
}

// NodeInitVersion -
var NodeInitVersion = []*model.EnterpriseLanguageVersion{
	{
		Lang:        "node",
		Version:     "24.8.0",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Node24.8.0.tar.gz",
	}, {
		Lang:        "node",
		Version:     "23.11.1",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Node23.11.1.tar.gz",
	}, {
		Lang:        "node",
		Version:     "22.19.0",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Node22.19.0.tar.gz",
	}, {
		Lang:        "node",
		Version:     "21.7.3",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Node21.7.3.tar.gz",
	}, {
		Lang:        "node",
		Version:     "20.19.5",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Node20.19.5.tar.gz",
	}, {
		Lang:        "node",
		Version:     "20.0.0",
		FirstChoice: true,
		Show:        true,
		System:      true,
		FileName:    "Node20.0.0.tar.gz",
	}, {
		Lang:        "node",
		Version:     "19.9.0",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Node19.9.0.tar.gz",
	}, {
		Lang:        "node",
		Version:     "18.16.0",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Node18.16.0.tar.gz",
	}, {
		Lang:        "node",
		Version:     "17.9.1",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Node17.9.1.tar.gz",
	}, {
		Lang:        "node",
		Version:     "16.20.0",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Node16.20.0.tar.gz",
	}, {
		Lang:        "node",
		Version:     "16.15.0",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Node16.15.0.tar.gz",
	}, {
		Lang:        "node",
		Version:     "15.14.0",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Node15.14.0.tar.gz",
	}, {
		Lang:        "node",
		Version:     "14.21.3",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Node14.21.3.tar.gz",
	}, {
		Lang:        "node",
		Version:     "13.14.0",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Node13.14.0.tar.gz",
	}, {
		Lang:        "node",
		Version:     "12.22.12",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Node12.22.12.tar.gz",
	}, {
		Lang:        "node",
		Version:     "11.15.0",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "Node11.15.0.tar.gz",
	},
}

// WebCompilerInitVersion -
var WebCompilerInitVersion = []*model.EnterpriseLanguageVersion{
	{
		Lang:        "java_server",
		Version:     "tomcat85",
		FirstChoice: true,
		Show:        true,
		System:      true,
		FileName:    "tomcat85.tar.gz",
	}, {
		Lang:        "java_server",
		Version:     "tomcat7",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "tomcat7.tar.gz",
	}, {
		Lang:        "java_server",
		Version:     "tomcat8",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "tomcat8.tar.gz",
	}, {
		Lang:        "java_server",
		Version:     "tomcat9",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "tomcat9.tar.gz",
	}, {
		Lang:        "java_server",
		Version:     "jetty7",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "jetty7.tar.gz",
	}, {
		Lang:        "java_server",
		Version:     "jetty9",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "jetty9.tar.gz",
	},
}

// WebRuntimeInitVersion -
var WebRuntimeInitVersion = []*model.EnterpriseLanguageVersion{
	{
		Lang:        "web_runtime",
		Version:     "nginx",
		FirstChoice: true,
		Show:        true,
		System:      true,
		FileName:    "nginx-1.22.1-ubuntu-22.04.2.tar.gz",
	}, {
		Lang:        "web_runtime",
		Version:     "apache",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "apache-2.2.19.tar.gz",
	},
}

// NetCompilerInitVersion -
var NetCompilerInitVersion = []*model.EnterpriseLanguageVersion{
	{
		Lang:        "net_sdk",
		Version:     "2.2",
		FirstChoice: true,
		Show:        true,
		System:      true,
		FileName:    "mcr.microsoft.com/dotnet/core/sdk:2.2",
	}, {
		Lang:        "net_sdk",
		Version:     "2.1",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "mcr.microsoft.com/dotnet/core/sdk:2.1",
	},
}

// NetRuntimeInitVersion -
var NetRuntimeInitVersion = []*model.EnterpriseLanguageVersion{
	{
		Lang:        "net_runtime",
		Version:     "2.2",
		FirstChoice: true,
		Show:        true,
		System:      true,
		FileName:    "mcr.microsoft.com/dotnet/core/aspnet:2.2",
	}, {
		Lang:        "net_runtime",
		Version:     "2.1",
		FirstChoice: false,
		Show:        true,
		System:      true,
		FileName:    "mcr.microsoft.com/dotnet/core/aspnet:2.1",
	},
}
