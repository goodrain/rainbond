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
	"time"

	"github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/db/dao"
	"github.com/goodrain/rainbond/db/mysql"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

//Manager db manager
type Manager interface {
	CloseManager() error
	Begin() *gorm.DB
	DB() *gorm.DB
	EnsureEndTransactionFunc() func(tx *gorm.DB)
	VolumeTypeDao() dao.VolumeTypeDao
	LicenseDao() dao.LicenseDao
	AppDao() dao.AppDao
	ApplicationDao() dao.ApplicationDao
	ApplicationDaoTransactions(db *gorm.DB) dao.ApplicationDao
	AppConfigGroupDao() dao.AppConfigGroupDao
	AppConfigGroupDaoTransactions(db *gorm.DB) dao.AppConfigGroupDao
	AppConfigGroupServiceDao() dao.AppConfigGroupServiceDao
	AppConfigGroupServiceDaoTransactions(db *gorm.DB) dao.AppConfigGroupServiceDao
	AppConfigGroupItemDao() dao.AppConfigGroupItemDao
	AppConfigGroupItemDaoTransactions(db *gorm.DB) dao.AppConfigGroupItemDao
	K8sResourceDao() dao.K8sResourceDao
	K8sResourceDaoTransactions(db *gorm.DB) dao.K8sResourceDao
	EnterpriseDao() dao.EnterpriseDao
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
	TenantPluginVersionConfigDao() dao.TenantPluginVersionConfigDao
	TenantPluginVersionConfigDaoTransactions(db *gorm.DB) dao.TenantPluginVersionConfigDao
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

	NotificationEventDao() dao.NotificationEventDao
	AppBackupDao() dao.AppBackupDao
	AppBackupDaoTransactions(db *gorm.DB) dao.AppBackupDao
	ServiceSourceDao() dao.ServiceSourceDao

	// gateway
	CertificateDao() dao.CertificateDao
	CertificateDaoTransactions(db *gorm.DB) dao.CertificateDao
	RuleExtensionDao() dao.RuleExtensionDao
	RuleExtensionDaoTransactions(db *gorm.DB) dao.RuleExtensionDao
	HTTPRuleDao() dao.HTTPRuleDao
	HTTPRuleDaoTransactions(db *gorm.DB) dao.HTTPRuleDao
	HTTPRuleRewriteDao() dao.HTTPRuleRewriteDao
	HTTPRuleRewriteDaoTransactions(db *gorm.DB) dao.HTTPRuleRewriteDao
	TCPRuleDao() dao.TCPRuleDao
	TCPRuleDaoTransactions(db *gorm.DB) dao.TCPRuleDao
	GwRuleConfigDao() dao.GwRuleConfigDao
	GwRuleConfigDaoTransactions(db *gorm.DB) dao.GwRuleConfigDao

	// third-party service
	EndpointsDao() dao.EndpointsDao
	EndpointsDaoTransactions(db *gorm.DB) dao.EndpointsDao
	ThirdPartySvcDiscoveryCfgDao() dao.ThirdPartySvcDiscoveryCfgDao
	ThirdPartySvcDiscoveryCfgDaoTransactions(db *gorm.DB) dao.ThirdPartySvcDiscoveryCfgDao

	TenantServceAutoscalerRulesDao() dao.TenantServceAutoscalerRulesDao
	TenantServceAutoscalerRulesDaoTransactions(db *gorm.DB) dao.TenantServceAutoscalerRulesDao
	TenantServceAutoscalerRuleMetricsDao() dao.TenantServceAutoscalerRuleMetricsDao
	TenantServceAutoscalerRuleMetricsDaoTransactions(db *gorm.DB) dao.TenantServceAutoscalerRuleMetricsDao
	TenantServiceScalingRecordsDao() dao.TenantServiceScalingRecordsDao
	TenantServiceScalingRecordsDaoTransactions(db *gorm.DB) dao.TenantServiceScalingRecordsDao

	TenantServiceMonitorDao() dao.TenantServiceMonitorDao
	TenantServiceMonitorDaoTransactions(db *gorm.DB) dao.TenantServiceMonitorDao

	ComponentK8sAttributeDao() dao.ComponentK8sAttributeDao
	ComponentK8sAttributeDaoTransactions(db *gorm.DB) dao.ComponentK8sAttributeDao
}

var defaultManager Manager

var supportDrivers map[string]struct{}

func init() {
	supportDrivers = map[string]struct{}{
		"mysql":       {},
		"cockroachdb": {},
		"sqlite": {},
	}
}

//CreateManager 创建manager
func CreateManager(config config.Config) (err error) {
	if _, ok := supportDrivers[config.DBType]; !ok {
		return fmt.Errorf("DB drivers: %s not supported", config.DBType)
	}

	for {
		defaultManager, err = mysql.CreateManager(config)
		if err == nil {
			logrus.Infof("db manager is ready")
			break
		}
		logrus.Errorf("get db manager failed, try time is %d,%s", 10, err.Error())
		time.Sleep(10 * time.Second)
	}
	//TODO:etcd db plugin
	//defaultManager, err = etcd.CreateManager(config)
	return
}

//CloseManager close db manager
func CloseManager() error {
	if defaultManager == nil {
		return errors.New("default db manager not init")
	}
	return defaultManager.CloseManager()
}

//GetManager get db manager
func GetManager() Manager {
	return defaultManager
}

// SetTestManager sets the default manager for unit test
func SetTestManager(m Manager) {
	defaultManager = m
}
