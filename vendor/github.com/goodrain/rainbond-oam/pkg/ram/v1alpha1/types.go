// RAINBOND, Application Management Platform
// Copyright (C) 2020-2020 Goodrain Co., Ltd.

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

package v1alpha1

import (
	"encoding/json"
	"fmt"

	"github.com/goodrain/rainbond-oam/pkg/util"
)

//RainbondApplicationConfig store app version templete
type RainbondApplicationConfig struct {
	AppKeyID        string           `json:"group_key"`
	AppName         string           `json:"group_name"`
	AppVersion      string           `json:"group_version"`
	TempleteVersion string           `json:"template_version"`
	Components      []*Component     `json:"apps"`
	Plugins         []Plugin         `json:"plugins,omitempty"`
	AppConfigGroups []AppConfigGroup `json:"app_config_groups"`
}

//HandleNullValue handle null value
func (s *RainbondApplicationConfig) HandleNullValue() {
	if s.TempleteVersion == "" {
		s.TempleteVersion = "v2"
	}
	if s.Plugins == nil {
		s.Plugins = []Plugin{}
	}
	for i := range s.Components {
		s.Components[i].HandleNullValue()
	}
	for i := range s.Plugins {
		s.Plugins[i].HandleNullValue()
	}
}

//Validation validation app templete
func (s *RainbondApplicationConfig) Validation() error {
	if len(s.Components) == 0 {
		return fmt.Errorf("no app in templete")
	}
	for _, app := range s.Components {
		if err := app.Validation(); err != nil {
			return err
		}
	}
	for _, plugin := range s.Plugins {
		if err := plugin.Validation(); err != nil {
			return err
		}
	}
	return nil
}

//JSON return json string
func (s *RainbondApplicationConfig) JSON() string {
	body, _ := json.Marshal(s)
	return string(body)
}

//DeployType deploy type
// TODO update it stateless_multiple, stateless_singleton
type DeployType string

//StatelessSingletionDeployType stateless
var StatelessSingletionDeployType DeployType = "stateless_singleton"

//StatelessMultipleDeployType -
var StatelessMultipleDeployType DeployType = "stateless_multiple"

//StateMultipleDeployType -
var StateMultipleDeployType DeployType = "state_multiple"

//StateSingletonDeployType state
var StateSingletonDeployType DeployType = "state_singleton"

//ServiceType 服务类型
type ServiceType string

//ApplicationServiceType 普通应用
var ApplicationServiceType = "application"

//HelmChartServiceType helm应用
var HelmChartServiceType = "helm-chart"

//ComponentVolumeList volume list
type ComponentVolumeList []ComponentVolume

//Add add volume
func (s *ComponentVolumeList) Add(volume ComponentVolume) {
	for _, v := range *s {
		if v.VolumeName == volume.VolumeName {
			if v.VolumeMountPath == volume.VolumeMountPath {
				return
			}
			volume.VolumeName = volume.VolumeName + util.NewUUID()[24:]
		}
	}
	*s = append(*s, volume)
}

