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
	"github.com/jinzhu/gorm"

	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/builder/exector"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/worker/discover/model"
	"github.com/goodrain/rainbond/worker/server/pb"
)

//ServiceHandler service handler
type ServiceHandler interface {
	ServiceBuild(tenantID, serviceID string, r *api_model.BuildServiceStruct) error
	AddLabel(l *api_model.LabelsStruct, serviceID string) error
	DeleteLabel(l *api_model.LabelsStruct, serviceID string) error
	UpdateLabel(l *api_model.LabelsStruct, serviceID string) error
	StartStopService(s *api_model.StartStopStruct) error
	ServiceVertical(ctx context.Context, v *model.VerticalScalingTaskBody) error
	ServiceHorizontal(h *model.HorizontalScalingTaskBody) error
	ServiceUpgrade(r *model.RollingUpgradeTaskBody) error
	ServiceCreate(ts *api_model.ServiceStruct) error
	ServiceUpdate(sc map[string]interface{}) error
	LanguageSet(langS *api_model.LanguageSet) error
	GetService(tenantID string) ([]*dbmodel.TenantServices, error)
	GetServicesByAppID(appID string, page, pageSize int) (*api_model.ListServiceResponse, error)
	GetPagedTenantRes(offset, len int) ([]*api_model.TenantResource, int, error)
	GetTenantRes(uuid string) (*api_model.TenantResource, error)
	CodeCheck(c *api_model.CheckCodeStruct) error
	ServiceDepend(action string, ds *api_model.DependService) error
	EnvAttr(action string, at *dbmodel.TenantServiceEnvVar) error
	PortVar(action string, tenantID, serviceID string, vp *api_model.ServicePorts, oldPort int) error
	CreatePorts(tenantID, serviceID string, vps *api_model.ServicePorts) error
	PortOuter(tenantName, serviceID string, containerPort int, servicePort *api_model.ServicePortInnerOrOuter) (*dbmodel.TenantServiceLBMappingPort, string, error)
	PortInner(tenantName, serviceID, operation string, port int) error
	VolumnVar(avs *dbmodel.TenantServiceVolume, tenantID, fileContent, action string) *util.APIHandleError
	UpdVolume(sid string, req *api_model.UpdVolumeReq) error
	VolumeDependency(tsr *dbmodel.TenantServiceMountRelation, action string) *util.APIHandleError
	GetDepVolumes(serviceID string) ([]*dbmodel.TenantServiceMountRelation, *util.APIHandleError)
	GetVolumes(serviceID string) ([]*api_model.VolumeWithStatusStruct, *util.APIHandleError)
	ServiceProbe(tsp *dbmodel.TenantServiceProbe, action string) error
	RollBack(rs *api_model.RollbackStruct) error
	GetStatus(serviceID string) (*api_model.StatusList, error)
	GetServicesStatus(tenantID string, services []string) []map[string]interface{}
	GetEnterpriseRunningServices(enterpriseID string) ([]string, *util.APIHandleError)
	CreateTenant(*dbmodel.Tenants) error
	CreateTenandIDAndName(eid string) (string, string, error)
	GetPods(serviceID string) (*K8sPodInfos, error)
	GetMultiServicePods(serviceIDs []string) (*K8sPodInfos, error)
	GetComponentPodNums(ctx context.Context, componentIDs []string) (map[string]int32, error)
	TransServieToDelete(tenantID, serviceID string) error
	TenantServiceDeletePluginRelation(tenantID, serviceID, pluginID string) *util.APIHandleError
	GetTenantServicePluginRelation(serviceID string) ([]*dbmodel.TenantServicePluginRelation, *util.APIHandleError)
	SetTenantServicePluginRelation(tenantID, serviceID string, pss *api_model.PluginSetStruct) (*dbmodel.TenantServicePluginRelation, *util.APIHandleError)
	UpdateTenantServicePluginRelation(serviceID string, pss *api_model.PluginSetStruct) (*dbmodel.TenantServicePluginRelation, *util.APIHandleError)
	UpdateVersionEnv(uve *api_model.SetVersionEnv) *util.APIHandleError
	DeletePluginConfig(serviceID, pluginID string) *util.APIHandleError
	ServiceCheck(*api_model.ServiceCheckStruct) (string, string, *util.APIHandleError)
	GetServiceCheckInfo(uuid string) (*exector.ServiceCheckResult, *util.APIHandleError)
	GetServiceDeployInfo(tenantID, serviceID string) (*pb.DeployInfo, *util.APIHandleError)
	ListVersionInfo(serviceID string) (*api_model.BuildListRespVO, error)

	AddAutoscalerRule(req *api_model.AutoscalerRuleReq) error
	UpdAutoscalerRule(req *api_model.AutoscalerRuleReq) error
	ListScalingRecords(serviceID string, page, pageSize int) ([]*dbmodel.TenantServiceScalingRecords, int, error)

	UpdateServiceMonitor(tenantID, serviceID, name string, update api_model.UpdateServiceMonitorRequestStruct) (*dbmodel.TenantServiceMonitor, error)
	DeleteServiceMonitor(tenantID, serviceID, name string) (*dbmodel.TenantServiceMonitor, error)
	AddServiceMonitor(tenantID, serviceID string, add api_model.AddServiceMonitorRequestStruct) (*dbmodel.TenantServiceMonitor, error)

	SyncComponentBase(tx *gorm.DB, app *dbmodel.Application, components []*api_model.Component) error
	SyncComponentMonitors(tx *gorm.DB,app *dbmodel.Application, components []*api_model.Component) error
	SyncComponentPorts(tx *gorm.DB, app *dbmodel.Application, components []*api_model.Component) error
	SyncComponentRelations(tx *gorm.DB, app *dbmodel.Application, components []*api_model.Component) error
	SyncComponentEnvs(tx *gorm.DB, app *dbmodel.Application, components []*api_model.Component) error
	SyncComponentVolumeRels(tx *gorm.DB, app *dbmodel.Application, components []*api_model.Component) error
	SyncComponentVolumes(tx *gorm.DB,  components []*api_model.Component) error
	SyncComponentConfigFiles(tx *gorm.DB,  components []*api_model.Component) error
	SyncComponentProbes(tx *gorm.DB,  components []*api_model.Component) error
	SyncComponentLabels(tx *gorm.DB,  components []*api_model.Component) error
	SyncComponentPlugins(tx *gorm.DB, app *dbmodel.Application, components []*api_model.Component) error
	SyncComponentScaleRules(tx *gorm.DB,  components []*api_model.Component) error
}
