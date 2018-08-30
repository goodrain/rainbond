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

package dao

import (
	"time"

	"github.com/goodrain/rainbond/db/model"
)

//Dao 数据持久化层接口
type Dao interface {
	AddModel(model.Interface) error
	UpdateModel(model.Interface) error
}

//DelDao 删除接口
type DelDao interface {
	DeleteModel(serviceID string, arg ...interface{}) error
}

//TenantDao tenant dao
type TenantDao interface {
	Dao
	GetTenantByUUID(uuid string) (*model.Tenants, error)
	GetTenantIDByName(tenantName string) (*model.Tenants, error)
	GetALLTenants() ([]*model.Tenants, error)
	GetTenantByEid(eid string) ([]*model.Tenants, error)
	GetPagedTenants(offset, len int) ([]*model.Tenants, error)
	GetTenantIDsByNames(names []string) ([]string, error)
	GetTenantByUUIDIsExist(uuid string) bool
}

//TenantDao tenant dao
type AppDao interface {
	Dao
	GetByEventId(eventID string) (*model.AppStatus, error)
	DeleteModelByEventId(eventID string) error
}

//LicenseDao LicenseDao
type LicenseDao interface {
	Dao
	//DeleteLicense(token string) error
	ListLicenses() ([]*model.LicenseInfo, error)
}

//EventLogDao EventLogDao
type EventLogDao interface {
	Dao
	GetEventLogMessages(eventID string) ([]*model.EventLogMessage, error)
	DeleteServiceLog(serviceID string) error
	DeleteServiceEventLog(obj *model.EventLogMessage) error
	GetAllServiceEventLog() ([]*model.EventLogMessage, error)
	DeleteServiceEventLogByEventId(eventId string) error
}

//TenantServiceDao TenantServiceDao
type TenantServiceDao interface {
	Dao
	GetServiceByID(serviceID string) (*model.TenantServices, error)
	GetServiceByServiceAlias(serviceAlias string) (*model.TenantServices, error)
	GetServiceByIDs(serviceIDs []string) ([]*model.TenantServices, error)
	GetServiceAliasByIDs(uids []string) ([]*model.TenantServices, error)
	GetServiceByTenantIDAndServiceAlias(tenantID, serviceName string) (*model.TenantServices, error)
	SetTenantServiceStatus(serviceID, status string) error
	GetServicesByTenantID(tenantID string) ([]*model.TenantServices, error)
	GetServicesByTenantIDs(tenantIDs []string) ([]*model.TenantServices, error)
	GetServicesAllInfoByTenantID(tenantID string) ([]*model.TenantServices, error)
	DeleteServiceByServiceID(serviceID string) error
	GetServiceMemoryByTenantIDs(tenantIDs, serviceIDs []string) (map[string]map[string]interface{}, error)
	GetServiceMemoryByServiceIDs(serviceIDs []string) (map[string]map[string]interface{}, error)
	GetPagedTenantService(offset, len int, serviceIDs []string) ([]map[string]interface{}, int, error)
	GetAllServices() ([]*model.TenantServices, error)
	GetAllServicesID() ([]*model.TenantServices, error)
}

//TenantServiceDeleteDao TenantServiceDeleteDao
type TenantServiceDeleteDao interface {
	Dao
	GetTenantServicesDeleteByCreateTime(createTime time.Time) ([]*model.TenantServicesDelete, error)
	DeleteTenantServicesDelete(record *model.TenantServicesDelete) error
}

//TenantServicesPortDao TenantServicesPortDao
type TenantServicesPortDao interface {
	Dao
	DelDao
	GetPortsByServiceID(serviceID string) ([]*model.TenantServicesPort, error)
	GetOuterPorts(serviceID string) ([]*model.TenantServicesPort, error)
	GetInnerPorts(serviceID string) ([]*model.TenantServicesPort, error)
	GetPort(serviceID string, port int) (*model.TenantServicesPort, error)
	DELPortsByServiceID(serviceID string) error
}

//TenantPluginDao TenantPluginDao
type TenantPluginDao interface {
	Dao
	GetPluginByID(pluginID, tenantID string) (*model.TenantPlugin, error)
	DeletePluginByID(pluginID, tenantID string) error
	GetPluginsByTenantID(tenantID string) ([]*model.TenantPlugin, error)
}

