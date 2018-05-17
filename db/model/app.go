package model

type AppStatus struct {
	EventID     string `gorm:"column:event_id;size:32;primary_key" json:"event_id"`
	Format      string `gorm:"column:format;size:32" json:"format"` // only rainbond-app/docker-compose
	SourceDir   string `gorm:"column:source_dir;size:255" json:"source_dir"`
	Status      string `gorm:"column:status;size:32" json:"status"` // only exporting/importing/failed/success
	TarFileHref string `gorm:"column:tar_file_href;size:255" json:"tar_file_href"`
	Metadata    string `gorm:"column:metadata" json:"metadata"`
}

//TableName 表名
func (t *AppStatus) TableName() string {
	return "app_status"
}
