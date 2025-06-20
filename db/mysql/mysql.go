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
	"os"
	"strconv"
	"sync"
	"time"

	gormbulkups "github.com/atcdot/gorm-bulk-upsert"

	"github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"

	//import sqlite
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/sirupsen/logrus"

	// import sql driver manually
	_ "github.com/go-sql-driver/mysql"
	// import postgres
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

// Manager db manager
type Manager struct {
	db      *gorm.DB
	config  config.Config
	initOne sync.Once
	models  []model.Interface
}

// CreateManager create manager
func CreateManager(config config.Config) (*Manager, error) {
	var db *gorm.DB
	if config.DBType == "mysql" {
		logrus.Info("mysql db driver create")
		var err error
		db, err = gorm.Open("mysql", config.MysqlConnectionInfo+"?charset=utf8mb4&parseTime=True&loc=Local")
		if err != nil {
			return nil, err
		}
		// 获取底层的 sql.DB 对象
		sqlDB := db.DB()
		if err != nil {
			logrus.Errorf("failed to get sql.DB from gorm.DB: %v", err)
			return nil, err
		}
		// 循环 Ping 操作，最多重试 5 次，每次间隔 10 秒
		for {
			if err := sqlDB.Ping(); err != nil {
				logrus.Errorf("failed to connect to database: %v", err)
				// 等待 10 秒再重试
				time.Sleep(2 * time.Second)
			} else {
				logrus.Info("数据库连接成功")
				break
			}
		}

		// 设置连接池参数
		maxOpenConns := 2500
		maxIdleConns := 500
		maxLifeTime := 5
		if os.Getenv("DB_MAX_OPEN_CONNS") != "" {
			openCon, err := strconv.Atoi(os.Getenv("DB_MAX_OPEN_CONNS"))
			if err == nil {
				maxOpenConns = openCon
			}
		}
		if os.Getenv("DB_MAX_IDLE_CONNS") != "" {
			idleCon, err := strconv.Atoi(os.Getenv("DB_MAX_IDLE_CONNS"))
			if err == nil {
				maxIdleConns = idleCon
			}
		}
		if os.Getenv("DB_CONN_MAX_LIFE_TIME") != "" {
			lifeTime, err := strconv.Atoi(os.Getenv("DB_CONN_MAX_LIFE_TIME"))
			if err == nil {
				maxLifeTime = lifeTime
			}
		}
		// 配置连接池参数
		sqlDB.SetMaxOpenConns(maxOpenConns)                                // 设置最大打开连接数
		sqlDB.SetMaxIdleConns(maxIdleConns)                                // 设置最大空闲连接数
		sqlDB.SetConnMaxLifetime(time.Duration(maxLifeTime) * time.Minute) //
	}
	if config.DBType == "cockroachdb" {
		var err error
		addr := config.MysqlConnectionInfo
		db, err = gorm.Open("postgres", addr)
		if err != nil {
			return nil, err
		}
	}
	if config.DBType == "sqlite" {
		_, err := os.Stat("/db")
		if err != nil {
			if !os.IsExist(err) {
				err := os.MkdirAll("/db", 0777)
				if err != nil {
					return nil, err
				}
			}
		}
		db, err = gorm.Open("sqlite3", "/db/region.sqlite3")
		if err != nil {
			return nil, err
		}
		db.Exec("PRAGMA journal_mode = WAL")
	}
	if config.ShowSQL {
		db = db.Debug()
	}
	logrus.Info("db init success")
	manager := &Manager{
		db:      db,
		config:  config,
		initOne: sync.Once{},
	}

	db.SetLogger(manager)
	logrus.Info("register table model")
	manager.RegisterTableModel()
	logrus.Info("check table")
	manager.CheckTable()
	logrus.Debug("mysql db driver create")
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
	m.models = append(m.models, &model.RegionUserInfo{})
	m.models = append(m.models, &model.TenantServicesStreamPluginPort{})
	m.models = append(m.models, &model.RegionAPIClass{})
	m.models = append(m.models, &model.RegionProcotols{})
	m.models = append(m.models, &model.LocalScheduler{})
	m.models = append(m.models, &model.NotificationEvent{})
	m.models = append(m.models, &model.AppStatus{})
	m.models = append(m.models, &model.AppBackup{})
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

// CheckTable check and create tables
func (m *Manager) CheckTable() {
	m.initOne.Do(func() {
		for _, md := range m.models {
			if !m.db.HasTable(md) {
				if m.config.DBType == "mysql" {
					err := m.db.Set("gorm:table_options", "ENGINE=InnoDB charset=utf8mb4").CreateTable(md).Error
					if err != nil {
						logrus.Errorf("auto create table %s to db error."+err.Error(), md.TableName())
					} else {
						logrus.Infof("auto create table %s to db success", md.TableName())
					}
				}
				if m.config.DBType == "cockroachdb" { //cockroachdb
					err := m.db.CreateTable(md).Error
					if err != nil {
						logrus.Errorf("auto create cockroachdb table %s to db error."+err.Error(), md.TableName())
					} else {
						logrus.Infof("auto create cockroachdb table %s to db success", md.TableName())
					}
				}
				if m.config.DBType == "sqlite" {
					err := m.db.CreateTable(md).Error
					if err != nil {
						logrus.Errorf("auto create sqlite table %s to db error."+err.Error(), md.TableName())
					} else {
						logrus.Infof("auto create sqlite table %s to db success", md.TableName())
					}
				}
			} else {
				if err := m.db.AutoMigrate(md).Error; err != nil {
					logrus.Errorf("auto Migrate table %s to db error."+err.Error(), md.TableName())
				}
			}
		}
		m.patchTable()
	})
}

func (m *Manager) patchTable() {
	count := -1
	switch m.config.DBType {
	case "mysql":
		if err := m.db.Exec("alter table enterprise_language_version add unique index if not exists lang_version_unique (lang, version);").Error; err != nil {
			logrus.Errorf("add unique index for enterprise_language_version error: %s", err.Error())
		}
	case "sqlite":
		if err := m.db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS lang_version_unique ON enterprise_language_version(lang, version);").Error; err != nil {
			logrus.Errorf("add unique index for enterprise_language_version error: %s", err.Error())
		}
	}
	m.db.Model(&model.EnterpriseLanguageVersion{}).Count(&count)
	if count == 0 {
		m.initLanguageVersion()
	}
	if m.config.DBType == "sqlite" {
		return
	}
	if err := m.db.Exec("alter table tenant_services_envs modify column attr_value text;").Error; err != nil {
		logrus.Errorf("alter table tenant_services_envs error %s", err.Error())
	}

	if err := m.db.Exec("alter table tenant_services_event modify column request_body varchar(1024);").Error; err != nil {
		logrus.Errorf("alter table tenant_services_envent error %s", err.Error())
	}

	if err := m.db.Exec("update gateway_tcp_rule set ip=? where ip=?", "0.0.0.0", "").Error; err != nil {
		logrus.Errorf("update gateway_tcp_rule data error %s", err.Error())
	}
	if err := m.db.Exec("alter table tenant_services_volume modify column volume_type varchar(64);").Error; err != nil {
		logrus.Errorf("alter table tenant_services_volume error: %s", err.Error())
	}
	if err := m.db.Exec("update tenants set namespace=uuid where namespace is NULL;").Error; err != nil {
		logrus.Errorf("update tenants namespace error: %s", err.Error())
	}
	if err := m.db.Exec("update applications set k8s_app=concat('app-',LEFT(app_id,8)) where k8s_app is NULL;").Error; err != nil {
		logrus.Errorf("update tenants namespace error: %s", err.Error())
	}
	if err := m.db.Exec("update tenant_services set k8s_component_name=service_alias where k8s_component_name is NULL;").Error; err != nil {
		logrus.Errorf("update tenants namespace error: %s", err.Error())
	}
	if err := m.db.Exec("alter  table tenant_services_probe modify column cmd longtext;").Error; err != nil {
		logrus.Errorf("alter table tenant_services_probe error: %s", err.Error())
	}
	if err := m.db.Exec("alter  table app_config_group_item modify column item_value longtext;").Error; err != nil {
		logrus.Errorf("alter table app_config_group_item error: %s", err.Error())
	}

	if err := m.db.Exec("alter table applications modify column governance_mode varchar(255) DEFAULT 'KUBERNETES_NATIVE_SERVICE';").Error; err != nil {
		logrus.Errorf("alter table applications error: %s", err.Error())
	}

	if err := m.db.Exec("alter table tenant_services_volume_type modify column storage_class_detail longtext;").Error; err != nil {
		logrus.Errorf("alter table applications error: %s", err.Error())
	}
}

func (m *Manager) initLanguageVersion() {
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
	dbType := m.db.Dialect().GetName()
	if dbType == "sqlite3" {
		for _, version := range versions {
			if err := m.db.Create(version).Error; err != nil {
				logrus.Error("batch Update or update k8sResources error:", err)
			}
		}
		return
	}
	var objects []interface{}
	for _, version := range versions {
		objects = append(objects, *version)
	}
	if err := gormbulkups.BulkUpsert(m.db, objects, 2000); err != nil {
		logrus.Errorf("create K8sResource groups in batch failure: %v", err)
	}
}

// GolangInitVersion -
var GolangInitVersion = []*model.EnterpriseLanguageVersion{
	{
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
