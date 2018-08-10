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
	"net/url"
	"time"

	"fmt"
	"strings"

	dbmodel "github.com/goodrain/rainbond/db/model"
)

//ServiceGetCommon path参数
//swagger:parameters getVolumes getDepVolumes
type ServiceGetCommon struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
}

//ComposerStruct ComposerStruct
// swagger:parameters resolve
type ComposerStruct struct {
	// in : body
	Body struct {
		Lang string `json:"default_runtime" validate:"default_runtime"`
		Data struct {
			JSON struct {
				PlatForm struct {
					PHP string `json:"php" validate:"php"`
				}
			}
			Packages []string `json:"packages" validate:"packages"`
			Lock     struct {
				PlatForm struct {
					PHP string `json:"php" validate:"php"`
				}
			}
		}
	}
}

//CreateServiceStruct serviceCreate struct
// swagger:parameters createService
type CreateServiceStruct struct {
	// in: path
	// required: true
	TenantName string `gorm:"column:tenant_name;size:32" json:"tenant_name" validate:"tenant_name"`
	// in:body
	Body struct {
		// 租户id
		// in: body
		// required: false
		TenantID string `gorm:"column:tenant_id;size:32" json:"tenant_id" validate:"tenant_id"`
		// 应用id
		// in: body
		// required: false
		ServiceID string `gorm:"column:service_id;size:32" json:"service_id" validate:"service_id"`
		// 操作人
		// in: body
		// required: false
		Operator string `json:"operator" validate:"operator"`
		// 应用标签,value
		// in: body
		// required: false
		ServiceLabel string `json:"service_label" validate:"service_label"`
		// 节点标签,格式: v1,v2
		// in: body
		// required: false
		NodeLabel string `json:"node_label" validate:"node_label"`
		// 依赖id, 格式: []struct TenantServiceRelation
		// in: body
		// required: false
		DependIDs []dbmodel.TenantServiceRelation `json:"depend_ids" validate:"depend_ids"`
		// 持久化目录信息, 格式: []struct TenantServiceVolume
		// in: body
		// required: false
		VolumesInfo []dbmodel.TenantServiceVolume `json:"volumes_info" validate:"volumes_info"`
		// 环境变量信息, 格式: []struct TenantServiceEnvVar
		// in: body
		// required: false
		EnvsInfo []dbmodel.TenantServiceEnvVar `json:"envs_info" validate:"envs_info"`
		// 端口信息, 格式: []struct TenantServicesPort
		// in: body
		// required: false
		PortsInfo []dbmodel.TenantServicesPort `json:"ports_info" validate:"ports_info"`
		// 服务key
		// in: body
		// required: false
		ServiceKey string `gorm:"column:service_key;size:32" json:"service_key" validate:"service_key"`
		// 服务别名
		// in: body
		// required: true
		ServiceAlias string `gorm:"column:service_alias;size:30" json:"service_alias" validate:"service_alias"`
		// 服务描述
		// in: body
		// required: false
		Comment string `gorm:"column:comment" json:"comment" validate:"comment"`
		// 服务版本
		// in: body
		// required: false
		ServiceVersion string `gorm:"column:service_version;size:32" json:"service_version" validate:"service_version"`
		// 镜像名称
		// in: body
		// required: false
		ImageName string `gorm:"column:image_name;size:100" json:"image_name" validate:"image_name"`
		// 容器CPU权重
		// in: body
		// required: false
		ContainerCPU int `gorm:"column:container_cpu;default:500" json:"container_cpu" validate:"container_cpu"`
		// 容器最大内存
		// in: body
		// required: false
		ContainerMemory int `gorm:"column:container_memory;default:128" json:"container_memory" validate:"container_memory"`
		// 容器启动命令
		// in: body
		// required: false
		ContainerCMD string `gorm:"column:container_cmd;size:2048" json:"container_cmd" validate:"container_cmd"`
		// 容器环境变量
		// in: body
		// required: false
		ContainerEnv string `gorm:"column:container_env;size:255" json:"container_env" validate:"container_env"`
		// 卷名字
		// in: body
		// required: false
		VolumePath string `gorm:"column:volume_path" json:"volume_path" validate:"volume_path"`
		// 容器挂载目录
		// in: body
		// required: false
		VolumeMountPath string `gorm:"column:volume_mount_path" json:"volume_mount_path" validate:"volume_mount_path"`
		// 宿主机目录
		// in: body
		// required: false
		HostPath string `gorm:"column:host_path" json:"host_path" validate:"host_path"`
		// 扩容方式；0:无状态；1:有状态；2:分区
		// in: body
		// required: false
		ExtendMethod string `gorm:"column:extend_method;default:'stateless';" json:"extend_method" validate:"extend_method"`
		// 节点数
		// in: body
		// required: false
		Replicas int `gorm:"column:replicas;default:1" json:"replicas" validate:"replicas"`
		// 部署版本
		// in: body
		// required: false
		DeployVersion string `gorm:"column:deploy_version" json:"deploy_version" validate:"deploy_version"`
		// 服务分类：application,cache,store
		// in: body
		// required: false
		Category string `gorm:"column:category" json:"category" validate:"category"`
		// 最新操作ID
		// in: body
		// required: false
		EventID string `gorm:"column:event_id" json:"event_id" validate:"event_id"`
		// 服务类型
		// in: body
		// required: false
		ServiceType string `gorm:"column:service_type" json:"service_type" validate:"service_type"`
		// 镜像来源
		// in: body
		// required: false
		Namespace string `gorm:"column:namespace" json:"namespace" validate:"namespace"`
		// 共享类型shared、exclusive
		// in: body
		// required: false
		VolumeType string `gorm:"column:volume_type;default:'shared'" json:"volume_type" validate:"volume_type"`
		// 端口类型，one_outer;dif_protocol;multi_outer
		// in: body
		// required: false
		PortType string `gorm:"column:port_type;default:'multi_outer'" json:"port_type" validate:"port_type"`
		// 更新时间
		// in: body
		// required: false
		UpdateTime time.Time `gorm:"column:update_time" json:"update_time" validate:"update_time"`
		// 服务创建类型cloud云市服务,assistant云帮服务
		// in: body
		// required: false
		ServiceOrigin string `gorm:"column:service_origin;default:'assistant'" json:"service_origin" validate:"service_origin"`
		// 代码来源:gitlab,github
		// in: body
		// required: false
		CodeFrom string `gorm:"column:code_from" json:"code_from" validate:"code_from"`
	}
}

