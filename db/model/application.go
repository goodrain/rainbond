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

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

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

const (
	// K8sResourceDeleteStatusActive means the resource has no accepted delete request.
	K8sResourceDeleteStatusActive = iota
	// K8sResourceDeleteStatusDeleting means the Region has accepted a delete request.
	K8sResourceDeleteStatusDeleting
	// K8sResourceDeleteStatusFailed means the latest physical delete attempt failed.
	K8sResourceDeleteStatusFailed
)

const (
	// K8sResourceOriginLegacy is the conservative policy for records created
	// before source ownership was persisted.
	K8sResourceOriginLegacy = "LEGACY"
	// K8sResourceOriginUser is a resource created from the user K8s-resource UI.
	K8sResourceOriginUser = "USER"
	// K8sResourceOriginMarket is a resource installed from an app-market template.
	K8sResourceOriginMarket = "MARKET"
	// K8sResourceOriginImported is a resource created by YAML/Helm import.
	K8sResourceOriginImported = "IMPORTED"
	// K8sResourceOriginGovernance is a platform-owned ServiceMesh resource.
	K8sResourceOriginGovernance = "GOVERNANCE"
	// K8sResourceOriginGateway is a platform-owned Gateway HTTPRoute resource.
	K8sResourceOriginGateway = "GATEWAY"
)

// K8sResourceDeleteTarget identifies a resource for controller-side delete acceptance without carrying YAML.
// ResourceID is the preferred Region record identity. Name and Kind support the legacy compatibility path only.
type K8sResourceDeleteTarget struct {
	ResourceID       uint   `json:"resource_id"`
	Name             string `json:"name"`
	Kind             string `json:"kind"`
	ResourceIdentity string `json:"resource_identity"`
}

// K8sResourcePhysicalIdentity returns a SHA-256 key for the durable Kubernetes
// object address. It keeps the indexed value MySQL-safe under utf8mb4 while
// retaining the full group/resource/namespace/name identity semantics. Kind
// alone is insufficient: CRDs can share a kind and names are only unique inside
// an API resource and namespace.
func K8sResourcePhysicalIdentity(group, resource, namespace, name string) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s\x00%s", group, resource, namespace, name)))
	return hex.EncodeToString(digest[:])
}

// K8sResourceDeleteIdentityAmbiguousError reports legacy name/kind matches that cannot be safely accepted.
type K8sResourceDeleteIdentityAmbiguousError struct {
	AppID       string
	Name        string
	Kind        string
	ResourceIDs []uint
}

func (e *K8sResourceDeleteIdentityAmbiguousError) Error() string {
	return fmt.Sprintf("k8s resource delete identity is ambiguous for app %q, name %q, kind %q: record IDs %v", e.AppID, e.Name, e.Kind, e.ResourceIDs)
}

// K8sResourceDeleteTargetNotFoundError reports a Region delete target that is missing or belongs to another app.
type K8sResourceDeleteTargetNotFoundError struct {
	AppID  string
	Target K8sResourceDeleteTarget
}

func (e *K8sResourceDeleteTargetNotFoundError) Error() string {
	if e.Target.ResourceID > 0 {
		return fmt.Sprintf("k8s resource delete target %d was not found for app %q", e.Target.ResourceID, e.AppID)
	}
	return fmt.Sprintf("k8s resource delete target name %q, kind %q was not found for app %q", e.Target.Name, e.Target.Kind, e.AppID)
}

// K8sResourceCreateBlockedError reports an attempted create/sync whose saved
// resource is still deleting or needs explicit cleanup retry.
type K8sResourceCreateBlockedError struct {
	AppID        string
	Name         string
	Kind         string
	DeleteStatus int
}

