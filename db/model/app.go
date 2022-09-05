package model

// AppStatus app status
type AppStatus struct {
	EventID     string `gorm:"column:event_id;size:32;primary_key" json:"event_id"`
	Format      string `gorm:"column:format;size:32" json:"format"` // only rainbond-app/docker-compose/slug
	SourceDir   string `gorm:"column:source_dir;size:255" json:"source_dir"`
	Apps        string `gorm:"column:apps;type:text" json:"apps"`
	Status      string `gorm:"column:status;size:32" json:"status"` // only exporting/importing/failed/success/cleaned
	TarFileHref string `gorm:"column:tar_file_href;size:255" json:"tar_file_href"`
	Metadata    string `gorm:"column:metadata;type:text" json:"metadata"`
}

//TableName 表名
func (t *AppStatus) TableName() string {
	return "region_app_status"
}

//AppBackup app backup info
type AppBackup struct {
	Model
	EventID  string `gorm:"column:event_id;size:32;" json:"event_id"`
	BackupID string `gorm:"column:backup_id;size:32;" json:"backup_id"`
	GroupID  string `gorm:"column:group_id;size:32;" json:"group_id"`
	//Status in starting,failed,success,restore
	Status     string `gorm:"column:status;size:32" json:"status"`
	Version    string `gorm:"column:version;size:32" json:"version"`
	SourceDir  string `gorm:"column:source_dir;size:255" json:"source_dir"`
	SourceType string `gorm:"column:source_type;size:255;default:'local'" json:"source_type"`
	BackupMode string `gorm:"column:backup_mode;size:32" json:"backup_mode"`
	BuckupSize int64  `gorm:"column:backup_size;type:bigint" json:"backup_size"`
	Deleted    bool   `gorm:"column:deleted" json:"deleted"`
}

//TableName 表名
func (t *AppBackup) TableName() string {
	return "region_app_backup"
}
