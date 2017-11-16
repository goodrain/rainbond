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

package version2

import (
	"github.com/goodrain/rainbond/pkg/api/controller"
	"github.com/goodrain/rainbond/pkg/api/middleware"

	"github.com/go-chi/chi"
)

//PluginRouter plugin router
func (v2 *V2) pluginRouter() chi.Router {
	r := chi.NewRouter()
	//初始化应用信息
	r.Use(middleware.InitPlugin)
	//plugin uri
	//update/delete plugin
	r.Put("/", controller.GetManager().PluginAction)
	r.Delete("/", controller.GetManager().PluginAction)
	r.Post("/build", controller.GetManager().PluginBuild)
	r.Post("/default-env", controller.GetManager().PluginDefaultENV)
	r.Get("/default-env", controller.GetManager().GetPluginDefaultEnvs)
	r.Put("/default-env/{env_name}", controller.GetManager().PluginDefaultENV)
	r.Delete("/default-env/{env_name}", controller.GetManager().PluginDefaultENV)
	//get this plugin all build version
	r.Get("/build-version", controller.GetManager().GetAllPluginBuildVersons)
	r.Get("/build-version/{version_id}", controller.GetManager().GetPluginBuildVersion)
	r.Delete("/build-version/{version_id}", controller.GetManager().DeletePluginBuildVersion)
	return r
}

func (v2 *V2) serviceRelatePluginRouter() chi.Router {
	r := chi.NewRouter()
	//插件
	//service relate plugin
	// v2/tenant/tenant_name/services/service_alias/plugin/xxx
	r.Post("/", controller.GetManager().PluginSet)
	r.Put("/", controller.GetManager().PluginSet)
	r.Get("/", controller.GetManager().PluginSet)
	r.Delete("/{plugin_id}", controller.GetManager().DeletePluginRelation)
	// plugin enable set env,包含普通设置和预设可修改部分
	r.Post("/{plugin_id}/setenv", controller.GetManager().SetVersionEnv)
	r.Put("/{plugin_id}/setenv/{env_name}", controller.GetManager().UpdateVersionEnv)
	r.Get("/{plugin_id}/envs", controller.GetManager().GePluginEnvWhichCanBeSet)
	return r
}