//Component component model
type Component struct {
	// container limit memory, unit MB
	Memory                    int                       `json:"memory"`
	CPU                       int                       `json:"cpu"`
	Probes                    []ComponentProbe          `json:"probes"`
	AppImage                  ImageInfo                 `json:"service_image"`
	ComponentID               string                    `json:"service_id"`
	DeployType                DeployType                `json:"extend_method"`
	ServiceKey                string                    `json:"service_key"`
	ServiceShareID            string                    `json:"service_share_uuid,omitempty"`
	ShareType                 string                    `json:"share_type,omitempty"`
	MntReleationList          []ComponentShareVolume    `json:"mnt_relation_list"`
	ServiceSource             string                    `json:"service_source"`
	DepServiceMapList         []ComponentDep            `json:"dep_service_map_list"`
	ServiceConnectInfoMapList []ComponentEnv            `json:"service_connect_info_map_list"`
	ServiceVolumeMapList      ComponentVolumeList       `json:"service_volume_map_list"`
	Version                   string                    `json:"version"`
	Ports                     []ComponentPort           `json:"port_map_list"`
	ServiceName               string                    `json:"service_name"`
	Category                  string                    `json:"category"`
	Envs                      []ComponentEnv            `json:"service_env_map_list"`
	ServiceAlias              string                    `json:"service_alias"`
	DeployVersion             string                    `json:"deploy_version"`
	ExtendMethodRule          ComponentExtendMethodRule `json:"extend_method_map"`
	ServiceType               string                    `json:"service_type"`
	ServiceCname              string                    `json:"service_cname"`
	ShareImage                string                    `json:"share_image"`
	Image                     string                    `json:"image"`
	Cmd                       string                    `json:"cmd"`
	Language                  string                    `json:"language"`
	ServicePluginConfigs      []ComponentPluginConfig   `json:"service_related_plugin_config,omitempty"`
	ComponentMonitor          []ComponentMonitor        `json:"component_monitor"`
}

//HandleNullValue 处理null值
func (s *Component) HandleNullValue() {
	if s.ServicePluginConfigs == nil {
		s.ServicePluginConfigs = []ComponentPluginConfig{}
	}
	if s.Envs == nil {
		s.Envs = []ComponentEnv{}
	}
	if s.Ports == nil {
		s.Ports = []ComponentPort{}
	}
	if s.ServiceVolumeMapList == nil {
		s.ServiceVolumeMapList = []ComponentVolume{}
	}
	if s.ServiceConnectInfoMapList == nil {
		s.ServiceConnectInfoMapList = []ComponentEnv{}
	}
	if s.DepServiceMapList == nil {
		s.DepServiceMapList = []ComponentDep{}
	}
	if s.MntReleationList == nil {
		s.MntReleationList = []ComponentShareVolume{}
	}
	if s.Probes == nil {
		s.Probes = []ComponentProbe{}
	}
}

//Validation -
func (s *Component) Validation() error {
	return nil
}

//ComponentProbe probe
type ComponentProbe struct {
	ID                 int    `json:"ID" bson:"ID"`
	InitialDelaySecond int    `json:"initial_delay_second" bson:"initial_delay_second"`
	FailureThreshold   int    `json:"failure_threshold" bson:"failure_threshold"`
	ServiceID          string `json:"service_id" bson:"service_id"`
	HTTPHeader         string `json:"http_header" bson:"http_header"`
	Cmd                string `json:"cmd" bson:"cmd"`
	ProbeID            string `json:"probe_id" bson:"probe_id"`
	Scheme             string `json:"scheme" bson:"scheme"`
	SuccessThreshold   int    `json:"success_threshold" bson:"success_threshold"`
	TimeoutSecond      int    `json:"timeout_second" bson:"timeout_second"`
	IsUsed             bool   `json:"is_used" bson:"is_used"`
	PeriodSecond       int    `json:"period_second" bson:"period_second"`
	Port               int    `json:"port" bson:"port"`
	Mode               string `json:"mode" bson:"mode"`
	Path               string `json:"path" bson:"path"`
}

//Validation probe validation
func (s *ComponentProbe) Validation() error {
	if s.Port == 0 && s.Cmd == "" {
		return fmt.Errorf("probe endpoint port is 0")
	}
	return nil
}

//ImageInfo -
type ImageInfo struct {
	HubPassword string `json:"hub_password" bson:"hub_password"`
	Namespace   string `json:"namespace" bson:"namespace"`
	HubURL      string `json:"hub_url" bson:"hub_url"`
	HubUser     string `json:"hub_user" bson:"hub_user"`
	IsTrust     bool   `json:"is_trust" bson:"is_trust"`
}

