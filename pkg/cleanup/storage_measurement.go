package cleanup

import "time"

// StorageMeasurement describes the backing filesystem of a verified Registry
// root. Changes on a shared filesystem are not necessarily attributable to GC.
type StorageMeasurement struct {
	Protocol           int       `json:"protocol"`
	StorageID          string    `json:"storage_id"`
	Generation         string    `json:"generation"`
	BindingFingerprint string    `json:"binding_fingerprint"`
	FilesystemID       string    `json:"filesystem_id"`
	ObservedAt         time.Time `json:"observed_at"`
	TotalBytes         uint64    `json:"total_bytes"`
	FreeBytes          uint64    `json:"free_bytes"`
	AvailableBytes     uint64    `json:"available_bytes"`
	TotalInodes        *uint64   `json:"total_inodes"`
	FreeInodes         *uint64   `json:"free_inodes"`
}
