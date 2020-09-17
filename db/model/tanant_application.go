package model

// TenantApplication -
type TenantApplication struct {
	Model
	AppID    string `gorm:"column:appID;size:32" json:"appID"`
	TenantID string `gorm:"column:tenantID;size:32" json:"tenantID"`
}

// TableName return tableName "tenant_application"
func (t *TenantApplication) TableName() string {
	return "tenant_application"
}
