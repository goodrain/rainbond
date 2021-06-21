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
	"fmt"
	"os"
	"strings"
	"time"
)

//Model 默认字段
type Model struct {
	ID        uint      `gorm:"column:ID;primary_key"`
	CreatedAt time.Time `gorm:"column:create_time" json:"create_time"`
}

//IDModel 默认ID字段
type IDModel struct {
	ID uint `gorm:"column:ID;primary_key"`
}

//Interface model interface
type Interface interface {
	TableName() string
}

// TenantStatus -
type TenantStatus string

var (
	// TenantStatusNormal -
	TenantStatusNormal TenantStatus = "normal"
	// TenantStatusDeleting -
	TenantStatusDeleting TenantStatus = "deleting"
	// TenantStatusDeleteFailed -
	TenantStatusDeleteFailed TenantStatus = "delete_failed"
)

func (t TenantStatus) String() string {
	return string(t)
}

//Tenants 租户信息
type Tenants struct {
	Model
	Name        string `gorm:"column:name;size:40;unique_index"`
	UUID        string `gorm:"column:uuid;size:33;unique_index"`
	EID         string `gorm:"column:eid"`
	LimitMemory int    `gorm:"column:limit_memory"`
	Status      string `gorm:"column:status;default:'normal'"`
}

//TableName 返回租户表名称
func (t *Tenants) TableName() string {
	return "tenants"
}

// ServiceKind kind of service
type ServiceKind string

// ServiceKindThirdParty means third-party service
var ServiceKindThirdParty ServiceKind = "third_party"

// ServiceKindInternal means internal service
var ServiceKindInternal ServiceKind = "internal"

func (s ServiceKind) String() string {
	return string(s)
}

// ServiceType type of service
type ServiceType string

// String imple String
func (s ServiceType) String() string {
	return string(s)
}

// IsState is state type or not
func (s ServiceType) IsState() bool {
	if s == ServiceTypeStateMultiple || s == ServiceTypeStateSingleton {
		return true
	}
	return false
}

// IsSingleton is singleton or not
func (s ServiceType) IsSingleton() bool {
	if s == "" {
		return false
	}
	if s == ServiceTypeStatelessMultiple || s == ServiceTypeStateMultiple {
		return false
	}
	return true
}

// IsState is state service or stateless service
// TODO fanyangyang 根据组件简单判断是否是有状态
func (t *TenantServices) IsState() bool {
	if t.ExtendMethod == "" {
		return false
	}
	return ServiceType(t.ExtendMethod).IsState()
}

// IsSingleton is singleton or multiple service
func (t *TenantServices) IsSingleton() bool {
	if t.ExtendMethod == "" {
		return false
	}
	return ServiceType(t.ExtendMethod).IsSingleton()
}

// ServiceTypeUnknown unknown
var ServiceTypeUnknown ServiceType = "unknown"

//ServiceTypeStatelessSingleton stateless_singleton
var ServiceTypeStatelessSingleton ServiceType = "stateless_singleton"

// ServiceTypeStatelessMultiple stateless_multiple
var ServiceTypeStatelessMultiple ServiceType = "stateless_multiple"

// ServiceTypeStateSingleton state_singleton
var ServiceTypeStateSingleton ServiceType = "state_singleton"

// ServiceTypeStateMultiple state_multiple
var ServiceTypeStateMultiple ServiceType = "state_multiple"

