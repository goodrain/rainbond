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

package db

import (
	"errors"
	"fmt"

	"github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/db/dao"
	"github.com/goodrain/rainbond/db/mysql"

	"github.com/jinzhu/gorm"
)

//Manager db manager
type Manager interface {
	CloseManager() error
	Begin() *gorm.DB
	LicenseDao() dao.LicenseDao
	AppDao() dao.AppDao
	TenantDao() dao.TenantDao
	TenantDaoTransactions(db *gorm.DB) dao.TenantDao
	TenantServiceDao() dao.TenantServiceDao
	TenantServiceDeleteDao() dao.TenantServiceDeleteDao
	TenantServiceDaoTransactions(db *gorm.DB) dao.TenantServiceDao
	TenantServiceDeleteDaoTransactions(db *gorm.DB) dao.TenantServiceDeleteDao
	TenantServicesPortDao() dao.TenantServicesPortDao
	TenantServicesPortDaoTransactions(*gorm.DB) dao.TenantServicesPortDao
	TenantServiceRelationDao() dao.TenantServiceRelationDao
	TenantServiceRelationDaoTransactions(*gorm.DB) dao.TenantServiceRelationDao
	TenantServiceEnvVarDao() dao.TenantServiceEnvVarDao
	TenantServiceEnvVarDaoTransactions(*gorm.DB) dao.TenantServiceEnvVarDao
	TenantServiceMountRelationDao() dao.TenantServiceMountRelationDao
	TenantServiceMountRelationDaoTransactions(db *gorm.DB) dao.TenantServiceMountRelationDao
	TenantServiceVolumeDao() dao.TenantServiceVolumeDao
	TenantServiceVolumeDaoTransactions(*gorm.DB) dao.TenantServiceVolumeDao
	TenantServiceConfigFileDao() dao.TenantServiceConfigFileDao
	TenantServiceConfigFileDaoTransactions(*gorm.DB) dao.TenantServiceConfigFileDao
	ServiceProbeDao() dao.ServiceProbeDao
	ServiceProbeDaoTransactions(*gorm.DB) dao.ServiceProbeDao
	TenantServiceLBMappingPortDao() dao.TenantServiceLBMappingPortDao
	TenantServiceLBMappingPortDaoTransactions(*gorm.DB) dao.TenantServiceLBMappingPortDao
	TenantServiceLabelDao() dao.TenantServiceLabelDao
	TenantServiceLabelDaoTransactions(db *gorm.DB) dao.TenantServiceLabelDao
	LocalSchedulerDao() dao.LocalSchedulerDao
	TenantPluginDaoTransactions(db *gorm.DB) dao.TenantPluginDao
	TenantPluginDao() dao.TenantPluginDao
	TenantPluginDefaultENVDaoTransactions(db *gorm.DB) dao.TenantPluginDefaultENVDao
	TenantPluginDefaultENVDao() dao.TenantPluginDefaultENVDao
	TenantPluginBuildVersionDao() dao.TenantPluginBuildVersionDao
	TenantPluginBuildVersionDaoTransactions(db *gorm.DB) dao.TenantPluginBuildVersionDao
	TenantPluginVersionENVDao() dao.TenantPluginVersionEnvDao
	TenantPluginVersionENVDaoTransactions(db *gorm.DB) dao.TenantPluginVersionEnvDao
	TenantServicePluginRelationDao() dao.TenantServicePluginRelationDao
	TenantServicePluginRelationDaoTransactions(db *gorm.DB) dao.TenantServicePluginRelationDao
	TenantServicesStreamPluginPortDao() dao.TenantServicesStreamPluginPortDao
	TenantServicesStreamPluginPortDaoTransactions(db *gorm.DB) dao.TenantServicesStreamPluginPortDao

	CodeCheckResultDao() dao.CodeCheckResultDao
	CodeCheckResultDaoTransactions(db *gorm.DB) dao.CodeCheckResultDao

	ServiceEventDao() dao.EventDao
	ServiceEventDaoTransactions(db *gorm.DB) dao.EventDao

	VersionInfoDao() dao.VersionInfoDao
	VersionInfoDaoTransactions(db *gorm.DB) dao.VersionInfoDao

	RegionUserInfoDao() dao.RegionUserInfoDao
	RegionUserInfoDaoTransactions(db *gorm.DB) dao.RegionUserInfoDao

	RegionAPIClassDao() dao.RegionAPIClassDao
	RegionAPIClassDaoTransactions(db *gorm.DB) dao.RegionAPIClassDao

	RegionProcotolsDao() dao.RegionProcotolsDao
	RegionProcotolsDaoTransactions(db *gorm.DB) dao.RegionProcotolsDao

	NotificationEventDao() dao.NotificationEventDao
	AppBackupDao() dao.AppBackupDao
	ServiceSourceDao() dao.ServiceSourceDao

	// gateway
	CertificateDao() dao.CertificateDao
	CertificateDaoTransactions(db *gorm.DB) dao.CertificateDao
	RuleExtensionDao() dao.RuleExtensionDao
	RuleExtensionDaoTransactions(db *gorm.DB) dao.RuleExtensionDao
	HttpRuleDao() dao.HTTPRuleDao
	HttpRuleDaoTransactions(db *gorm.DB) dao.HTTPRuleDao
	TcpRuleDao() dao.TCPRuleDao
	TcpRuleDaoTransactions(db *gorm.DB) dao.TCPRuleDao
	IPPortDao() dao.IPPortDao
	IPPortDaoTransactions(db *gorm.DB) dao.IPPortDao
	IPPoolDao() dao.IPPoolDao
}

var defaultManager Manager

//CreateManager 创建manager
func CreateManager(config config.Config) (err error) {
	if config.DBType == "mysql" || config.DBType == "cockroachdb" || config.DBType == "sqlite3" {
		defaultManager, err = mysql.CreateManager(config)
		return err
	} else {
		//TODO:etcd 插件实现
		//defaultManager, err = etcd.CreateManager(config)
	}
	return fmt.Errorf("Db drivers not supported")
}

//CloseManager 关闭
func CloseManager() error {
	if defaultManager == nil {
		return errors.New("default db manager not init")
	}
	return defaultManager.CloseManager()
}

//GetManager 获取管理器
func GetManager() Manager {
	return defaultManager
}
