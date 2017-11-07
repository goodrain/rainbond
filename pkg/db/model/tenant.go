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

package model

import "time"
import "strings"
import "fmt"
import "os"

//Model 默认字段
type Model struct {
	ID        uint      `gorm:"column:ID;primary_key"`
	CreatedAt time.Time `gorm:"column:create_time"`
}

//IDModel 默认ID字段
type IDModel struct {
	ID uint `gorm:"column:ID;primary_key"`
}

//Interface model interface
type Interface interface {
	TableName() string
}

//Tenants 租户信息
type Tenants struct {
	Model
	Name string `gorm:"column:name;size:40;unique_index"`
	UUID string `gorm:"column:uuid;size:33;unique_index"`
	EID  string `gorm:"column:eid"`
}

//TableName 返回租户表名称
func (t *Tenants) TableName() string {
	return "tenants"
}

//TenantServices 租户应用
type TenantServices struct {
	Model
	TenantID  string `gorm:"column:tenant_id;size:32" json:"tenant_id"`
	ServiceID string `gorm:"column:service_id;size:32" json:"service_id"`
	// 服务key
	ServiceKey string `gorm:"column:service_key;size:32" json:"service_key"`
	// 服务别名
	ServiceAlias string `gorm:"column:service_alias;size:30" json:"service_alias"`
	// 服务描述
	Comment string `gorm:"column:comment" json:"comment"`
	// 服务版本
	ServiceVersion string `gorm:"column:service_version;size:32" json:"service_version"`
	// 镜像名称
	ImageName string `gorm:"column:image_name;size:100" json:"image_name"`
	// 容器CPU权重
	ContainerCPU int `gorm:"column:container_cpu;default:500" json:"container_cpu"`
	// 容器最大内存
	ContainerMemory int `gorm:"column:container_memory;default:128" json:"container_memory"`
	// 容器启动命令
	ContainerCMD string `gorm:"column:container_cmd;size:2048" json:"container_cmd"`
	// 容器环境变量
	ContainerEnv string `gorm:"column:container_env;size:255" json:"container_env"`
	// 卷名字
	VolumePath string `gorm:"column:volume_path" json:"volume_path"`
	// 容器挂载目录
	VolumeMountPath string `gorm:"column:volume_mount_path" json:"volume_mount_path"`
	// 宿主机目录
	HostPath string `gorm:"column:host_path" json:"host_path"`
	// 扩容方式；0:无状态；1:有状态；2:分区
	ExtendMethod string `gorm:"column:extend_method;default:'stateless';" json:"extend_method"`
	// 节点数
	Replicas int `gorm:"column:replicas;default:1" json:"replicas"`
	// 部署版本
	DeployVersion string `gorm:"column:deploy_version" json:"deploy_version"`
	// 服务分类：application,cache,store
	Category string `gorm:"column:category" json:"category"`
	// 服务当前状态：undeploy,running,closed,unusual,starting,checking,stoping
	CurStatus string `gorm:"column:cur_status;default:'undeploy'" json:"cur_status"`
	// 计费状态 为1 计费，为0不计费
	Status int `gorm:"column:status;default:0" json:"status"`
	// 最新操作ID
	EventID string `gorm:"column:event_id" json:"event_id"`
	// 服务类型
	ServiceType string `gorm:"column:service_type" json:"service_type"`
	// 镜像来源
	Namespace string `gorm:"column:namespace" json:"namespace"`
	// 共享类型shared、exclusive
	VolumeType string `gorm:"column:volume_type;default:'shared'" json:"volume_type"`
	// 端口类型，one_outer;dif_protocol;multi_outer
	PortType string `gorm:"column:port_type;default:'multi_outer'" json:"port_type"`
	// 更新时间
	UpdateTime time.Time `gorm:"column:update_time" json:"update_time"`
	// 服务创建类型cloud云市服务,assistant云帮服务
	ServiceOrigin string `gorm:"column:service_origin;default:'assistant'" json:"service_origin"`
	// 代码来源:gitlab,github
	CodeFrom string `gorm:"column:code_from" json:"code_from"`

	Domain string `gorm:"column:domain" json:"domain"`
}

