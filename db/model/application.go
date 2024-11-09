// RAINBOND, Application Management Platform
// Copyright (C) 2020-2022 Goodrain Co., Ltd.

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

const (
	// GovernanceModeBuildInServiceMesh means the governance mode is BUILD_IN_SERVICE_MESH
	GovernanceModeBuildInServiceMesh = "BUILD_IN_SERVICE_MESH"
	// GovernanceModeKubernetesNativeService means the governance mode is KUBERNETES_NATIVE_SERVICE
	GovernanceModeKubernetesNativeService = "KUBERNETES_NATIVE_SERVICE"
	// GovernanceModeIstioServiceMesh means the governance mode is ISTIO_SERVICE_MESH
	GovernanceModeIstioServiceMesh = "ISTIO_SERVICE_MESH"
)

const (
	// GovernanceModeKubernetesNativeServiceDesc -
	GovernanceModeKubernetesNativeServiceDesc = "该模式组件间使用Kubernetes service名称域名进行通信，用户需要配置每个组件端口注册的service名称，治理能力有限"
)

// app type
const (
	AppTypeRainbond = "rainbond"
	AppTypeHelm     = "helm"
)

// Application -
type Application struct {
	Model
	EID             string `gorm:"column:eid" json:"eid"`
	TenantID        string `gorm:"column:tenant_id" json:"tenant_id"`
	AppName         string `gorm:"column:app_name" json:"app_name"`
	AppID           string `gorm:"column:app_id" json:"app_id"`
	AppType         string `gorm:"column:app_type;default:'rainbond'" json:"app_type"`
	AppStoreName    string `gorm:"column:app_store_name" json:"app_store_name"`
	AppStoreURL     string `gorm:"column:app_store_url" json:"app_store_url"`
	AppTemplateName string `gorm:"column:app_template_name" json:"app_template_name"`
	Version         string `gorm:"column:version" json:"version"`
	GovernanceMode  string `gorm:"column:governance_mode;default:'KUBERNETES_NATIVE_SERVICE'" json:"governance_mode"`
	K8sApp          string `gorm:"column:k8s_app" json:"k8s_app"`
}

// TableName return tableName "application"
func (t *Application) TableName() string {
	return "applications"
}

// ConfigGroupService -
type ConfigGroupService struct {
	Model
	AppID           string `gorm:"column:app_id" json:"-"`
	ConfigGroupName string `gorm:"column:config_group_name" json:"-"`
	ServiceID       string `gorm:"column:service_id" json:"service_id"`
	ServiceAlias    string `gorm:"column:service_alias" json:"service_alias"`
}

// TableName return tableName "application"
func (t *ConfigGroupService) TableName() string {
	return "app_config_group_service"
}

// ConfigGroupItem -
type ConfigGroupItem struct {
	Model
	AppID           string `gorm:"column:app_id" json:"-"`
	ConfigGroupName string `gorm:"column:config_group_name" json:"-"`
	ItemKey         string `gorm:"column:item_key" json:"item_key"`
	ItemValue       string `gorm:"column:item_value;type:longtext" json:"item_value"`
}

// TableName return tableName "application"
func (t *ConfigGroupItem) TableName() string {
	return "app_config_group_item"
}

// ApplicationConfigGroup -
type ApplicationConfigGroup struct {
	Model
	AppID           string `gorm:"column:app_id" json:"app_id"`
	ConfigGroupName string `gorm:"column:config_group_name" json:"config_group_name"`
	DeployType      string `gorm:"column:deploy_type;default:'env'" json:"deploy_type"`
	Enable          bool   `gorm:"column:enable" json:"enable"`
}

// TableName return tableName "app_config_group"
func (t *ApplicationConfigGroup) TableName() string {
	return "app_config_group"
}

// K8sResource Save k8s resources under the application
type K8sResource struct {
	Model
	AppID string `gorm:"column:app_id" json:"app_id"`
	Name  string `gorm:"column:name" json:"name"`
	// The resource kind is the same as that in k8s cluster
	Kind string `gorm:"column:kind" json:"kind"`
	// Yaml file for the storage resource
	Content string `gorm:"column:content;type:longtext" json:"content"`
	// resource create error overview
	ErrorOverview string `gorm:"column:status;type:longtext" json:"error_overview"`
	//whether it was created successfully
	State int `gorm:"column:success;type:int" json:"state"`
}

// TableName return tableName "k8s_resources"
func (k *K8sResource) TableName() string {
	return "k8s_resources"
}

// AppGrayRelease -
type AppGrayRelease struct {
	Model
	AppID            string `gorm:"column:app_id" json:"app_id"`
	EntryComponentID string `gorm:"column:entry_component_id" json:"entry_component_id"`
	EntryHTTPRoute   string `gorm:"column:entry_http_route" json:"entry_http_route"`
	FlowEntryRule    string `gorm:"column:flow_entry_rule;type:longtext" json:"flow_entry_rule"`
	GrayStrategyType string `gorm:"column:gray_strategy_type" json:"gray_strategy_type"`
	GrayStrategy     string `gorm:"column:gray_strategy;type:longtext" json:"gray_strategy"`
	Status           bool   `gorm:"column:status" json:"status"`
	TraceType        string `gorm:"column:trace_type" json:"trace_type"`
}

// TableName return tableName "app_gray_release"
func (k *AppGrayRelease) TableName() string {
	return "app_gray_release"
}

// EnterpriseLanguageVersion language model
type EnterpriseLanguageVersion struct {
	Model
	Lang        string `gorm:"column:lang;uniqueIndex:idx_lang_version" json:"lang"`
	Version     string `gorm:"column:version;uniqueIndex:idx_lang_version" json:"version"`
	FirstChoice bool   `gorm:"column:first_choice" json:"first_choice"`
	EventID     string `gorm:"column:event_id" json:"event_id"`
	FileName    string `gorm:"column:file_name" json:"file_name"`
	System      bool   `gorm:"column:system" json:"system"`
	Show        bool   `gorm:"column:is_show" json:"show"`
}

// TableName return tableName "k8s_resources"
func (k *EnterpriseLanguageVersion) TableName() string {
	return "enterprise_language_version"
}
