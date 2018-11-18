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

import (
	"github.com/goodrain/rainbond/db/dao"
	mysqldao "github.com/goodrain/rainbond/db/mysql/dao"
	"github.com/jinzhu/gorm"
)

//Manager db manager
type MockManager struct {
}

func (m *MockManager) CloseManager() error {
	return nil
}

//Begin begin a transaction
func (m *MockManager) Begin() *gorm.DB {
	return nil
}

//LicenseDao LicenseDao
func (m *MockManager) LicenseDao() dao.LicenseDao {
	return nil
}

//EventLogDao EventLogDao
func (m *MockManager) EventLogDao() dao.EventLogDao {
	return nil
}

//EventLogDaoTransactions EventLogDao
func (m *MockManager) EventLogDaoTransactions(db *gorm.DB) dao.EventLogDao {
	return nil
}

//TenantDao 租户数据
func (m *MockManager) TenantDao() dao.TenantDao {
	return &mysqldao.MockTenantDaoImpl{}
}

//TenantDaoTransactions 租户数据，带操作事务
func (m *MockManager) TenantDaoTransactions(db *gorm.DB) dao.TenantDao {
	return nil
}

//TenantServiceDao TenantServiceDao
func (m *MockManager) TenantServiceDao() dao.TenantServiceDao {
	return &mysqldao.MockTenantServicesDaoImpl{}
}

//TenantServiceDaoTransactions TenantServiceDaoTransactions
func (m *MockManager) TenantServiceDaoTransactions(db *gorm.DB) dao.TenantServiceDao {
	return nil
}

//TenantServiceDeleteDao TenantServiceDeleteDao
func (m *MockManager) TenantServiceDeleteDao() dao.TenantServiceDeleteDao {
	return nil
}

//TenantServiceDeleteDaoTransactions TenantServiceDeleteDaoTransactions
func (m *MockManager) TenantServiceDeleteDaoTransactions(db *gorm.DB) dao.TenantServiceDeleteDao {
	return nil
}

//TenantServicesPortDao TenantServicesPortDao
func (m *MockManager) TenantServicesPortDao() dao.TenantServicesPortDao {
	return &mysqldao.MockTenantServicesPortDaoImpl{}
}

//TenantServicesPortDaoTransactions TenantServicesPortDaoTransactions
func (m *MockManager) TenantServicesPortDaoTransactions(db *gorm.DB) dao.TenantServicesPortDao {
	return nil
}

//TenantServiceRelationDao TenantServiceRelationDao
func (m *MockManager) TenantServiceRelationDao() dao.TenantServiceRelationDao {
	return nil
}

//TenantServiceRelationDaoTransactions TenantServiceRelationDaoTransactions
func (m *MockManager) TenantServiceRelationDaoTransactions(db *gorm.DB) dao.TenantServiceRelationDao {
	return nil
}

//TenantServiceEnvVarDao TenantServiceEnvVarDao
func (m *MockManager) TenantServiceEnvVarDao() dao.TenantServiceEnvVarDao {
	return nil
}

//TenantServiceEnvVarDaoTransactions TenantServiceEnvVarDaoTransactions
func (m *MockManager) TenantServiceEnvVarDaoTransactions(db *gorm.DB) dao.TenantServiceEnvVarDao {
	return nil
}

//TenantServiceMountRelationDao TenantServiceMountRelationDao
func (m *MockManager) TenantServiceMountRelationDao() dao.TenantServiceMountRelationDao {
	return nil
}

//TenantServiceMountRelationDaoTransactions TenantServiceMountRelationDaoTransactions
func (m *MockManager) TenantServiceMountRelationDaoTransactions(db *gorm.DB) dao.TenantServiceMountRelationDao {
	return nil
}

//TenantServiceVolumeDao TenantServiceVolumeDao
func (m *MockManager) TenantServiceVolumeDao() dao.TenantServiceVolumeDao {
	return nil
}

//TenantServiceVolumeDaoTransactions TenantServiceVolumeDaoTransactions
func (m *MockManager) TenantServiceVolumeDaoTransactions(db *gorm.DB) dao.TenantServiceVolumeDao {
	return nil
}

//TenantServiceLabelDao TenantServiceLabelDao
func (m *MockManager) TenantServiceLabelDao() dao.TenantServiceLabelDao {
	return nil
}

