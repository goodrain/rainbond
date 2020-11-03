package model

// Application -
type Application struct {
	Model
	AppName  string `gorm:"column:app_name" json:"app_name"`
	AppID    string `gorm:"column:app_id" json:"app_id"`
	TenantID string `gorm:"column:tenant_id" json:"tenant_id"`
}

// TableName return tableName "application"
func (t *Application) TableName() string {
	return "application"
}

// ServiceConfigGroup -
type ServiceConfigGroup struct {
	Model
	AppID           string `gorm:"column:app_id" json:"-"`
	ConfigGroupName string `gorm:"column:config_group_name" json:"-"`
	ServiceID       string `gorm:"column:service_id" json:"service_id"`
	ServiceAlias    string `gorm:"column:service_alias" json:"service_alias"`
}

// TableName return tableName "application"
func (t *ServiceConfigGroup) TableName() string {
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
}

// TableName return tableName "application"
func (t *ApplicationConfigGroup) TableName() string {
	return "app_config_group"
}
