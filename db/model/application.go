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