//TenantServices app service base info
type TenantServices struct {
	Model
	TenantID  string `gorm:"column:tenant_id;size:32" json:"tenant_id"`
	ServiceID string `gorm:"column:service_id;size:32" json:"service_id"`
	// 服务key
	ServiceKey string `gorm:"column:service_key;size:32" json:"service_key"`
	// 服务别名
	ServiceAlias string `gorm:"column:service_alias;size:30" json:"service_alias"`
	// service regist endpoint name(host name), used of statefulset
	ServiceName string `gorm:"column:service_name;size:100" json:"service_name"`
	// （This field is not currently used, use ExtendMethod）Service type now service support
	ServiceType string `gorm:"column:service_type;size:32" json:"service_type"`
	// 服务描述
	Comment string `gorm:"column:comment" json:"comment"`
	// 容器CPU权重
	ContainerCPU int `gorm:"column:container_cpu;default:500" json:"container_cpu"`
	// 容器最大内存
	ContainerMemory int `gorm:"column:container_memory;default:128" json:"container_memory"`
	// container GPU, The amount of video memory applied for GPU. The unit is MiB
	// default is 0, That means no GPU is required
	ContainerGPU int `gorm:"column:container_gpu;default:0" json:"container_gpu"`
	//UpgradeMethod service upgrade controller type
	//such as : `Rolling` `OnDelete`
	UpgradeMethod string `gorm:"column:upgrade_method;default:'Rolling'" json:"upgrade_method"`
	// 组件类型  component deploy type stateless_singleton/stateless_multiple/state_singleton/state_multiple
	ExtendMethod string `gorm:"column:extend_method;default:'stateless';" json:"extend_method"`
	// 节点数
	Replicas int `gorm:"column:replicas;default:1" json:"replicas"`
	// 部署版本
	DeployVersion string `gorm:"column:deploy_version" json:"deploy_version"`
	// 服务分类：application,cache,store
	Category string `gorm:"column:category" json:"category"`
	// 服务当前状态：undeploy,running,closed,unusual,starting,checking,stoping(deprecated)
	CurStatus string `gorm:"column:cur_status;default:'undeploy'" json:"cur_status"`
	// 计费状态 为1 计费，为0不计费 (deprecated)
	Status int `gorm:"column:status;default:0" json:"status"`
	// 最新操作ID
	EventID string `gorm:"column:event_id" json:"event_id"`
	// 租户ID
	Namespace string `gorm:"column:namespace" json:"namespace"`
	// 更新时间
	UpdateTime time.Time `gorm:"column:update_time" json:"update_time"`
	// 服务创建类型cloud云市服务,assistant云帮服务
	ServiceOrigin string `gorm:"column:service_origin;default:'assistant'" json:"service_origin"`
	// kind of service. option: internal, third_party
	Kind string `gorm:"column:kind;default:'internal'" json:"kind"`
	// service bind appID
	AppID string `gorm:"column:app_id" json:"app_id"`
}

//Image 镜像
type Image struct {
	Host      string
	Namespace string
	Name      string
}

func (i Image) String() string {
	if i.Namespace == "" {
		return fmt.Sprintf("%s/%s", i.Host, i.Name)
	}
	return fmt.Sprintf("%s/%s/%s", i.Host, i.Namespace, i.Name)
}

//ParseImage 简单解析镜像名
func ParseImage(name string) (image Image) {
	i := strings.IndexRune(name, '/')
	if i == -1 || (!strings.ContainsAny(name[:i], ".:") && name[:i] != "localhost") {
		image.Host, image.Name = "docker.io", name
	} else {
		image.Host, image.Name = name[:i], name[i+1:]
	}

	j := strings.IndexRune(image.Name, '/')
	if j != -1 {
		image.Namespace = image.Name[:j]
		image.Name = image.Name[j+1:]
	}
	return
}

//CreateShareSlug 生成源码包分享的地址
func (t *TenantServices) CreateShareSlug(servicekey, namespace, version string) string {
	return fmt.Sprintf("%s/%s/%s_%s.tgz", namespace, servicekey, version, t.DeployVersion)
}

