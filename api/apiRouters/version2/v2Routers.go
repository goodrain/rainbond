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

package version2

import (
	"github.com/goodrain/rainbond/api/controller"
	"github.com/goodrain/rainbond/api/middleware"

	"github.com/go-chi/chi"
)

//V2 v2
type V2 struct{}

//Routes routes
func (v2 *V2) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/show", controller.GetManager().Show)
	r.Post("/show", controller.GetManager().Show)
	r.Mount("/tenants", v2.tenantRouter())
	r.Mount("/nodes", v2.nodesRouter())
	r.Mount("/notificationEvent", v2.notificationEventRouter())
	r.Mount("/cluster", v2.clusterRouter())
	r.Mount("/resources", v2.resourcesRouter())
	r.Mount("/prometheus", v2.prometheusRouter())
	r.Get("/event", controller.GetManager().Event)
	r.Mount("/app", v2.appRouter())
	r.Get("/health", controller.GetManager().Health)
	r.Post("/alertmanager-webhook", controller.GetManager().AlertManagerWebHook)
	return r
}

func (v2 *V2) tenantRouter() chi.Router {
	r := chi.NewRouter()
	r.Post("/", controller.GetManager().Tenant)
	r.Mount("/{tenant_name}", v2.tenantNameRouter())
	r.Get("/", controller.GetManager().Tenant)
	r.Get("/services-count", controller.GetManager().ServicesCount)
	return r
}

func (v2 *V2) tenantNameRouter() chi.Router {
	r := chi.NewRouter()
	//初始化租户和服务信
	r.Use(middleware.InitTenant)
	r.Put("/", controller.GetManager().Tenant)
	r.Get("/", controller.GetManager().Tenant)
	r.Delete("/", controller.GetManager().Tenant)
	//租户中的日志
	r.Post("/event-log", controller.GetManager().TenantLogByAction)
	r.Get("/protocols", controller.GetManager().GetSupportProtocols)
	//插件预安装
	r.Post("/transplugins", controller.GetManager().TransPlugins)
	//代码检测
	r.Post("/code-check", controller.GetManager().CheckCode)
	r.Post("/servicecheck", controller.Check)
	r.Get("/servicecheck/{uuid}", controller.GetServiceCheckInfo)
	r.Post("/cloud-share", controller.GetManager().ShareCloud)
	r.Get("/resources", controller.GetManager().SingleTenantResources)
	r.Get("/certificates", controller.GetManager().Entrance)
	r.Post("/certificates", controller.GetManager().Entrance)
	r.Delete("/certificates/{certificatesName}", controller.GetManager().Entrance)
	r.Get("/certificates/{certificatesName}", controller.GetManager().Entrance)
	r.Get("/services", controller.GetManager().ServicesInfo)
	//创建应用
	r.Post("/services", controller.GetManager().CreateService)
	r.Post("/plugin", controller.GetManager().PluginAction)
	r.Post("/plugins/{plugin_id}/share", controller.GetManager().SharePlugin)
	r.Get("/plugins/{plugin_id}/share/{share_id}", controller.GetManager().SharePluginResult)
	r.Get("/plugin", controller.GetManager().PluginAction)
	r.Post("/services_status", controller.GetManager().StatusServiceList)
	r.Mount("/services/{service_alias}", v2.serviceRouter())
	r.Mount("/plugin/{plugin_id}", v2.pluginRouter())
	r.Mount("/sources", v2.defineSourcesRouter())
	r.Get("/event", controller.GetManager().Event)
	r.Get("/chargesverify", controller.ChargesVerifyController)
	//get some service pod info
	r.Get("/pods", controller.Pods)
	//app backup
	r.Get("/groupapp/backups", controller.Backups)
	r.Post("/groupapp/backups", controller.NewBackups)
	r.Post("/groupapp/backupcopy", controller.BackupCopy)
	r.Get("/groupapp/backups/{backup_id}", controller.GetBackup)
	r.Delete("/groupapp/backups/{backup_id}", controller.DeleteBackup)
	r.Post("/groupapp/backups/{backup_id}/restore", controller.Restore)
	r.Get("/groupapp/backups/{backup_id}/restore/{restore_id}", controller.RestoreResult)
	r.Post("/deployversions", controller.GetManager().GetManyDeployVersion)
	return r
}

