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

package controller

import (
	"net/http"

	"github.com/goodrain/rainbond/pkg/api/handler"
	"github.com/goodrain/rainbond/pkg/api/middleware"

	"github.com/go-chi/chi"

	api_model "github.com/goodrain/rainbond/pkg/api/model"
	httputil "github.com/goodrain/rainbond/pkg/util/http"
)

//PluginAction plugin action
func (t *TenantStruct) PluginAction(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "PUT":
		t.UpdatePlugin(w, r)
	case "DELETE":
		t.DeletePlugin(w, r)
	case "POST":
		t.CreatePlugin(w, r)
	case "GET":
		t.GetPlugins(w, r)
	}
}

//CreatePlugin add plugin
func (t *TenantStruct) CreatePlugin(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v2/tenants/{tenant_name}/plugin v2 createPlugin
	//
	// 创建插件
	//
	// create plugin
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	tenantName := r.Context().Value(middleware.ContextKey("tenant_name")).(string)
	var cps api_model.CreatePluginStruct
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &cps.Body, nil); !ok {
		return
	}
	cps.Body.TenantID = tenantID
	cps.TenantName = tenantName
	if err := handler.GetPluginManager().CreatePluginAct(&cps); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//UpdatePlugin UpdatePlugin
func (t *TenantStruct) UpdatePlugin(w http.ResponseWriter, r *http.Request) {
	// swagger:operation PUT /v2/tenants/{tenant_name}/plugin/{plugin_id} v2 updatePlugin
	//
	// 插件更新 全量更新，但pluginID和所在租户不提供修改
	//
	// update plugin
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式

	pluginID := r.Context().Value(middleware.ContextKey("plugin_id")).(string)
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	var ups api_model.UpdatePluginStruct
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &ups.Body, nil); !ok {
		return
	}
	if err := handler.GetPluginManager().UpdatePluginAct(pluginID,tenantID, &ups); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//DeletePlugin DeletePlugin
func (t *TenantStruct) DeletePlugin(w http.ResponseWriter, r *http.Request) {
	// swagger:operation DELETE /v2/tenants/{tenant_name}/plugin/{plugin_id} v2 deletePlugin
	//
	// 插件删除
	//
	// delete plugin
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式
	pluginID := r.Context().Value(middleware.ContextKey("plugin_id")).(string)
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	if err := handler.GetPluginManager().DeletePluginAct(pluginID, tenantID); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//GetPlugins GetPlugins
func (t *TenantStruct) GetPlugins(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /v2/tenants/{tenant_name}/plugin v2 getPlugins
	//
	// 获取当前租户下所有的可用插件
	//
	// get plugins
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	plugins, err := handler.GetPluginManager().GetPlugins(tenantID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, plugins)
}

//PluginDefaultENV PluginDefaultENV
func (t *TenantStruct) PluginDefaultENV(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		t.AddDefatultENV(w, r)
	case "DELETE":
		t.DeleteDefaultENV(w, r)
	case "PUT":
		t.UpdateDefaultENV(w, r)
	}
}

//AddDefatultENV AddDefatultENV
func (t *TenantStruct) AddDefatultENV(w http.ResponseWriter, r *http.Request) {
	pluginID := r.Context().Value(middleware.ContextKey("plugin_id")).(string)
	versionID := chi.URLParam(r, "version_id")
	var est api_model.ENVStruct
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &est.Body, nil); !ok {
		return
	}
	est.VersionID = versionID
	est.PluginID = pluginID
	if err := handler.GetPluginManager().AddDefaultEnv(&est); err != nil {
		err.Handle(r, w)
		return
	}
}

//DeleteDefaultENV DeleteDefaultENV
func (t *TenantStruct) DeleteDefaultENV(w http.ResponseWriter, r *http.Request) {
	pluginID := r.Context().Value(middleware.ContextKey("plugin_id")).(string)
	envName := chi.URLParam(r, "env_name")
	versionID := chi.URLParam(r, "version_id")
	if err := handler.GetPluginManager().DeleteDefaultEnv(pluginID, versionID, envName); err != nil {
		err.Handle(r, w)
		return
	}
}

