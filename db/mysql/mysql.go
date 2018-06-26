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
	"sync"

	"github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/db/model"

	"github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
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
		db, err = gorm.Open("mysql", config.MysqlConnectionInfo+"?charset=utf8&parseTime=True&loc=Local")
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

//Print Print
func (m *Manager) Print(v ...interface{}) {
	logrus.Info(v)
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
	m.models = append(m.models, &model.K8sService{})
	m.models = append(m.models, &model.K8sDeployReplication{})
	m.models = append(m.models, &model.K8sPod{})
	m.models = append(m.models, &model.ServiceProbe{})
	m.models = append(m.models, &model.TenantServiceStatus{})
	m.models = append(m.models, &model.LicenseInfo{})
	m.models = append(m.models, &model.TenantServicesDelete{})
	//vs map port
	m.models = append(m.models, &model.TenantServiceLBMappingPort{})
	m.models = append(m.models, &model.TenantPlugin{})
	m.models = append(m.models, &model.TenantPluginBuildVersion{})
	m.models = append(m.models, &model.TenantServicePluginRelation{})
	m.models = append(m.models, &model.TenantPluginVersionEnv{})
	m.models = append(m.models, &model.CodeCheckResult{})
	m.models = append(m.models, &model.AppPublish{})
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
}

//CheckTable check and create tables
func (m *Manager) CheckTable() {
	m.initOne.Do(func() {
		for _, md := range m.models {
			if !m.db.HasTable(md) {
				if m.config.DBType == "mysql" {
					err := m.db.Set("gorm:table_options", "ENGINE=InnoDB charset=utf8").CreateTable(md).Error
					if err != nil {
						logrus.Errorf("auto create table %s to db error."+err.Error(), md.TableName())
					} else {
						logrus.Infof("auto create table %s to db success", md.TableName())
					}
				} else { //cockroachdb
					err := m.db.CreateTable(md).Error
					if err != nil {
						logrus.Errorf("auto create cockroachdb table %s to db error."+err.Error(), md.TableName())
					} else {
						logrus.Infof("auto create cockroachdb table %s to db success", md.TableName())
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
	// Permissions set
	var rac model.RegionAPIClass
	if err := m.db.Where("class_level=? and prefix=?", "server_source", "/v2/show").Find(&rac).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			data := map[string]string{
				"/v2/show":       "server_source",
				"/v2/opentsdb":   "server_source",
				"/v2/resources":  "server_source",
				"/v2/builder":    "server_source",
				"/v2/tenants":    "server_source",
				"/v2/app":        "server_source",
				"/api/v1":        "server_source",
				"/v2/nodes":      "node_manager",
				"/v2/job":        "node_manager",
				"/v2/tasks":      "node_manager",
				"/v2/taskgroups": "node_manager",
				"/v2/tasktemps":  "node_manager",
				"/v2/configs":    "node_manager",
			}
			tx := m.Begin()
			var rollback bool
			for k, v := range data {
				if err := m.RegionAPIClassDaoTransactions(tx).AddModel(&model.RegionAPIClass{
					ClassLevel: v,
					Prefix:     k,
				}); err != nil {
					tx.Rollback()
					rollback = true
					break
				}
			}
			if !rollback {
				tx.Commit()
			}
		}
	}

	//Port Protocol support
	var rps model.RegionProcotols
	if err := m.db.Where("protocol_group=? and protocol_child=?", "http", "http").Find(&rps).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			data := map[string][]string{
				"http":   []string{"http"},
				"stream": []string{"mysql", "tcp", "udp"},
			}
			tx := m.Begin()
			var rollback bool
			for k, v := range data {
				for _, v1 := range v {
					if err := m.RegionProcotolsDaoTransactions(tx).AddModel(&model.RegionProcotols{
						ProtocolGroup: k,
						ProtocolChild: v1,
						APIVersion:    "v2",
						IsSupport:     true,
					}); err != nil {
						tx.Rollback()
						rollback = true
						break
					}
				}
			}
			if !rollback {
				tx.Commit()
			}
		}
	}
	//set plugin version image name length
	if err := m.db.Exec("alter table tenant_plugin_build_version modify column base_image varchar(200);").Error; err != nil {
		logrus.Errorf("alter table tenant_plugin_build_version error %s", err.Error())
	}
	if err := m.db.Exec("alter table tenant_plugin_build_version modify column build_local_image varchar(200);").Error; err != nil {
		logrus.Errorf("alter table tenant_plugin_build_version error %s", err.Error())
	}
}
