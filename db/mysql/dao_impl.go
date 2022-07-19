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
	"github.com/jinzhu/gorm"

	"github.com/goodrain/rainbond/db/dao"
	mysqldao "github.com/goodrain/rainbond/db/mysql/dao"
)

// VolumeTypeDao volumeTypeDao
func (m *Manager) VolumeTypeDao() dao.VolumeTypeDao {
	return &mysqldao.VolumeTypeDaoImpl{
		DB: m.db,
	}
}

//LicenseDao LicenseDao
func (m *Manager) LicenseDao() dao.LicenseDao {
	return &mysqldao.LicenseDaoImpl{
		DB: m.db,
	}
}

// EnterpriseDao enterprise dao
func (m *Manager) EnterpriseDao() dao.EnterpriseDao {
	return &mysqldao.EnterpriseDaoImpl{
		DB: m.db,
	}
}

//TenantDao 租户数据
func (m *Manager) TenantDao() dao.TenantDao {
	return &mysqldao.TenantDaoImpl{
		DB: m.db,
	}
}

//TenantDaoTransactions 租户数据，带操作事务
func (m *Manager) TenantDaoTransactions(db *gorm.DB) dao.TenantDao {
	return &mysqldao.TenantDaoImpl{
		DB: db,
	}
}

//TenantServiceDao TenantServiceDao
func (m *Manager) TenantServiceDao() dao.TenantServiceDao {
	return &mysqldao.TenantServicesDaoImpl{
		DB: m.db,
	}
}

//TenantServiceDaoTransactions TenantServiceDaoTransactions
func (m *Manager) TenantServiceDaoTransactions(db *gorm.DB) dao.TenantServiceDao {
	return &mysqldao.TenantServicesDaoImpl{
		DB: db,
	}
}

//TenantServiceDeleteDao TenantServiceDeleteDao
func (m *Manager) TenantServiceDeleteDao() dao.TenantServiceDeleteDao {
	return &mysqldao.TenantServicesDeleteImpl{
		DB: m.db,
	}
}

//TenantServiceDeleteDaoTransactions TenantServiceDeleteDaoTransactions
func (m *Manager) TenantServiceDeleteDaoTransactions(db *gorm.DB) dao.TenantServiceDeleteDao {
	return &mysqldao.TenantServicesDeleteImpl{
		DB: db,
	}
}

//TenantServicesPortDao TenantServicesPortDao
func (m *Manager) TenantServicesPortDao() dao.TenantServicesPortDao {
	return &mysqldao.TenantServicesPortDaoImpl{
		DB: m.db,
	}
}

//TenantServicesPortDaoTransactions TenantServicesPortDaoTransactions
func (m *Manager) TenantServicesPortDaoTransactions(db *gorm.DB) dao.TenantServicesPortDao {
	return &mysqldao.TenantServicesPortDaoImpl{
		DB: db,
	}
}

//TenantServiceRelationDao TenantServiceRelationDao
func (m *Manager) TenantServiceRelationDao() dao.TenantServiceRelationDao {
	return &mysqldao.TenantServiceRelationDaoImpl{
		DB: m.db,
	}
}

//TenantServiceRelationDaoTransactions TenantServiceRelationDaoTransactions
func (m *Manager) TenantServiceRelationDaoTransactions(db *gorm.DB) dao.TenantServiceRelationDao {
	return &mysqldao.TenantServiceRelationDaoImpl{
		DB: db,
	}
}

//TenantServiceEnvVarDao TenantServiceEnvVarDao
func (m *Manager) TenantServiceEnvVarDao() dao.TenantServiceEnvVarDao {
	return &mysqldao.TenantServiceEnvVarDaoImpl{
		DB: m.db,
	}
}

//TenantServiceEnvVarDaoTransactions TenantServiceEnvVarDaoTransactions
func (m *Manager) TenantServiceEnvVarDaoTransactions(db *gorm.DB) dao.TenantServiceEnvVarDao {
	return &mysqldao.TenantServiceEnvVarDaoImpl{
		DB: db,
	}
}

//TenantServiceMountRelationDao TenantServiceMountRelationDao
func (m *Manager) TenantServiceMountRelationDao() dao.TenantServiceMountRelationDao {
	return &mysqldao.TenantServiceMountRelationDaoImpl{
		DB: m.db,
	}
}