//UpdateDefaultENV UpdateDefaultENV
func (t *TenantStruct) UpdateDefaultENV(w http.ResponseWriter, r *http.Request) {

	pluginID := r.Context().Value(middleware.ContextKey("plugin_id")).(string)
	versionID := chi.URLParam(r, "version_id")
	var est api_model.ENVStruct
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &est.Body, nil); !ok {
		return
	}
	est.PluginID = pluginID
	est.VersionID = versionID
	if err := handler.GetPluginManager().UpdateDefaultEnv(&est); err != nil {
		err.Handle(r, w)
		return
	}
}

//GetPluginDefaultEnvs GetPluginDefaultEnvs
func (t *TenantStruct) GetPluginDefaultEnvs(w http.ResponseWriter, r *http.Request) {
	pluginID := r.Context().Value(middleware.ContextKey("plugin_id")).(string)
	versionID := chi.URLParam(r, "version_id")
	envs, err := handler.GetPluginManager().GetDefaultEnv(pluginID, versionID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, envs)
}

//PluginBuild PluginBuild
// swagger:operation POST /v2/tenants/{tenant_name}/plugin/{plugin_id}/build v2 buildPlugin
//
// 构建plugin
//
// build plugin
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) PluginBuild(w http.ResponseWriter, r *http.Request) {
	var build api_model.BuildPluginStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &build.Body, nil)
	if !ok {
		return
	}
	tenantName := r.Context().Value(middleware.ContextKey("tenant_name")).(string)
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	pluginID := r.Context().Value(middleware.ContextKey("plugin_id")).(string)
	build.TenantName = tenantName
	build.PluginID = pluginID
	build.Body.TenantID = tenantID
	pbv, err := handler.GetPluginManager().BuildPluginManual(&build)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, pbv)
}

//GetAllPluginBuildVersons 获取该插件所有的构建版本
// swagger:operation GET /v2/tenants/{tenant_name}/plugin/{plugin_id}/build-version v2 allPluginVersions
//
// 获取所有的构建版本信息
//
// all plugin versions
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) GetAllPluginBuildVersons(w http.ResponseWriter, r *http.Request) {
	pluginID := r.Context().Value(middleware.ContextKey("plugin_id")).(string)
	versions, err := handler.GetPluginManager().GetAllPluginBuildVersions(pluginID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, versions)
}

//GetPluginBuildVersion 获取某构建版本信息
// swagger:operation GET /v2/tenants/{tenant_name}/plugin/{plugin_id}/build-version/{version_id} v2 pluginVersion
//
// 获取某个构建版本信息
//
// plugin version
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) GetPluginBuildVersion(w http.ResponseWriter, r *http.Request) {
	pluginID := r.Context().Value(middleware.ContextKey("plugin_id")).(string)
	versionID := chi.URLParam(r, "version_id")
	version, err := handler.GetPluginManager().GetPluginBuildVersion(pluginID, versionID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, version)
}

//DeletePluginBuildVersion DeletePluginBuildVersion
// swagger:operation DELETE /v2/tenants/{tenant_name}/plugin/{plugin_id}/build-version/{version_id} v2 deletePluginVersion
//
// 删除某个构建版本信息
//
// delete plugin version
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) DeletePluginBuildVersion(w http.ResponseWriter, r *http.Request) {
	pluginID := r.Context().Value(middleware.ContextKey("plugin_id")).(string)
	versionID := chi.URLParam(r, "version_id")
	err := handler.GetPluginManager().DeletePluginBuildVersion(pluginID, versionID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//PluginSet PluginSet
func (t *TenantStruct) PluginSet(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "PUT":
		t.updatePluginSet(w, r)
	case "POST":
		t.addPluginSet(w, r)
	case "GET":
		t.getPluginSet(w, r)
	}
}

// swagger:operation PUT /v2/tenants/{tenant_name}/services/{service_alias}/plugin v2 updatePluginSet
//
// 更新插件设定
//
// update plugin setting
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) updatePluginSet(w http.ResponseWriter, r *http.Request) {
	var pss api_model.PluginSetStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &pss.Body, nil)
	if !ok {
		return
	}
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	if err := handler.GetServiceManager().UpdateTenantServicePluginRelation(serviceID, &pss); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

