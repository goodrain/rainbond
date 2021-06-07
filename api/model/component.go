package model

import (
	dbmodel "github.com/goodrain/rainbond/db/model"
	"time"
)

type ComponentBase struct {
	// in: body
	// required: true
	ComponentID string `json:"component_id" validate:"component_id"`
	// 服务名称，用于有状态服务DNS
	// in: body
	// required: false
	ComponentName string `json:"component_name" validate:"component_name"`
	// 服务别名
	// in: body
	// required: true
	ComponentAlias string `json:"component_alias" validate:"component_alias"`
	// 服务描述
	// in: body
	// required: false
	Comment string `json:"comment" validate:"comment"`
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
	// 容器GPU
	// in: body
	// required: false
	ContainerGPU int `json:"container_gpu" validate:"container_gpu"`
	// 扩容方式；0:无状态；1:有状态；2:分区(v5.2用于接收组件的类型)
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
	// 最新操作ID
	// in: body
	// required: false
	EventID string `json:"event_id" validate:"event_id"`
	// 镜像来源
	// in: body
	// required: false
	Namespace string `json:"namespace" validate:"namespace"`
	// 更新时间
	// in: body
	// required: false
	UpdateTime time.Time `json:"update_time" validate:"update_time"`
	// 服务创建类型cloud云市服务,assistant云帮服务
	// in: body
	// required: false
	ServiceOrigin string `json:"service_origin" validate:"service_origin"`
	Kind          string `json:"kind" validate:"kind|in:internal,third_party"`
}

// DbModel -
func (c *ComponentBase) DbModel(tenantID, appID string) *dbmodel.TenantServices {
	return &dbmodel.TenantServices{
		TenantID:        tenantID,
		ServiceID:       c.ComponentID,
		ServiceAlias:    c.ComponentAlias,
		ServiceName:     c.ComponentName,
		ServiceType:     c.ExtendMethod,
		Comment:         c.Comment,
		ContainerCPU:    c.ContainerCPU,
		ContainerMemory: c.ContainerMemory,
		ExtendMethod:    c.ExtendMethod,
		Replicas:        c.Replicas,
		DeployVersion:   c.DeployVersion,
		Category:        c.Category,
		EventID:         c.EventID,
		Namespace:       tenantID,
		ServiceOrigin:   c.ServiceOrigin,
		Kind:            c.Kind,
		AppID:           appID,
		UpdateTime:      time.Now(),
	}
}

type TenantComponentRelation struct {
	DependServiceID   string `json:"depend_service_id"`
	DependServiceType string `json:"dep_service_type"`
	DependOrder       int    `json:"dep_order"`
}

func (t *TenantComponentRelation) DbModel(tenantID, componentID string) *dbmodel.TenantServiceRelation {
	return &dbmodel.TenantServiceRelation{
		TenantID:          tenantID,
		ServiceID:         componentID,
		DependServiceID:   t.DependServiceID,
		DependServiceType: t.DependServiceType,
		DependOrder:       t.DependOrder,
	}
}

type ComponentConfigFile struct {
	VolumeName  string `json:"volume_name"`
	FileContent string `json:"filename"`
}

func (c *ComponentConfigFile) DbModel(componentID string) *dbmodel.TenantServiceConfigFile {
	return &dbmodel.TenantServiceConfigFile{
		ServiceID:   componentID,
		VolumeName:  c.VolumeName,
		FileContent: c.FileContent,
	}
}

type VolumeRelation struct {
	DependServiceID string `json:"dep_service_id"`
	VolumePath      string `json:"mnt_name"`
	HostPath        string `json:"mnt_dir"`
	VolumeName      string `json:"volume_name"`
	VolumeType      string `json:"volume_type"`
}

func (v *VolumeRelation) DbModel(tenantID, componentID string) *dbmodel.TenantServiceMountRelation {
	return &dbmodel.TenantServiceMountRelation{
		TenantID:        tenantID,
		ServiceID:       componentID,
		DependServiceID: v.DependServiceID,
		VolumePath:      v.VolumePath,
		HostPath:        v.HostPath,
		VolumeName:      v.VolumeName,
		VolumeType:      v.VolumeType,
	}
}

type ComponentVolume struct {
	Category           string `json:"category"`
	VolumeType         string `json:"volume_type"`
	VolumeName         string `json:"volume_name"`
	HostPath           string `json:"host_path"`
	VolumePath         string `json:"volume_path"`
	IsReadOnly         bool   `json:"is_read_only"`
	VolumeCapacity     int64  `json:"volume_capacity"`
	AccessMode         string `json:"access_mode"`
	SharePolicy        string `json:"share_policy"`
	BackupPolicy       string `json:"backup_policy"`
	ReclaimPolicy      string `json:"reclaim_policy"`
	AllowExpansion     bool   `json:"allow_expansion"`
	VolumeProviderName string `json:"volume_provider_name"`
}

func (v *ComponentVolume) DbModel(componentID string) *dbmodel.TenantServiceVolume {
	return &dbmodel.TenantServiceVolume{
		ServiceID:          componentID,
		Category:           v.Category,
		VolumeType:         v.VolumeType,
		VolumeName:         v.VolumeName,
		HostPath:           v.HostPath,
		VolumePath:         v.VolumePath,
		IsReadOnly:         v.IsReadOnly,
		VolumeCapacity:     v.VolumeCapacity,
		AccessMode:         v.AccessMode,
		SharePolicy:        v.SharePolicy,
		BackupPolicy:       v.BackupPolicy,
		ReclaimPolicy:      v.ReclaimPolicy,
		AllowExpansion:     v.AllowExpansion,
		VolumeProviderName: v.VolumeProviderName,
	}
}

type ComponentLabel struct {
	LabelKey   string `json:"label_key"`
	LabelValue string `json:"label_value"`
}

func (l *ComponentLabel) DbModel(componentID string) *dbmodel.TenantServiceLable {
	return &dbmodel.TenantServiceLable{
		ServiceID:  componentID,
		LabelKey:   l.LabelKey,
		LabelValue: l.LabelValue,
	}
}

// Component All attributes related to the component
type Component struct {
	ComponentBase      ComponentBase                    `json:"component_base"`
	HTTPRules          []AddHTTPRuleStruct              `json:"http_rules"`
	TCPRules           []AddTCPRuleStruct               `json:"tcp_rules"`
	Monitors           []AddServiceMonitorRequestStruct `json:"monitors"`
	Ports              []TenantServicesPort             `json:"ports"`
	Relations          []TenantComponentRelation        `json:"relations"`
	Envs               []AddTenantServiceEnvVar         `json:"envs"`
	Probes             []ServiceProbe                   `json:"probes"`
	AppConfigGroupRels []AppConfigGroupRelations        `json:"app_config_groups"`
	Labels             []ComponentLabel                 `json:"labels"`
	Plugins            []ComponentPlugin                `json:"plugins"`
	AutoScaleRule      AutoScalerRule                   `json:"auto_scale_rule"`
	ConfigFiles        []ComponentConfigFile            `json:"config_files"`
	VolumeRelations    []VolumeRelation                 `json:"volume_relations"`
	Volumes            []ComponentVolume                `json:"volumes"`
}

// SyncComponentReq -
type SyncComponentReq struct {
	Components []*Component `json:"-"`
}
