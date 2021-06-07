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

package model

import dbmodel "github.com/goodrain/rainbond/db/model"

//CreatePluginStruct CreatePluginStruct
//swagger:parameters createPlugin
type CreatePluginStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: body
	Body struct {
		//插件id
		//in: body
		//required: true
		PluginID string `json:"plugin_id" validate:"plugin_id|required"`
		//in: body
		//required: true
		PluginName string `json:"plugin_name" validate:"plugin_name|required"`
		//插件用途描述
		//in: body
		//required: false
		PluginInfo string `json:"plugin_info" validate:"plugin_info"`
		// 插件docker地址
		// in:body
		// required: false
		ImageURL string `json:"image_url" validate:"image_url"`
		//git地址
		//in: body
		//required: false
		GitURL string `json:"git_url" validate:"git_url"`
		//构建模式
		//in: body
		//required: false
		BuildModel string `json:"build_model" validate:"build_model"`
		//插件模式
		//in: body
		//required: false
		PluginModel string `json:"plugin_model" validate:"plugin_model"`
		//租户id
		//in: body
		//required: false
		TenantID string `json:"tenant_id" validate:"tenant_id"`
	}
}

//UpdatePluginStruct UpdatePluginStruct
//swagger:parameters updatePlugin
type UpdatePluginStruct struct {
	// 租户名称
	// in: path
	// required: true
	TenantName string `json:"tenant_name" validate:"tenant_name|required"`
	// 插件id
	// in: path
	// required: true
	PluginID string `json:"plugin_id" validate:"tenant_name|required"`
	// in: body
	Body struct {
		//插件名称
		//in: body
		//required: false
		PluginName string `json:"plugin_name" validate:"plugin_name"`
		//插件用途描述
		//in: body
		//required: false
		PluginInfo string `json:"plugin_info" validate:"plugin_info"`
		//插件docker地址
		//in: body
		//required: false
		ImageURL string `json:"image_url" validate:"image_url"`
		//git地址
		//in: body
		//required: false
		GitURL string `json:"git_url" validate:"git_url"`
		//构建模式
		//in: body
		//required: false
		BuildModel string `json:"build_model" validate:"build_model"`
		//插件模式
		//in: body
		//required: false
		PluginModel string `json:"plugin_model" validate:"plugin_model"`
	}
}

//DeletePluginStruct deletePluginStruct
//swagger:parameters deletePlugin
type DeletePluginStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name" validate:"tenant_name|required"`
	// in: path
	// required: true
	PluginID string `json:"plugin_id" validate:"plugin_id|required"`
}

//ENVStruct ENVStruct
//swagger:parameters adddefaultenv updatedefaultenv
type ENVStruct struct {
	// 租户名称
	// in: path
	// required: true
	TenantName string `json:"tenant_name" validate:"tenant_name"`
	// 插件id
	// in: path
	// required: true
	PluginID string `json:"plugin_id" validate:"plugin_id"`
	// 构建版本
	// in: path
	// required; true
	VersionID string `json:"version_id" validate:"version_id"`
	//in : body
	Body struct {
		//in: body
		//required: true
		EVNInfo []*PluginDefaultENV
	}
}

//DeleteENVstruct DeleteENVstruct
//swagger:parameters deletedefaultenv
type DeleteENVstruct struct {
	// 租户名称
	// in: path
	// required: true
	TenantName string `json:"tenant_name" validate:"tenant_name|required"`
	// 插件id
	// in: path
	// required: true
	PluginID string `json:"plugin_id" validate:"plugin_id|required"`
	// 构建版本
	// in: path
	// required; true
	VersionID string `json:"version_id" validate:"version_id|required"`
	//配置项名称
	//in: path
	//required: true
	ENVName string `json:"env_name" validate:"env_name|required"`
}

//PluginDefaultENV 插件默认环境变量
type PluginDefaultENV struct {
	//对应插件id
	//in: body
	//required: true
	PluginID string `json:"plugin_id" validate:"plugin_id"`
	//构建版本id
	//in: body
	//required: true
	VersionID string `json:"version_id" validate:"version_id"`
	//配置项名称
	//in: body
	//required: true
	ENVName string `json:"env_name" validate:"env_name"`
	//配置项值
	//in: body
	//required: true
	ENVValue string `json:"env_value" validate:"env_value"`
	//是否可以被使用者修改
	//in :body
	//required: false
	IsChange bool `json:"is_change" validate:"is_change|bool"`
}