//TenantServiceLabelDaoTransactions TenantServiceLabelDaoTransactions
func (m *MockManager) TenantServiceLabelDaoTransactions(db *gorm.DB) dao.TenantServiceLabelDao {
	return nil
}

//K8sServiceDao K8sServiceDao
func (m *MockManager) K8sServiceDao() dao.K8sServiceDao {
	return nil
}

//K8sServiceDaoTransactions K8sServiceDaoTransactions
func (m *MockManager) K8sServiceDaoTransactions(db *gorm.DB) dao.K8sServiceDao {
	return nil
}

//K8sDeployReplicationDao K8sDeployReplicationDao
func (m *MockManager) K8sDeployReplicationDao() dao.K8sDeployReplicationDao {
	return nil
}

//K8sPodDao K8sPodDao
func (m *MockManager) K8sPodDao() dao.K8sPodDao {
	return nil
}

//K8sPodDaoTransactions K8sPodDaoTransactions
func (m *MockManager) K8sPodDaoTransactions(db *gorm.DB) dao.K8sPodDao {
	return nil
}

//ServiceProbeDao ServiceProbeDao
func (m *MockManager) ServiceProbeDao() dao.ServiceProbeDao {
	return nil
}

//ServiceProbeDaoTransactions ServiceProbeDaoTransactions
func (m *MockManager) ServiceProbeDaoTransactions(db *gorm.DB) dao.ServiceProbeDao {
	return nil
}

//TenantServiceLBMappingPortDao TenantServiceLBMappingPortDao
func (m *MockManager) TenantServiceLBMappingPortDao() dao.TenantServiceLBMappingPortDao {
	return nil
}

//TenantServiceLBMappingPortDaoTransactions TenantServiceLBMappingPortDaoTransactions
func (m *MockManager) TenantServiceLBMappingPortDaoTransactions(db *gorm.DB) dao.TenantServiceLBMappingPortDao {
	return nil
}

//TenantServiceStatusDao TenantServiceStatusDao
func (m *MockManager) TenantServiceStatusDao() dao.ServiceStatusDao {
	return nil
}

//TenantServiceStatusDaoTransactions TenantServiceStatusDaoTransactions
func (m *MockManager) TenantServiceStatusDaoTransactions(db *gorm.DB) dao.ServiceStatusDao {
	return nil
}

//TenantPluginDao TenantPluginDao
func (m *MockManager) TenantPluginDao() dao.TenantPluginDao {
	return nil
}

//TenantPluginDaoTransactions TenantPluginDaoTransactions
func (m *MockManager) TenantPluginDaoTransactions(db *gorm.DB) dao.TenantPluginDao {
	return nil
}

//TenantPluginBuildVersionDao TenantPluginBuildVersionDao
func (m *MockManager) TenantPluginBuildVersionDao() dao.TenantPluginBuildVersionDao {
	return nil
}

//TenantPluginBuildVersionDaoTransactions TenantPluginBuildVersionDaoTransactions
func (m *MockManager) TenantPluginBuildVersionDaoTransactions(db *gorm.DB) dao.TenantPluginBuildVersionDao {
	return nil
}

//TenantPluginDefaultENVDao TenantPluginDefaultENVDao
func (m *MockManager) TenantPluginDefaultENVDao() dao.TenantPluginDefaultENVDao {
	return nil
}

//TenantPluginDefaultENVDaoTransactions TenantPluginDefaultENVDaoTransactions
func (m *MockManager) TenantPluginDefaultENVDaoTransactions(db *gorm.DB) dao.TenantPluginDefaultENVDao {
	return nil
}

//TenantPluginVersionENVDao TenantPluginVersionENVDao
func (m *MockManager) TenantPluginVersionENVDao() dao.TenantPluginVersionEnvDao {
	return nil
}

//TenantPluginVersionENVDaoTransactions TenantPluginVersionENVDaoTransactions
func (m *MockManager) TenantPluginVersionENVDaoTransactions(db *gorm.DB) dao.TenantPluginVersionEnvDao {
	return nil
}

//TenantServicePluginRelationDao TenantServicePluginRelationDao
func (m *MockManager) TenantServicePluginRelationDao() dao.TenantServicePluginRelationDao {
	return nil
}

