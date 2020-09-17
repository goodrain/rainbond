package model

// TenantApplication -
type TenantApplication struct {
	Model
	ApplicationName string `gorm:"column:applicationName" json:"applicationName"`
	ApplicationID   int64  `gorm:"column:applicationID" json:"applicationID"`
	TenantID        string `gorm:"column:tenantID;size:32" json:"tenantID"`
}

// TableName return tableName "tenant_application"
func (t *TenantApplication) TableName() string {
	return "tenant_application"
}