//BuildPluginStruct BuildPluginStruct
//swagger:parameters buildPlugin
type BuildPluginStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name" validate:"tenant_name"`
	// in: path
	// required: true
	PluginID string `json:"plugin_id" validate:"plugin_id"`
	//in: body
	Body struct {
		// the event id
		// in: body
		// required: false
		EventID string `json:"event_id" validate:"event_id"`
		// 插件CPU权重, 默认125
		// in: body
		// required: true
		PluginCPU int `json:"plugin_cpu" validate:"plugin_cpu|required"`
		// 插件最大内存, 默认50
		// in: body
		// required: true
		PluginMemory int `json:"plugin_memory" validate:"plugin_memory|required"`
		// 插件cmd, 默认50
		// in: body
		// required: false
		PluginCMD string `json:"plugin_cmd" validate:"plugin_cmd"`
		// 插件的版本号
		// in: body
		// required: true
		BuildVersion string `json:"build_version" validate:"build_version|required"`
		// 插件构建版本号
		// in: body
		// required: true
		DeployVersion string `json:"deploy_version" validate:"deploy_version"`
		// git地址分支信息，默认为master
		// in: body
		// required: false
		RepoURL string `json:"repo_url" validate:"repo_url"`
		// git username
		// in: body
		// required: false
		Username string `json:"username"`
		// git password
		// in: body
		// required: false
		Password string `json:"password"`
		// 版本信息, 协助选择插件版本
		// in:body
		// required: true
		Info string `json:"info" validate:"info"`
		// 操作人
		// in: body
		// required: false
		Operator string `json:"operator" validate:"operator"`
		//租户id
		// in: body
		// required: true
		TenantID string `json:"tenant_id" validate:"tenant_id"`
		// 镜像地址
		// in: body
		// required: false
		BuildImage string `json:"build_image" validate:"build_image"`
		//ImageInfo
		ImageInfo struct {
			HubURL      string `json:"hub_url"`
			HubUser     string `json:"hub_user"`
			HubPassword string `json:"hub_password"`
			Namespace   string `json:"namespace"`
			IsTrust     bool   `json:"is_trust,omitempty"`
		} `json:"ImageInfo" validate:"ImageInfo"`
	}
}

//PluginBuildVersionStruct PluginBuildVersionStruct
//swagger:parameters deletePluginVersion pluginVersion
type PluginBuildVersionStruct struct {
	//in: path
	//required: true
	TenantName string `json:"tenant_name" validate:"tenant_name"`
	//in: path
	//required: true
	PluginID string `json:"plugin_id" validate:"plugin_id"`
	//in: path
	//required: true
	VersionID string `json:"version_id" validate:"version_id"`
}

//AllPluginBuildVersionStruct AllPluginBuildVersionStruct
//swagger:parameters allPluginVersions
type AllPluginBuildVersionStruct struct {
	//in: path
	//required: true
	TenantName string `json:"tenant_name" validate:"tenant_name"`
	//in: path
	//required: true
	PluginID string `json:"plugin_id" validate:"plugin_id"`
}

//PluginSetStruct PluginSetStruct
//swagger:parameters updatePluginSet addPluginSet
type PluginSetStruct struct {
	//in: path
	//required: true
	TenantName string `json:"tenant_name"`
	//in: path
	//required: true
	ServiceAlias string `json:"service_alias"`
	// in: body
	Body struct {
		//plugin id
		//in: body
		//required: true
		PluginID string `json:"plugin_id" validate:"plugin_id"`
		// plugin version
		//in: body
		//required: true
		VersionID string `json:"version_id" validate:"version_id"`
		// plugin is uesd
		//in: body
		//required: false
		Switch bool `json:"switch" validate:"switch|bool"`
		// plugin cpu size default 125
		// in: body
		// required: false
		PluginCPU int `json:"plugin_cpu" validate:"plugin_cpu"`
		// plugin memory size default 64
		// in: body
		// required: false
		PluginMemory int `json:"plugin_memory" validate:"plugin_memory"`
		// app plugin config
		// in: body
		// required: true
		ConfigEnvs ConfigEnvs `json:"config_envs" validate:"config_envs"`
	}
}

//GetPluginsStruct GetPluginsStruct
//swagger:parameters getPlugins
type GetPluginsStruct struct {
	//in: path
	//required: true
	TenantName string `json:"tenant_name"`
}

//GetPluginSetStruct GetPluginSetStruct
//swagger:parameters getPluginSet
type GetPluginSetStruct struct {
	//in: path
	//required: true
	TenantName string `json:"tenant_name"`
	//in: path
	//required: true
	ServiceAlias string `json:"service_alias"`
}

//DeletePluginSetStruct DeletePluginSetStruct
//swagger:parameters deletePluginRelation
type DeletePluginSetStruct struct {
	//in: path
	//required: true
	TenantName string `json:"tenant_name"`
	//in: path
	//required: true
	ServiceAlias string `json:"service_alias"`
	//插件id
	//in: path
	//required: true
	PluginID string `json:"plugin_id"`
}