//TenantPluginDefaultENVDao TenantPluginDefaultENVDao
type TenantPluginDefaultENVDao interface {
	Dao
	GetDefaultENVByName(pluginID, name, versionID string) (*model.TenantPluginDefaultENV, error)
	GetDefaultENVSByPluginID(pluginID, versionID string) ([]*model.TenantPluginDefaultENV, error)
	//GetDefaultENVSByPluginIDCantBeSet(pluginID string) ([]*model.TenantPluginDefaultENV, error)
	DeleteDefaultENVByName(pluginID, name, versionID string) error
	DeleteAllDefaultENVByPluginID(PluginID string) error
	DeleteDefaultENVByPluginIDAndVersionID(pluginID, versionID string) error
	GetALLMasterDefultENVs(pluginID string) ([]*model.TenantPluginDefaultENV, error)
	GetDefaultEnvWhichCanBeSetByPluginID(pluginID, versionID string) ([]*model.TenantPluginDefaultENV, error)
}

//TenantPluginBuildVersionDao TenantPluginBuildVersionDao
type TenantPluginBuildVersionDao interface {
	Dao
	DeleteBuildVersionByVersionID(versionID string) error
	DeleteBuildVersionByPluginID(pluginID string) error
	GetBuildVersionByPluginID(pluginID string) ([]*model.TenantPluginBuildVersion, error)
	GetBuildVersionByVersionID(pluginID, versionID string) (*model.TenantPluginBuildVersion, error)
	GetLastBuildVersionByVersionID(pluginID, versionID string) (*model.TenantPluginBuildVersion, error)
	GetBuildVersionByDeployVersion(pluginID, versionID, deployVersion string) (*model.TenantPluginBuildVersion, error)
}

//TenantPluginVersionEnvDao TenantPluginVersionEnvDao
type TenantPluginVersionEnvDao interface {
	Dao
	DeleteEnvByEnvName(envName, pluginID, serviceID string) error
	DeleteEnvByPluginID(serviceID, pluginID string) error
	DeleteEnvByServiceID(serviceID string) error
	GetVersionEnvByServiceID(serviceID string, pluginID string) ([]*model.TenantPluginVersionEnv, error)
	GetVersionEnvByEnvName(serviceID, pluginID, envName string) (*model.TenantPluginVersionEnv, error)
}

//TenantServicePluginRelationDao TenantServicePluginRelationDao
type TenantServicePluginRelationDao interface {
	Dao
	DeleteRelationByServiceIDAndPluginID(serviceID, pluginID string) error
	DeleteALLRelationByServiceID(serviceID string) error
	DeleteALLRelationByPluginID(pluginID string) error
	GetALLRelationByServiceID(serviceID string) ([]*model.TenantServicePluginRelation, error)
	GetRelateionByServiceIDAndPluginID(serviceID, pluginID string) (*model.TenantServicePluginRelation, error)
	CheckSomeModelPluginByServiceID(serviceID, pluginModel string) (bool, error)
	CheckSomeModelLikePluginByServiceID(serviceID, pluginModel string) (bool, error)
}

//TenantServiceRelationDao TenantServiceRelationDao
type TenantServiceRelationDao interface {
	Dao
	DelDao
	GetTenantServiceRelations(serviceID string) ([]*model.TenantServiceRelation, error)
	GetTenantServiceRelationsByDependServiceID(dependServiceID string) ([]*model.TenantServiceRelation, error)
	HaveRelations(serviceID string) bool
	DELRelationsByServiceID(serviceID string) error
	DeleteRelationByDepID(serviceID, depID string) error
}

//TenantServicesStreamPluginPortDao TenantServicesStreamPluginPortDao
type TenantServicesStreamPluginPortDao interface {
	Dao
	GetPluginMappingPorts(serviceID string, pluginModel string) ([]*model.TenantServicesStreamPluginPort, error)
	SetPluginMappingPort(
		tenantID string,
		serviceID string,
		pluginModel string,
		containerPort int,
	) (int, error)
	DeletePluginMappingPortByContainerPort(
		serviceID string,
		pluginModel string,
		containerPort int,
	) error
	DeleteAllPluginMappingPortByServiceID(serviceID string) error
	GetPluginMappingPortByServiceIDAndContainerPort(
		serviceID string,
		pluginModel string,
		containerPort int,
	) (*model.TenantServicesStreamPluginPort, error)
}

//TenantServiceEnvVarDao TenantServiceEnvVarDao
type TenantServiceEnvVarDao interface {
	Dao
	DelDao
	//service_id__in=sids, scope__in=("outer", "both")
	GetDependServiceEnvs(serviceIDs []string, scopes []string) ([]*model.TenantServiceEnvVar, error)
	GetServiceEnvs(serviceID string, scopes []string) ([]*model.TenantServiceEnvVar, error)
	GetEnv(serviceID, envName string) (*model.TenantServiceEnvVar, error)
	DELServiceEnvsByServiceID(serviceID string) error
}

