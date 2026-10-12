package cleanup

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// CertifyManagedPackageWriters promotes one fixed package root only after the
// current platform rollout proves both reference and upload coordination, no
// legacy upload mutation remains unresolved, and the store has no other work.
func CertifyManagedPackageWriters(database *gorm.DB, binding StorageRegistration, self *CoordinationRequest, inspect func() ([]ReferenceWriter, error)) (StorageObservation, bool, error) {
	var denied StorageObservation
	if database == nil || inspect == nil {
		return denied, false, ErrCoordinationUnavailable
	}
	kind, err := ManagedNodeStorageKind(binding)
	if err != nil || (kind != "upload_events" && kind != "upload_components") {
		return denied, false, ErrCoordinationChanged
	}
	expected := sha256.Sum256([]byte("node-upload-packages\x00" + binding.VolumeUID + "\x00" + kind))
	if binding.StorageID != hex.EncodeToString(expected[:]) {
		return denied, false, ErrCoordinationChanged
	}
	if self != nil && (!self.valid() || self.Kind != "delete" || self.StorageID != binding.StorageID || self.Generation != binding.Generation) {
		return denied, false, ErrCoordinationChanged
	}
	fingerprint, err := binding.Fingerprint()
	if err != nil {
		return denied, false, err
	}
	tx := database.Begin()
	if tx.Error != nil {
		return denied, false, tx.Error
	}
	defer tx.Rollback()
	row, err := lockCleanupStorage(tx, CoordinationRequest{StorageID: binding.StorageID, Generation: binding.Generation})
	if err != nil {
		return denied, false, err
	}
	if row.RegistrationFingerprint != fingerprint {
		return denied, false, ErrCoordinationChanged
	}
	observed := StorageObservation{StorageID: row.StorageID, Generation: row.Generation, RegistrationFingerprint: row.RegistrationFingerprint, Mode: row.Mode, Revision: row.Revision}
	if row.MaintenanceOperationID != "" || (row.Mode != "collecting" && row.Mode != "ready") {
		return observed, false, nil
	}
	setMode := func(mode string) error {
		if observed.Mode == mode {
			return nil
		}
		if err := tx.Model(&model.CleanupStorage{}).Where("storage_id = ?", binding.StorageID).Update("mode", mode).Error; err != nil {
			return err
		}
		if err := advanceCleanupRevision(tx, row); err != nil {
			return err
		}
		observed.Mode = mode
		observed.Revision++
		return nil
	}
	writers, inspectionErr := inspect()
	complete := inspectionErr == nil
	if errors.Is(inspectionErr, ErrCoordinationBusy) {
		complete = false
	} else if inspectionErr != nil {
		complete = false
	}
	if complete {
		complete, err = ReferenceWriterCoverageRegistered(tx, writers)
		if err != nil {
			return denied, false, err
		}
	}
	if complete {
		complete, err = UploadWriterCoverageRegistered(tx, writers)
		if err != nil {
			return denied, false, err
		}
	}
	var unfinishedUploads int64
	if complete {
		if err := tx.Model(&model.PackageUploadUse{}).Where("state <> ?", "finished").Count(&unfinishedUploads).Error; err != nil {
			return denied, false, err
		}
		complete = unfinishedUploads == 0
	}
	if !complete {
		if err := setMode("collecting"); err != nil {
			return denied, false, err
		}
		if err := tx.Commit().Error; err != nil {
			return denied, false, err
		}
		return observed, false, nil
	}
	query := tx.Model(&model.CleanupOperation{}).Where("storage_id = ? AND state <> ?", binding.StorageID, "finished")
	if self != nil {
		var own model.CleanupOperation
		err := tx.Where("operation_id = ?", self.OperationID).First(&own).Error
		if err != nil && !gorm.IsRecordNotFoundError(err) {
			return denied, false, err
		}
		if err == nil && !self.matches(own) {
			return denied, false, ErrCoordinationChanged
		}
		query = query.Where("operation_id <> ?", self.OperationID)
	}
	var unfinished int64
	if err := query.Count(&unfinished).Error; err != nil {
		return denied, false, err
	}
	if unfinished != 0 {
		return observed, false, nil
	}
	if err := setMode("ready"); err != nil {
		return denied, false, err
	}
	if err := tx.Commit().Error; err != nil {
		return denied, false, err
	}
	return observed, true, nil
}