//TenantServiceMountRelationDaoTransactions TenantServiceMountRelationDaoTransactions
func (m *Manager) TenantServiceMountRelationDaoTransactions(db *gorm.DB) dao.TenantServiceMountRelationDao {
	return &mysqldao.TenantServiceMountRelationDaoImpl{
		DB: db,
	}
}

//TenantServiceVolumeDao TenantServiceVolumeDao
func (m *Manager) TenantServiceVolumeDao() dao.TenantServiceVolumeDao {
	return &mysqldao.TenantServiceVolumeDaoImpl{
		DB: m.db,
	}
}

//TenantServiceVolumeDaoTransactions TenantServiceVolumeDaoTransactions
func (m *Manager) TenantServiceVolumeDaoTransactions(db *gorm.DB) dao.TenantServiceVolumeDao {
	return &mysqldao.TenantServiceVolumeDaoImpl{
		DB: db,
	}
}

//TenantServiceConfigFileDao TenantServiceConfigFileDao
func (m *Manager) TenantServiceConfigFileDao() dao.TenantServiceConfigFileDao {
	return &mysqldao.TenantServiceConfigFileDaoImpl{
		DB: m.db,
	}
}

//TenantServiceConfigFileDaoTransactions -
func (m *Manager) TenantServiceConfigFileDaoTransactions(db *gorm.DB) dao.TenantServiceConfigFileDao {
	return &mysqldao.TenantServiceConfigFileDaoImpl{
		DB: m.db,
	}
}

//TenantServiceLabelDao TenantServiceLabelDao
func (m *Manager) TenantServiceLabelDao() dao.TenantServiceLabelDao {
	return &mysqldao.ServiceLabelDaoImpl{
		DB: m.db,
	}
}

//TenantServiceLabelDaoTransactions TenantServiceLabelDaoTransactions
func (m *Manager) TenantServiceLabelDaoTransactions(db *gorm.DB) dao.TenantServiceLabelDao {
	return &mysqldao.ServiceLabelDaoImpl{
		DB: db,
	}
}

//ServiceProbeDao ServiceProbeDao
func (m *Manager) ServiceProbeDao() dao.ServiceProbeDao {
	return &mysqldao.ServiceProbeDaoImpl{
		DB: m.db,
	}
}

//ServiceProbeDaoTransactions ServiceProbeDaoTransactions
func (m *Manager) ServiceProbeDaoTransactions(db *gorm.DB) dao.ServiceProbeDao {
	return &mysqldao.ServiceProbeDaoImpl{
		DB: db,
	}
}

//TenantServiceLBMappingPortDao TenantServiceLBMappingPortDao
func (m *Manager) TenantServiceLBMappingPortDao() dao.TenantServiceLBMappingPortDao {
	return &mysqldao.TenantServiceLBMappingPortDaoImpl{
		DB: m.db,
	}
}

//TenantServiceLBMappingPortDaoTransactions TenantServiceLBMappingPortDaoTransactions
func (m *Manager) TenantServiceLBMappingPortDaoTransactions(db *gorm.DB) dao.TenantServiceLBMappingPortDao {
	return &mysqldao.TenantServiceLBMappingPortDaoImpl{
		DB: db,
	}
}

//TenantPluginDao TenantPluginDao
func (m *Manager) TenantPluginDao() dao.TenantPluginDao {
	return &mysqldao.PluginDaoImpl{
		DB: m.db,
	}
}

//TenantPluginDaoTransactions TenantPluginDaoTransactions
func (m *Manager) TenantPluginDaoTransactions(db *gorm.DB) dao.TenantPluginDao {
	return &mysqldao.PluginDaoImpl{
		DB: db,
	}
}

//TenantPluginBuildVersionDao TenantPluginBuildVersionDao
func (m *Manager) TenantPluginBuildVersionDao() dao.TenantPluginBuildVersionDao {
	return &mysqldao.PluginBuildVersionDaoImpl{
		DB: m.db,
	}
}

