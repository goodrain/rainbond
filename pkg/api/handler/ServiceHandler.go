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

package handler

import (
	"github.com/goodrain/rainbond/cmd/api/option"
	api_model "github.com/goodrain/rainbond/pkg/api/model"
	"github.com/goodrain/rainbond/pkg/api/util"
	dbmodel "github.com/goodrain/rainbond/pkg/db/model"
	"github.com/goodrain/rainbond/pkg/worker/discover/model"
)

//ServiceHandler service handler
type ServiceHandler interface {
	ServiceBuild(tenantID, serviceID string, r *api_model.BuildServiceStruct) error
	AddLabel(kind, serviceID string, valueList []string) error
	DeleteLabel(kind, serviceID string, amp []string) error
	UpdateServiceLabel(serviceID, value string) error
	StartStopService(s *api_model.StartStopStruct) error
	ServiceVertical(v *model.VerticalScalingTaskBody) error
	ServiceHorizontal(h *model.HorizontalScalingTaskBody) error
	ServiceUpgrade(r *model.RollingUpgradeTaskBody) error
	ServiceCreate(ts *api_model.ServiceStruct) error
	ServiceUpdate(sc map[string]interface{}) error
	LanguageSet(langS *api_model.LanguageSet) error
	GetService(tenantID string) ([]*dbmodel.TenantServices, error)
	CodeCheck(c *api_model.CheckCodeStruct) error
	ShareCloud(c *api_model.CloudShareStruct) error
	ServiceDepend(action string, ds *api_model.DependService) error
	EnvAttr(action string, at *dbmodel.TenantServiceEnvVar) error
	PortVar(action string, tenantID, serviceID string, vp *api_model.ServicePorts) error
	PortOuter(tenantName, serviceID, operation string, port int) (*dbmodel.TenantServiceLBMappingPort, string, error)
	PortInner(tenantName, serviceID, operation string, port int) error
	VolumnVar(tsv *dbmodel.TenantServiceVolume, tenantID, action string) *util.APIHandleError
	VolumeDependency(tsr *dbmodel.TenantServiceMountRelation, action string) *util.APIHandleError
	GetDepVolumes(serviceID string) ([]*dbmodel.TenantServiceMountRelation, *util.APIHandleError)
	GetVolumes(serviceID string) ([]*dbmodel.TenantServiceVolume, *util.APIHandleError)
	ServiceProbe(tsp *dbmodel.ServiceProbe, action string) error
	RollBack(rs *api_model.RollbackStruct) error
	GetStatus(serviceID string) (*api_model.StatusList, error)
	GetServicesStatus(tenantID string, services []string) ([]*dbmodel.TenantServiceStatus, error)
	CreateTenant(*dbmodel.Tenants) error
	CreateTenandIDAndName(eid string) (string, string, error)
	GetPods(serviceID string) ([]*dbmodel.K8sPod, error)
	TransServieToDelete(serviceID string) error
	TenantServiceDeletePluginRelation(serviceID, pluginID string) *util.APIHandleError
	GetTenantServicePluginRelation(serviceID string) ([]*dbmodel.TenantServicePluginRelation, *util.APIHandleError)
	SetTenantServicePluginRelation(serviceID string, pss *api_model.PluginSetStruct) *util.APIHandleError
	UpdateTenantServicePluginRelation(serviceID string, pss *api_model.PluginSetStruct) *util.APIHandleError
	SetVersionEnv(serviecID, pluginID string, sve *api_model.SetVersionEnv) *util.APIHandleError
	UpdateVersionEnv(serviceID string, uve *api_model.UpdateVersionEnv) *util.APIHandleError
}

var defaultServieHandler ServiceHandler

//CreateServiceManger create service manager
func CreateServiceManger(conf option.Config) error {
	var err error
	defaultServieHandler, err = CreateManager(conf)
	if err != nil {
		return err
	}
	return nil
}

//GetServiceManager get manager
func GetServiceManager() ServiceHandler {
	return defaultServieHandler
}
