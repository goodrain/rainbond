package model

type AppStatus struct {
	EventID     string `gorm:"column:event_id;size:32;primary_key" json:"event_id"`
	GroupKey    string `gorm:"column:group_key;size:64" json:"group_key"`
	Version     string `gorm:"column:version;size:32" json:"version"`
	Format      string `gorm:"column:format;size:32" json:"format"` // only rainbond-app/docker-compose
	SourceDir   string `gorm:"column:source_dir;size:255" json:"source_dir"`
	Status      string `gorm:"column:status;size:32" json:"status"` // only exporting/importing/failed/success
	TarFile     string `gorm:"column:tar_file;size:255" json:"tar_file"`
	TarFileHref string `gorm:"column:tar_file_href;size:255" json:"tar_file_href"`
	TimeStamp   int    `gorm:"column:timestamp" json:"timestamp"`
}

//TableName 表名
func (t *AppStatus) TableName() string {
	return "app_status"
}
