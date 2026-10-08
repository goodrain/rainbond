package cleanup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// RegistryCoverage is produced by trusted live inspection, not request fields.
// The inspector must account for every selected ingress and platform writer.
type RegistryCoverage struct {
	Ingress    []ParticipantRegistration
	Writers    []ReferenceWriter
	References RegionReferenceInventory
}

// CertifyRegistry checks current deployment evidence and durable enrollment
// under the storage lock. It neither deletes data nor ignores uncertain writes.
// self can exempt only the exact original deletion during revalidation.
func CertifyRegistry(database *gorm.DB, binding StorageRegistration, self *CoordinationRequest, inspect func([]ParticipantRegistration) (RegistryCoverage, error)) (StorageObservation, bool, error) {
	denied := StorageObservation{}
	fingerprint, err := binding.Fingerprint()
	domain := sha256.Sum256([]byte("registry-filesystem\x00" + binding.VolumeUID))
	if err != nil || database == nil || inspect == nil || binding.StorageID != hex.EncodeToString(domain[:]) {
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
	observed := StorageObservation{StorageID: row.StorageID, Generation: row.Generation, RegistrationFingerprint: fingerprint, Mode: row.Mode, Revision: row.Revision}
	if row.MaintenanceOperationID != "" || (row.Mode != "ready" && row.Mode != "collecting") {
		return observed, false, nil
	}
	finish := func(ready bool) (StorageObservation, bool, error) {
		mode := "collecting"
		if ready {
			mode = "ready"
		}
		if observed.Mode != mode {
			if err := tx.Model(&model.CleanupStorage{}).Where("storage_id = ?", binding.StorageID).Update("mode", mode).Error; err != nil {
				return denied, false, err
			}
			if err := advanceCleanupRevision(tx, row); err != nil {
				return denied, false, err
			}
			observed.Mode = mode
			observed.Revision++
		}
		if err := tx.Commit().Error; err != nil {
			return denied, false, err
		}
		return observed, ready, nil
	}
	var rows []model.CleanupParticipant
	if err := tx.Where("storage_id = ? AND generation = ? AND role = ?", binding.StorageID, binding.Generation, "registry-ingress").Limit(257).Find(&rows).Error; err != nil {
		return denied, false, err
	}
	if len(rows) == 0 || len(rows) > 256 {
		return finish(false)
	}
	records := []ParticipantRegistration{}
	hashes := map[string]string{}
	for _, record := range rows {
		p := ParticipantRegistration{StorageID: record.StorageID, Generation: record.Generation, Owner: record.Owner, Role: record.Role, PodUID: record.PodUID, ContainerID: record.ContainerID, ImageID: record.ImageID, BindingFingerprint: record.BindingFingerprint}
		records = append(records, p)
		hashes[p.Owner] = record.RegistrationHash
	}
	coverage, inspectionErr := inspect(records)
	if inspectionErr != nil || len(coverage.Ingress) == 0 || len(coverage.Ingress) > 32 {
		return finish(false)
	}
	seen := map[string]bool{}
	for _, p := range coverage.Ingress {
		if !p.valid() || p.StorageID != binding.StorageID || p.Generation != binding.Generation || p.Role != "registry-ingress" || p.BindingFingerprint != fingerprint || seen[p.PodUID] {
			return finish(false)
		}
		raw, _ := json.Marshal(p)
		sum := sha256.Sum256(raw)
		if hashes[p.Owner] != hex.EncodeToString(sum[:]) {
			return finish(false)
		}
		seen[p.PodUID] = true
	}
	registered, err := ReferenceWriterCoverageRegistered(tx, coverage.Writers)
	if err != nil {
		return denied, false, err
	}
	if !registered {
		return finish(false)
	}
	references, err := collectRegionReferenceImages(tx)
	if err != nil {
		return denied, false, err
	}
	if !MergeReferenceInventories(references, coverage.References).Complete {
		return finish(false)
	}
	query := tx.Model(&model.CleanupOperation{}).Where("storage_id = ? AND state <> ?", binding.StorageID, "finished")
	if self != nil {
		var original model.CleanupOperation
		if err := tx.Where("operation_id = ?", self.OperationID).First(&original).Error; err != nil {
			return denied, false, err
		}
		if !self.matches(original) || original.State != "active" {
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
	return finish(true)
}
