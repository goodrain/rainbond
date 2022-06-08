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

import (
	"github.com/docker/distribution/reference"
	"github.com/sirupsen/logrus"
)

//TenantPlugin plugin model
type TenantPlugin struct {
	Model
	PluginID string `gorm:"column:plugin_id;size:32"`
	//plugin name
	PluginName string `gorm:"column:plugin_name;size:32" json:"plugin_name"`
	//plugin describe
	PluginInfo string `gorm:"column:plugin_info;size:255" json:"plugin_info"`
	//plugin build by docker image name
	ImageURL string `gorm:"column:image_url" json:"image_url"`
	//plugin build by git code url
	GitURL string `gorm:"column:git_url" json:"git_url"`
	//build mode
	BuildModel string `gorm:"column:build_model" json:"build_model"`
	//plugin model InitPlugin,InBoundNetPlugin,OutBoundNetPlugin
	PluginModel string `gorm:"column:plugin_model" json:"plugin_model"`
	//tenant id
	TenantID string `gorm:"column:tenant_id" json:"tenant_id"`
	//tenant_name Used to calculate CPU and Memory.
	Domain string `gorm:"column:domain" json:"domain"`
	//gitlab; github The deprecated
	CodeFrom string `gorm:"column:code_from" json:"code_from"`
}

//TableName table name
func (t *TenantPlugin) TableName() string {
	return "tenant_plugin"
}

//TenantPluginDefaultENV plugin default env config
type TenantPluginDefaultENV struct {
	Model
	//plugin id
	PluginID string `gorm:"column:plugin_id" json:"plugin_id"`
	//plugin version
	VersionID string `gorm:"column:version_id;size:32" json:"version_id"`
	//env name
	ENVName string `gorm:"column:env_name" json:"env_name"`
	//env value
	ENVValue string `gorm:"column:env_value" json:"env_value"`
	//value is change
	IsChange bool `gorm:"column:is_change;default:false" json:"is_change"`
}

//TableName table name
func (t *TenantPluginDefaultENV) TableName() string {
	return "tenant_plugin_default_env"
}

//TenantPluginBuildVersion plugin build version
type TenantPluginBuildVersion struct {
	Model
	//plugin version eg v1.0.0
	VersionID string `gorm:"column:version_id;size:32" json:"version_id"`
	//deploy version eg 20180528071717
	DeployVersion   string `gorm:"column:deploy_version;size:32" json:"deploy_version"`
	PluginID        string `gorm:"column:plugin_id;size:32" json:"plugin_id"`
	Kind            string `gorm:"column:kind;size:24" json:"kind"`
	BaseImage       string `gorm:"column:base_image;size:200" json:"base_image"`
	BuildLocalImage string `gorm:"column:build_local_image;size:200" json:"build_local_image"`
	BuildTime       string `gorm:"column:build_time" json:"build_time"`
	Repo            string `gorm:"column:repo" json:"repo"`
	GitURL          string `gorm:"column:git_url" json:"git_url"`
	Info            string `gorm:"column:info" json:"info"`
	Status          string `gorm:"column:status;size:24" json:"status"`
	// container default cpu
	ContainerCPU int `gorm:"column:container_cpu;default:0" json:"container_cpu"`
	// container default memory
	ContainerMemory int `gorm:"column:container_memory;default:0" json:"container_memory"`
	// container args
	ContainerCMD string `gorm:"column:container_cmd;size:2048" json:"container_cmd"`
}

//TableName table name
func (t *TenantPluginBuildVersion) TableName() string {
	return "tenant_plugin_build_version"
}

//CreateShareImage CreateShareImage
func (t *TenantPluginBuildVersion) CreateShareImage(hubURL, namespace string) (string, error) {
	_, err := reference.ParseAnyReference(t.BuildLocalImage)
	if err != nil {
		logrus.Errorf("reference image error: %s", err.Error())
		return "", err
	}
	image := ParseImage(t.BuildLocalImage)
	if hubURL != "" {
		image.Host = hubURL
	}
	if namespace != "" {
		image.Namespace = namespace
	}
	image.Name = image.Name + "_" + t.VersionID
	return image.String(), nil
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

//TableName table name
func (t *TenantPluginVersionEnv) TableName() string {
	return "tenant_plugin_version_env"
}

//TenantPluginVersionDiscoverConfig service plugin config that can be dynamic discovery
type TenantPluginVersionDiscoverConfig struct {
	Model
	PluginID  string `gorm:"column:plugin_id;size:32" json:"plugin_id"`
	ServiceID string `gorm:"column:service_id;size:32" json:"service_id"`
	ConfigStr string `gorm:"column:config_str;" sql:"type:longtext;" json:"config_str"`
}

//TableName table name
func (t *TenantPluginVersionDiscoverConfig) TableName() string {
	return "tenant_plugin_version_config"
}

//TenantServicePluginRelation TenantServicePluginRelation
type TenantServicePluginRelation struct {
	Model
	VersionID   string `gorm:"column:version_id;size:32" json:"version_id"`
	PluginID    string `gorm:"column:plugin_id;size:32" json:"plugin_id"`
	ServiceID   string `gorm:"column:service_id;size:32" json:"service_id"`
	PluginModel string `gorm:"column:plugin_model;size:24" json:"plugin_model"`
	// container default cpu  v3.5.1 add
	ContainerCPU int `gorm:"column:container_cpu;default:0" json:"container_cpu"`
	// container default memory  v3.5.1 add
	ContainerMemory int  `gorm:"column:container_memory;default:0" json:"container_memory"`
	Switch          bool `gorm:"column:switch;default:0" json:"switch"`
}

//TableName table name
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

//InBoundNetPlugin 入站治理网络插件
var InBoundNetPlugin = "net-plugin:up"

//OutBoundNetPlugin 出站治理网络插件
var OutBoundNetPlugin = "net-plugin:down"

//InBoundAndOutBoundNetPlugin 出站和入站治理
var InBoundAndOutBoundNetPlugin = "net-plugin:in-and-out"

//GeneralPlugin 一般插件,默认分类,优先级最低
var GeneralPlugin = "general-plugin"
