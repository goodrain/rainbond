package model

// Application -
type Application struct {
	Model
	AppName  string `gorm:"column:appName" json:"appName"`
	AppID    int64  `gorm:"column:appID" json:"appID"`
	TenantID string `gorm:"column:tenantID;size:32" json:"tenantID"`
}

// TableName return tableName "tenant_application"
func (t *Application) TableName() string {
	return "tenant_application"
}