//TenantPluginBuildVersionDaoTransactions TenantPluginBuildVersionDaoTransactions
func (m *Manager) TenantPluginBuildVersionDaoTransactions(db *gorm.DB) dao.TenantPluginBuildVersionDao {
	return &mysqldao.PluginBuildVersionDaoImpl{
		DB: db,
	}
}

//TenantPluginDefaultENVDao TenantPluginDefaultENVDao
func (m *Manager) TenantPluginDefaultENVDao() dao.TenantPluginDefaultENVDao {
	return &mysqldao.PluginDefaultENVDaoImpl{
		DB: m.db,
	}
}

//TenantPluginDefaultENVDaoTransactions TenantPluginDefaultENVDaoTransactions
func (m *Manager) TenantPluginDefaultENVDaoTransactions(db *gorm.DB) dao.TenantPluginDefaultENVDao {
	return &mysqldao.PluginDefaultENVDaoImpl{
		DB: db,
	}
}

//TenantPluginVersionENVDao TenantPluginVersionENVDao
func (m *Manager) TenantPluginVersionENVDao() dao.TenantPluginVersionEnvDao {
	return &mysqldao.PluginVersionEnvDaoImpl{
		DB: m.db,
	}
}

//TenantPluginVersionENVDaoTransactions TenantPluginVersionENVDaoTransactions
func (m *Manager) TenantPluginVersionENVDaoTransactions(db *gorm.DB) dao.TenantPluginVersionEnvDao {
	return &mysqldao.PluginVersionEnvDaoImpl{
		DB: db,
	}
}

//TenantPluginVersionConfigDao TenantPluginVersionENVDao
func (m *Manager) TenantPluginVersionConfigDao() dao.TenantPluginVersionConfigDao {
	return &mysqldao.PluginVersionConfigDaoImpl{
		DB: m.db,
	}
}

//TenantPluginVersionConfigDaoTransactions TenantPluginVersionConfigDaoTransactions
func (m *Manager) TenantPluginVersionConfigDaoTransactions(db *gorm.DB) dao.TenantPluginVersionConfigDao {
	return &mysqldao.PluginVersionConfigDaoImpl{
		DB: db,
	}
}

//TenantServicePluginRelationDao TenantServicePluginRelationDao
func (m *Manager) TenantServicePluginRelationDao() dao.TenantServicePluginRelationDao {
	return &mysqldao.TenantServicePluginRelationDaoImpl{
		DB: m.db,
	}
}

//TenantServicePluginRelationDaoTransactions TenantServicePluginRelationDaoTransactions
func (m *Manager) TenantServicePluginRelationDaoTransactions(db *gorm.DB) dao.TenantServicePluginRelationDao {
	return &mysqldao.TenantServicePluginRelationDaoImpl{
		DB: db,
	}
}

//TenantServicesStreamPluginPortDao TenantServicesStreamPluginPortDao
func (m *Manager) TenantServicesStreamPluginPortDao() dao.TenantServicesStreamPluginPortDao {
	return &mysqldao.TenantServicesStreamPluginPortDaoImpl{
		DB: m.db,
	}
}

//TenantServicesStreamPluginPortDaoTransactions TenantServicesStreamPluginPortDaoTransactions
func (m *Manager) TenantServicesStreamPluginPortDaoTransactions(db *gorm.DB) dao.TenantServicesStreamPluginPortDao {
	return &mysqldao.TenantServicesStreamPluginPortDaoImpl{
		DB: db,
	}
}

//CodeCheckResultDao CodeCheckResultDao
func (m *Manager) CodeCheckResultDao() dao.CodeCheckResultDao {
	return &mysqldao.CodeCheckResultDaoImpl{
		DB: m.db,
	}
}

//CodeCheckResultDaoTransactions CodeCheckResultDaoTransactions
func (m *Manager) CodeCheckResultDaoTransactions(db *gorm.DB) dao.CodeCheckResultDao {
	return &mysqldao.CodeCheckResultDaoImpl{
		DB: db,
	}
}

//ServiceEventDao TenantServicePluginRelationDao
func (m *Manager) ServiceEventDao() dao.EventDao {
	return &mysqldao.EventDaoImpl{
		DB: m.db,
	}
}