func (e *K8sResourceCreateBlockedError) Error() string {
	return fmt.Sprintf("k8s resource %s/%s for app %q is pending cleanup (delete status %d)", e.Kind, e.Name, e.AppID, e.DeleteStatus)
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
	// ResourceUID is captured from a successful Kubernetes create/sync/update.
	// Delete workers use it as a precondition to avoid deleting recreated objects.
	ResourceUID string `gorm:"column:resource_uid;size:255;default:''" json:"resource_uid"`
	// ResourceIdentity is the SHA-256 key of the exact Kubernetes API resource,
	// namespace and name. It is intentionally independent of AppID so cleanup
	// reservations protect a reinstall into another application as well.
	ResourceIdentity string `gorm:"column:resource_identity;size:64;default:'';index:idx_k8s_resource_identity" json:"resource_identity"`
	// ResourceOrigin determines whether finalizer removal can ever be attempted.
	ResourceOrigin string `gorm:"column:resource_origin;size:32;default:'LEGACY'" json:"resource_origin"`
	// ForceFinalizerAllowed makes the approved force policy auditable.
	ForceFinalizerAllowed bool `gorm:"column:force_finalizer_allowed;default:false" json:"force_finalizer_allowed"`
	// resource create error overview
	ErrorOverview string `gorm:"column:status;type:longtext" json:"error_overview"`
	//whether it was created successfully
	State int `gorm:"column:success;type:int" json:"state"`
	// DeleteStatus is independent from State, which continues to describe create/update results.
	DeleteStatus int `gorm:"column:delete_status;default:0;index:idx_k8s_resource_delete_due" json:"delete_status"`
	// DeleteError contains the latest physical deletion failure summary.
	DeleteError string `gorm:"column:delete_error;type:longtext" json:"delete_error"`
	// DeleteAttempts counts physical delete attempts made by the Region worker.
	DeleteAttempts int `gorm:"column:delete_attempts;default:0" json:"delete_attempts"`
	// DeleteStartedAt records when the current deletion lifecycle was accepted.
	DeleteStartedAt *time.Time `gorm:"column:delete_started_at" json:"delete_started_at"`
	// DeleteGeneration changes on every accepted deletion lifecycle for this Region record.
	DeleteGeneration int64 `gorm:"column:delete_generation;default:0" json:"delete_generation"`
	// DeleteNextRetryAt is the earliest time the recovery scanner may requeue this deletion.
	DeleteNextRetryAt *time.Time `gorm:"column:delete_next_retry_at;index:idx_k8s_resource_delete_due" json:"delete_next_retry_at"`
	// DeleteLeaseUntil prevents concurrent workers from processing the same deletion generation.
	DeleteLeaseUntil *time.Time `gorm:"column:delete_lease_until" json:"delete_lease_until"`
	// DeleteLeaseToken identifies the worker that currently owns the deletion lease.
	DeleteLeaseToken string `gorm:"column:delete_lease_token;size:64;default:''" json:"-"`
}

// AllowsForceFinalizerRemoval verifies both the persisted flag and source
// policy. A bad row cannot enable finalizer stripping for governance, gateway,
// or legacy resources.
func (t *K8sResource) AllowsForceFinalizerRemoval() bool {
	if !t.ForceFinalizerAllowed {
		return false
	}
	switch t.ResourceOrigin {
	case K8sResourceOriginUser, K8sResourceOriginMarket, K8sResourceOriginImported:
		return true
	default:
		return false
	}
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
const (
	LongVersionBuildStrategySlug = "slug"
	LongVersionBuildStrategyCNB  = "cnb"
)

type EnterpriseLanguageVersion struct {
	Model
	Lang          string `gorm:"column:lang;uniqueIndex:idx_lang_version" json:"lang"`
	Version       string `gorm:"column:version;uniqueIndex:idx_lang_version" json:"version"`
	BuildStrategy string `gorm:"column:build_strategy;size:32" json:"build_strategy"`
	FirstChoice   bool   `gorm:"column:first_choice" json:"first_choice"`
	EventID       string `gorm:"column:event_id" json:"event_id"`
	FileName      string `gorm:"column:file_name" json:"file_name"`
	System        bool   `gorm:"column:system" json:"system"`
	Show          bool   `gorm:"column:is_show" json:"show"`
	IsAllowed     bool   `gorm:"column:is_allowed" json:"is_allowed"`
}

// TableName return tableName "k8s_resources"
func (k *EnterpriseLanguageVersion) TableName() string {
	return "enterprise_language_version"
}

// EnterpriseOverScore language model
type EnterpriseOverScore struct {
	Model
	OverScoreRate string `gorm:"column:over_score_rate;uniqueIndex:idx_over_score_rate" json:"over_score_rate"`
}

// TableName return tableName "k8s_resources"
func (k *EnterpriseOverScore) TableName() string {
	return "enterprise_over_score"
}
