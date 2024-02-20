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

package handler

import (
	"context"
	"net/http"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/builder/exector"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/worker/discover/model"
	"github.com/goodrain/rainbond/worker/server/pb"
	"github.com/jinzhu/gorm"
)

// ServiceHandler service handler
type ServiceHandler interface {
	ServiceBuild(tenantID, serviceID string, r *apimodel.BuildServiceStruct) error
	AddLabel(l *apimodel.LabelsStruct, serviceID string) error
	DeleteLabel(l *apimodel.LabelsStruct, serviceID string) error
	UpdateLabel(l *apimodel.LabelsStruct, serviceID string) error
	StartStopService(s *apimodel.StartStopStruct) error
	PauseUNPauseService(serviceID string, pauseORunpause string) error
	ServiceVertical(ctx context.Context, v *model.VerticalScalingTaskBody) error
	ServiceHorizontal(h *model.HorizontalScalingTaskBody) error
	ServiceUpgrade(r *model.RollingUpgradeTaskBody) error
	ServiceCreate(ts *apimodel.ServiceStruct) error
	ServiceUpdate(sc map[string]interface{}) error
	LanguageSet(langS *apimodel.LanguageSet) error
	GetService(tenantID string) ([]*dbmodel.TenantServices, error)
	GetServicesByAppID(appID string, page, pageSize int) (*apimodel.ListServiceResponse, error)
	GetPagedTenantRes(offset, len int) ([]*apimodel.TenantResource, int, error)
	GetTenantRes(uuid string) (*apimodel.TenantResource, error)
	CodeCheck(c *apimodel.CheckCodeStruct) error
	ServiceDepend(action string, ds *apimodel.DependService) error
	EnvAttr(action string, at *dbmodel.TenantServiceEnvVar) error
	PortVar(action string, tenantID, serviceID string, vp *apimodel.ServicePorts, oldPort int) error
	CreatePorts(tenantID, serviceID string, vps *apimodel.ServicePorts) error
	PortOuter(tenantName, serviceID string, containerPort int, servicePort *apimodel.ServicePortInnerOrOuter) (*dbmodel.TenantServiceLBMappingPort, string, error)
	PortInner(tenantName, serviceID, operation string, port int) error
	VolumnVar(avs *dbmodel.TenantServiceVolume, tenantID, fileContent, action string) *util.APIHandleError
	UpdVolume(sid string, req *apimodel.UpdVolumeReq) error
	VolumeDependency(tsr *dbmodel.TenantServiceMountRelation, action string) *util.APIHandleError
	GetDepVolumes(serviceID string) ([]*dbmodel.TenantServiceMountRelation, *util.APIHandleError)
	GetVolumes(serviceID string) ([]*apimodel.VolumeWithStatusStruct, *util.APIHandleError)
	ServiceProbe(tsp *dbmodel.TenantServiceProbe, action string) error
	RollBack(rs *apimodel.RollbackStruct) error
	GetStatus(serviceID string) (*apimodel.StatusList, error)
	GetServicesStatus(tenantID string, services []string) []map[string]interface{}
	GetEnterpriseServicesStatus(enterpriseID string) (map[string]string, *util.APIHandleError)
	CreateTenant(*dbmodel.Tenants) error
	CreateTenandIDAndName(eid string) (string, string, error)
	GetPods(serviceID string) (*K8sPodInfos, error)
	GetMultiServicePods(serviceIDs []string) (*K8sPodInfos, error)
	GetComponentPodNums(ctx context.Context, componentIDs []string) (map[string]int32, error)
	TransServieToDelete(ctx context.Context, tenantID, serviceID string) error
	TenantServiceDeletePluginRelation(tenantID, serviceID, pluginID string) *util.APIHandleError
	GetTenantServicePluginRelation(serviceID string) ([]*dbmodel.TenantServicePluginRelation, *util.APIHandleError)
	SetTenantServicePluginRelation(tenantID, serviceID string, pss *apimodel.PluginSetStruct) (*dbmodel.TenantServicePluginRelation, *util.APIHandleError)
	UpdateTenantServicePluginRelation(serviceID string, pss *apimodel.PluginSetStruct) (*dbmodel.TenantServicePluginRelation, *util.APIHandleError)
	UpdateVersionEnv(uve *apimodel.SetVersionEnv) *util.APIHandleError
	DeletePluginConfig(serviceID, pluginID string) *util.APIHandleError
	ServiceCheck(*apimodel.ServiceCheckStruct) (string, string, *util.APIHandleError)
	RegistryImageRepositories(namespace string) ([]string, *util.APIHandleError)
	RegistryImageTags(repository string) ([]string, *util.APIHandleError)
	GetServiceCheckInfo(uuid string) (*exector.ServiceCheckResult, *util.APIHandleError)
	GetServiceDeployInfo(tenantID, serviceID string) (*pb.DeployInfo, *util.APIHandleError)
	ListVersionInfo(serviceID string) (*apimodel.BuildListRespVO, error)
	EventBuildVersion(serviceID, buildVersion string) (*apimodel.BuildListRespVO, error)

	AddAutoscalerRule(req *apimodel.AutoscalerRuleReq) error
	UpdAutoscalerRule(req *apimodel.AutoscalerRuleReq) error
	ListScalingRecords(serviceID string, page, pageSize int) ([]*dbmodel.TenantServiceScalingRecords, int, error)

	UpdateServiceMonitor(tenantID, serviceID, name string, update apimodel.UpdateServiceMonitorRequestStruct) (*dbmodel.TenantServiceMonitor, error)
	DeleteServiceMonitor(tenantID, serviceID, name string) (*dbmodel.TenantServiceMonitor, error)
	AddServiceMonitor(tenantID, serviceID string, add apimodel.AddServiceMonitorRequestStruct) (*dbmodel.TenantServiceMonitor, error)

	ReviseAttributeAffinityByArch(attributeValue string, arch string) (string, error)
	GetK8sAttribute(componentID, name string) (*dbmodel.ComponentK8sAttributes, error)
	CreateK8sAttribute(tenantID, componentID string, k8sAttr *apimodel.ComponentK8sAttribute) error
	UpdateK8sAttribute(componentID string, k8sAttributes *apimodel.ComponentK8sAttribute) error
	DeleteK8sAttribute(componentID, name string) error

	SyncComponentBase(tx *gorm.DB, app *dbmodel.Application, components []*apimodel.Component) error
	SyncComponentMonitors(tx *gorm.DB, app *dbmodel.Application, components []*apimodel.Component) error
	SyncComponentPorts(tx *gorm.DB, app *dbmodel.Application, components []*apimodel.Component) error
	SyncComponentRelations(tx *gorm.DB, app *dbmodel.Application, components []*apimodel.Component) error
	SyncComponentEnvs(tx *gorm.DB, app *dbmodel.Application, components []*apimodel.Component) error
	SyncComponentVolumeRels(tx *gorm.DB, app *dbmodel.Application, components []*apimodel.Component) error
	SyncComponentVolumes(tx *gorm.DB, components []*apimodel.Component) error
	SyncComponentConfigFiles(tx *gorm.DB, components []*apimodel.Component) error
	SyncComponentProbes(tx *gorm.DB, components []*apimodel.Component) error
	SyncComponentLabels(tx *gorm.DB, components []*apimodel.Component) error
	SyncComponentPlugins(tx *gorm.DB, app *dbmodel.Application, components []*apimodel.Component) error
	SyncComponentScaleRules(tx *gorm.DB, components []*apimodel.Component) error
	SyncComponentEndpoints(tx *gorm.DB, components []*apimodel.Component) error
	SyncComponentK8sAttributes(tx *gorm.DB, app *dbmodel.Application, components []*apimodel.Component) error

	Log(w http.ResponseWriter, r *http.Request, component *dbmodel.TenantServices, podName, containerName string, follow bool) error
}