//ComponentPort port
type ComponentPort struct {
	PortAlias     string `json:"port_alias" bson:"port_alias"`
	Protocol      string `json:"protocol" bson:"protocol"`
	TenantID      string `json:"tenant_id" bson:"tenant_id"`
	ContainerPort int    `json:"container_port" bson:"container_port"`
	IsOuter       bool   `json:"is_outer_service" bson:"is_outer_service"`
	IsInner       bool   `json:"is_inner_service" bson:"is_inner_service"`
}

//ComponentEnv env
type ComponentEnv struct {
	AttrName  string `json:"attr_name" bson:"attr_name"`
	Name      string `json:"name" bson:"name"`
	IsChange  bool   `json:"is_change" bson:"is_change"`
	AttrValue string `json:"attr_value" bson:"attr_value"`
}

//ComponentExtendMethodRule -
//服务伸缩规则，目前仅包含手动伸缩的规则
type ComponentExtendMethodRule struct {
	MinNode    int `json:"min_node" bson:"min_node"`
	StepMemory int `json:"step_memory" bson:"step_memory"`
	IsRestart  int `json:"is_restart" bson:"is_restart"`
	StepNode   int `json:"step_node" bson:"step_node"`
	MaxMemory  int `json:"max_memory" bson:"max_memory"`
	MaxNode    int `json:"max_node" bson:"max_node"`
	MinMemory  int `json:"min_memory" bson:"min_memory"`
}

//DefaultExtendMethodRule default Scaling rules
func DefaultExtendMethodRule() ComponentExtendMethodRule {
	return ComponentExtendMethodRule{
		MinNode:    1,
		MinMemory:  64,
		MaxMemory:  1024 * 64,
		MaxNode:    1024,
		StepMemory: 64,
		StepNode:   1,
	}
}

//Plugin  templete plugin model
type Plugin struct {
	ID            int                 `json:"ID" bson:"id"`
	Origin        string              `json:"origin" bson:"origin"`
	CodeRepo      string              `json:"code_repo" bson:"code_repo"`
	PluginAlias   string              `json:"plugin_alias" bson:"plugin_alias"`
	PluginName    string              `json:"plugin_name" bson:"plugin_name"`
	CreateTime    string              `json:"create_time" bson:"create_time"`
	ShareImage    string              `json:"share_image" bson:"share_image"`
	ConfigGroups  []PluginConfigGroup `json:"config_groups,omitempty" bson:"config_groups"`
	PluginKey     string              `json:"plugin_key" bson:"plugin_key"`
	BuildSource   string              `json:"build_source" bson:"build_source"`
	Desc          string              `json:"desc" bson:"desc"`
	PluginID      string              `json:"plugin_id" bson:"plugin_id"`
	Category      string              `json:"category" bson:"category"`
	OriginShareID string              `json:"origin_share_id" bson:"origin_share_id"`
	Image         string              `json:"image" bson:"image"`
	PluginImage   ImageInfo           `json:"plugin_image" bson:"plugin_image"`
	BuildVersion  string              `json:"build_version" bson:"build_version"`
}

//Validation validation app templete
func (s *Plugin) Validation() error {
	return nil
}

//HandleNullValue 处理null值数据
func (s *Plugin) HandleNullValue() {
	if s.ConfigGroups == nil {
		s.ConfigGroups = []PluginConfigGroup{}
	}
}

//PluginConfigGroup 插件配置定义
type PluginConfigGroup struct {
	ID              int                       `json:"ID" bson:"id"`
	ConfigName      string                    `json:"config_name" bson:"config_name"`
	Options         []PluginConfigGroupOption `json:"options,omitempty" bson:"options"`
	BuildVersion    string                    `json:"build_version" bson:"build_version"`
	PluginID        string                    `json:"plugin_id" bson:"plugin_id"`
	Injection       string                    `json:"injection" bson:"injection"`
	ServiceMetaType string                    `json:"service_meta_type" bson:"service_meta_type"`
}