//ChangeDelete ChangeDelete
func (t *TenantServices) ChangeDelete() *TenantServicesDelete {
	delete := TenantServicesDelete(*t)
	delete.UpdateTime = time.Now()
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
	return fmt.Sprintf("%d.%s.%s.%s", containerPort, t.ServiceAlias, tenantName, exDomain)
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
	// service regist endpoint name(host name), used of statefulset
	ServiceName string `gorm:"column:service_name;size:100" json:"service_name"`
	// Service type now service support stateless_singleton/stateless_multiple/state_singleton/state_multiple
	ServiceType string `gorm:"column:service_type;size:20" json:"service_type"`
	// 服务描述
	Comment string `gorm:"column:comment" json:"comment"`
	// 容器CPU权重
	ContainerCPU int `gorm:"column:container_cpu;default:500" json:"container_cpu"`
	// 容器最大内存
	ContainerMemory int `gorm:"column:container_memory;default:128" json:"container_memory"`
	// container GPU, The amount of video memory applied for GPU. The unit is MiB
	// default is 0, That means no GPU is required
	ContainerGPU int `gorm:"column:container_gpu;default:0" json:"container_gpu"`
	//UpgradeMethod service upgrade controller type
	//such as : `Rolling` `OnDelete`
	UpgradeMethod string `gorm:"column:upgrade_method;default:'Rolling'" json:"upgrade_method"`
	// 扩容方式；0:无状态；1:有状态；2:分区
	ExtendMethod string `gorm:"column:extend_method;default:'stateless';" json:"extend_method"`
	// 节点数
	Replicas int `gorm:"column:replicas;default:1" json:"replicas"`
	// 部署版本
	DeployVersion string `gorm:"column:deploy_version" json:"deploy_version"`
	// 服务分类：application,cache,store
	Category string `gorm:"column:category" json:"category"`
	// 服务当前状态：undeploy,running,closed,unusual,starting,checking,stoping(deprecated)
	CurStatus string `gorm:"column:cur_status;default:'undeploy'" json:"cur_status"`
	// 计费状态 为1 计费，为0不计费 (deprecated)
	Status int `gorm:"column:status;default:0" json:"status"`
	// 最新操作ID
	EventID string `gorm:"column:event_id" json:"event_id"`
	// 租户ID
	Namespace string `gorm:"column:namespace" json:"namespace"`
	// 更新时间
	UpdateTime time.Time `gorm:"column:update_time" json:"update_time"`
	// 服务创建类型cloud云市服务,assistant云帮服务
	ServiceOrigin string `gorm:"column:service_origin;default:'assistant'" json:"service_origin"`
	// kind of service. option: internal, third_party
	Kind string `gorm:"column:kind;default:'internal'" json:"kind"`
	// service bind appID
	AppID string `gorm:"column:app_id" json:"app_id"`
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
	Protocol       string `gorm:"column:protocol" validate:"protocol|required|in:http,https,tcp,grpc,udp,mysql" json:"protocol"`
	PortAlias      string `gorm:"column:port_alias" validate:"port_alias|required|alpha_dash" json:"port_alias"`
	IsInnerService *bool  `gorm:"column:is_inner_service" validate:"is_inner_service|bool" json:"is_inner_service"`
	IsOuterService *bool  `gorm:"column:is_outer_service" validate:"is_outer_service|bool" json:"is_outer_service"`
	K8sServiceName string `gorm:"column:k8s_service_name" json:"k8s_service_name"`
}

// Key returns the key of TenantServicesPort.
func (t *TenantServicesPort) Key() string {
	return fmt.Sprintf("%s/%s/%d", t.TenantID, t.ServiceID, t.ContainerPort)
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
	return "tenant_services_relation"
}

//TenantServiceEnvVar  应用环境变量
type TenantServiceEnvVar struct {
	Model
	TenantID      string `gorm:"column:tenant_id;size:32" validate:"tenant_id|between:30,33" json:"tenant_id"`
	ServiceID     string `gorm:"column:service_id;size:32" validate:"service_id|between:30,33" json:"service_id"`
	ContainerPort int    `gorm:"column:container_port" validate:"container_port|numeric_between:1,65535" json:"container_port"`
	Name          string `gorm:"column:name;size:1024" validate:"name" json:"name"`
	AttrName      string `gorm:"column:attr_name;size:1024" validate:"env_name|required" json:"attr_name"`
	AttrValue     string `gorm:"column:attr_value;type:text" validate:"env_value|required" json:"attr_value"`
	IsChange      bool   `gorm:"column:is_change" validate:"is_change|bool" json:"is_change"`
	Scope         string `gorm:"column:scope;default:'outer'" validate:"scope|in:outer,inner,both" json:"scope"`
}

//TableName 表名
func (t *TenantServiceEnvVar) TableName() string {
	//TODO:表名修改
	return "tenant_services_envs"
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
	VolumeType string `gorm:"column:volume_type" json:"volume_type" validate:"volume_type|required"`
}

//TableName 表名
func (t *TenantServiceMountRelation) TableName() string {
	return "tenant_services_mnt_relation"
}

//VolumeType 存储类型
type VolumeType string

//ShareFileVolumeType 共享文件存储
var ShareFileVolumeType VolumeType = "share-file"

//LocalVolumeType 本地文件存储
var LocalVolumeType VolumeType = "local"

//MemoryFSVolumeType 内存文件存储
var MemoryFSVolumeType VolumeType = "memoryfs"