//ServiceEventDaoTransactions TenantServicePluginRelationDaoTransactions
func (m *Manager) ServiceEventDaoTransactions(db *gorm.DB) dao.EventDao {
	return &mysqldao.EventDaoImpl{
		DB: db,
	}
}

//VersionInfoDao VersionInfoDao
func (m *Manager) VersionInfoDao() dao.VersionInfoDao {
	return &mysqldao.VersionInfoDaoImpl{
		DB: m.db,
	}
}

//VersionInfoDaoTransactions VersionInfoDaoTransactions
func (m *Manager) VersionInfoDaoTransactions(db *gorm.DB) dao.VersionInfoDao {
	return &mysqldao.VersionInfoDaoImpl{
		DB: db,
	}
}

//LocalSchedulerDao 本地调度信息
func (m *Manager) LocalSchedulerDao() dao.LocalSchedulerDao {
	return &mysqldao.LocalSchedulerDaoImpl{
		DB: m.db,
	}
}

//RegionUserInfoDao RegionUserInfoDao
func (m *Manager) RegionUserInfoDao() dao.RegionUserInfoDao {
	return &mysqldao.RegionUserInfoDaoImpl{
		DB: m.db,
	}
}

//RegionUserInfoDaoTransactions RegionUserInfoDaoTransactions
func (m *Manager) RegionUserInfoDaoTransactions(db *gorm.DB) dao.RegionUserInfoDao {
	return &mysqldao.RegionUserInfoDaoImpl{
		DB: db,
	}
}

//RegionAPIClassDao RegionAPIClassDao
func (m *Manager) RegionAPIClassDao() dao.RegionAPIClassDao {
	return &mysqldao.RegionAPIClassDaoImpl{
		DB: m.db,
	}
}

//RegionAPIClassDaoTransactions RegionAPIClassDaoTransactions
func (m *Manager) RegionAPIClassDaoTransactions(db *gorm.DB) dao.RegionAPIClassDao {
	return &mysqldao.RegionAPIClassDaoImpl{
		DB: db,
	}
}

//NotificationEventDao NotificationEventDao
func (m *Manager) NotificationEventDao() dao.NotificationEventDao {
	return &mysqldao.NotificationEventDaoImpl{
		DB: m.db,
	}
}

//AppDao app export and import info
func (m *Manager) AppDao() dao.AppDao {
	return &mysqldao.AppDaoImpl{
		DB: m.db,
	}
}

// ApplicationDao -
func (m *Manager) ApplicationDao() dao.ApplicationDao {
	return &mysqldao.ApplicationDaoImpl{
		DB: m.db,
	}
}

//ApplicationDaoTransactions -
func (m *Manager) ApplicationDaoTransactions(db *gorm.DB) dao.ApplicationDao {
	return &mysqldao.ApplicationDaoImpl{
		DB: db,
	}
}

// AppConfigGroupDao -
func (m *Manager) AppConfigGroupDao() dao.AppConfigGroupDao {
	return &mysqldao.AppConfigGroupDaoImpl{
		DB: m.db,
	}
}

//AppConfigGroupDaoTransactions -
func (m *Manager) AppConfigGroupDaoTransactions(db *gorm.DB) dao.AppConfigGroupDao {
	return &mysqldao.AppConfigGroupDaoImpl{
		DB: db,
	}
}

// AppConfigGroupServiceDao -
func (m *Manager) AppConfigGroupServiceDao() dao.AppConfigGroupServiceDao {
	return &mysqldao.AppConfigGroupServiceDaoImpl{
		DB: m.db,
	}
}

//AppConfigGroupServiceDaoTransactions -
func (m *Manager) AppConfigGroupServiceDaoTransactions(db *gorm.DB) dao.AppConfigGroupServiceDao {
	return &mysqldao.AppConfigGroupServiceDaoImpl{
		DB: db,
	}
}

// AppConfigGroupItemDao -
func (m *Manager) AppConfigGroupItemDao() dao.AppConfigGroupItemDao {
	return &mysqldao.AppConfigGroupItemDaoImpl{
		DB: m.db,
	}
}

//AppConfigGroupItemDaoTransactions -
func (m *Manager) AppConfigGroupItemDaoTransactions(db *gorm.DB) dao.AppConfigGroupItemDao {
	return &mysqldao.AppConfigGroupItemDaoImpl{
		DB: db,
	}
}