//PluginConfigGroupOption 插件配置项定义
type PluginConfigGroupOption struct {
	ID               int    `json:"ID" bson:"id"`
	AttrValue        string `json:"attr_alt_value" bson:"attr_alt_value"`
	AttrType         string `json:"attr_type" bson:"attr_type"`
	IsChange         bool   `json:"is_change" bson:"is_change"`
	BuildVersion     string `json:"build_version" bson:"build_version"`
	PluginID         string `json:"plugin_id" bson:"plugin_id"`
	ServiceMetaType  string `json:"service_meta_type" bson:"service_meta_type"`
	AttrDefaultValue string `json:"attr_default_value" bson:"attr_default_value"`
	AttrName         string `json:"attr_name" bson:"attr_name"`
	AttrInfo         string `json:"attr_info" bson:"attr_info"`
	Protocol         string `json:"protocol" bson:"protocol"`
}

//ComponentShareVolume 共享其他服务存储信息
type ComponentShareVolume struct {
	VolumeName       string `json:"mnt_name" bson:"mnt_name"`
	VolumeMountDir   string `json:"mnt_dir" bson:"mnt_dir"`
	ShareServiceUUID string `json:"service_share_uuid" bson:"service_share_uuid"`
}

//ComponentDep 服务依赖关系数据
type ComponentDep struct {
	DepServiceKey string `json:"dep_service_key" bson:"dep_service_key"`
}

//VolumeType volume type
type VolumeType string

//ShareFileVolumeType 共享文件存储
var ShareFileVolumeType VolumeType = "share-file"

//LocalVolumeType 本地文件存储
var LocalVolumeType VolumeType = "local"

//MemoryFSVolumeType 内存文件存储
var MemoryFSVolumeType VolumeType = "memoryfs"

//ConfigFileVolumeType configuration file volume type
var ConfigFileVolumeType VolumeType = "config-file"

func (vt VolumeType) String() string {
	return string(vt)
}

// AccessMode volume access mode
type AccessMode string

// RWOAccessMode write and read only one node
var RWOAccessMode AccessMode = "RWO"

// RWXAccessMode write and read multi node
var RWXAccessMode AccessMode = "RWX"

// ROXAccessMode only read
var ROXAccessMode AccessMode = "ROX"

//ComponentVolume volume config
type ComponentVolume struct {
	VolumeName      string     `json:"volume_name"`
	FileConent      string     `json:"file_content"`
	VolumeMountPath string     `json:"volume_path"`
	VolumeType      VolumeType `json:"volume_type"`
	VolumeCapacity  int        `json:"volume_capacity"`
	AccessMode      AccessMode `json:"access_mode"`
	SharingPolicy   string     `json:"sharing_policy"`
}

//ComponentPluginConfig 服务插件配置数据
type ComponentPluginConfig struct {
	CreateTime      string                   `json:"create_time"`
	PluginStatus    bool                     `json:"plugin_status"`
	ServiceID       string                   `json:"service_id"`
	PluginID        string                   `json:"plugin_id"`
	ServiceMetaType string                   `json:"service_meta_type"`
	MemoryRequired  int                      `json:"memory_required"`
	CPURequired     int                      `json:"cpu_required"`
	Attr            []map[string]interface{} `json:"attr"`
	//插件类型
	PluginKey    string `json:"plugin_key"`
	BuildVersion string `json:"build_version"`
}

//ComponentMonitor component monitor plugin
type ComponentMonitor struct {
	Name            string `json:"name"`
	ServiceShowName string `json:"service_show_name"`
	Port            int    `json:"port"`
	Path            string `json:"path"`
	Interval        string `json:"interval"`
}

//AppConfigGroup app config groups
type AppConfigGroup struct {
	Name          string            `json:"name"`
	InjectionType string            `json:"injection_type"`
	ConfigItems   map[string]string `json:"config_items"`
	ComponentIDs  []string          `json:"component_ids"`
}