//IsSlug 是否是slug应用
func (t *TenantServices) IsSlug() bool {
	return strings.HasPrefix(t.ImageName, "goodrain.me/runner")
}

//ChangeDelete ChangeDelete
func (t *TenantServices) ChangeDelete() *TenantServicesDelete {
	delete := TenantServicesDelete(*t)
	return &delete
}

//Autodomain 构建默认域名
func (t *TenantServices) Autodomain(tenantName string, containerPort int) string {
	exDomain := os.Getenv("EX_DOMAIN")
	if exDomain == "" {
		return ""
	}
	if strings.Contains(exDomain, ":") {
		exDomain = strings.Split(exDomain, ":")[0]
	}
	if t.PortType == "dif_protocol" {
		return fmt.Sprintf("%s.%s.%s", t.ServiceAlias, tenantName, exDomain)
	}
	if t.PortType == "multi_outer" {
		return fmt.Sprintf("%d.%s.%s.%s", containerPort, t.ServiceAlias, tenantName, exDomain)
	}
	if t.PortType == "one_outer" {
		return fmt.Sprintf("%s.%s.%s", t.ServiceAlias, tenantName, exDomain)
	}
	return ""
}

//TableName 表名
func (t *TenantServices) TableName() string {
	return "tenant_services"
}

//TenantServicesDelete 已删除的应用表
type TenantServicesDelete struct {
	Model
	TenantID  string `gorm:"column:tenant_id;size:32" json:"tenant_id"`
	ServiceID string `gorm:"column:service_id;size:32" json:"service_id"`
	// 服务key
	ServiceKey string `gorm:"column:service_key;size:32" json:"service_key"`
	// 服务别名
	ServiceAlias string `gorm:"column:service_alias;size:30" json:"service_alias"`
	// 服务描述
	Comment string `gorm:"column:comment" json:"comment"`
	// 服务版本
	ServiceVersion string `gorm:"column:service_version;size:32" json:"service_version"`
	// 镜像名称
	ImageName string `gorm:"column:image_name;size:100" json:"image_name"`
	// 容器CPU权重
	ContainerCPU int `gorm:"column:container_cpu;default:500" json:"container_cpu"`
	// 容器最大内存
	ContainerMemory int `gorm:"column:container_memory;default:128" json:"container_memory"`
	// 容器启动命令
	ContainerCMD string `gorm:"column:container_cmd;size:2048" json:"container_cmd"`
	// 容器环境变量
	ContainerEnv string `gorm:"column:container_env;size:255" json:"container_env"`
	// 卷名字
	VolumePath string `gorm:"column:volume_path" json:"volume_path"`
	// 容器挂载目录
	VolumeMountPath string `gorm:"column:volume_mount_path" json:"volume_mount_path"`
	// 宿主机目录
	HostPath string `gorm:"column:host_path" json:"host_path"`
	// 扩容方式；0:无状态；1:有状态；2:分区
	ExtendMethod string `gorm:"column:extend_method;default:'stateless';" json:"extend_method"`
	// 节点数
	Replicas int `gorm:"column:replicas;default:1" json:"replicas"`
	// 部署版本
	DeployVersion string `gorm:"column:deploy_version" json:"deploy_version"`
	// 服务分类：application,cache,store
	Category string `gorm:"column:category" json:"category"`
	// 服务当前状态：undeploy,running,closed,unusual,starting,checking,stoping
	CurStatus string `gorm:"column:cur_status;default:'undeploy'" json:"cur_status"`
	// 计费状态 为1 计费，为0不计费
	Status int `gorm:"column:status;default:0" json:"status"`
	// 最新操作ID
	EventID string `gorm:"column:event_id" json:"event_id"`
	// 服务类型
	ServiceType string `gorm:"column:service_type" json:"service_type"`
	// 镜像来源
	Namespace string `gorm:"column:namespace" json:"namespace"`
	// 共享类型shared、exclusive
	VolumeType string `gorm:"column:volume_type;default:'shared'" json:"volume_type"`
	// 端口类型，one_outer;dif_protocol;multi_outer
	PortType string `gorm:"column:port_type;default:'multi_outer'" json:"port_type"`
	// 更新时间
	UpdateTime time.Time `gorm:"column:update_time" json:"update_time"`
	// 服务创建类型cloud云市服务,assistant云帮服务
	ServiceOrigin string `gorm:"column:service_origin;default:'assistant'" json:"service_origin"`
	// 代码来源:gitlab,github
	CodeFrom string `gorm:"column:code_from" json:"code_from"`

	Domain string `gorm:"column:domain" json:"domain"`
}

