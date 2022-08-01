package model

import (
	"fmt"
	"time"

	dbmodel "github.com/goodrain/rainbond/db/model"
)

// ComponentBase -
type ComponentBase struct {
	// in: body
	// required: true
	ComponentID string `json:"component_id" validate:"required"`
	// 服务名称，用于有状态服务DNS
	// in: body
	// required: false
	ComponentName string `json:"component_name" validate:"component_name"`
	// 服务别名
	// in: body
	// required: true
	ComponentAlias string `json:"component_alias" validate:"required"`
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
	// 服务创建类型cloud云市服务,assistant云帮服务
	// in: body
	// required: false
	ServiceOrigin    string `json:"service_origin" validate:"service_origin"`
	Kind             string `json:"kind" validate:"kind|in:internal,third_party"`
	K8sComponentName string `json:"k8s_component_name" validate:"k8s_component_name"`
}

// DbModel return database model
func (c *ComponentBase) DbModel(tenantID, appID, deployVersion string) *dbmodel.TenantServices {
	return &dbmodel.TenantServices{
		TenantID:         tenantID,
		ServiceID:        c.ComponentID,
		ServiceAlias:     c.ComponentAlias,
		ServiceName:      c.ComponentName,
		ServiceType:      c.ExtendMethod,
		Comment:          c.Comment,
		ContainerCPU:     c.ContainerCPU,
		ContainerMemory:  c.ContainerMemory,
		ContainerGPU:     c.ContainerGPU,
		ExtendMethod:     c.ExtendMethod,
		Replicas:         c.Replicas,
		DeployVersion:    deployVersion,
		Category:         c.Category,
		EventID:          c.EventID,
		Namespace:        tenantID,
		ServiceOrigin:    c.ServiceOrigin,
		Kind:             c.Kind,
		AppID:            appID,
		UpdateTime:       time.Now(),
		K8sComponentName: c.K8sComponentName,
	}
}

// TenantComponentRelation -
type TenantComponentRelation struct {
	DependServiceID   string `json:"dep_service_id"`
	DependServiceType string `json:"dep_service_type"`
	DependOrder       int    `json:"dep_order"`
}

// DbModel return database model
func (t *TenantComponentRelation) DbModel(tenantID, componentID string) *dbmodel.TenantServiceRelation {
	return &dbmodel.TenantServiceRelation{
		TenantID:          tenantID,
		ServiceID:         componentID,
		DependServiceID:   t.DependServiceID,
		DependServiceType: t.DependServiceType,
		DependOrder:       t.DependOrder,
	}
}

// ComponentConfigFile -
type ComponentConfigFile struct {
	VolumeName  string `json:"volume_name"`
	FileContent string `json:"file_content"`
}

// DbModel return database model
func (c *ComponentConfigFile) DbModel(componentID string) *dbmodel.TenantServiceConfigFile {
	return &dbmodel.TenantServiceConfigFile{
		ServiceID:   componentID,
		VolumeName:  c.VolumeName,
		FileContent: c.FileContent,
	}
}

// VolumeRelation -
type VolumeRelation struct {
	MountPath        string `json:"mount_path"`
	DependServiceID  string `json:"dep_service_id"`
	DependVolumeName string `json:"dep_volume_name"`
}

// Key returns the key of VolumeRelation.
func (v *VolumeRelation) Key() string {
	return fmt.Sprintf("%s/%s", v.DependServiceID, v.DependVolumeName)
}

// DbModel return database model
func (v *VolumeRelation) DbModel(tenantID, componentID, hostPath, volumeType string) *dbmodel.TenantServiceMountRelation {
	return &dbmodel.TenantServiceMountRelation{
		TenantID:        tenantID,
		ServiceID:       componentID,
		DependServiceID: v.DependServiceID,
		VolumePath:      v.MountPath,
		HostPath:        hostPath,
		VolumeName:      v.DependVolumeName,
		VolumeType:      volumeType,
	}
}

// ComponentVolume -
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
	Mode               *int32 `json:"mode"`
}

// Key returns the key of ComponentVolume.
func (v *ComponentVolume) Key(componentID string) string {
	return fmt.Sprintf("%s/%s", componentID, v.VolumeName)
}

