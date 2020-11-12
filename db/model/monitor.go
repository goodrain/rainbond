package model

//TenantServiceMonitor custom service monitor
type TenantServiceMonitor struct {
	Model
	TenantID        string `gorm:"column:tenant_id;size:40;primary_key" json:"tenant_id"`
	ServiceID       string `gorm:"column:service_id;size:40" json:"service_id"`
	Name            string `gorm:"column:name;size:40;primary_key" json:"name"`
	ServiceShowName string `gorm:"column:service_show_name" json:"service_show_name"`
	Port            int    `gorm:"column:port;size:5" json:"port"`
	Path            string `gorm:"column:path;size:255" json:"path"`
	Interval        string `gorm:"column:interval;size:20" json:"interval"`
}

// TableName returns table name of TenantServiceMonitor
func (TenantServiceMonitor) TableName() string {
	return "tenant_services_monitor"
}