// UpdateServiceStruct service update
// swagger:parameters updateService
type UpdateServiceStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	//in: body
	Body struct {
		// 容器启动命令
		// in: body
		// required: false
		ContainerCMD string `gorm:"column:container_cmd;size:2048" json:"container_cmd" validate:"container_cmd"`
		// 镜像名称
		// in: body
		// required: false
		ImageName string `gorm:"column:image_name;size:100" json:"image_name" validate:"image_name"`
		// 容器最大内存
		// in: body
		// required: false
		ContainerMemory int `gorm:"column:container_memory;default:128" json:"container_memory" validate:"container_memory"`
	}
}

//StartStopStruct start struct
type StartStopStruct struct {
	ServiceID     string
	TenantID      string
	DeployVersion string
	EventID       string
	TaskType      string
}

//LanguageSet set language
type LanguageSet struct {
	ServiceID string `json:"service_id"`
	Language  string `json:"language"`
}

//ServiceStruct service struct
type ServiceStruct struct {
	TenantID string `json:"tenant_id" validate:"tenant_id"`
	// in: path
	// required: true
	ServiceID string `json:"service_id" validate:"service_id"`
	// 服务key
	// in: body
	// required: false
	ServiceKey string `json:"service_key" validate:"service_key"`
	// 服务别名
	// in: body
	// required: true
	ServiceAlias string `json:"service_alias" validate:"service_alias"`
	// 服务描述
	// in: body
	// required: false
	Comment string `json:"comment" validate:"comment"`
	// 服务版本
	// in: body
	// required: false
	ServiceVersion string `json:"service_version" validate:"service_version"`
	// 镜像名称
	// in: body
	// required: false
	ImageName string `json:"image_name" validate:"image_name"`
	// 容器CPU权重
	// in: body
	// required: false
	ContainerCPU int `json:"container_cpu" validate:"container_cpu"`
	// 容器最大内存
	// in: body
	// required: false
	ContainerMemory int `json:"container_memory" validate:"container_memory"`
	// 容器启动命令
	// in: body
	// required: false
	ContainerCMD string `json:"container_cmd" validate:"container_cmd"`
	// 容器环境变量
	// in: body
	// required: false
	ContainerEnv string `json:"container_env" validate:"container_env"`
	// 卷名字
	// in: body
	// required: false
	VolumePath string `json:"volume_path" validate:"volume_path"`
	// 容器挂载目录
	// in: body
	// required: false
	VolumeMountPath string `json:"volume_mount_path" validate:"volume_mount_path"`
	// 宿主机目录
	// in: body
	// required: false
	HostPath string `json:"host_path" validate:"host_path"`
	// 扩容方式；0:无状态；1:有状态；2:分区
	// in: body
	// required: false
	ExtendMethod string `json:"extend_method" validate:"extend_method"`
	// 节点数
	// in: body
	// required: false
	Replicas int `json:"replicas" validate:"replicas"`
	// 部署版本
	// in: body
	// required: false
	DeployVersion string `json:"deploy_version" validate:"deploy_version"`
	// 服务分类：application,cache,store
	// in: body
	// required: false
	Category string `json:"category" validate:"category"`
	// 服务当前状态：undeploy,running,closed,unusual,starting,checking,stoping
	// in: body
	// required: false
	CurStatus string `json:"cur_status" validate:"cur_status"`
	// 最新操作ID
	// in: body
	// required: false
	EventID string `json:"event_id" validate:"event_id"`
	// 服务类型
	// in: body
	// required: false
	ServiceType string `json:"service_type" validate:"service_type"`
	// 镜像来源
	// in: body
	// required: false
	Namespace string `json:"namespace" validate:"namespace"`
	// 共享类型shared、exclusive
	// in: body
	// required: false
	VolumeType string `json:"volume_type" validate:"volume_type"`
	// 端口类型，one_outer;dif_protocol;multi_outer
	// in: body
	// required: false
	PortType string `json:"port_type" validate:"port_type"`
	// 更新时间
	// in: body
	// required: false
	UpdateTime time.Time `json:"update_time" validate:"update_time"`
	// 服务创建类型cloud云市服务,assistant云帮服务
	// in: body
	// required: false
	ServiceOrigin string `json:"service_origin" validate:"service_origin"`
	// 代码来源:gitlab,github
	// in: body
	// required: false
	CodeFrom string `json:"code_from" validate:"code_from"`

	Domain string `json:"domain" validate:"domain"`

	ServiceLabel   string                               `json:"service_label"  validate:"service_label"`
	NodeLabel      string                               `json:"node_label"  validate:"node_label"`
	Operator       string                               `json:"operator"  validate:"operator"`
	RepoURL        string                               `json:"repo_url" validate:"repo_url"`
	DependIDs      []dbmodel.TenantServiceRelation      `json:"depend_ids"`
	VolumesInfo    []dbmodel.TenantServiceVolume        `json:"volumes_info"`
	DepVolumesInfo []dbmodel.TenantServiceMountRelation `json:"dep_volumes_info"`
	EnvsInfo       []dbmodel.TenantServiceEnvVar        `json:"envs_info"`
	PortsInfo      []dbmodel.TenantServicesPort         `json:"ports_info"`
}

