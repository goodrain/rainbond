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

// ConfigItem -
type ConfigItem struct {
	Key   string `gorm:"column:key" json:"key"`
	Value string `gorm:"column:value" json:"value"`
}

// ApplicationConfigGroup -
type ApplicationConfigGroup struct {
	Model
	AppID           string        `gorm:"column:app_id" json:"app_id"`
	ConfigGroupName string        `gorm:"column:config_group_name" json:"config_group_name"`
	DeployType      string        `gorm:"column:deploy_type,default:'env'" json:"deploy_type"`
	ServiceIDs      []string      `gorm:"-" json:"service_ids"`
	ConfigItems     []*ConfigItem `gorm:"-" json:"config_items"`
}

// TableName return tableName "application"
func (t *ApplicationConfigGroup) TableName() string {
	return "application_config_group"
}