//AppBackupDao group app backup info
func (m *Manager) AppBackupDao() dao.AppBackupDao {
	return &mysqldao.AppBackupDaoImpl{
		DB: m.db,
	}
}

// AppBackupDaoTransactions -
func (m *Manager) AppBackupDaoTransactions(db *gorm.DB) dao.AppBackupDao {
	return &mysqldao.AppBackupDaoImpl{
		DB: db,
	}
}

//ServiceSourceDao service source db impl
func (m *Manager) ServiceSourceDao() dao.ServiceSourceDao {
	return &mysqldao.ServiceSourceImpl{
		DB: m.db,
	}
}

//CertificateDao CertificateDao
func (m *Manager) CertificateDao() dao.CertificateDao {
	return &mysqldao.CertificateDaoImpl{
		DB: m.db,
	}
}

//CertificateDaoTransactions CertificateDaoTransactions
func (m *Manager) CertificateDaoTransactions(db *gorm.DB) dao.CertificateDao {
	return &mysqldao.CertificateDaoImpl{
		DB: db,
	}
}

//RuleExtensionDao RuleExtensionDao
func (m *Manager) RuleExtensionDao() dao.RuleExtensionDao {
	return &mysqldao.RuleExtensionDaoImpl{
		DB: m.db,
	}
}

//RuleExtensionDaoTransactions RuleExtensionDaoTransactions
func (m *Manager) RuleExtensionDaoTransactions(db *gorm.DB) dao.RuleExtensionDao {
	return &mysqldao.RuleExtensionDaoImpl{
		DB: db,
	}
}

//HTTPRuleDao HTTPRuleDao
func (m *Manager) HTTPRuleDao() dao.HTTPRuleDao {
	return &mysqldao.HTTPRuleDaoImpl{
		DB: m.db,
	}
}

//HTTPRuleDaoTransactions -
func (m *Manager) HTTPRuleDaoTransactions(db *gorm.DB) dao.HTTPRuleDao {
	return &mysqldao.HTTPRuleDaoImpl{
		DB: db,
	}
}

// HTTPRuleRewriteDao HTTPRuleRewriteDao
func (m *Manager) HTTPRuleRewriteDao() dao.HTTPRuleRewriteDao {
	return &mysqldao.HTTPRuleRewriteDaoTmpl{
		DB: m.db,
	}
}

//HTTPRuleRewriteDaoTransactions -
func (m *Manager) HTTPRuleRewriteDaoTransactions(db *gorm.DB) dao.HTTPRuleRewriteDao {
	return &mysqldao.HTTPRuleRewriteDaoTmpl{
		DB: db,
	}
}

//TCPRuleDao TCPRuleDao
func (m *Manager) TCPRuleDao() dao.TCPRuleDao {
	return &mysqldao.TCPRuleDaoTmpl{
		DB: m.db,
	}
}

//TCPRuleDaoTransactions TCPRuleDaoTransactions
func (m *Manager) TCPRuleDaoTransactions(db *gorm.DB) dao.TCPRuleDao {
	return &mysqldao.TCPRuleDaoTmpl{
		DB: db,
	}
}

// EndpointsDao returns a new EndpointDaoImpl with default *gorm.DB.
func (m *Manager) EndpointsDao() dao.EndpointsDao {
	return &mysqldao.EndpointDaoImpl{
		DB: m.db,
	}
}

// EndpointsDaoTransactions returns a new EndpointDaoImpl with the givem *gorm.DB.
func (m *Manager) EndpointsDaoTransactions(db *gorm.DB) dao.EndpointsDao {
	return &mysqldao.EndpointDaoImpl{
		DB: db,
	}
}

// ThirdPartySvcDiscoveryCfgDao returns a new ThirdPartySvcDiscoveryCfgDao.
func (m *Manager) ThirdPartySvcDiscoveryCfgDao() dao.ThirdPartySvcDiscoveryCfgDao {
	return &mysqldao.ThirdPartySvcDiscoveryCfgDaoImpl{
		DB: m.db,
	}
}

