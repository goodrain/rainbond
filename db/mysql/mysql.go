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
	"sync"

	"github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/sirupsen/logrus"
	// import sql driver manually
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

//Manager db manager
type Manager struct {
	db      *gorm.DB
	config  config.Config
	initOne sync.Once
	models  []model.Interface
}

//CreateManager create manager
func CreateManager(config config.Config) (*Manager, error) {
	var db *gorm.DB
	if config.DBType == "mysql" {
		var err error
		db, err = gorm.Open("mysql", config.MysqlConnectionInfo+"?charset=utf8mb4&parseTime=True&loc=Local")
		if err != nil {
			return nil, err
		}
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
	manager := &Manager{
		db:      db,
		config:  config,
		initOne: sync.Once{},
	}
	db.SetLogger(manager)
	manager.RegisterTableModel()
	manager.CheckTable()
	logrus.Debug("mysql db driver create")
	return manager, nil
}

//CloseManager 关闭管理器
func (m *Manager) CloseManager() error {
	return m.db.Close()
}

//Begin begin a transaction
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

//Print Print
func (m *Manager) Print(v ...interface{}) {
	logrus.Info(v...)
}

//RegisterTableModel register table model
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
}

//CheckTable check and create tables
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
}