//ConfigFileVolumeType configuration file volume type
var ConfigFileVolumeType VolumeType = "config-file"

// CephRBDVolumeType ceph rbd volume type
var CephRBDVolumeType VolumeType = "ceph-rbd"

// AliCloudVolumeType alicloud volume type
var AliCloudVolumeType VolumeType = "alicloud-disk"

// MakeNewVolume make volumeType
func MakeNewVolume(name string) VolumeType {
	return VolumeType(name)
}

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
	VolumeType string `gorm:"column:volume_type;size:64" json:"volume_type"`
	//存储名称
	VolumeName string `gorm:"column:volume_name;size:40" json:"volume_name"`
	//主机地址
	HostPath string `gorm:"column:host_path" json:"host_path"`
	//挂载地址
	VolumePath string `gorm:"column:volume_path" json:"volume_path"`
	//是否只读
	IsReadOnly bool `gorm:"column:is_read_only;default:0" json:"is_read_only"`
	// VolumeCapacity 存储大小
	VolumeCapacity int64 `gorm:"column:volume_capacity" json:"volume_capacity"`
	// AccessMode 读写模式（Important! A volume can only be mounted using one access mode at a time, even if it supports many. For example, a GCEPersistentDisk can be mounted as ReadWriteOnce by a single node or ReadOnlyMany by many nodes, but not at the same time. #https://kubernetes.io/docs/concepts/storage/persistent-volumes/#access-modes）
	AccessMode string `gorm:"column:access_mode" json:"access_mode"`
	// SharePolicy 共享模式
	SharePolicy string `gorm:"column:share_policy" json:"share_policy"`
	// BackupPolicy 备份策略
	BackupPolicy string `gorm:"column:backup_policy" json:"backup_policy"`
	// ReclaimPolicy 回收策略
	ReclaimPolicy string `json:"reclaim_policy"`
	// AllowExpansion 是否支持扩展
	AllowExpansion bool `gorm:"column:allow_expansion" json:"allow_expansion"`
	// VolumeProviderName 使用的存储驱动别名
	VolumeProviderName string `gorm:"column:volume_provider_name" json:"volume_provider_name"`
}

//TableName 表名
func (t *TenantServiceVolume) TableName() string {
	return "tenant_services_volume"
}

// Key returns the key of TenantServiceVolume.
func (t *TenantServiceVolume) Key() string {
	return fmt.Sprintf("%s/%s", t.ServiceID, t.VolumeName)
}

// TenantServiceConfigFile represents a data in configMap which is one of the types of volumes
type TenantServiceConfigFile struct {
	Model
	ServiceID   string `gorm:"column:service_id;size:32" json:"service_id"`
	VolumeName  string `gorm:"column:volume_name;size:32" json:"volume_name"`
	FileContent string `gorm:"column:file_content;size:65535" json:"filename"`
}

// TableName returns table name of TenantServiceConfigFile.
func (t *TenantServiceConfigFile) TableName() string {
	return "tenant_service_config_file"
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
	return "tenant_services_label"
}

//LabelKeyNodeSelector 节点选择标签
var LabelKeyNodeSelector = "node-selector"

//LabelKeyNodeAffinity 节点亲和标签
var LabelKeyNodeAffinity = "node-affinity"

//LabelKeyServiceType 应用部署类型标签
// TODO fanyangyang 待删除，组件类型记录在tenant_service表中
var LabelKeyServiceType = "service-type"

//LabelKeyServiceAffinity 应用亲和标签
var LabelKeyServiceAffinity = "service-affinity"

//LabelKeyServiceAntyAffinity 应用反亲和标签
var LabelKeyServiceAntyAffinity = "service-anti-affinity"

// LabelKeyServicePrivileged -
var LabelKeyServicePrivileged = "privileged"

