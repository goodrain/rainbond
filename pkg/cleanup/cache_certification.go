package cleanup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// CertifyManagedCacheWriters derives readiness from trusted live inspection,
// matching startup records and quiescence under the storage lock. Callers cannot
// submit coverage claims through HTTP. self may exempt only the exact original
// deletion operation during revalidation, never a producer or another task.
func CertifyManagedCacheWriters(database *gorm.DB, binding StorageRegistration, self *CoordinationRequest, inspect func() ([]ParticipantRegistration, error)) (StorageObservation, bool, error) {
	var denied StorageObservation
	fingerprint, err := binding.Fingerprint()
	if err != nil {
		return denied, false, err
	}
	domain := sha256.Sum256([]byte("managed-build-cache\x00" + binding.VolumeUID))
	if binding.RootPath != "/cache/build" || binding.StorageID != hex.EncodeToString(domain[:]) || inspect == nil {
		return denied, false, ErrCoordinationChanged
	}
	if self != nil && (!self.valid() || self.Kind != "delete" || self.StorageID != binding.StorageID || self.Generation != binding.Generation) {
		return denied, false, ErrCoordinationChanged
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
	members, inspectionErr := inspect()
	if errors.Is(inspectionErr, ErrCoordinationBusy) {
		return observed, false, nil
	}
	complete := inspectionErr == nil && len(members) > 0 && len(members) <= 64
	seen := map[string]bool{}
	for _, member := range members {
		if !member.valid() || member.Role != "cache-builder" || member.StorageID != binding.StorageID || member.Generation != binding.Generation || member.BindingFingerprint != fingerprint || seen[member.Owner] {
			complete = false
			break
		}
		seen[member.Owner] = true
		raw, _ := json.Marshal(member)
		sum := sha256.Sum256(raw)
		var recorded model.CleanupParticipant
		err := tx.Where("storage_id = ? AND generation = ? AND role = ? AND owner = ?", binding.StorageID, binding.Generation, "cache-builder", member.Owner).First(&recorded).Error
		if err != nil && !gorm.IsRecordNotFoundError(err) {
			return denied, false, err
		}
		if err != nil || recorded.RegistrationHash != hex.EncodeToString(sum[:]) {
			complete = false
			break
		}
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
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return denied, false, err
	}
	if count != 0 {
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
