package model

import "time"

// CleanupReferenceWriter records a verified runtime's reference-guard protocol.
// It is independent of any storage enrollment and does not grant readiness.
type CleanupReferenceWriter struct {
	ID            string `gorm:"type:varchar(64);primary_key"`
	Namespace     string `gorm:"type:varchar(63);not null;index:idx_cleanup_writer_namespace"`
	PodName       string `gorm:"type:varchar(253);not null"`
	PodUID        string `gorm:"type:varchar(64);not null;index:idx_cleanup_writer_pod"`
	ContainerName string `gorm:"type:varchar(63);not null"`
	ContainerID   string `gorm:"type:varchar(256);not null"`
	ImageID       string `gorm:"type:varchar(512);not null"`
	Role          string `gorm:"type:varchar(32);not null"`
	Protocol      string `gorm:"type:varchar(32);not null"`
	Fingerprint   string `gorm:"type:varchar(64);not null"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// TableName returns the runtime reference-writer evidence table.
func (CleanupReferenceWriter) TableName() string { return "cleanup_reference_writers" }