func (v2 *V2) serviceRouter() chi.Router {
	r := chi.NewRouter()
	//初始化应用信息
	r.Use(middleware.InitService)
	//应用部署(act)
	//r.Post("/deploy", controller.GetManager().DeployService)
	r.Put("/", controller.GetManager().UpdateService)
	//应用构建(act)
	r.Post("/build", controller.GetManager().BuildService)
	//应用起停
	r.Post("/start", controller.GetManager().StartService)
	r.Post("/stop", controller.GetManager().StopService)
	r.Post("/restart", controller.GetManager().RestartService)
	//应用伸缩
	r.Put("/vertical", controller.GetManager().VerticalService)
	r.Put("/horizontal", controller.GetManager().HorizontalService)
	//设置应用语言(act)
	r.Post("/language", controller.GetManager().SetLanguage)
	//应用信息获取修改与删除(source)
	r.Get("/", controller.GetManager().SingleServiceInfo)
	r.Delete("/", controller.GetManager().SingleServiceInfo)
	//应用升级(act)
	r.Post("/upgrade", controller.GetManager().UpgradeService)
	//应用状态获取(act)
	r.Get("/status", controller.GetManager().StatusService)
	//构建版本列表
	r.Get("/build-list", controller.GetManager().BuildList)
	//构建版本操作
	r.Get("/build-version/{build_version}", controller.GetManager().BuildVersionInfo)
	r.Get("/deployversion", controller.GetManager().GetDeployVersion)
	r.Delete("/build-version/{build_version}", controller.GetManager().BuildVersionInfo)
	//应用分享
	r.Post("/share", controller.GetManager().Share)
	r.Get("/share/{share_id}", controller.GetManager().ShareResult)
	//应用日志相关
	r.Post("/log", controller.GetManager().Logs)
	r.Get("/log-file", controller.GetManager().LogList)
	//r.Get("/log-file/{fileName}", controller.GetManager().LogFile)
	r.Get("/log-instance", controller.GetManager().LogSocket)
	r.Post("/event-log", controller.GetManager().LogByAction)

	//应用依赖关系增加与删除(source)
	r.Post("/dependency", controller.GetManager().Dependency)
	r.Delete("/dependency", controller.GetManager().Dependency)
	//环境变量增删改(source)
	r.Post("/env", controller.GetManager().Env)
	r.Put("/env", controller.GetManager().Env)
	r.Delete("/env", controller.GetManager().Env)
	//端口变量增删改(source)
	r.Post("/ports", controller.GetManager().Ports)
	r.Put("/ports", controller.GetManager().PutPorts)
	r.Put("/ports/{port}", controller.GetManager().Ports)
	r.Delete("/ports/{port}", controller.GetManager().Ports)
	r.Put("/ports/{port}/outer", controller.GetManager().PortOuterController)
	r.Put("/ports/{port}/inner", controller.GetManager().PortInnerController)
	r.Put("/ports/{port}/changelbport", controller.GetManager().ChangeLBPort)

	//应用版本回滚(act)
	r.Post("/rollback", controller.GetManager().RollBack)

	//持久化信息API v2.1 支持多种持久化格式
	r.Post("/volumes", controller.AddVolume)
	r.Get("/volumes", controller.GetVolume)
	r.Delete("/volumes/{volume_name}", controller.DeleteVolume)
	r.Post("/depvolumes", controller.AddVolumeDependency)
	r.Delete("/depvolumes", controller.DeleteVolumeDependency)
	r.Get("/depvolumes", controller.GetDepVolume)
	//持久化信息API v2
	r.Post("/volume-dependency", controller.GetManager().VolumeDependency)
	r.Delete("/volume-dependency", controller.GetManager().VolumeDependency)
	r.Post("/volume", controller.GetManager().AddVolume)
	r.Delete("/volume", controller.GetManager().DeleteVolume)

	//获取应用实例情况(source)
	r.Get("/pods", controller.GetManager().Pods)

	//应用探针 增 删 改(surce)
	r.Post("/probe", controller.GetManager().Probe)
	r.Put("/probe", controller.GetManager().Probe)
	r.Delete("/probe", controller.GetManager().Probe)

	//应用标签 增 删 (source)
	r.Post("/service-label", controller.GetManager().ServiceLabel)
	r.Put("/service-label", controller.GetManager().ServiceLabel)
	//节点标签 增 删
	r.Post("/node-label", controller.GetManager().NodeLabel)
	r.Delete("/node-label", controller.GetManager().NodeLabel)

	//获取租户所有域名
	r.Get("/get-domains", controller.GetManager().Entrance)

	//租户域名 增加 删除(sources)
	r.Post("/domains", controller.GetManager().Entrance)
	r.Delete("/domains/{domain_name}", controller.GetManager().Entrance)

	//插件
	r.Mount("/plugin", v2.serviceRelatePluginRouter())

	//rule
	r.Mount("/net-rule", v2.rulesRouter())
	return r
}

func (v2 *V2) clusterRouter() chi.Router {
	r := chi.NewRouter()
	return r
}

func (v2 *V2) resourcesRouter() chi.Router {
	r := chi.NewRouter()
	r.Post("/tenants", controller.GetManager().TenantResources)
	r.Post("/services", controller.GetManager().ServiceResources)
	r.Get("/tenants/sum", controller.GetManager().SumTenants)
	//tenants's resource
	r.Get("/tenants/res", controller.GetManager().TenantsWithResource)
	r.Get("/tenants/res/page/{curPage}/size/{pageLen}", controller.GetManager().TenantsWithResource)
	r.Get("/tenants/query/{tenant_name}", controller.GetManager().TenantsQuery)
	r.Get("/tenants/{tenant_name}/res", controller.GetManager().TenantsGetByName)
	return r
}

func (v2 *V2) prometheusRouter() chi.Router {
	r := chi.NewRouter()
	return r
}

func (v2 *V2) appRouter() chi.Router {
	r := chi.NewRouter()
	r.Post("/export", controller.GetManager().ExportApp)
	r.Get("/export/{eventId}", controller.GetManager().ExportApp)

	r.Get("/download/{format}/{fileName}", controller.GetManager().Download)
	r.Post("/upload", controller.GetManager().Upload)

	r.Post("/import/ids/{eventId}", controller.GetManager().ImportID)
	r.Get("/import/ids/{eventId}", controller.GetManager().ImportID)
	r.Delete("/import/ids/{eventId}", controller.GetManager().ImportID)

	r.Post("/import", controller.GetManager().ImportApp)
	r.Get("/import/{eventId}", controller.GetManager().ImportApp)
	return r
}

func (v2 *V2) notificationEventRouter() chi.Router {
	r := chi.NewRouter()
	r.Get("/", controller.GetNotificationEvents)
	r.Put("/{serviceAlias}", controller.HandleNotificationEvent)
	r.Get("/{hash}", controller.GetNotificationEvent)
	return r
}