// swagger:operation POST /v2/tenants/{tenant_name}/services/{service_alias}/plugin v2 addPluginSet
//
// 添加插件设定
//
// add plugin setting
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) addPluginSet(w http.ResponseWriter, r *http.Request) {
	var pss api_model.PluginSetStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &pss.Body, nil)
	if !ok {
		return
	}
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	if err := handler.GetServiceManager().SetTenantServicePluginRelation(tenantID, serviceID, &pss); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

// swagger:operation GET /v2/tenants/{tenant_name}/services/{service_alias}/plugin v2 getPluginSet
//
// 获取插件设定
//
// get plugin setting
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) getPluginSet(w http.ResponseWriter, r *http.Request) {
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	gps, err := handler.GetServiceManager().GetTenantServicePluginRelation(serviceID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, gps)

}

//DeletePluginRelation DeletePluginRelation
// swagger:operation DELETE /v2/tenants/{tenant_name}/services/{service_alias}/plugin/{plugin_id} v2 deletePluginRelation
//
// 删除插件依赖
//
// delete plugin relation
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) DeletePluginRelation(w http.ResponseWriter, r *http.Request) {
	pluginID := chi.URLParam(r, "plugin_id")
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	serviceAlias := r.Context().Value(middleware.ContextKey("service_alias")).(string)
	if err := handler.GetServiceManager().TenantServiceDeletePluginRelation(serviceID, pluginID); err != nil {
		err.Handle(r, w)
		return
	}
	if err := handler.GetServiceManager().DeleteComplexEnvs(tenantID, serviceAlias, pluginID); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//GePluginEnvWhichCanBeSet GePluginEnvWhichCanBeSet
// swagger:operation GET /v2/tenants/{tenant_name}/services/{service_alias}/plugin/{plugin_id}/envs v2 getVersionEnvs
//
// 获取可配置的env; 从service plugin对应中取, 若不存在则返回默认可修改的变量
//
// get version env
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) GePluginEnvWhichCanBeSet(w http.ResponseWriter, r *http.Request) {
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	pluginID := chi.URLParam(r, "plugin_id")
	envs, err := handler.GetPluginManager().GetEnvsWhichCanBeSet(serviceID, pluginID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, envs)
}

//SetVersionEnv SetVersionEnv
// swagger:operation POST /v2/tenants/{tenant_name}/services/{service_alias}/plugin/{plugin_id}/setenv v2 setVersionEnv
//
// 设置用户可配的环境变量
//
// set version env
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) SetVersionEnv(w http.ResponseWriter, r *http.Request) {
	var sve api_model.SetVersionEnv
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &sve.Body, nil)
	if !ok {
		return
	}
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	serviceAlias := r.Context().Value(middleware.ContextKey("service_alias")).(string)
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	pluginID := chi.URLParam(r, "plugin_id")
	sve.PluginID = pluginID
	sve.Body.TenantID = tenantID
	sve.ServiceAlias = serviceAlias
	sve.Body.ServiceID = serviceID
	if err := handler.GetServiceManager().SetVersionEnv(&sve); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//UpdateVersionEnv UpdateVersionEnv
// swagger:operation PUT /v2/tenants/{tenant_name}/services/{service_alias}/plugin/{plugin_id}/upenv v2 updateVersionEnv
//
// 修改用户可配的环境变量
//
// update version env
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) UpdateVersionEnv(w http.ResponseWriter, r *http.Request) {
	var uve api_model.SetVersionEnv
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &uve.Body, nil)
	if !ok {
		return
	}
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	serviceAlias := r.Context().Value(middleware.ContextKey("service_alias")).(string)
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	pluginID := chi.URLParam(r, "plugin_id")
	uve.PluginID = pluginID
	uve.Body.TenantID = tenantID
	uve.ServiceAlias = serviceAlias
	uve.Body.ServiceID = serviceID
	if err := handler.GetServiceManager().UpdateVersionEnv(&uve); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}