//TableName 表名
func (t *TenantServicesDelete) TableName() string {
	return "tenant_services_delete"
}

//TenantServicesPort 应用端口信息
type TenantServicesPort struct {
	Model
	TenantID       string `gorm:"column:tenant_id;size:32" validate:"tenant_id|between:30,33" json:"tenant_id"`
	ServiceID      string `gorm:"column:service_id;size:32" validate:"service_id|between:30,33" json:"service_id"`
	ContainerPort  int    `gorm:"column:container_port" validate:"container_port|required|numeric_between:1,65535" json:"container_port"`
	MappingPort    int    `gorm:"column:mapping_port" validate:"mapping_port|required|numeric_between:1,65535" json:"mapping_port"`
	Protocol       string `gorm:"column:protocol" validate:"protocol|required|in:http,https,stream,grpc" json:"protocol"`
	PortAlias      string `gorm:"column:port_alias" validate:"port_alias|required|alpha_dash" json:"port_alias"`
	IsInnerService bool   `gorm:"column:is_inner_service" validate:"is_inner_service|bool" json:"is_inner_service"`
	IsOuterService bool   `gorm:"column:is_outer_service" validate:"is_outer_service|bool" json:"is_outer_service"`
}

//TableName 表名
func (t *TenantServicesPort) TableName() string {
	return "tenant_services_port"
}

//TenantServiceLBMappingPort stream应用端口映射情况
type TenantServiceLBMappingPort struct {
	Model
	ServiceID string `gorm:"column:service_id;size:32"`
	//负载均衡VS使用端口
	Port int `gorm:"column:port;unique_index"`
	//此字段废除
	//	IP string `gorm:"column:ip"`
	//应用原端口
	ContainerPort int `gorm:"column:container_port"`
}

//TableName 表名
func (t *TenantServiceLBMappingPort) TableName() string {
	return "tenant_lb_mapping_port"
}

//TenantServiceRelation 应用依赖关系
type TenantServiceRelation struct {
	Model
	TenantID          string `gorm:"column:tenant_id;size:32" validate:"tenant_id" json:"tenant_id"`
	ServiceID         string `gorm:"column:service_id;size:32" validate:"service_id" json:"service_id"`
	DependServiceID   string `gorm:"column:dep_service_id;size:32" validate:"depend_service_id" json:"depend_service_id"`
	DependServiceType string `gorm:"column:dep_service_type" validate:"dep_service_type" json:"dep_service_type"`
	DependOrder       int    `gorm:"column:dep_order" validate:"dep_order" json:"dep_order"`
}

//TableName 表名
func (t *TenantServiceRelation) TableName() string {
	return "tenant_service_relation"
}

