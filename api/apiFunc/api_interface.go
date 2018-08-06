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

package apiFunc

import (
	"net/http"
)

//TenantInterface interface
type TenantInterface interface {
	TenantInterfaceWithV1
	AllTenantResources(w http.ResponseWriter, r *http.Request)
	TenantResources(w http.ResponseWriter, r *http.Request)
	ServiceResources(w http.ResponseWriter, r *http.Request)
	Tenant(w http.ResponseWriter, r *http.Request)
	ServicesInfo(w http.ResponseWriter, r *http.Request)
	TenantsWithResource(w http.ResponseWriter, r *http.Request)
	TenantsQuery(w http.ResponseWriter, r *http.Request)
	TenantsGetByName(w http.ResponseWriter, r *http.Request)
	SumTenants(w http.ResponseWriter, r *http.Request)
	SingleTenantResources(w http.ResponseWriter, r *http.Request)
	GetSupportProtocols(w http.ResponseWriter, r *http.Request)
	TransPlugins(w http.ResponseWriter, r *http.Request)
	ServicesCount(w http.ResponseWriter, r *http.Request)
	GetManyDeployVersion(w http.ResponseWriter, r *http.Request)
}

//ServiceInterface ServiceInterface
type ServiceInterface interface {
	SetLanguage(w http.ResponseWriter, r *http.Request)
	SingleServiceInfo(w http.ResponseWriter, r *http.Request)
	CheckCode(w http.ResponseWriter, r *http.Request)
	ShareCloud(w http.ResponseWriter, r *http.Request)
	Event(w http.ResponseWriter, r *http.Request)
	BuildList(w http.ResponseWriter, r *http.Request)
	CreateService(w http.ResponseWriter, r *http.Request)
	UpdateService(w http.ResponseWriter, r *http.Request)
	Dependency(w http.ResponseWriter, r *http.Request)
	Env(w http.ResponseWriter, r *http.Request)
	Ports(w http.ResponseWriter, r *http.Request)
	PutPorts(w http.ResponseWriter, r *http.Request)
	PortOuterController(w http.ResponseWriter, r *http.Request)
	PortInnerController(w http.ResponseWriter, r *http.Request)
	ChangeLBPort(w http.ResponseWriter, r *http.Request)
	RollBack(w http.ResponseWriter, r *http.Request)
	AddVolume(w http.ResponseWriter, r *http.Request)
	DeleteVolume(w http.ResponseWriter, r *http.Request)
	Pods(w http.ResponseWriter, r *http.Request)
	VolumeDependency(w http.ResponseWriter, r *http.Request)
	Probe(w http.ResponseWriter, r *http.Request)
	ServiceLabel(w http.ResponseWriter, r *http.Request)
	NodeLabel(w http.ResponseWriter, r *http.Request)
	Share(w http.ResponseWriter, r *http.Request)
	ShareResult(w http.ResponseWriter, r *http.Request)
	BuildVersionInfo(w http.ResponseWriter, r *http.Request)
	GetDeployVersion(w http.ResponseWriter, r *http.Request)
}

//EntranceInterface EntranceInterface
type EntranceInterface interface {
	DomainList(w http.ResponseWriter, r *http.Request)
	Domain(w http.ResponseWriter, r *http.Request)
	AllCerts(w http.ResponseWriter, r *http.Request)
	Cert(w http.ResponseWriter, r *http.Request)
}

//TenantInterfaceWithV1 funcs for both v2 and v1
type TenantInterfaceWithV1 interface {
	StartService(w http.ResponseWriter, r *http.Request)
	StopService(w http.ResponseWriter, r *http.Request)
	RestartService(w http.ResponseWriter, r *http.Request)
	VerticalService(w http.ResponseWriter, r *http.Request)
	HorizontalService(w http.ResponseWriter, r *http.Request)
	BuildService(w http.ResponseWriter, r *http.Request)
	DeployService(w http.ResponseWriter, r *http.Request)
	UpgradeService(w http.ResponseWriter, r *http.Request)
	StatusService(w http.ResponseWriter, r *http.Request)
	StatusServiceList(w http.ResponseWriter, r *http.Request)
	StatusContainerID(w http.ResponseWriter, r *http.Request)
}

//LogInterface log interface
type LogInterface interface {
	Logs(w http.ResponseWriter, r *http.Request)
	LogList(w http.ResponseWriter, r *http.Request)
	LogFile(w http.ResponseWriter, r *http.Request)
	LogSocket(w http.ResponseWriter, r *http.Request)
	LogByAction(w http.ResponseWriter, r *http.Request)
	TenantLogByAction(w http.ResponseWriter, r *http.Request)
}

//PluginInterface plugin interface
type PluginInterface interface {
	PluginAction(w http.ResponseWriter, r *http.Request)
	PluginDefaultENV(w http.ResponseWriter, r *http.Request)
	PluginBuild(w http.ResponseWriter, r *http.Request)
	GetAllPluginBuildVersons(w http.ResponseWriter, r *http.Request)
	GetPluginBuildVersion(w http.ResponseWriter, r *http.Request)
	DeletePluginBuildVersion(w http.ResponseWriter, r *http.Request)
	//plugin
	PluginSet(w http.ResponseWriter, r *http.Request)
	DeletePluginRelation(w http.ResponseWriter, r *http.Request)
	GePluginEnvWhichCanBeSet(w http.ResponseWriter, r *http.Request)
	UpdateVersionEnv(w http.ResponseWriter, r *http.Request)
	GetPluginDefaultEnvs(w http.ResponseWriter, r *http.Request)
	SharePlugin(w http.ResponseWriter, r *http.Request)
	SharePluginResult(w http.ResponseWriter, r *http.Request)
}

//RulesInterface RulesInterface
type RulesInterface interface {
	SetDownStreamRule(w http.ResponseWriter, r *http.Request)
	GetDownStreamRule(w http.ResponseWriter, r *http.Request)
	DeleteDownStreamRule(w http.ResponseWriter, r *http.Request)
	UpdateDownStreamRule(w http.ResponseWriter, r *http.Request)
}

//SourcesInterface SourcesInterface
type SourcesInterface interface {
	SetDefineSource(w http.ResponseWriter, r *http.Request)
	DeleteDefineSource(w http.ResponseWriter, r *http.Request)
	GetDefineSource(w http.ResponseWriter, r *http.Request)
	UpdateDefineSource(w http.ResponseWriter, r *http.Request)
}

type AppInterface interface {
	ExportApp(w http.ResponseWriter, r *http.Request)
	Download(w http.ResponseWriter, r *http.Request)
	Upload(w http.ResponseWriter, r *http.Request)
	ImportID(w http.ResponseWriter, r *http.Request)
	ImportApp(w http.ResponseWriter, r *http.Request)
}
