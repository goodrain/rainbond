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

package dao

import "github.com/goodrain/rainbond/pkg/db/model"

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
	//GetTenantsOrderByUsedMemPaged(page,pagesize int) ([]*model.Tenants, error)
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
}

//TenantServiceDao TenantServiceDao
type TenantServiceDao interface {
	Dao
	GetServiceByID(serviceID string) (*model.TenantServices, error)
	GetServiceAliasByIDs(uids []string) ([]*model.TenantServices, error)
	GetServiceByTenantIDAndServiceAlias(tenantID, serviceName string) (*model.TenantServices, error)
	SetTenantServiceStatus(serviceID, status string) error
	GetServicesByTenantID(tenantID string) ([]*model.TenantServices, error)
	GetServicesAllInfoByTenantID(tenantID string) ([]*model.TenantServices, error)
	DeleteServiceByServiceID(serviceID string) error
	GetCPUAndMEM(tenantName []string) ([]*map[string]interface{}, error)
}

//TenantServiceDeleteDao TenantServiceDeleteDao
type TenantServiceDeleteDao interface {
	Dao
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
	GetPluginByID(pluginID string) (*model.TenantPlugin, error)
	DeletePluginByID(pluginID string) error
	GetPluginsByTenantID(tenantID string) ([]*model.TenantPlugin, error)
}

//TenantPluginDefaultENVDao TenantPluginDefaultENVDao
type TenantPluginDefaultENVDao interface {
	Dao
	GetDefaultENVByName(pluginID, ENVName string) (*model.TenantPluginDefaultENV, error)
	GetDefaultENVSByPluginID(pluginID string) ([]*model.TenantPluginDefaultENV, error)
	GetDefaultENVSByPluginIDCantBeSet(pluginID string) ([]*model.TenantPluginDefaultENV, error)
	DeleteDefaultENVByName(pluginID, ENVName string) error
	DeleteAllDefaultENVByPluginID(PluginID string) error
	GetDefaultEnvWhichCanBeSetByPluginID(pluginID string) ([]*model.TenantPluginDefaultENV, error)
}

//TenantPluginBuildVersionDao TenantPluginBuildVersionDao
type TenantPluginBuildVersionDao interface {
	Dao
	DeleteBuildVersionByVersionID(versionID string) error
	DeleteBuildVersionByPluginID(pluginID string) error
	GetBuildVersionByPluginID(pluginID string) ([]*model.TenantPluginBuildVersion, error)
	GetBuildVersionByVersionID(pluginID, versionID string) (*model.TenantPluginBuildVersion, error)
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
}

//TenantServiceLBMappingPortDao vs lb mapping port dao
type TenantServiceLBMappingPortDao interface {
	Dao
	GetTenantServiceLBMappingPort(serviceID string, containerPort int) (*model.TenantServiceLBMappingPort, error)
	GetTenantServiceLBMappingPortByService(serviceID string) (*model.TenantServiceLBMappingPort, error)
	CreateTenantServiceLBMappingPort(serviceID string, containerPort int) (*model.TenantServiceLBMappingPort, error)
	DELServiceLBMappingPortByServiceID(serviceID string) error
}

//TenantServiceLabelDao TenantServiceLabelDao
type TenantServiceLabelDao interface {
	Dao
	DelDao
	GetTenantServiceLabel(serviceID string) ([]*model.TenantServiceLable, error)
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
	//不真正删除，设置IS_DELETE 为true
	DeleteK8sDeployReplicationByService(serviceID string) error
	GetReplications() ([]*model.K8sDeployReplication, error)
	BeachDelete([]uint) error
}

//K8sPodDao pod info dao
type K8sPodDao interface {
	Dao
	DeleteK8sPod(serviceID string) error
	DeleteK8sPodByName(podName string) error
	GetPodByService(serviceID string) ([]*model.K8sPod, error)
	GetPodByReplicationID(replicationID string) ([]*model.K8sPod, error)
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
	GetTenantStatus(tenantID string) ([]*model.TenantServiceStatus, error)
	GetTenantServicesStatus(serviceIDs []string) ([]*model.TenantServiceStatus, error)
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
	GetEventByServiceID(serviceID string) ([]*model.ServiceEvent, error)
}

//VersionInfoDao VersionInfoDao
type VersionInfoDao interface {
	Dao
	GetVersionByEventID(eventID string) (*model.VersionInfo, error)
	GetVersionByDeployVersion(version string) (*model.VersionInfo, error)
	GetVersionByServiceID(serviceID string) ([]*model.VersionInfo, error)
	DeleteVersionByEventID(eventID string) error
}

//RegionUserInfoDao UserRegionInfoDao
type RegionUserInfoDao interface {
	Dao
	GetALLTokenInValidityPeriod() ([]*model.RegionUserInfo, error)
	GetTokenByEid(eid string) (*model.RegionUserInfo, error)
}
