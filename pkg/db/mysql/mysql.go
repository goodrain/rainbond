// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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

/*
Copyright 2017 The Goodrain Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import (
	"sync"

	"github.com/goodrain/rainbond/pkg/db/config"
	"github.com/goodrain/rainbond/pkg/db/model"

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

//CreateManager 创建manager
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
		addr := config.MysqlConnectionInfo + "?sslmode=disable"
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

//RegisterTableModel 注册表结构
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
	m.models = append(m.models, &model.EventLogMessage{})
	m.models = append(m.models, &model.TenantPlugin{})
	m.models = append(m.models, &model.TenantPluginDefaultENV{})
	m.models = append(m.models, &model.TenantPluginBuildVersion{})
	m.models = append(m.models, &model.TenantServicePluginRelation{})
	m.models = append(m.models, &model.TenantPluginVersionEnv{})
}

//CheckTable 检测表结构
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
			}
		}
		m.patchTable()
	})
}

func (m *Manager) patchTable() {
	// m.db.Exec("alter table tenant_services add replica_id varchar(32)")
	// m.db.Exec("alter table tenant_services add status int(11) default 0")
	// m.db.Exec("alter table tenant_services add node_label varchar(40)")
}
