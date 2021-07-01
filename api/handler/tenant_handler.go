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

	"github.com/goodrain/rainbond/api/model"
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	dbmodel "github.com/goodrain/rainbond/db/model"
)

//TenantHandler tenant handler
type TenantHandler interface {
	GetTenants(query string) ([]*dbmodel.Tenants, error)
	GetTenantsByName(name string) (*dbmodel.Tenants, error)
	GetTenantsByEid(eid, query string) ([]*dbmodel.Tenants, error)
	GetTenantsByUUID(uuid string) (*dbmodel.Tenants, error)
	GetTenantsName() ([]string, error)
	StatsMemCPU(services []*dbmodel.TenantServices) (*api_model.StatsInfo, error)
	TotalMemCPU(services []*dbmodel.TenantServices) (*api_model.StatsInfo, error)
	GetTenantsResources(ctx context.Context, tr *api_model.TenantResources) (map[string]map[string]interface{}, error)
	GetTenantResource(tenantID string) (TenantResourceStats, error)
	GetAllocatableResources(ctx context.Context) (*ClusterResourceStats, error)
	GetServicesResources(tr *api_model.ServicesResources) (map[string]map[string]interface{}, error)
	TenantsSum() (int, error)
	GetProtocols() ([]*dbmodel.RegionProcotols, *util.APIHandleError)
	TransPlugins(tenantID, tenantName, fromTenant string, pluginList []string) *util.APIHandleError
	GetServicesStatus(ids string) map[string]string
	IsClosedStatus(status string) bool
	BindTenantsResource(source []*dbmodel.Tenants) api_model.TenantList
	UpdateTenant(*dbmodel.Tenants) error
	DeleteTenant(ctx context.Context, tenantID string) error
	GetClusterResource(ctx context.Context) *ClusterResourceStats
	CheckResourceName(ctx context.Context, namespace string, req *model.CheckResourceNameReq) (*model.CheckResourceNameResp, error)
}