//DependService struct for depend service
type DependService struct {
	TenantID       string `json:"tenant_id"`
	ServiceID      string `json:"service_id"`
	DepServiceID   string `json:"dep_service_id"`
	DepServiceType string `json:"dep_service_type"`
	Action         string `json:"action"`
}

//Attr attr
type Attr struct {
	Action    string `json:"action"`
	TenantID  string `json:"tenant_id"`
	ServiceID string `json:"service_id"`
	AttrName  string `json:"env_name"`
	AttrValue string `json:"env_value"`
}

// DeleteServicePort service port
// swagger:parameters deletePort
type DeleteServicePort struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	// 容器端口
	// in: path
	// required: true
	Port int `json:"port"`
}

//TenantResources TenantResources
// swagger:parameters tenantResources
type TenantResources struct {
	// in: body
	Body struct {
		// in: body
		// required: true
		TenantNames []string `json:"tenant_name" validate:"tenant_name"`
	}
}

//ServicesResources ServicesResources
// swagger:parameters serviceResources
type ServicesResources struct {
	// in: body
	Body struct {
		// in: body
		// required: true
		ServiceIDs []string `json:"service_ids" validate:"service_ids"`
	}
}

// CommandResponse api统一返回结构
// swagger:response commandResponse
type CommandResponse struct {
	// in: body
	Body struct {
		//参数验证错误信息
		ValidationError url.Values `json:"validation_error,omitempty"`
		//API错误信息
		Msg string `json:"msg,omitempty"`
		//单资源实体
		Bean interface{} `json:"bean,omitempty"`
		//资源列表
		List interface{} `json:"list,omitempty"`
		//数据集总数
		ListAllNumber int `json:"number,omitempty"`
		//当前页码数
		Page int `json:"page,omitempty"`
	}
}

// ServicePortInnerOrOuter service port
// swagger:parameters PortInnerController PortOuterController
type ServicePortInnerOrOuter struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	// in: path
	// required: true
	Port int `json:"port"`
	//in: body
	Body struct {
		// 操作值 `close` or `open`
		// in: body
		// required: true
		Operation string `json:"operation"  validate:"operation|required|in:open,close"`
	}
}

// ServiceLBPortChange change lb port
// swagger:parameters changelbport
type ServiceLBPortChange struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	// in: path
	// required: true
	Port int `json:"port"`
	//in: body
	Body struct {
		// in: body
		// required: true
		ChangePort int `json:"change_port"  validate:"change_port|required"`
	}
}

//RollbackStruct struct
type RollbackStruct struct {
	TenantID      string `json:"tenant_id"`
	ServiceID     string `json:"service_id"`
	EventID       string `json:"event_id;default:system"`
	Operator      string `json:"operator"`
	DeployVersion string `json:"deploy_version"`
}

//StatusList status list
type StatusList struct {
	TenantID      string     `json:"tenant_id"`
	ServiceID     string     `json:"service_id"`
	ServiceAlias  string     `json:"service_alias"`
	DeployVersion string     `json:"deploy_version"`
	Replicas      int        `json:"replicas"`
	ContainerMem  int        `json:"container_memory"`
	CurStatus     string     `json:"cur_status"`
	ContainerCPU  int        `json:"container_cpu"`
	StatusCN      string     `json:"status_cn"`
	PodList       []PodsList `json:"pod_list"`
}