// DbModel return database model
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
		Mode:               v.Mode,
	}
}

// ComponentLabel -
type ComponentLabel struct {
	LabelKey   string `json:"label_key"`
	LabelValue string `json:"label_value"`
}

// DbModel return database model
func (l *ComponentLabel) DbModel(componentID string) *dbmodel.TenantServiceLable {
	return &dbmodel.TenantServiceLable{
		ServiceID:  componentID,
		LabelKey:   l.LabelKey,
		LabelValue: l.LabelValue,
	}
}

//ComponentEnv  -
type ComponentEnv struct {
	ContainerPort int    `validate:"container_port|numeric_between:1,65535" json:"container_port"`
	Name          string `validate:"name" json:"name"`
	AttrName      string `validate:"attr_name|required" json:"attr_name"`
	AttrValue     string `validate:"attr_value" json:"attr_value"`
	IsChange      bool   `validate:"is_change|bool" json:"is_change"`
	Scope         string `validate:"scope|in:outer,inner,both,build" json:"scope"`
}

// DbModel return database model
func (e *ComponentEnv) DbModel(tenantID, componentID string) *dbmodel.TenantServiceEnvVar {
	return &dbmodel.TenantServiceEnvVar{
		TenantID:      tenantID,
		ServiceID:     componentID,
		Name:          e.Name,
		AttrName:      e.AttrName,
		AttrValue:     e.AttrValue,
		ContainerPort: e.ContainerPort,
		IsChange:      true,
		Scope:         e.Scope,
	}
}

// Component All attributes related to the component
type Component struct {
	ComponentBase          ComponentBase                    `json:"component_base"`
	HTTPRules              []AddHTTPRuleStruct              `json:"http_rules"`
	TCPRules               []AddTCPRuleStruct               `json:"tcp_rules"`
	HTTPRuleConfigs        []HTTPRuleConfig                 `json:"http_rule_configs"`
	Monitors               []AddServiceMonitorRequestStruct `json:"monitors"`
	Ports                  []TenantServicesPort             `json:"ports"`
	Relations              []TenantComponentRelation        `json:"relations"`
	Envs                   []ComponentEnv                   `json:"envs"`
	Probes                 []ServiceProbe                   `json:"probes"`
	AppConfigGroupRels     []AppConfigGroupRelations        `json:"app_config_groups"`
	Labels                 []ComponentLabel                 `json:"labels"`
	Plugins                []ComponentPlugin                `json:"plugins"`
	AutoScaleRule          AutoScalerRule                   `json:"auto_scale_rule"`
	ConfigFiles            []ComponentConfigFile            `json:"config_files"`
	VolumeRelations        []VolumeRelation                 `json:"volume_relations"`
	Volumes                []ComponentVolume                `json:"volumes"`
	Endpoint               *Endpoints                       `json:"endpoint"`
	ComponentK8sAttributes []ComponentK8sAttribute          `json:"component_k8s_attributes"`
}

// SyncComponentReq -
type SyncComponentReq struct {
	Components         []*Component `json:"components"`
	DeleteComponentIDs []string     `json:"delete_component_ids"`
}

// ComponentK8sAttribute -
type ComponentK8sAttribute struct {
	// Name Define the attribute name, which is currently supported
	// [nodeSelector/labels/tolerations/volumes/serviceAccountName/privileged/affinity/volumeMounts]
	// The field name should be the same as that in the K8s resource yaml file.
	Name string `json:"name"`

	// The field type defines how the attribute is stored. Currently, `json/yaml/string` are supported
	SaveType string `json:"save_type"`

	// Define the attribute value, which is stored in the database.
	// The value is stored in the database in the form of `json/yaml/string`.
	AttributeValue string `json:"attribute_value"`
}

// DbModel return database model
func (k *ComponentK8sAttribute) DbModel(tenantID, componentID string) *dbmodel.ComponentK8sAttributes {
	return &dbmodel.ComponentK8sAttributes{
		TenantID:       tenantID,
		ComponentID:    componentID,
		Name:           k.Name,
		SaveType:       k.SaveType,
		AttributeValue: k.AttributeValue,
	}
}

// DeleteK8sAttributeReq -
type DeleteK8sAttributeReq struct {
	Name string `json:"name"`
}