//TenantServiceProbe 应用探针信息
type TenantServiceProbe struct {
	Model
	ServiceID string `gorm:"column:service_id;size:32" json:"service_id" validate:"service_id|between:30,33"`
	ProbeID   string `gorm:"column:probe_id;size:32" json:"probe_id" validate:"probe_id|between:30,33"`
	Mode      string `gorm:"column:mode;default:'liveness'" json:"mode" validate:"mode"`
	Scheme    string `gorm:"column:scheme;default:'scheme'" json:"scheme" validate:"scheme"`
	Path      string `gorm:"column:path" json:"path" validate:"path"`
	Port      int    `gorm:"column:port;size:5;default:80" json:"port" validate:"port|required|numeric_between:1,65535"`
	Cmd       string `gorm:"column:cmd;size:150" json:"cmd" validate:"cmd"`
	//http请求头，key=value,key2=value2
	HTTPHeader string `gorm:"column:http_header;size:300" json:"http_header" validate:"http_header"`
	//初始化等候时间
	InitialDelaySecond int `gorm:"column:initial_delay_second;size:2;default:4" json:"initial_delay_second" validate:"initial_delay_second"`
	//检测间隔时间
	PeriodSecond int `gorm:"column:period_second;size:2;default:3" json:"period_second" validate:"period_second"`
	//检测超时时间
	TimeoutSecond int `gorm:"column:timeout_second;size:3;default:5" json:"timeout_second" validate:"timeout_second"`
	//是否启用
	IsUsed *int `gorm:"column:is_used;size:1;default:1" json:"is_used" validate:"is_used"`
	//标志为失败的检测次数
	FailureThreshold int `gorm:"column:failure_threshold;size:2;default:3" json:"failure_threshold" validate:"failure_threshold"`
	//标志为成功的检测次数
	SuccessThreshold int    `gorm:"column:success_threshold;size:2;default:1" json:"success_threshold" validate:"success_threshold"`
	FailureAction    string `gorm:"column:failure_action;" json:"failure_action" validate:"failure_action"`
}

// FailureActionType  type of failure action.
type FailureActionType string

func (fat FailureActionType) String() string {
	return string(fat)
}

const (
	// IgnoreFailureAction do nothing when the probe result is a failure
	IgnoreFailureAction FailureActionType = "ignore"
	// OfflineFailureAction offline the probe object when the probe result is a failure
	OfflineFailureAction FailureActionType = "readiness"
	// RestartFailureAction restart the probe object when the probe result is a failure
	RestartFailureAction FailureActionType = "liveness"
)

//TableName 表名
func (t *TenantServiceProbe) TableName() string {
	return "tenant_services_probe"
}

// TenantServiceAutoscalerRules -
type TenantServiceAutoscalerRules struct {
	Model
	RuleID      string `gorm:"column:rule_id;unique;size:32"`
	ServiceID   string `gorm:"column:service_id;size:32"`
	Enable      bool   `gorm:"column:enable"`
	XPAType     string `gorm:"column:xpa_type;size:3"`
	MinReplicas int    `gorm:"colume:min_replicas"`
	MaxReplicas int    `gorm:"colume:max_replicas"`
}

// TableName -
func (t *TenantServiceAutoscalerRules) TableName() string {
	return "tenant_services_autoscaler_rules"
}

// TenantServiceAutoscalerRuleMetrics -
type TenantServiceAutoscalerRuleMetrics struct {
	Model
	RuleID            string `gorm:"column:rule_id;size:32;not null"`
	MetricsType       string `gorm:"column:metric_type;not null"`
	MetricsName       string `gorm:"column:metric_name;not null"`
	MetricTargetType  string `gorm:"column:metric_target_type;not null"`
	MetricTargetValue int    `gorm:"column:metric_target_value;not null"`
}

// TableName -
func (t *TenantServiceAutoscalerRuleMetrics) TableName() string {
	return "tenant_services_autoscaler_rule_metrics"
}

// TenantServiceScalingRecords -
type TenantServiceScalingRecords struct {
	Model
	ServiceID   string    `gorm:"column:service_id" json:"-"`
	RuleID      string    `gorm:"column:rule_id" json:"rule_id"`
	EventName   string    `gorm:"column:event_name;not null" json:"record_id"`
	RecordType  string    `gorm:"column:record_type" json:"record_type"`
	Reason      string    `gorm:"column:reason" json:"reason"`
	Count       int32     `gorm:"column:count" json:"count"`
	Description string    `gorm:"column:description;size:1023" json:"description"`
	Operator    string    `gorm:"column:operator" json:"operator"`
	LastTime    time.Time `gorm:"column:last_time" json:"last_time"`
}

// TableName -
func (t *TenantServiceScalingRecords) TableName() string {
	return "tenant_services_scaling_records"
}

// ServiceID -
type ServiceID struct {
	ServiceID string `gorm:"column:service_id" json:"-"`
}