//TenantServicePluginRelationDaoTransactions TenantServicePluginRelationDaoTransactions
func (m *MockManager) TenantServicePluginRelationDaoTransactions(db *gorm.DB) dao.TenantServicePluginRelationDao {
	return nil
}

//TenantServicesStreamPluginPortDao TenantServicesStreamPluginPortDao
func (m *MockManager) TenantServicesStreamPluginPortDao() dao.TenantServicesStreamPluginPortDao {
	return nil
}

//TenantServicesStreamPluginPortDaoTransactions TenantServicesStreamPluginPortDaoTransactions
func (m *MockManager) TenantServicesStreamPluginPortDaoTransactions(db *gorm.DB) dao.TenantServicesStreamPluginPortDao {
	return nil
}

//CodeCheckResultDao CodeCheckResultDao
func (m *MockManager) CodeCheckResultDao() dao.CodeCheckResultDao {
	return nil
}

//CodeCheckResultDaoTransactions CodeCheckResultDaoTransactions
func (m *MockManager) CodeCheckResultDaoTransactions(db *gorm.DB) dao.CodeCheckResultDao {
	return nil
}

//AppPublishDao AppPublishDao
func (m *MockManager) AppPublishDao() dao.AppPublishDao {
	return nil
}

//AppPublishDaoTransactions AppPublishDaoTransactions
func (m *MockManager) AppPublishDaoTransactions(db *gorm.DB) dao.AppPublishDao {
	return nil
}

//ServiceEventDao TenantServicePluginRelationDao
func (m *MockManager) ServiceEventDao() dao.EventDao {
	return nil
}

//ServiceEventDaoTransactions TenantServicePluginRelationDaoTransactions
func (m *MockManager) ServiceEventDaoTransactions(db *gorm.DB) dao.EventDao {
	return nil
}

//VersionInfoDao VersionInfoDao
func (m *MockManager) VersionInfoDao() dao.VersionInfoDao {
	return nil
}

//VersionInfoDaoTransactions VersionInfoDaoTransactions
func (m *MockManager) VersionInfoDaoTransactions(db *gorm.DB) dao.VersionInfoDao {
	return nil
}

//LocalSchedulerDao 本地调度信息
func (m *MockManager) LocalSchedulerDao() dao.LocalSchedulerDao {
	return nil
}

//RegionUserInfoDao RegionUserInfoDao
func (m *MockManager) RegionUserInfoDao() dao.RegionUserInfoDao {
	return nil
}

//RegionUserInfoDaoTransactions RegionUserInfoDaoTransactions
func (m *MockManager) RegionUserInfoDaoTransactions(db *gorm.DB) dao.RegionUserInfoDao {
	return nil
}

//RegionAPIClassDao RegionAPIClassDao
func (m *MockManager) RegionAPIClassDao() dao.RegionAPIClassDao {
	return nil
}

//RegionAPIClassDaoTransactions RegionAPIClassDaoTransactions
func (m *MockManager) RegionAPIClassDaoTransactions(db *gorm.DB) dao.RegionAPIClassDao {
	return nil
}

//RegionProcotolsDao RegionProcotolsDao
func (m *MockManager) RegionProcotolsDao() dao.RegionProcotolsDao {
	return nil
}

//RegionProcotolsDaoTransactions RegionProcotolsDao
func (m *MockManager) RegionProcotolsDaoTransactions(db *gorm.DB) dao.RegionProcotolsDao {
	return nil
}

//NotificationEventDao NotificationEventDao
func (m *MockManager) NotificationEventDao() dao.NotificationEventDao {
	return nil
}

//AppDao app export and import info
func (m *MockManager) AppDao() dao.AppDao {
	return nil
}

//AppBackupDao group app backup info
func (m *MockManager) AppBackupDao() dao.AppBackupDao {
	return nil
}

//ServiceSourceDao service source db impl
func (m *MockManager) ServiceSourceDao() dao.ServiceSourceDao {
	return nil
}

func (m *MockManager) CertificateDao() dao.CertificateDao {
	return nil
}

func (m *MockManager) RuleExtensionDao() dao.RuleExtensionDao {
	return nil
}

func (m *MockManager) HttpRuleDao() dao.HttpRuleDao {
	return &mysqldao.MockHttpRuleDaoImpl{}
}

func (m *MockManager) StreamRuleDao() dao.StreamRuleDao {
	return &mysqldao.MockStreamRuleDaoTmpl{}
}