//PodsList pod list
type PodsList struct {
	PodIP    string `json:"pod_ip"`
	Phase    string `json:"phase"`
	PodName  string `json:"pod_name"`
	NodeName string `json:"node_name"`
}

//StatsInfo stats info
type StatsInfo struct {
	UUID string `json:"uuid"`
	CPU  int    `json:"cpu"`
	MEM  int    `json:"memory"`
}

//TotalStatsInfo total stats info
type TotalStatsInfo struct {
	Data []*StatsInfo `json:"data"`
}

//LicenseInfo license info
type LicenseInfo struct {
	Code       string   `json:"code"`
	Company    string   `json:"company"`
	Node       int      `json:"node"`
	CPU        int      `json:"cpu"`
	MEM        int      `json:"memory"`
	Tenant     int      `json:"tenant"`
	EndTime    string   `json:"end_time"`
	StartTime  string   `json:"start_time"`
	DataCenter int      `json:"data_center"`
	ModuleList []string `json:"module_list"`
}

// AddTenantStruct AddTenantStruct
// swagger:parameters addTenant
type AddTenantStruct struct {
	//in: body
	Body struct {
		// the tenant id
		// in: body
		// required: false
		TenantID string `json:"tenant_id" validate:"tenant_id"`
		// the tenant name
		// in: body
		// required: false
		TenantName string `json:"tenant_name" validate:"tenant_name"`
		// the eid
		// in : body
		// required: false
		Eid   string `json:"eid" validata:"eid"`
		Token string `json:"token" validate:"token"`
	}
}

// ServicesInfoStruct ServicesInfoStruct
// swagger:parameters getServiceInfo
type ServicesInfoStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
}

// SetLanguageStruct SetLanguageStruct
// swagger:parameters setLanguage
type SetLanguageStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	//in: body
	Body struct {
		// the tenant id
		// in: body
		// required: true
		EventID string `json:"event_id"`
		// the language
		// in: body
		// required: true
		Language string `json:"language"`
	}
}

//StartServiceStruct StartServiceStruct
//swagger:parameters startService stopService restartService
type StartServiceStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	//in: body
	Body struct {
		// the tenant id
		// in: body
		// required: false
		EventID string `json:"event_id"`
	}
}

//VerticalServiceStruct VerticalServiceStruct
//swagger:parameters verticalService
type VerticalServiceStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	//in: body
	Body struct {
		// the event id
		// in: body
		// required: false
		EventID string `json:"event_id"`
		// cpu数量
		// in: body
		// required: false
		ContainerCPU int `json:"container_cpu"`
		// 内存大小
		// in: body
		// required: false
		ContainerMemory int `json:"container_memory"`
	}
}

//HorizontalServiceStruct HorizontalServiceStruct
//swagger:parameters horizontalService
type HorizontalServiceStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	//in: body
	Body struct {
		// the event id
		// in: body
		// required: false
		EventID string `json:"event_id"`
		// 伸缩数量
		// in: body
		// required: false
		NodeNUM int `json:"node_num"`
	}
}

