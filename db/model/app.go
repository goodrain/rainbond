package model

type AppStatus struct {
	GroupKey  string `gorm:"column:group_key;size:64;primary_key"`
	Version   string `gorm:"column:version;size:32"`
	Format    string `gorm:"column:format;size:32"` // only rainbond-app/docker-compose
	EventID   string `gorm:"column:event_id;size:32"`
	SourceDir string `gorm:"column:source_dir;size:255"`
	Status    string `gorm:"column:status;size:32"` // only exporting/importing/failed/success
	TarFile   string `gorm:"column:tar_file;size:255"`
	TimeStamp int    `gorm:"column:timestamp"`
}

//TableName 表名
func (t *AppStatus) TableName() string {
	return "app_status"
}