//TenantServiceEnvVar  应用环境变量
type TenantServiceEnvVar struct {
	Model
	TenantID      string `gorm:"column:tenant_id;size:32" validate:"tenant_id|between:30,33" json:"tenant_id"`
	ServiceID     string `gorm:"column:service_id;size:32" validate:"service_id|between:30,33" json:"service_id"`
	ContainerPort int    `gorm:"column:container_port" validate:"container_port|numeric_between:1,65535" json:"container_port"`
	Name          string `gorm:"column:name;size:100" validate:"name" json:"name"`
	AttrName      string `gorm:"column:attr_name" validate:"env_name|required" json:"attr_name"`
	AttrValue     string `gorm:"column:attr_value" validate:"env_value|required" json:"attr_value"`
	IsChange      bool   `gorm:"column:is_change" validate:"is_change|bool" json:"is_change"`
	Scope         string `gorm:"column:scope;default:'outer'" validate:"scope|in:outer,inner,both" json:"scope"`
}

//TableName 表名
func (t *TenantServiceEnvVar) TableName() string {
	//TODO:表名修改
	return "tenant_service_evn_var"
}

//TenantServiceMountRelation 应用挂载依赖纪录
type TenantServiceMountRelation struct {
	Model
	TenantID        string `gorm:"column:tenant_id;size:32" json:"tenant_id" validate:"tenant_id|between:30,33"`
	ServiceID       string `gorm:"column:service_id;size:32" json:"service_id" validate:"service_id|between:30,33"`
	DependServiceID string `gorm:"column:dep_service_id;size:32" json:"dep_service_id" validate:"dep_service_id|between:30,33"`
	//挂载路径(挂载应用可自定义)
	VolumePath string `gorm:"column:mnt_name" json:"volume_path" validate:"volume_path|required"`
	//主机路径(依赖应用的共享存储对应的主机路径)
	HostPath string `gorm:"column:mnt_dir" json:"host_path" validate:"host_path"`
	//存储名称(依赖应用的共享存储对应的名称)
	VolumeName string `gorm:"column:volume_name;size:40" json:"volume_name" validate:"volume_name|required"`
}

//TableName 表名
func (t *TenantServiceMountRelation) TableName() string {
	return "tenant_service_mnt_relation"
}

//VolumeType 存储类型
type VolumeType string

//ShareFileVolumeType 共享文件存储
var ShareFileVolumeType VolumeType = "share-file"

//LocalVolumeType 本地文件存储
var LocalVolumeType VolumeType = "local"

//MemoryFSVolumeType 内存文件存储
var MemoryFSVolumeType VolumeType = "memoryfs"

func (vt VolumeType) String() string {
	return string(vt)
}

//TenantServiceVolume 应用持久化纪录
type TenantServiceVolume struct {
	Model
	ServiceID string `gorm:"column:service_id;size:32" json:"service_id"`
	//服务类型
	Category string `gorm:"column:category;size:50" json:"category"`
	//存储类型（share,local,tmpfs）
	VolumeType string `gorm:"column:volume_type;size:20" json:"volume_type"`
	//存储名称
	VolumeName string `gorm:"column:volume_name;size:40" json:"volume_name"`
	//主机地址
	HostPath string `gorm:"column:host_path" json:"host_path"`
	//挂载地址
	VolumePath string `gorm:"column:volume_path" json:"volume_path"`
	//是否只读
	IsReadOnly bool `gorm:"column:is_read_only;default:false" json:"is_read_only"`
}

//TableName 表名
func (t *TenantServiceVolume) TableName() string {
	return "tenant_service_volume"
}

//TenantServiceLable 应用高级标签
type TenantServiceLable struct {
	Model
	ServiceID  string `gorm:"column:service_id;size:32"`
	LabelKey   string `gorm:"column:label_key;size:50"`
	LabelValue string `gorm:"column:label_value;size:50"`
}

//TableName 表名
func (t *TenantServiceLable) TableName() string {
	return "tenant_service_label"
}

//TenantServiceStatus 应用实时状态
type TenantServiceStatus struct {
	Model
	ServiceID string `gorm:"column:service_id;size:32"`
	Status    string `gorm:"column:status;size:24"`
	//undeploy 1, closed 2, stopping 3, starting 4, running 5
}

//TableName 表名
func (t *TenantServiceStatus) TableName() string {
	return "tenant_service_status"
}