//BuildServiceStruct BuildServiceStruct
//swagger:parameters serviceBuild
type BuildServiceStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name" validate:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias" validate:"service_alias"`
	//in: body
	Body struct {
		// the event id
		// in: body
		// required: false
		EventID string `json:"event_id" validate:"event_id"`
		// 变量
		// in: body
		// required: false
		ENVS map[string]string `json:"envs" validate:"envs"`
		// 应用构建类型
		// in: body
		// required: true
		Kind string `json:"kind" validate:"kind|required"`
		// 后续动作, 根据该值进行一键部署，如果不传值，则默认只进行构建
		// in: body
		// required: false
		Action string `json:"action" validate:"action"`
		// 镜像地址
		// in: body
		// required: false
		ImageURL string `json:"image_url" validate:"image_url"`
		// 部署的版本号
		// in: body
		// required: true
		DeployVersion string `json:"deploy_version" validate:"deploy_version|required"`
		// git地址
		// in: body
		// required: false
		RepoURL string `json:"repo_url" validate:"repo_url"`
		// branch 分支信息
		// in: body
		// required: false
		Branch string `json:"branch" validate:"branch"`
		// 操作人员
		// in: body
		// required: false
		Lang         string `json:"lang" validate:"lang"`
		Runtime      string `json:"runtime" validate:"runtime"`
		ServiceType  string `json:"service_type" validate:"service_type"`
		User         string `json:"user" validate:"user"`
		Password     string `json:"password" validate:"password"`
		Operator     string `json:"operator" validate:"operator"`
		TenantName   string `json:"tenant_name"`
		ServiceAlias string `json:"service_alias"`
		//用于云市代码包创建
		SlugInfo struct {
			SlugPath    string `json:"slug_path"`
			FTPHost     string `json:"ftp_host"`
			FTPPort     string `json:"ftp_port"`
			FTPUser     string `json:"ftp_username"`
			FTPPassword string `json:"ftp_password"`
		} `json:"slug_info"`
	}
}

//V1BuildServiceStruct V1BuildServiceStruct
type V1BuildServiceStruct struct {
	// in: path
	// required: true
	ServiceID string `json:"service_id" validate:"service_id"`
	Body      struct {
		ServiceID     string `json:"service_id" validate:"service_id"`
		EventID       string `json:"event_id" validate:"event_id"`
		ENVS          string `json:"envs" validate:"envs"`
		Kind          string `json:"kind" validate:"kind"`
		Action        string `json:"action" validate:"action"`
		ImageURL      string `json:"image_url" validate:"image_url"`
		DeployVersion string `json:"deploy_version" validate:"deploy_version|required"`
		RepoURL       string `json:"repo_url" validate:"repo_url"`
		GitURL        string `json:"gitUrl" validate:"gitUrl"`
		Operator      string `json:"operator" validate:"operator"`
	}
}

//UpgradeServiceStruct UpgradeServiceStruct
//swagger:parameters upgradeService
type UpgradeServiceStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	//in: body
	Body struct {
		// the event id
		// in: body
		// required: false
		EventID string `json:"event_id"`
		// 版本号
		// in: body
		// required: true
		DeployVersion int `json:"deploy_version"`
		// 操作人员
		// in: body
		// required: false
		Operator int `json:"operator"`
	}
}

//StatusServiceStruct StatusServiceStruct
//swagger:parameters serviceStatus
type StatusServiceStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
}

//StatusServiceListStruct StatusServiceListStruct
//swagger:parameters serviceStatuslist
type StatusServiceListStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: body
	// required: true
	Body struct {
		// 需要获取状态的服务ID列表,若不指定，返回租户所有应用的状态
		// in: body
		// required: true
		ServiceIDs []string `json:"service_ids" validate:"service_ids|required"`
	}
}

//AddServiceLabelStruct AddServiceLabelStruct
//swagger:parameters addServiceLabel updateServiceLabel
type AddServiceLabelStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	// in: body
	Body struct {
		// 标签值,格式为"v1"
		// in: bod
		// required: true
		LabelValues string `json:"label_values"`
	}
}

//AddNodeLabelStruct AddNodeLabelStruct
//swagger:parameters addNodeLabel deleteNodeLabel
type AddNodeLabelStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	// in: body
	Body struct {
		// 标签值,格式为"[v1, v2, v3]"
		// in: body
		// required: true
		LabelValues []string `json:"label_values" validate:"label_values|required"`
	}
}

//GetSingleServiceInfoStruct GetSingleServiceInfoStruct
//swagger:parameters getService deleteService
type GetSingleServiceInfoStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
}

//CheckCodeStruct CheckCodeStruct
//swagger:parameters checkCode
type CheckCodeStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: body
	Body struct {
		// git分支详情
		// in: body
		// required: true
		GitURL string `json:"git_url" validate:"git_url|required"`
		// git地址
		// in: body
		// required: true
		URLRepos string `json:"url_repos" validate:"url_repos|required"`
		// 检测类型, "first_check"
		// in: body
		// required: true
		CheckType string `json:"check_type" validate:"check_type|required"`
		// 代码分支
		// in: body
		// required: true
		CodeVersion string `json:"code_version" validate:"code_version|required"`
		// git project id, 0
		// in: body
		// required: true
		GitProjectID int `json:"git_project_id" validate:"git_project_id|required"`
		// git源, "gitlab_manual"
		// in: body
		// required: true
		CodeFrom string `json:"code_from" validate:"code_from|required"`
		// 租户id
		// in: body
		// required: false
		TenantID string `json:"tenant_id" validate:"tenant_id"`
		Action   string `json:"action"`
		// 应用id
		// in: body
		// required: true
		ServiceID string `json:"service_id"`
	}
}

//ServiceCheckStruct 应用检测，支持源码检测，镜像检测，dockerrun检测
//swagger:parameters serviceCheck
type ServiceCheckStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: body
	Body struct {
		//uuid
		// in: body
		CheckUUID string `json:"uuid"`
		//检测来源类型
		// in: body
		// required: true
		SourceType string `json:"source_type" validate:"source_type|required|in:docker-run,docker-compose,sourcecode"`

		// 检测来源定义，
		// 代码： https://github.com/shurcooL/githubql.git master
		// docker-run: docker run --name xxx nginx:latest nginx
		// docker-compose: compose全文
		// in: body
		// required: true
		SourceBody string `json:"source_body" validate:"source_body|required"`
		TenantID   string
		Username   string `json:"username"`
		Password   string `json:"password"`
		EventID    string `json:"event_id"`
	}
}

//GetServiceCheckInfoStruct 获取应用检测信息
//swagger:parameters getServiceCheckInfo
type GetServiceCheckInfoStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	UUID string `json:"uuid"`
}

//CloudShareStruct CloudShareStruct
//swagger:parameters sharecloud
type CloudShareStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: body
	Body struct {
		// 分享类型，app_slug／app_image
		// in: body
		// required: true
		Kind  string `json:"kind" validate:"kind|required|in:app_slug,app_image"`
		Slug  SlugShare
		Image ImageShare
	}
}

//PublicShare share共用结构
type PublicShare struct {
	ServiceKey string         `json:"service_key" validate:"service_key"`
	APPVersion string         `json:"app_version" validate:"app_version"`
	IsOuter    bool           `json:"is_outer" validate:"is_outer"`
	Action     string         `json:"action" validate:"action"`
	ShareID    string         `json:"share_id" validate:"share_id"`
	EventID    string         `json:"event_id" validate:"event_id"`
	Dest       string         `json:"dest" validate:"dest|in:yb,ys"`
	ServiceID  string         `json:"service_id" validate:"service_id"`
	ShareConf  ShareConfItems `json:"share_conf" validate:"share_conf"`
}

//SlugShare Slug 类型
type SlugShare struct {
	PublicShare
	ServiceKey    string `json:"service_key" validate:"service_key"`
	APPVersion    string `json:"app_version" validate:"app_version"`
	DeployVersion string `json:"deploy_version" validate:"deploy_version"`
	TenantID      string `json:"tenant_id" validate:"tenant_id"`
	Dest          string `json:"dest" validate:"dest|in:yb,ys"`
}

//ImageShare image 类型
type ImageShare struct {
	PublicShare
	Image string `json:"image" validate:"image"`
}

//ShareConfItems 分享相关配置
type ShareConfItems struct {
	FTPHost       string `json:"ftp_host" validate:"ftp_host"`
	FTPPort       int    `json:"ftp_port" validate:"ftp_port"`
	FTPUserName   string `json:"ftp_username" valiate:"ftp_username"`
	FTPPassWord   string `json:"ftp_password" validate:"ftp_password"`
	FTPNamespace  string `json:"ftp_namespace" validate:"ftp_namespace"`
	OuterRegistry string `json:"outer_registry" validate:"outer_registry"`
}

//AddDependencyStruct AddDependencyStruct
//swagger:parameters addDependency deleteDependency
type AddDependencyStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	// in: body
	Body struct {
		// 被依赖的应用id
		// in: body
		// required: true
		DepServiceID string `json:"dep_service_id"`
		// 被依赖的应用类型,添加时需要传值, 删除时不需要传值
		// in: body
		// required: false
		DepServiceType string `json:"dep_service_type"`
		// 不明，默认传 1, 可以不传
		// in: body
		// required: false
		DepOrder string `json:"dep_order"`
	}
}

//AddEnvStruct AddEnvStruct
//swagger:parameters addEnv deleteEnv
type AddEnvStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	// in: body
	Body struct {
		// 端口
		// in: body
		// required: false
		ContainerPort int `json:"container_port"`
		// name
		// in: body
		// required: false
		Name string `json:"name"`
		// 变量名称
		// in: body
		// required: true
		AttrName string `json:"env_name"`
		// 变量值, 增加时需要传值, 删除时可以不传
		// in: body
		// required: false
		AttrValue string `json:"env_value"`
		// 是否可以修改
		// in: body
		// required: false
		IsChange bool `json:"is_change"`
		// 应用范围: inner or outer or both
		// in: body
		// required: false
		Scope string `json:"scope"`
	}
}

//RollBackStruct RollBackStruct
//swagger:parameters rollback
type RollBackStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	// in: body
	Body struct {
		// event_id
		// in: body
		// required: false
		EventID string `json:"event_id"`
		// 回滚到的版本号
		// in: body
		// required: true
		DeployVersion string `json:"deploy_version"`
		// 操作人
		// in: body
		// required: false
		Operator string `json:"operator"`
	}
}

//AddProbeStruct AddProbeStruct
//swagger:parameters addProbe updateProbe
type AddProbeStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	// in: body
	Body struct {
		// 探针id
		// in: body
		// required: true
		ProbeID string `json:"probe_id"`
		// mode
		// in: body
		// required: false
		Mode string `json:"mode"`
		// mode
		// in: body
		// required: false
		Scheme string `json:"scheme"`
		// path
		// in: body
		// required: false
		Path string `json:"path"`
		// 端口, 默认为80
		// in: body
		// required: false
		Port int `json:"port"`
		// 运行命令
		// in: body
		// required: false
		Cmd string `json:"cmd"`
		// http请求头,key=value,key2=value2
		// in: body
		// required: false
		HTTPHeader string `json:"http_header"`
		// 初始化等候时间, 默认为1
		// in: body
		// required: false
		InitialDelaySecond int `json:"initial_delay_second"`
		// 检测间隔时间, 默认为3
		// in: body
		// required: false
		PeriodSecond int `json:"period_second"`
		// 检测超时时间, 默认为30
		// in: body
		// required: false
		TimeoutSecond int `json:"timeout_second"`
		// 是否启用
		// in: body
		// required: false
		IsUsed int `json:"is_used"`
		// 标志为失败的检测次数
		// in: body
		// required: false
		FailureThreshold int `json:"failure_threshold"`
		// 标志为成功的检测次数
		// in: body
		// required: false
		SuccessThreshold int `json:"success_threshold"`
	}
}

//DeleteProbeStruct DeleteProbeStruct
//swagger:parameters deleteProbe
type DeleteProbeStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	// in: body
	Body struct {
		// 探针id
		// in: body
		// required: true
		ProbeID string `json:"probe_id"`
	}
}

//PodsStructStruct PodsStructStruct
//swagger:parameters getPodsInfo
type PodsStructStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
}

//Login SSHLoginStruct
//swagger:parameters login
type Login struct {
	// in: body
	Body struct {
		// ip:端口
		// in: body
		// required: true
		HostPort string `json:"hostport"`
		// 登录类型
		// in: body
		// required: true
		LoginType bool `json:"type"`
		// 节点类型
		// in: body
		// required: true
		HostType string `json:"hosttype"`
		// root密码
		// in: body
		// required: false
		RootPwd string `json:"pwd,omitempty"`
	}
}

//Labels LabelsStruct
//swagger:parameters labels
type Labels struct {
	// in: path
	// required: true
	NodeID string `json:"node"`
	// in: body
	Body struct {
		// label值列表
		// in: body
		// required: true
		Labels []string `json:"labels"`
	}
}

//Model 默认字段
type Model struct {
	ID uint
	//CreatedAt time.Time
}

//AddTenantServiceEnvVar  应用环境变量
type AddTenantServiceEnvVar struct {
	Model
	TenantID      string `validate:"tenant_id|between:30,33" json:"tenant_id"`
	ServiceID     string `validate:"service_id|between:30,33" json:"service_id"`
	ContainerPort int    `validate:"container_port|numeric_between:1,65535" json:"container_port"`
	Name          string `validate:"name" json:"name"`
	AttrName      string `validate:"env_name|required" json:"env_name"`
	AttrValue     string `validate:"env_value|required" json:"env_value"`
	IsChange      bool   `validate:"is_change|bool" json:"is_change"`
	Scope         string `validate:"scope|in:outer,inner,both" json:"scope"`
}

//DelTenantServiceEnvVar  应用环境变量
type DelTenantServiceEnvVar struct {
	Model
	TenantID      string `validate:"tenant_id|between:30,33" json:"tenant_id"`
	ServiceID     string `validate:"service_id|between:30,33" json:"service_id"`
	ContainerPort int    `validate:"container_port|numeric_between:1,65535" json:"container_port"`
	Name          string `validate:"name" json:"name"`
	AttrName      string `validate:"env_name|required" json:"env_name"`
	AttrValue     string `validate:"env_value" json:"env_value"`
	IsChange      bool   `validate:"is_change|bool" json:"is_change"`
	Scope         string `validate:"scope|in:outer,inner,both" json:"scope"`
}

//ServicePorts service ports
type ServicePorts struct {
	Port []*TenantServicesPort
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

// AddServicePort service port
// swagger:parameters addPort updatePort
type AddServicePort struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	//in: body
	Body struct {
		//in: body
		ServicePorts
	}
}

type plugin struct {
	// the container port for this serviceport
	// in: body
	// required: true
	ContainerPort int32 `json:"container_port"`
	// the mapping port for this serviceport
	// in: body
	// required: true
	MappingPort int32 `json:"mapping_port"`
	// the protocol for this serviceport
	// in: body
	// required: true
	Protocol string `json:"protocol"`
	// the port alias for this serviceport
	// in: body
	// required: true
	PortAlias string `json:"port_alias"`
	// 是否开启对内服务
	// in: body
	Inner bool `json:"is_inner_service"`
	// 是否开启对外服务
	// in: body
	Outer bool `json:"is_outer_service"`
}

//ServiceProbe 应用探针信息
type ServiceProbe struct {
	Model
	ServiceID string `gorm:"column:service_id;size:32" json:"service_id" validate:"service_id|between:30,33"`
	ProbeID   string `gorm:"column:probe_id;size:32" json:"probe_id" validate:"probe_id|required|between:30,33"`
	Mode      string `gorm:"column:mode;default:'liveness'" json:"mode" validate:"mode"`
	Scheme    string `gorm:"column:scheme;default:'scheme'" json:"scheme" validate:"scheme"`
	Path      string `gorm:"column:path" json:"path" validate:"path"`
	Port      int    `gorm:"column:port;size:5;default:80" json:"port" validate:"port|numeric_between:1,65535"`
	Cmd       string `gorm:"column:cmd;size:150" json:"cmd" validate:"cmd"`
	//http请求头，key=value,key2=value2
	HTTPHeader string `gorm:"column:http_header;size:300" json:"http_header" validate:"http_header"`
	//初始化等候时间
	InitialDelaySecond int `gorm:"column:initial_delay_second;size:2;default:1" json:"initial_delay_second" validate:"initial_delay_second"`
	//检测间隔时间
	PeriodSecond int `gorm:"column:period_second;size:2;default:3" json:"period_second" validate:"period_second"`
	//检测超时时间
	TimeoutSecond int `gorm:"column:timeout_second;size:3;default:30" json:"timeout_second" validate:"timeout_second"`
	//是否启用
	IsUsed int `gorm:"column:is_used;size:1;default:0" json:"is_used" validate:"is_used|in:0,1"`
	//标志为失败的检测次数
	FailureThreshold int `gorm:"column:failure_threshold;size:2;default:3" json:"failure_threshold" validate:"failure_threshold"`
	//标志为成功的检测次数
	SuccessThreshold int `gorm:"column:success_threshold;size:2;default:1" json:"success_threshold" validate:"success_threshold"`
}

//TenantServiceVolume 应用持久化记录
type TenantServiceVolume struct {
	Model
	ServiceID string `gorm:"column:service_id;size:32" json:"service_id" validate:"service_id"`
	//服务类型
	Category   string `gorm:"column:category;size:50" json:"category" validate:"category|required"`
	HostPath   string `gorm:"column:host_path" json:"host_path" validate:"host_path|required"`
	VolumePath string `gorm:"column:volume_path" json:"volume_path" validate:"volume_path|required"`
	IsReadOnly bool   `gorm:"column:is_read_only;default:false" json:"is_read_only" validate:"is_read_only|bool"`
}

// GetSupportProtocols GetSupportProtocols
// swagger:parameters getSupportProtocols
type GetSupportProtocols struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
}

//ServiceShare service share
// swagger:parameters shareService
type ServiceShare struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	//in: body
	Body struct {
		//in: body
		//应用分享Key
		ServiceKey string `json:"service_key" validate:"service_key|required"`
		AppVersion string `json:"app_version" validate:"app_version|required"`
		EventID    string `json:"event_id"`
		ShareUser  string `json:"share_user"`
		ShareScope string `json:"share_scope"`
		ImageInfo  struct {
			HubURL      string `json:"hub_url"`
			HubUser     string `json:"hub_user"`
			HubPassword string `json:"hub_password"`
			Namespace   string `json:"namespace"`
			IsTrust     bool   `json:"is_trust,omitempty" validate:"is_trust"`
		} `json:"image_info,omitempty"`
		SlugInfo struct {
			Namespace   string `json:"namespace"`
			FTPHost     string `json:"ftp_host"`
			FTPPort     string `json:"ftp_port"`
			FTPUser     string `json:"ftp_username"`
			FTPPassword string `json:"ftp_password"`
		} `json:"slug_info,omitempty"`
	}
}

type ExportAppStruct struct {
	SourceDir string `json:"source_dir"`
	Body      struct {
		EventID       string `json:"event_id"`
		GroupKey      string `json:"group_key"` // TODO 考虑去掉
		Version       string `json:"version"`   // TODO 考虑去掉
		Format        string `json:"format"`    // only rainbond-app/docker-compose
		GroupMetadata string `json:"group_metadata"`
	}
}

func BuildMQBodyFrom(app *ExportAppStruct) *MQBody {
	return &MQBody{
		EventID:   app.Body.EventID,
		GroupKey:  app.Body.GroupKey,
		Version:   app.Body.Version,
		Format:    app.Body.Format,
		SourceDir: app.SourceDir,
	}
}

type MQBody struct {
	EventID   string `json:"event_id"`
	GroupKey  string `json:"group_key"`
	Version   string `json:"version"`
	Format    string `json:"format"` // only rainbond-app/docker-compose
	SourceDir string `json:"source_dir"`
}

func NewAppStatusFromExport(app *ExportAppStruct) *dbmodel.AppStatus {
	fields := strings.Split(app.SourceDir, "/")
	tarName := fields[len(fields)-1]
	tarFileHref := fmt.Sprintf("/v2/app/download/%s/%s.tar", app.Body.Format, tarName)
	return &dbmodel.AppStatus{
		Format:      app.Body.Format,
		EventID:     app.Body.EventID,
		SourceDir:   app.SourceDir,
		Status:      "exporting",
		TarFileHref: tarFileHref,
	}
}

type ImportAppStruct struct {
	EventID      string       `json:"event_id"`
	SourceDir    string       `json:"source_dir"`
	Apps         []string     `json:"apps"`
	Format       string       `json:"format"`
	ServiceImage ServiceImage `json:"service_image"`
	ServiceSlug  ServiceSlug  `json:"service_slug"`
}

type ServiceImage struct {
	HubUrl      string `json:"hub_url"`
	HubUser     string `json:"hub_user"`
	HubPassword string `json:"hub_password"`
	NameSpace   string `json:"namespace"`
}

type ServiceSlug struct {
	FtpHost     string `json:"ftp_host"`
	FtpPort     string `json:"ftp_port"`
	FtpUsername string `json:"ftp_username"`
	FtpPassword string `json:"ftp_password"`
	NameSpace   string `json:"namespace"`
}

func NewAppStatusFromImport(app *ImportAppStruct) *dbmodel.AppStatus {
	var apps string
	for _, app := range app.Apps {
		app += ":pending"
		if apps == "" {
			apps += app
		} else {
			apps += "," + app
		}
	}

	return &dbmodel.AppStatus{
		EventID:   app.EventID,
		Format:    app.Format,
		SourceDir: app.SourceDir,
		Apps:      apps,
		Status:    "importing",
	}
}
