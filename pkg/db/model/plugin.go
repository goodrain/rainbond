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

//TenantPlugin 插件表
type TenantPlugin struct {
	Model
	//插件id
	PluginID string `gorm:"column:plugin_id;size:32"`
	//插件名称
	PluginName string `gorm:"column:plugin_name;size:32" json:"plugin_name"`
	//插件用途描述
	PluginInfo string `gorm:"column:plugin_info;size:100" json:"plugin_info"`
	//插件docker地址
	ImageURL string `gorm:"column:image_url" json:"image_url"`
	//git地址
	GitURL string `gorm:"column:git_url" json:"git_url"`
	//构建模式
	BuildModel string `gorm:"column:build_model" json:"build_model"`
	//插件模式
	PluginModel string `gorm:"column:plugin_model" json:"plugin_model"`
	//租户id
	TenantID string `gorm:"column:tenant_id" json:"tenant_id"`
	//tenant_name 统计cpu mem使用
	Domain string `gorm:"column:domain" json:"domain"`
	//gitlab; github
	CodeFrom string `gorm:"column:code_from" json:"code_from"`
}

//TableName 表名
func (t *TenantPlugin) TableName() string {
	return "tenant_plugin"
}

//TenantPluginDefaultENV 插件默认环境变量
type TenantPluginDefaultENV struct {
	Model
	//对应插件id
	PluginID string `gorm:"column:plugin_id" json:"plugin_id"`
	//构建版本
	VersionID string `gorm:"column:version_id;size:32" json:"version_id"`
	//配置项名称
	ENVName string `gorm:"column:env_name" json:"env_name"`
	//配置项值
	ENVValue string `gorm:"column:env_value" json:"env_value"`
	//使用人是否可改
	IsChange bool `gorm:"column:is_change;default:false" json:"is_change"`
}

//TableName 表名
func (t *TenantPluginDefaultENV) TableName() string {
	return "tenant_plugin_default_env"
}

//TenantPluginDefaultConf 插件默认配置表 由console提供
type TenantPluginDefaultConf struct {
	Model
	//对应插件id
	PluginID string `gorm:"column:plugin_id"`
	//配置项名称
	ConfName string `gorm:"column:conf_name"`
	//配置项值
	ConfValue string `gorm:"column:conf_value"`
	//配置项类型，由console提供
	ConfType string `gorm:"column:conf_type"`
}

//TableName 表名
func (t *TenantPluginDefaultConf) TableName() string {
	return "tenant_plugin_default_conf"
}

//TenantPluginBuildVersion 插件构建版本表
type TenantPluginBuildVersion struct {
	Model
	VersionID       string `gorm:"column:version_id;size:32" json:"version_id"`
	PluginID        string `gorm:"column:plugin_id;size:32" json:"plugin_id"`
	Kind            string `gorm:"column:kind;size:24" json:"kind"`
	BaseImage       string `gorm:"column:base_image;size:100" json:"base_image"`
	BuildLocalImage string `gorm:"column:build_local_image;size:100" json:"build_local_image"`
	BuildTime       string `gorm:"column:build_time" json:"build_time"`
	Repo            string `gorm:"column:repo" json:"repo"`
	GitURL          string `gorm:"column:git_url" json:"git_url"`
	Info            string `gorm:"column:info" json:"info"`
	Status          string `gorm:"column:status;size:24" json:"status"`
	// 容器CPU权重
	ContainerCPU int `gorm:"column:container_cpu;default:125" json:"container_cpu"`
	// 容器最大内存
	ContainerMemory int `gorm:"column:container_memory;default:50" json:"container_memory"`
	// 容器启动命令
	ContainerCMD string `gorm:"column:container_cmd;size:2048" json:"container_cmd"`
}

//TableName 表名
func (t *TenantPluginBuildVersion) TableName() string {
	return "tenant_plugin_build_version"
}

//TenantPluginVersionEnv TenantPluginVersionEnv
type TenantPluginVersionEnv struct {
	Model
	//VersionID string `gorm:"column:version_id;size:32"`
	PluginID  string `gorm:"column:plugin_id;size:32" json:"plugin_id"`
	EnvName   string `gorm:"column:env_name" json:"env_name"`
	EnvValue  string `gorm:"column:env_value" json:"env_value"`
	ServiceID string `gorm:"column:service_id" json:"service_id"`
}

//TableName 表名
func (t *TenantPluginVersionEnv) TableName() string {
	return "tenant_plugin_version_env"
}

//TenantServicePluginRelation TenantServicePluginRelation
type TenantServicePluginRelation struct {
	Model
	VersionID   string `gorm:"column:version_id;size:32" json:"version_id"`
	PluginID    string `gorm:"column:plugin_id;size:32" json:"plugin_id"`
	ServiceID   string `gorm:"column:service_id;size:32" json:"service_id"`
	PluginModel string `gorm:"column:plugin_model;size:24" json:"plugin_model"`
	Switch      bool   `gorm:"column:switch;default:false" json:"switch"`
}

//TableName 表名
func (t *TenantServicePluginRelation) TableName() string {
	return "tenant_service_plugin_relation"
}

//TenantServicesStreamPluginPort 绑定stream类型插件后端口映射信息
type TenantServicesStreamPluginPort struct {
	Model
	TenantID      string `gorm:"column:tenant_id;size:32" validate:"tenant_id|between:30,33" json:"tenant_id"`
	ServiceID     string `gorm:"column:service_id;size:32" validate:"service_id|between:30,33" json:"service_id"`
	PluginModel   string `gorm:"column:plugin_model;size:24" json:"plugin_model"`
	ContainerPort int    `gorm:"column:container_port" validate:"container_port|required|numeric_between:1,65535" json:"container_port"`
	PluginPort    int    `gorm:"column:plugin_port" json:"plugin_port"`
}

//TableName 表名
func (t *TenantServicesStreamPluginPort) TableName() string {
	return "tenant_services_stream_plugin_port"
}

//Plugin model 插件标签

//TODO: 插件类型名规定
//@ 1. 插件大类  xxx-plugin
//@ 2. 大类细分  冒号+细分 xxx-plugin:up  or  xxx-plugin:down

//InitPlugin 初始化插件
var InitPlugin = "init-plugin"

//UpNetPlugin 上游网络插件
var UpNetPlugin = "net-plugin:up"

//DownNetPlugin 下游网络插件
var DownNetPlugin = "net-plugin:down"

//GeneralPlugin 一般插件,默认分类,优先级最低
var GeneralPlugin = "general-plugin"