//TenantPlugin 插件表
type TenantPlugin struct {
	Model
	//插件id
	PluginID string `gorm:"column:plugin_id;size:32"`
	//插件名称
	PluginName string `gorm:"column:plugin_name;size:32"`
	//插件用途描述
	PluginInfo string `gorm:"column:plugin_info:size:100"`
	//插件docker地址
	ImageURL string `gorm:"column:image_url"`
	//插件goodrain地址
	ImageLocal string `gorm:"column:image_local"`
	//带分支信息的git地址
	Repo string `gorm:"column:repo"`
	//git地址
	GitURL string `gorm:"column:git_url"`
	//构建模式
	BuildModel string `gorm:"column:build_model"`
	//插件模式
	PluginModel string `gorm:"column:plugin_model"`
	//插件启动命令
	PluginCMD string `gorm:"column:plugin_cmd"`
	TenantID  string `gorm:"column:tenant_id"`
	//tenant_name 统计cpu mem使用
	Domain string `gorm:"column:domain"`
}

//TableName 表名
func (t *TenantPlugin) TableName() string {
	return "tenant_plugin"
}

//TenantPluginDefaultENV 插件默认环境变量
type TenantPluginDefaultENV struct {
	Model
	//对应插件id
	PluginID string `gorm:"column:plugin_id"`
	//配置项名称
	ENVName string `gorm:"column:env_name"`
	//配置项值
	ENVValue string `gorm:"column:env_value"`
	//使用人是否可改
	Change bool `gorm:"column:change;default:false"`
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
	VersionID       string `gorm:"column:version_id;size:32"`
	PluginID        string `gorm:"column:plugin_id;size:32"`
	Kind            string `gorm:"column:kind;size:24"`
	BaseImage       string `gorm:"column:base_image"`
	BuildLocalImage string `gorm:"column:build_local_image"`
	BuildTime       string `gorm:"column:build_time"`
	Repo            string `gorm:"column:repo"`
	GitURL          string `gorm:"column:git_url"`
	Info            string `gorm:"column:info"`
	Status          string `gorm:"column:status;size:24"`
}

//TableName 表名
func (t *TenantPluginBuildVersion) TableName() string {
	return "tenant_plugin_build_version"
}

//TenantPluginVersionEnv TenantPluginVersionEnv
type TenantPluginVersionEnv struct {
	Model
	//VersionID string `gorm:"column:version_id;size:32"`
	PluginID  string `gorm:"column:plugin_id;size:32"`
	EnvName   string `gorm:"column:env_name"`
	EnvValue  string `gorm:"column:env_value"`
	ServiceID string `gorm:"column:service_id"`
}

//TableName 表名
func (t *TenantPluginVersionEnv) TableName() string {
	return "tenant_plugin_version_env"
}

//TenantServicePluginRelation TenantServicePluginRelation
type TenantServicePluginRelation struct {
	Model
	VersionID string `gorm:"column:version_id;size:32"`
	PluginID  string `gorm:"column:plugin_id;size:32"`
	ServiceID string `gorm:"column:service_id;size:32"`
	Switch    bool   `gorm:"column:switch;default:false"`
}

//TableName 表名
func (t *TenantServicePluginRelation) TableName() string {
	return "tenant_service_plugin_relation"
}

//LabelKeyNodeSelector 节点选择标签
var LabelKeyNodeSelector = "node-selector"

//LabelKeyNodeAffinity 节点亲和标签
var LabelKeyNodeAffinity = "node-affinity"

//LabelKeyNodeAntyAffinity 节点反亲和标签
var LabelKeyNodeAntyAffinity = "node-anti-affinity"

//LabelKeyServiceType 应用部署类型标签
var LabelKeyServiceType = "service-type"

//LabelKeyServiceAffinity 应用亲和标签
var LabelKeyServiceAffinity = "service-affinity"

//LabelKeyServiceAntyAffinity 应用反亲和标签
var LabelKeyServiceAntyAffinity = "service-anti-affinity"