//TenantServiceMountRelationDao TenantServiceMountRelationDao
type TenantServiceMountRelationDao interface {
	Dao
	GetTenantServiceMountRelationsByService(serviceID string) ([]*model.TenantServiceMountRelation, error)
	DElTenantServiceMountRelationByServiceAndName(serviceID, mntDir string) error
	DELTenantServiceMountRelationByServiceID(serviceID string) error
	DElTenantServiceMountRelationByDepService(serviceID, depServiceID string) error
}

//TenantServiceVolumeDao TenantServiceVolumeDao
type TenantServiceVolumeDao interface {
	Dao
	DelDao
	GetTenantServiceVolumesByServiceID(serviceID string) ([]*model.TenantServiceVolume, error)
	DeleteTenantServiceVolumesByServiceID(serviceID string) error
	DeleteByServiceIDAndVolumePath(serviceID string, volumePath string) error
	GetVolumeByServiceIDAndName(serviceID, name string) (*model.TenantServiceVolume, error)
	GetAllVolumes() ([]*model.TenantServiceVolume, error)
}

//TenantServiceLBMappingPortDao vs lb mapping port dao
type TenantServiceLBMappingPortDao interface {
	Dao
	GetTenantServiceLBMappingPort(serviceID string, containerPort int) (*model.TenantServiceLBMappingPort, error)
	GetTenantServiceLBMappingPortByService(serviceID string) ([]*model.TenantServiceLBMappingPort, error)
	CreateTenantServiceLBMappingPort(serviceID string, containerPort int) (*model.TenantServiceLBMappingPort, error)
	DELServiceLBMappingPortByServiceID(serviceID string) error
	DELServiceLBMappingPortByServiceIDAndPort(serviceID string, lbPort int) error
	GetLBPortByTenantAndPort(tenantID string, lbport int) (*model.TenantServiceLBMappingPort, error)
}

//TenantServiceLabelDao TenantServiceLabelDao
type TenantServiceLabelDao interface {
	Dao
	DelDao
	GetTenantServiceLabel(serviceID string) ([]*model.TenantServiceLable, error)
	DeleteLabelByServiceID(serviceID string) error
	GetTenantServiceNodeSelectorLabel(serviceID string) ([]*model.TenantServiceLable, error)
	GetTenantServiceAffinityLabel(serviceID string) ([]*model.TenantServiceLable, error)
	GetTenantServiceTypeLabel(serviceID string) (*model.TenantServiceLable, error)
	DELTenantServiceLabelsByLabelvaluesAndServiceID(serviceID string, labelValues []string) error
}

//K8sServiceDao k8s service信息
type K8sServiceDao interface {
	Dao
	GetK8sService(serviceID string, containerPort int, isOut bool) (*model.K8sService, error)
	GetK8sServiceByReplicationID(replicationID string) (*model.K8sService, error)
	GetK8sServiceByTenantServiceID(tenantServiceID string) ([]*model.K8sService, error)
	DeleteK8sServiceByReplicationID(replicationID string) error
	GetK8sServiceByReplicationIDAndPort(replicationID string, port int, isOut bool) (*model.K8sService, error)
	DeleteK8sServiceByReplicationIDAndPort(replicationID string, port int, isOut bool) error
	DeleteK8sServiceByName(k8sServiceName string) error
	GetAllK8sService() ([]*model.K8sService, error)
	K8sServiceIsExist(tenantId string, K8sServiceID string) bool
}

//K8sDeployReplicationDao 部署信息
type K8sDeployReplicationDao interface {
	Dao
	GetK8sDeployReplication(replicationID string) (*model.K8sDeployReplication, error)
	//不真正删除，设置IS_DELETE 为true
	DeleteK8sDeployReplication(replicationID string) error
	GetK8sDeployReplicationByService(serviceID string) ([]*model.K8sDeployReplication, error)
	GetK8sCurrentDeployReplicationByService(serviceID string) (*model.K8sDeployReplication, error)
	DeleteK8sDeployReplicationByServiceAndVersion(serviceID, version string) error
	DeleteK8sDeployReplicationByServiceAndMarked(serviceID string) error
	//不真正删除，设置IS_DELETE 为true
	DeleteK8sDeployReplicationByService(serviceID string) error
	GetReplications() ([]*model.K8sDeployReplication, error)
	BeachDelete([]uint) error
	GetK8sDeployReplicationByIsDelete(rcType string, isDelete bool) ([]*model.K8sDeployReplication, error)
	GetK8sDeployReplicationIsExist(tenantId string, RcType string, RcId string, isDelete bool) (IsExist bool)
}

//K8sPodDao pod info dao
type K8sPodDao interface {
	Dao
	DeleteK8sPod(serviceID string) error
	DeleteK8sPodByName(podName string) error
	GetPodByService(serviceID string) ([]*model.K8sPod, error)
	GetPodByReplicationID(replicationID string) ([]*model.K8sPod, error)
	GetK8sPodByNotInPodNameList(podNameList []string) ([]*model.K8sPod, error)
}