//GetPluginEnvStruct GetPluginEnvStruct
//swagger:parameters getPluginEnv getPluginDefaultEnv
type GetPluginEnvStruct struct {
	//租户名称
	//in: path
	//required: true
	TenantName string `json:"tenant_name"`
	// 插件id
	// in: path
	// required: true
	PluginID string `json:"plugin_id"`
	// 构建版本id
	// in: path
	// required: true
	VersionID string `json:"version_id"`
}

//GetVersionEnvStruct GetVersionEnvStruct
//swagger:parameters getVersionEnvs
type GetVersionEnvStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	// 插件id
	// in: path
	// required: true
	PluginID string `json:"plugin_id"`
}

//SetVersionEnv SetVersionEnv
//swagger:parameters setVersionEnv updateVersionEnv
type SetVersionEnv struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	// 插件id
	// in: path
	// required: true
	PluginID string `json:"plugin_id"`
	//in: body
	Body struct {
		TenantID  string `json:"tenant_id"`
		ServiceID string `json:"service_id"`
		// 环境变量
		// in: body
		// required: true
		ConfigEnvs ConfigEnvs `json:"config_envs" validate:"config_envs"`
	}
}

//ConfigEnvs Config
type ConfigEnvs struct {
	NormalEnvs  []*VersionEnv `json:"normal_envs" validate:"normal_envs"`
	ComplexEnvs *ResourceSpec `json:"complex_envs" validate:"complex_envs"`
}

//VersionEnv VersionEnv
type VersionEnv struct {
	//变量名
	//in:body
	//required: true
	EnvName string `json:"env_name" validate:"env_name"`
	//变量值
	//in:body
	//required: true
	EnvValue string `json:"env_value" validate:"env_value"`
}

//TransPlugins TransPlugins
type TransPlugins struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	//in: body
	Body struct {
		// 从该租户安装
		// in: body
		// required: true
		FromTenantName string `json:"from_tenant_name" validate:"from_tenant_name"`
		// 插件id
		// in: body
		// required: true
		PluginsID []string `json:"plugins_id" validate:"plugins_id"`
	}
}

// PluginVersionEnv -
type PluginVersionEnv struct {
	EnvName  string `json:"env_name" validate:"env_name"`
	EnvValue string `json:"env_value" validate:"env_value"`
}

func (p *PluginVersionEnv) DbModel(componentID, pluginID string) *dbmodel.TenantPluginVersionEnv {
	return &dbmodel.TenantPluginVersionEnv{
		ServiceID: componentID,
		PluginID:  pluginID,
		EnvName:   p.EnvName,
		EnvValue:  p.EnvValue,
	}
}

// TenantPluginVersionConfig -
type TenantPluginVersionConfig struct {
	ConfigStr string `json:"config_str" validate:"config_str"`
}

func (p *TenantPluginVersionConfig) DbModel(componentID, pluginID string) *dbmodel.TenantPluginVersionDiscoverConfig {
	return &dbmodel.TenantPluginVersionDiscoverConfig{
		ServiceID: componentID,
		PluginID:  pluginID,
		ConfigStr: p.ConfigStr,
	}
}

// ComponentPlugin -
type ComponentPlugin struct {
	PluginID          string                    `json:"plugin_id"`
	VersionID         string                    `json:"version_id"`
	PluginModel       string                    `json:"plugin_model"`
	ContainerCPU      int                       `json:"container_cpu"`
	ContainerMemory   int                       `json:"container_memory"`
	Switch            bool                      `json:"switch"`
	VersionConfig     TenantPluginVersionConfig `json:"tenant_plugin_version_config"`
	PluginVersionEnvs []PluginVersionEnv        `json:"tenant_plugin_version_envs"`
}

func (p *ComponentPlugin) DbModel(componentID string) *dbmodel.TenantServicePluginRelation {
	return &dbmodel.TenantServicePluginRelation{
		VersionID:       p.VersionID,
		ServiceID:       componentID,
		PluginID:        p.PluginID,
		Switch:          p.Switch,
		PluginModel:     p.PluginModel,
		ContainerCPU:    p.ContainerCPU,
		ContainerMemory: p.ContainerMemory,
	}
}

type TenantServicesStreamPluginPort struct {
	TenantID      string `json:"tenant_id"`
	ServiceID     string `json:"service_id"`
	PluginModel   string `json:"plugin_model"`
	ContainerPort int    `json:"container_port"`
	PluginPort    int    `json:"plugin_port"`
}