// ThirdPartySvcDiscoveryCfgDaoTransactions returns a new ThirdPartySvcDiscoveryCfgDao.
func (m *Manager) ThirdPartySvcDiscoveryCfgDaoTransactions(db *gorm.DB) dao.ThirdPartySvcDiscoveryCfgDao {
	return &mysqldao.ThirdPartySvcDiscoveryCfgDaoImpl{
		DB: db,
	}
}

// GwRuleConfigDao creates a new dao.GwRuleConfigDao.
func (m *Manager) GwRuleConfigDao() dao.GwRuleConfigDao {
	return &mysqldao.GwRuleConfigDaoImpl{
		DB: m.db,
	}
}

// GwRuleConfigDaoTransactions creates a new dao.GwRuleConfigDao with special transaction.
func (m *Manager) GwRuleConfigDaoTransactions(db *gorm.DB) dao.GwRuleConfigDao {
	return &mysqldao.GwRuleConfigDaoImpl{
		DB: db,
	}
}

// TenantServceAutoscalerRulesDao -
func (m *Manager) TenantServceAutoscalerRulesDao() dao.TenantServceAutoscalerRulesDao {
	return &mysqldao.TenantServceAutoscalerRulesDaoImpl{
		DB: m.db,
	}
}

// TenantServceAutoscalerRulesDaoTransactions -
func (m *Manager) TenantServceAutoscalerRulesDaoTransactions(db *gorm.DB) dao.TenantServceAutoscalerRulesDao {
	return &mysqldao.TenantServceAutoscalerRulesDaoImpl{
		DB: db,
	}
}

// TenantServceAutoscalerRuleMetricsDao -
func (m *Manager) TenantServceAutoscalerRuleMetricsDao() dao.TenantServceAutoscalerRuleMetricsDao {
	return &mysqldao.TenantServceAutoscalerRuleMetricsDaoImpl{
		DB: m.db,
	}
}

// TenantServceAutoscalerRuleMetricsDaoTransactions -
func (m *Manager) TenantServceAutoscalerRuleMetricsDaoTransactions(db *gorm.DB) dao.TenantServceAutoscalerRuleMetricsDao {
	return &mysqldao.TenantServceAutoscalerRuleMetricsDaoImpl{
		DB: db,
	}
}

// TenantServiceScalingRecordsDao -
func (m *Manager) TenantServiceScalingRecordsDao() dao.TenantServiceScalingRecordsDao {
	return &mysqldao.TenantServiceScalingRecordsDaoImpl{
		DB: m.db,
	}
}

// TenantServiceScalingRecordsDaoTransactions -
func (m *Manager) TenantServiceScalingRecordsDaoTransactions(db *gorm.DB) dao.TenantServiceScalingRecordsDao {
	return &mysqldao.TenantServiceScalingRecordsDaoImpl{
		DB: db,
	}
}

//TenantServiceMonitorDao monitor dao
func (m *Manager) TenantServiceMonitorDao() dao.TenantServiceMonitorDao {
	return &mysqldao.TenantServiceMonitorDaoImpl{
		DB: m.db,
	}
}

//TenantServiceMonitorDaoTransactions monitor dao
func (m *Manager) TenantServiceMonitorDaoTransactions(db *gorm.DB) dao.TenantServiceMonitorDao {
	return &mysqldao.TenantServiceMonitorDaoImpl{
		DB: db,
	}
}

// ComponentK8sAttributeDao -
func (m *Manager) ComponentK8sAttributeDao() dao.ComponentK8sAttributeDao {
	return &mysqldao.ComponentK8sAttributeDaoImpl{
		DB: m.db,
	}
}

// ComponentK8sAttributeDaoTransactions -
func (m *Manager) ComponentK8sAttributeDaoTransactions(db *gorm.DB) dao.ComponentK8sAttributeDao {
	return &mysqldao.ComponentK8sAttributeDaoImpl{
		DB: db,
	}
}

// K8sResourceDao -
func (m *Manager) K8sResourceDao() dao.K8sResourceDao {
	return &mysqldao.K8sResourceDaoImpl{
		DB: m.db,
	}
}

// K8sResourceDaoTransactions -
func (m *Manager) K8sResourceDaoTransactions(db *gorm.DB) dao.K8sResourceDao {
	return &mysqldao.K8sResourceDaoImpl{
		DB: db,
	}
}
