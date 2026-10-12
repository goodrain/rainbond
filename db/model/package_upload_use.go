package model

import "time"

// PackageUploadUse records a native upload request until its storage outcome is known.
// Records survive process restarts; uncertainty never expires automatically.
type PackageUploadUse struct {
	Fingerprint string `gorm:"column:fingerprint;size:64;not null;default:''"`
	OperationID string `gorm:"column:operation_id;size:64;primary_key"`
	SessionID   string `gorm:"column:session_id;size:64;not null;index:idx_package_upload_session_state"`
	EventID     string `gorm:"column:event_id;size:64;not null"`
	Action      string `gorm:"column:action;size:20;not null"`
	State       string `gorm:"column:state;size:20;not null;index:idx_package_upload_session_state"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TableName identifies the persistent upload request ledger.
func (*PackageUploadUse) TableName() string { return "cleanup_package_upload_uses" }
