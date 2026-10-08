package cleanup

import (
	"encoding/json"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// IsValidStorageMeasurement validates observation structure, not its provenance.
func IsValidStorageMeasurement(value StorageMeasurement) bool {
	if value.Protocol != 1 || value.ObservedAt.IsZero() || !coordinationIdentity.MatchString(value.FilesystemID) || value.TotalBytes == 0 || value.FreeBytes > value.TotalBytes || value.AvailableBytes > value.FreeBytes {
		return false
	}
	if (value.TotalInodes == nil) != (value.FreeInodes == nil) {
		return false
	}
	return value.TotalInodes == nil || (*value.TotalInodes > 0 && *value.FreeInodes <= *value.TotalInodes)
}

// RecordMaintenanceMeasurement stores immutable executor observations. It does
// not confirm GC success, grant execution, or release maintenance protection.
func RecordMaintenanceMeasurement(database *gorm.DB, r CoordinationRequest, phase string, value StorageMeasurement) error {
	if !r.valid() || r.Kind != "gc" || (phase != "before" && phase != "after") || !IsValidStorageMeasurement(value) || value.StorageID != r.StorageID || value.Generation != r.Generation {
		return ErrCoordinationChanged
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	tx := database.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()
	store, err := lockCleanupStorage(tx, r)
	if err != nil {
		return err
	}
	if store.RegistrationFingerprint == "" || value.BindingFingerprint != store.RegistrationFingerprint {
		return ErrCoordinationChanged
	}
	var op model.CleanupOperation
	if err := tx.Where("operation_id = ?", r.OperationID).First(&op).Error; err != nil {
		return err
	}
	if !r.matches(op) {
		return ErrCoordinationChanged
	}
	field, existing := "before_measurement", op.BeforeMeasurement
	if phase == "after" {
		field, existing = "after_measurement", op.AfterMeasurement
	}
	if existing != "" {
		if existing != string(raw) {
			return ErrCoordinationChanged
		}
		return tx.Commit().Error
	}
	if store.MaintenanceOperationID != r.OperationID || store.Mode != "maintenance" {
		return ErrCoordinationChanged
	}
	if phase == "before" {
		if op.State != "exclusive" {
			return ErrCoordinationChanged
		}
	} else {
		if op.State != "restore_pending" || (op.Outcome != "succeeded" && op.Outcome != "failed") {
			return ErrCoordinationChanged
		}
		var before StorageMeasurement
		if json.Unmarshal([]byte(op.BeforeMeasurement), &before) != nil || !IsValidStorageMeasurement(before) || before.FilesystemID != value.FilesystemID || before.BindingFingerprint != value.BindingFingerprint || value.ObservedAt.Before(before.ObservedAt) {
			return ErrCoordinationChanged
		}
	}
	if err := tx.Model(&model.CleanupOperation{}).Where("operation_id = ?", r.OperationID).UpdateColumn(field, string(raw)).Error; err != nil {
		return err
	}
	if err := advanceCleanupRevision(tx, store); err != nil {
		return err
	}
	return tx.Commit().Error
}

// MaintenanceMeasurements reads observations without changing execution state.
func MaintenanceMeasurements(database *gorm.DB, r CoordinationRequest) (*StorageMeasurement, *StorageMeasurement, error) {
	if !r.valid() || r.Kind != "gc" {
		return nil, nil, ErrCoordinationChanged
	}
	var op model.CleanupOperation
	if err := database.Where("operation_id = ?", r.OperationID).First(&op).Error; err != nil {
		return nil, nil, err
	}
	if !r.matches(op) {
		return nil, nil, ErrCoordinationChanged
	}
	decode := func(raw string) (*StorageMeasurement, error) {
		if raw == "" {
			return nil, nil
		}
		var value StorageMeasurement
		if json.Unmarshal([]byte(raw), &value) != nil || !IsValidStorageMeasurement(value) || value.StorageID != r.StorageID || value.Generation != r.Generation {
			return nil, ErrCoordinationChanged
		}
		return &value, nil
	}
	before, err := decode(op.BeforeMeasurement)
	if err != nil {
		return nil, nil, err
	}
	after, err := decode(op.AfterMeasurement)
	return before, after, err
}
