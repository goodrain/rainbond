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
	"errors"
	"time"

	"github.com/goodrain/rainbond/db/model"
)

var (
	// ErrVolumeNotFound volume not found error, happens when haven't find any matched data
	ErrVolumeNotFound = errors.New("volume not found")
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

// EnterpriseDao enterprise dao
type EnterpriseDao interface {
	GetEnterpriseTenants(enterpriseID string) ([]*model.Tenants, error)
}

//TenantDao tenant dao
type TenantDao interface {
	Dao
	GetTenantByUUID(uuid string) (*model.Tenants, error)
	GetTenantIDByName(tenantName string) (*model.Tenants, error)
	GetALLTenants(query string) ([]*model.Tenants, error)
	GetTenantByEid(eid, query string) ([]*model.Tenants, error)
	GetPagedTenants(offset, len int) ([]*model.Tenants, error)
	GetTenantIDsByNames(names []string) ([]string, error)
	GetTenantLimitsByNames(names []string) (map[string]int, error)
	GetTenantByUUIDIsExist(uuid string) bool
	DelByTenantID(tenantID string) error
}

//AppDao tenant dao
type AppDao interface {
	Dao
	GetByEventId(eventID string) (*model.AppStatus, error)
	DeleteModelByEventId(eventID string) error
}

//ApplicationDao tenant Application Dao
type ApplicationDao interface {
	Dao
	ListApps(tenantID, appName string, page, pageSize int) ([]*model.Application, int64, error)
	GetAppByID(appID string) (*model.Application, error)
	DeleteApp(appID string) error
	GetByServiceID(sid string) (*model.Application, error)
}

//AppConfigGroupDao Application config group Dao
type AppConfigGroupDao interface {
	Dao
	GetConfigGroupByID(appID, configGroupName string) (*model.ApplicationConfigGroup, error)
	ListByServiceID(sid string) ([]*model.ApplicationConfigGroup, error)
	GetConfigGroupsByAppID(appID string, page, pageSize int) ([]*model.ApplicationConfigGroup, int64, error)
	DeleteConfigGroup(appID, configGroupName string) error
	DeleteByAppID(appID string) error
	CreateOrUpdateConfigGroupsInBatch(cgroups []*model.ApplicationConfigGroup) error
}

//AppConfigGroupServiceDao service config group Dao
type AppConfigGroupServiceDao interface {
	Dao
	GetConfigGroupServicesByID(appID, configGroupName string) ([]*model.ConfigGroupService, error)
	DeleteConfigGroupService(appID, configGroupName string) error
	DeleteEffectiveServiceByServiceID(serviceID string) error
	DeleteByComponentIDs(componentIDs []string) error
	CreateOrUpdateConfigGroupServicesInBatch(cgservices []*model.ConfigGroupService) error
	DeleteByAppID(appID string) error
}

//AppConfigGroupItemDao Application config item group Dao
type AppConfigGroupItemDao interface {
	Dao
	GetConfigGroupItemsByID(appID, configGroupName string) ([]*model.ConfigGroupItem, error)
	ListByServiceID(sid string) ([]*model.ConfigGroupItem, error)
	DeleteConfigGroupItem(appID, configGroupName string) error
	DeleteByAppID(appID string) error
	CreateOrUpdateConfigGroupItemsInBatch(cgitems []*model.ConfigGroupItem) error
}

// VolumeTypeDao volume type dao
type VolumeTypeDao interface {
	Dao
	DeleteModelByVolumeTypes(volumeType string) error
	GetAllVolumeTypes() ([]*model.TenantServiceVolumeType, error)
	GetAllVolumeTypesByPage(page int, pageSize int) ([]*model.TenantServiceVolumeType, error)
	GetVolumeTypeByType(vt string) (*model.TenantServiceVolumeType, error)
	CreateOrUpdateVolumeType(vt *model.TenantServiceVolumeType) (*model.TenantServiceVolumeType, error)
}

//LicenseDao LicenseDao
type LicenseDao interface {
	Dao
	//DeleteLicense(token string) error
	ListLicenses() ([]*model.LicenseInfo, error)
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
	GetServicesInfoByAppID(appID string, page, pageSize int) ([]*model.TenantServices, int64, error)
	CountServiceByAppID(appID string) (int64, error)
	GetServiceIDsByAppID(appID string) (re []model.ServiceID)
	GetServicesByServiceIDs(serviceIDs []string) ([]*model.TenantServices, error)
	DeleteServiceByServiceID(serviceID string) error
	GetServiceMemoryByTenantIDs(tenantIDs, serviceIDs []string) (map[string]map[string]interface{}, error)
	GetServiceMemoryByServiceIDs(serviceIDs []string) (map[string]map[string]interface{}, error)
	GetPagedTenantService(offset, len int, serviceIDs []string) ([]map[string]interface{}, int, error)
	GetAllServicesID() ([]*model.TenantServices, error)
	UpdateDeployVersion(serviceID, deployversion string) error
	ListThirdPartyServices() ([]*model.TenantServices, error)
	ListServicesByTenantID(tenantID string) ([]*model.TenantServices, error)
	GetServiceTypeByID(serviceID string) (*model.TenantServices, error)
	ListByAppID(appID string) ([]*model.TenantServices, error)
	BindAppByServiceIDs(appID string, serviceIDs []string) error
	CreateOrUpdateComponentsInBatch(components []*model.TenantServices) error
	DeleteByComponentIDs(tenantID, appID string, componentIDs []string) error
}

//TenantServiceDeleteDao TenantServiceDeleteDao
type TenantServiceDeleteDao interface {
	Dao
	GetTenantServicesDeleteByCreateTime(createTime time.Time) ([]*model.TenantServicesDelete, error)
	DeleteTenantServicesDelete(record *model.TenantServicesDelete) error
	List() ([]*model.TenantServicesDelete, error)
}

//TenantServicesPortDao TenantServicesPortDao
type TenantServicesPortDao interface {
	Dao
	DelDao
	GetByTenantAndName(tenantID, name string) (*model.TenantServicesPort, error)
	GetPortsByServiceID(serviceID string) ([]*model.TenantServicesPort, error)
	GetOuterPorts(serviceID string) ([]*model.TenantServicesPort, error)
	GetInnerPorts(serviceID string) ([]*model.TenantServicesPort, error)
	GetPort(serviceID string, port int) (*model.TenantServicesPort, error)
	GetOpenedPorts(serviceID string) ([]*model.TenantServicesPort, error)
	//GetDepUDPPort get all depend service udp port info
	GetDepUDPPort(serviceID string) ([]*model.TenantServicesPort, error)
	DELPortsByServiceID(serviceID string) error
	HasOpenPort(sid string) bool
	DelByServiceID(sid string) error
	ListInnerPortsByServiceIDs(serviceIDs []string) ([]*model.TenantServicesPort, error)
	ListByK8sServiceNames(serviceIDs []string) ([]*model.TenantServicesPort, error)
	CreateOrUpdatePortsInBatch(ports []*model.TenantServicesPort) error
	DeleteByComponentIDs(componentIDs []string) error
}

//TenantPluginDao TenantPluginDao
type TenantPluginDao interface {
	Dao
	GetPluginByID(pluginID, tenantID string) (*model.TenantPlugin, error)
	DeletePluginByID(pluginID, tenantID string) error
	GetPluginsByTenantID(tenantID string) ([]*model.TenantPlugin, error)
	ListByIDs(ids []string) ([]*model.TenantPlugin, error)
	ListByTenantID(tenantID string) ([]*model.TenantPlugin, error)
	CreateOrUpdatePluginsInBatch(plugins []*model.TenantPlugin) error
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
	ListSuccessfulOnesByPluginIDs(pluginIDs []string) ([]*model.TenantPluginBuildVersion, error)
}

//TenantPluginVersionEnvDao TenantPluginVersionEnvDao
type TenantPluginVersionEnvDao interface {
	Dao
	DeleteEnvByEnvName(envName, pluginID, serviceID string) error
	DeleteEnvByPluginID(serviceID, pluginID string) error
	DeleteEnvByServiceID(serviceID string) error
	GetVersionEnvByServiceID(serviceID string, pluginID string) ([]*model.TenantPluginVersionEnv, error)
	ListByServiceID(serviceID string) ([]*model.TenantPluginVersionEnv, error)
	GetVersionEnvByEnvName(serviceID, pluginID, envName string) (*model.TenantPluginVersionEnv, error)
	DeleteByComponentIDs(componentIDs []string) error
	CreateOrUpdatePluginVersionEnvsInBatch(versionEnvs []*model.TenantPluginVersionEnv) error
}

//TenantPluginVersionConfigDao service plugin config that can be dynamic discovery dao interface
type TenantPluginVersionConfigDao interface {
	Dao
	GetPluginConfig(serviceID, pluginID string) (*model.TenantPluginVersionDiscoverConfig, error)
	GetPluginConfigs(serviceID string) ([]*model.TenantPluginVersionDiscoverConfig, error)
	DeletePluginConfig(serviceID, pluginID string) error
	DeletePluginConfigByServiceID(serviceID string) error
	DeleteByComponentIDs(componentIDs []string) error
	CreateOrUpdatePluginVersionConfigsInBatch(versionConfigs []*model.TenantPluginVersionDiscoverConfig) error
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
	DeleteByComponentIDs(componentIDs []string) error
	CreateOrUpdatePluginRelsInBatch(relations []*model.TenantServicePluginRelation) error
}

//TenantServiceRelationDao TenantServiceRelationDao
type TenantServiceRelationDao interface {
	Dao
	DelDao
	GetTenantServiceRelations(serviceID string) ([]*model.TenantServiceRelation, error)
	ListByServiceIDs(serviceIDs []string) ([]*model.TenantServiceRelation, error)
	GetTenantServiceRelationsByDependServiceID(dependServiceID string) ([]*model.TenantServiceRelation, error)
	HaveRelations(serviceID string) bool
	DELRelationsByServiceID(serviceID string) error
	DeleteRelationByDepID(serviceID, depID string) error
	DeleteByComponentIDs(componentIDs []string) error
	CreateOrUpdateRelationsInBatch(relations []*model.TenantServiceRelation) error
}

//TenantServicesStreamPluginPortDao TenantServicesStreamPluginPortDao
type TenantServicesStreamPluginPortDao interface {
	Dao
	GetPluginMappingPorts(serviceID string) ([]*model.TenantServicesStreamPluginPort, error)
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
	ListByServiceID(sid string) ([]*model.TenantServicesStreamPluginPort, error)
	DeleteByComponentIDs(componentIDs []string) error
	CreateOrUpdateStreamPluginPortsInBatch(spPorts []*model.TenantServicesStreamPluginPort) error
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
	DelByServiceIDAndScope(sid, scope string) error
	CreateOrUpdateEnvsInBatch(envs []*model.TenantServiceEnvVar) error
	DeleteByComponentIDs(componentIDs []string) error
}

//TenantServiceMountRelationDao TenantServiceMountRelationDao
type TenantServiceMountRelationDao interface {
	Dao
	GetTenantServiceMountRelationsByService(serviceID string) ([]*model.TenantServiceMountRelation, error)
	DElTenantServiceMountRelationByServiceAndName(serviceID, mntDir string) error
	DELTenantServiceMountRelationByServiceID(serviceID string) error
	DElTenantServiceMountRelationByDepService(serviceID, depServiceID string) error
	DeleteByComponentIDs(componentIDs []string) error
	CreateOrUpdateVolumeRelsInBatch(volRels []*model.TenantServiceMountRelation) error
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
	GetVolumeByID(id int) (*model.TenantServiceVolume, error)
	DelShareableBySID(sid string) error
	ListVolumesByComponentIDs(componentIDs []string) ([]*model.TenantServiceVolume, error)
	DeleteByVolumeIDs(volumeIDs []uint) error
	CreateOrUpdateVolumesInBatch(volumes []*model.TenantServiceVolume) error
}

//TenantServiceConfigFileDao tenant service config file dao interface
type TenantServiceConfigFileDao interface {
	Dao
	GetConfigFileByServiceID(serviceID string) ([]*model.TenantServiceConfigFile, error)
	GetByVolumeName(sid, volumeName string) (*model.TenantServiceConfigFile, error)
	DelByVolumeID(sid string, volumeName string) error
	DelByServiceID(sid string) error
	DeleteByComponentIDs(componentIDs []string) error
	CreateOrUpdateConfigFilesInBatch(configFiles []*model.TenantServiceConfigFile) error
}

//TenantServiceLBMappingPortDao vs lb mapping port dao
type TenantServiceLBMappingPortDao interface {
	Dao
	GetTenantServiceLBMappingPort(serviceID string, containerPort int) (*model.TenantServiceLBMappingPort, error)
	GetLBMappingPortByServiceIDAndPort(serviceID string, port int) (*model.TenantServiceLBMappingPort, error)
	GetTenantServiceLBMappingPortByService(serviceID string) ([]*model.TenantServiceLBMappingPort, error)
	GetLBPortsASC() ([]*model.TenantServiceLBMappingPort, error)
	CreateTenantServiceLBMappingPort(serviceID string, containerPort int) (*model.TenantServiceLBMappingPort, error)
	DELServiceLBMappingPortByServiceID(serviceID string) error
	DELServiceLBMappingPortByServiceIDAndPort(serviceID string, lbPort int) error
	GetLBPortByTenantAndPort(tenantID string, lbport int) (*model.TenantServiceLBMappingPort, error)
	PortExists(port int) bool
}

//TenantServiceLabelDao TenantServiceLabelDao
type TenantServiceLabelDao interface {
	Dao
	DelDao
	GetTenantServiceLabel(serviceID string) ([]*model.TenantServiceLable, error)
	DeleteLabelByServiceID(serviceID string) error
	GetTenantServiceNodeSelectorLabel(serviceID string) ([]*model.TenantServiceLable, error)
	GetTenantNodeAffinityLabel(serviceID string) (*model.TenantServiceLable, error)
	GetTenantServiceAffinityLabel(serviceID string) ([]*model.TenantServiceLable, error)
	GetTenantServiceTypeLabel(serviceID string) (*model.TenantServiceLable, error)
	DelTenantServiceLabelsByLabelValuesAndServiceID(serviceID string) error
	DelTenantServiceLabelsByServiceIDKey(serviceID string, labelKey string) error
	DelTenantServiceLabelsByServiceIDKeyValue(serviceID string, labelKey string, labelValue string) error
	GetLabelByNodeSelectorKey(serviceID string, labelValue string) (*model.TenantServiceLable, error)
	GetPrivilegedLabel(serviceID string) (*model.TenantServiceLable, error)
	DeleteByComponentIDs(componentIDs []string) error
	CreateOrUpdateLabelsInBatch(labels []*model.TenantServiceLable) error
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
	GetServiceProbes(serviceID string) ([]*model.TenantServiceProbe, error)
	GetServiceUsedProbe(serviceID, mode string) (*model.TenantServiceProbe, error)
	DELServiceProbesByServiceID(serviceID string) error
	DelByServiceID(sid string) error
	DeleteByComponentIDs(componentIDs []string) error
	CreateOrUpdateProbesInBatch(probes []*model.TenantServiceProbe) error
}

//CodeCheckResultDao CodeCheckResultDao
type CodeCheckResultDao interface {
	Dao
	GetCodeCheckResult(serviceID string) (*model.CodeCheckResult, error)
	DeleteByServiceID(serviceID string) error
}

//EventDao EventDao
type EventDao interface {
	Dao
	CreateEventsInBatch(events []*model.ServiceEvent) error
	GetEventByEventID(eventID string) (*model.ServiceEvent, error)
	GetEventByEventIDs(eventIDs []string) ([]*model.ServiceEvent, error)
	GetEventByServiceID(serviceID string) ([]*model.ServiceEvent, error)
	DelEventByServiceID(serviceID string) error
	ListByTargetID(targetID string) ([]*model.ServiceEvent, error)
	GetEventsByTarget(target, targetID string, offset, liimt int) ([]*model.ServiceEvent, int, error)
	GetEventsByTenantID(tenantID string, offset, limit int) ([]*model.ServiceEvent, int, error)
	GetLastASyncEvent(target, targetID string) (*model.ServiceEvent, error)
	UnfinishedEvents(target, targetID string, optTypes ...string) ([]*model.ServiceEvent, error)
	LatestFailurePodEvent(podName string) (*model.ServiceEvent, error)
	UpdateReason(eventID string, reason string) error
}

//VersionInfoDao VersionInfoDao
type VersionInfoDao interface {
	Dao
	ListSuccessfulOnes() ([]*model.VersionInfo, error)
	GetVersionByEventID(eventID string) (*model.VersionInfo, error)
	GetVersionByDeployVersion(version, serviceID string) (*model.VersionInfo, error)
	GetVersionByServiceID(serviceID string) ([]*model.VersionInfo, error)
	GetLatestScsVersion(sid string) (*model.VersionInfo, error)
	GetAllVersionByServiceID(serviceID string) ([]*model.VersionInfo, error)
	DeleteVersionByEventID(eventID string) error
	DeleteVersionByServiceID(serviceID string) error
	GetVersionInfo(timePoint time.Time, serviceIDList []string) ([]*model.VersionInfo, error)
	DeleteVersionInfo(obj *model.VersionInfo) error
	DeleteFailureVersionInfo(timePoint time.Time, status string, serviceIDList []string) error
	SearchVersionInfo() ([]*model.VersionInfo, error)
	ListByServiceIDStatus(serviceID string, finalStatus *bool) ([]*model.VersionInfo, error)
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

//NotificationEventDao NotificationEventDao
type NotificationEventDao interface {
	Dao
	GetNotificationEventByHash(hash string) (*model.NotificationEvent, error)
	GetNotificationEventByKind(kind, kindID string) ([]*model.NotificationEvent, error)
	GetNotificationEventByTime(start, end time.Time) ([]*model.NotificationEvent, error)
	GetNotificationEventNotHandle() ([]*model.NotificationEvent, error)
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

//ServiceSourceDao service source dao
type ServiceSourceDao interface {
	Dao
	GetServiceSource(serviceID string) ([]*model.ServiceSourceConfig, error)
}

// CertificateDao -
type CertificateDao interface {
	Dao
	AddOrUpdate(mo model.Interface) error
	GetCertificateByID(certificateID string) (*model.Certificate, error)
}

// RuleExtensionDao -
type RuleExtensionDao interface {
	Dao
	GetRuleExtensionByRuleID(ruleID string) ([]*model.RuleExtension, error)
	DeleteRuleExtensionByRuleID(ruleID string) error
	DeleteByRuleIDs(ruleIDs []string) error
}

// HTTPRuleDao -
type HTTPRuleDao interface {
	Dao
	GetHTTPRuleByID(id string) (*model.HTTPRule, error)
	GetHTTPRuleByServiceIDAndContainerPort(serviceID string, containerPort int) ([]*model.HTTPRule, error)
	GetHTTPRulesByCertificateID(certificateID string) ([]*model.HTTPRule, error)
	DeleteHTTPRuleByID(id string) error
	DeleteHTTPRuleByServiceID(serviceID string) error
	ListByServiceID(serviceID string) ([]*model.HTTPRule, error)
	ListByComponentPort(componentID string, port int) ([]*model.HTTPRule, error)
	ListByCertID(certID string) ([]*model.HTTPRule, error)
	DeleteByComponentPort(componentID string, port int) error
	DeleteByComponentIDs(componentIDs []string) error
	CreateOrUpdateHTTPRuleInBatch(httpRules []*model.HTTPRule) error
}

// TCPRuleDao -
type TCPRuleDao interface {
	Dao
	GetTCPRuleByServiceIDAndContainerPort(serviceID string, containerPort int) ([]*model.TCPRule, error)
	GetTCPRuleByID(id string) (*model.TCPRule, error)
	GetTCPRuleByServiceID(sid string) ([]*model.TCPRule, error)
	DeleteByID(uuid string) error
	DeleteTCPRuleByServiceID(serviceID string) error
	ListByServiceID(serviceID string) ([]*model.TCPRule, error)
	GetUsedPortsByIP(ip string) ([]*model.TCPRule, error)
	DeleteByComponentPort(componentID string, port int) error
	DeleteByComponentIDs(componentIDs []string) error
	CreateOrUpdateTCPRuleInBatch(tcpRules []*model.TCPRule) error
}

// EndpointsDao is an interface for defining method
// for operating table 3rd_party_svc_endpoints.
type EndpointsDao interface {
	Dao
	GetByUUID(uuid string) (*model.Endpoint, error)
	DelByUUID(uuid string) error
	List(sid string) ([]*model.Endpoint, error)
	ListIsOnline(sid string) ([]*model.Endpoint, error)
	DeleteByServiceID(sid string) error
}

// ThirdPartySvcDiscoveryCfgDao is an interface for defining method
// for operating table 3rd_party_svc_discovery_cfg.
type ThirdPartySvcDiscoveryCfgDao interface {
	Dao
	GetByServiceID(sid string) (*model.ThirdPartySvcDiscoveryCfg, error)
	DeleteByServiceID(sid string) error
}

// GwRuleConfigDao is the interface that wraps the required methods to execute
// curd for table gateway_rule_config.
type GwRuleConfigDao interface {
	Dao
	DeleteByRuleID(rid string) error
	ListByRuleID(rid string) ([]*model.GwRuleConfig, error)
	DeleteByRuleIDs(ruleIDs []string) error
}

// TenantServceAutoscalerRulesDao -
type TenantServceAutoscalerRulesDao interface {
	Dao
	GetByRuleID(ruleID string) (*model.TenantServiceAutoscalerRules, error)
	ListByServiceID(serviceID string) ([]*model.TenantServiceAutoscalerRules, error)
	ListEnableOnesByServiceID(serviceID string) ([]*model.TenantServiceAutoscalerRules, error)
	DeleteByComponentIDs(componentIDs []string) error
	CreateOrUpdateScaleRulesInBatch(rules []*model.TenantServiceAutoscalerRules) error
}

// TenantServceAutoscalerRuleMetricsDao -
type TenantServceAutoscalerRuleMetricsDao interface {
	Dao
	UpdateOrCreate(metric *model.TenantServiceAutoscalerRuleMetrics) error
	ListByRuleID(ruleID string) ([]*model.TenantServiceAutoscalerRuleMetrics, error)
	DeleteByRuleID(ruldID string) error
	DeleteByRuleIDs(ruleIDs []string) error
	CreateOrUpdateScaleRuleMetricsInBatch(metrics []*model.TenantServiceAutoscalerRuleMetrics) error
}

// TenantServiceScalingRecordsDao -
type TenantServiceScalingRecordsDao interface {
	Dao
	UpdateOrCreate(new *model.TenantServiceScalingRecords) error
	ListByServiceID(serviceID string, offset, limit int) ([]*model.TenantServiceScalingRecords, error)
	CountByServiceID(serviceID string) (int, error)
}

// TenantServiceMonitorDao -
type TenantServiceMonitorDao interface {
	Dao
	GetByName(serviceID, name string) (*model.TenantServiceMonitor, error)
	GetByServiceID(serviceID string) ([]*model.TenantServiceMonitor, error)
	DeleteServiceMonitor(mo *model.TenantServiceMonitor) error
	DeleteServiceMonitorByServiceID(serviceID string) error
	DeleteByComponentIDs(componentIDs []string) error
	CreateOrUpdateMonitorInBatch(monitors []*model.TenantServiceMonitor) error
}