//LocalSchedulerDao 本地调度信息
type LocalSchedulerDao interface {
	Dao
	GetLocalScheduler(serviceID string) ([]*model.LocalScheduler, error)
}

//ServiceProbeDao ServiceProbeDao
type ServiceProbeDao interface {
	Dao
	DelDao
	GetServiceProbes(serviceID string) ([]*model.ServiceProbe, error)
	GetServiceUsedProbe(serviceID, mode string) (*model.ServiceProbe, error)
	DELServiceProbesByServiceID(serviceID string) error
}

//ServiceStatusDao service status
type ServiceStatusDao interface {
	Dao
	GetTenantServiceStatus(serviceID string) (*model.TenantServiceStatus, error)
	SetTenantServiceStatus(serviceID, status string) error
	GetRunningService() ([]*model.TenantServiceStatus, error)
	GetAll() ([]*model.TenantServiceStatus, error)
	GetNeedBillingService() ([]*model.TenantServiceStatus, error)
	GetTenantStatus(tenantID string) ([]*model.TenantServiceStatus, error)
	GetTenantServicesStatus(serviceIDs []string) ([]*model.TenantServiceStatus, error)
	DeleteByServiceID(serviceID string) error
}

//CodeCheckResultDao CodeCheckResultDao
type CodeCheckResultDao interface {
	Dao
	GetCodeCheckResult(serviceID string) (*model.CodeCheckResult, error)
}

//AppPublishDao AppPublishDao
type AppPublishDao interface {
	Dao
	GetAppPublish(serviceKey, appVersion string) (*model.AppPublish, error)
}

//EventDao EventDao
type EventDao interface {
	Dao
	GetEventByEventID(eventID string) (*model.ServiceEvent, error)
	GetEventByEventIDs(eventIDs []string) ([]*model.ServiceEvent, error)
	GetEventByServiceID(serviceID string) ([]*model.ServiceEvent, error)
	DelEventByServiceID(serviceID string) error
}

//VersionInfoDao VersionInfoDao
type VersionInfoDao interface {
	Dao
	GetVersionByEventID(eventID string) (*model.VersionInfo, error)
	GetVersionByDeployVersion(version, serviceID string) (*model.VersionInfo, error)
	GetVersionByServiceID(serviceID string) ([]*model.VersionInfo, error)
	DeleteVersionByEventID(eventID string) error
	DeleteVersionByServiceID(serviceID string) error
	GetVersionInfo(timePoint time.Time, serviceIdList []string) ([]*model.VersionInfo, error)
	DeleteVersionInfo(obj *model.VersionInfo) error
	DeleteFailureVersionInfo(timePoint time.Time, status string, serviceIdList []string) error
	SearchVersionInfo() ([]*model.VersionInfo, error)
}

//RegionUserInfoDao UserRegionInfoDao
type RegionUserInfoDao interface {
	Dao
	GetALLTokenInValidityPeriod() ([]*model.RegionUserInfo, error)
	GetTokenByEid(eid string) (*model.RegionUserInfo, error)
	GetTokenByTokenID(token string) (*model.RegionUserInfo, error)
}

//RegionAPIClassDao RegionAPIClassDao
type RegionAPIClassDao interface {
	Dao
	GetPrefixesByClass(apiClass string) ([]*model.RegionAPIClass, error)
	DeletePrefixInClass(apiClass, prefix string) error
}

//RegionProcotolsDao RegionProcotolsDao
type RegionProcotolsDao interface {
	Dao
	GetAllSupportProtocol(version string) ([]*model.RegionProcotols, error)
	GetProtocolGroupByProtocolChild(version, protocolChild string) (*model.RegionProcotols, error)
}

//NotificationEventDao NotificationEventDao
type NotificationEventDao interface {
	Dao
	GetNotificationEventByHash(hash string) (*model.NotificationEvent, error)
	GetNotificationEventByKind(kind, kindID string) ([]*model.NotificationEvent, error)
	GetNotificationEventByTime(start, end time.Time) ([]*model.NotificationEvent, error)
	GetNotificationEventNotHandle() ([]*model.NotificationEvent, error)
	GetNotificationEventGrouping(start, end time.Time) ([]*model.NotificationEvent, error)
}

//AppBackupDao group app backup history
type AppBackupDao interface {
	Dao
	CheckHistory(groupID, version string) bool
	GetAppBackups(groupID string) ([]*model.AppBackup, error)
	DeleteAppBackup(backupID string) error
	GetAppBackup(backupID string) (*model.AppBackup, error)
	GetDeleteAppBackup(backupID string) (*model.AppBackup, error)
	GetDeleteAppBackups() ([]*model.AppBackup, error)
}
