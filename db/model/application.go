package model

const (
	// GovernanceModeBuildInServiceMesh means the governance mode is BUILD_IN_SERVICE_MESH
	GovernanceModeBuildInServiceMesh = "BUILD_IN_SERVICE_MESH"
	// GovernanceModeKubernetesNativeService means the governance mode is KUBERNETES_NATIVE_SERVICE
	GovernanceModeKubernetesNativeService = "KUBERNETES_NATIVE_SERVICE"
)

// app type
const (
	AppTypeRainbond = "rainbond"
	AppTypeHelm     = "helm"
)

// IsGovernanceModeValid checks if the governanceMode is valid.
func IsGovernanceModeValid(governanceMode string) bool {
	return governanceMode == GovernanceModeBuildInServiceMesh || governanceMode == GovernanceModeKubernetesNativeService
}

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
	GovernanceMode  string `gorm:"column:governance_mode;default:'BUILD_IN_SERVICE_MESH'" json:"governance_mode"`
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
	ItemValue       string `gorm:"column:item_value" json:"item_value"`
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

// TableName return tableName "application"
func (t *ApplicationConfigGroup) TableName() string {
	return "app_config_group"
}
