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
	"github.com/goodrain/rainbond/db/dao"
	mysqldao "github.com/goodrain/rainbond/db/mysql/dao"

	"github.com/jinzhu/gorm"
)

//LicenseDao LicenseDao
func (m *Manager) LicenseDao() dao.LicenseDao {
	return &mysqldao.LicenseDaoImpl{
		DB: m.db,
	}
}

//EventLogDao EventLogDao
func (m *Manager) EventLogDao() dao.EventLogDao {
	return &mysqldao.EventLogMessageDaoImpl{
		DB: m.db,
	}
}

//EventLogDaoTransactions EventLogDao
func (m *Manager) EventLogDaoTransactions(db *gorm.DB) dao.EventLogDao {
	return &mysqldao.EventLogMessageDaoImpl{
		DB: db,
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

//K8sServiceDao K8sServiceDao
func (m *Manager) K8sServiceDao() dao.K8sServiceDao {
	return &mysqldao.K8sServiceDaoImpl{
		DB: m.db,
	}
}

//K8sServiceDaoTransactions K8sServiceDaoTransactions
func (m *Manager) K8sServiceDaoTransactions(db *gorm.DB) dao.K8sServiceDao {
	return &mysqldao.K8sServiceDaoImpl{
		DB: db,
	}
}

//K8sDeployReplicationDao K8sDeployReplicationDao
func (m *Manager) K8sDeployReplicationDao() dao.K8sDeployReplicationDao {
	return &mysqldao.K8sDeployReplicationDaoImpl{
		DB: m.db,
	}
}

//K8sPodDao K8sPodDao
func (m *Manager) K8sPodDao() dao.K8sPodDao {
	return &mysqldao.K8sPodDaoImpl{
		DB: m.db,
	}
}

//K8sPodDaoTransactions K8sPodDaoTransactions
func (m *Manager) K8sPodDaoTransactions(db *gorm.DB) dao.K8sPodDao {
	return &mysqldao.K8sPodDaoImpl{
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

//TenantServiceStatusDao TenantServiceStatusDao
func (m *Manager) TenantServiceStatusDao() dao.ServiceStatusDao {
	return &mysqldao.ServiceStatusDaoImpl{
		DB: m.db,
	}
}

//TenantServiceStatusDaoTransactions TenantServiceStatusDaoTransactions
func (m *Manager) TenantServiceStatusDaoTransactions(db *gorm.DB) dao.ServiceStatusDao {
	return &mysqldao.ServiceStatusDaoImpl{
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

//AppPublishDao AppPublishDao
func (m *Manager) AppPublishDao() dao.AppPublishDao {
	return &mysqldao.AppPublishDaoImpl{
		DB: m.db,
	}
}

//AppPublishDaoTransactions AppPublishDaoTransactions
func (m *Manager) AppPublishDaoTransactions(db *gorm.DB) dao.AppPublishDao {
	return &mysqldao.AppPublishDaoImpl{
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

//RegionProcotolsDao RegionProcotolsDao
func (m *Manager) RegionProcotolsDao() dao.RegionProcotolsDao {
	return &mysqldao.RegionProcotolsDaoImpl{
		DB: m.db,
	}
}

//RegionProcotolsDaoTransactions RegionProcotolsDao
func (m *Manager) RegionProcotolsDaoTransactions(db *gorm.DB) dao.RegionProcotolsDao {
	return &mysqldao.RegionProcotolsDaoImpl{
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

//AppBackupDao group app backup info
func (m *Manager) AppBackupDao() dao.AppBackupDao {
	return &mysqldao.AppBackupDaoImpl{
		DB: m.db,
	}
}
