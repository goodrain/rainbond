package model

import (
	"time"
)

// UploadSession 文件分片上传会话
type UploadSession struct {
	ID             string    `gorm:"column:id;size:64;primary_key" json:"id"`
	EventID        string    `gorm:"column:event_id;size:64;index" json:"event_id"`
	FileName       string    `gorm:"column:file_name;size:255;not null" json:"file_name"`
	FileSize       int64     `gorm:"column:file_size;not null" json:"file_size"`
	FileMD5        string    `gorm:"column:file_md5;size:32" json:"file_md5"`
	ChunkSize      int       `gorm:"column:chunk_size;not null" json:"chunk_size"`
	TotalChunks    int       `gorm:"column:total_chunks;not null" json:"total_chunks"`
	UploadedChunks string    `gorm:"column:uploaded_chunks;type:text" json:"uploaded_chunks"` // 逗号分隔的分片索引
	Status         string    `gorm:"column:status;size:20;not null;index" json:"status"`      // uploading, completed, failed, expired
	StoragePath    string    `gorm:"column:storage_path;size:512" json:"storage_path"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updated_at"`
	ExpiresAt      time.Time `gorm:"column:expires_at;index" json:"expires_at"`
}

// TableName 表名
func (u *UploadSession) TableName() string {
	return "upload_sessions"
}
