package cleanup

import (
	"strings"
	"time"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

var retiredNativeProducerKinds = map[string]bool{
	"backup_apps_restore": true,
	"image":               true,
	"image-share":         true,
	"import_app":          true,
	"plugin-dockerfile":   true,
	"plugin-image":        true,
	"service-check":       true,
	"share-plugin":        true,
	"source":              true,
	"tar-image":           true,
	"vm":                  true,
}

func reconcilableProducerOwner(owner string) bool {
	if strings.HasPrefix(owner, "registry-coordinator:") {
		return coordinationIdentity.MatchString(strings.TrimPrefix(owner, "registry-coordinator:"))
	}
	if strings.HasPrefix(owner, "native-") && strings.HasSuffix(owner, "-builder") {
		kind := strings.TrimSuffix(strings.TrimPrefix(owner, "native-"), "-builder")
		return retiredNativeProducerKinds[kind]
	}
	switch owner {
	case "api-workload", "helm-install", "helm-rollback", "helm-uninstall", "helm-upgrade",
		"api-package-cancel", "api-package-chunk", "api-package-complete", "api-package-expire":
		return true
	default:
		return false
	}
}

func earlierEpoch(current, candidate time.Time) time.Time {
	if current.IsZero() || candidate.Before(current) {
		return candidate
	}
	return current
}

// reconcileRetiredProducerEpoch releases only producer records that predate
// every currently verified Registry ingress and platform writer instance. The
// caller has already re-read Registry and Region references under the storage
// lock, so a committed old write is included in the new protection graph. A
// current or unknown producer, deletion, GC, or incomplete runtime epoch keeps
// the entire store protected.
func reconcileRetiredProducerEpoch(tx *gorm.DB, binding StorageRegistration, coverage RegistryCoverage, store *model.CleanupStorage) (bool, error) {
	if tx == nil || store == nil || store.StorageID != binding.StorageID || store.Generation != binding.Generation || len(coverage.Ingress) == 0 || len(coverage.Writers) == 0 {
		return false, ErrCoordinationChanged
	}
	cutoff := time.Time{}
	for _, participant := range coverage.Ingress {
		id, fingerprint, err := participantIdentity(participant)
		if err != nil || participant.StorageID != binding.StorageID || participant.Generation != binding.Generation {
			return false, ErrCoordinationChanged
		}
		var row model.CleanupParticipant
		if err := tx.Where("id = ?", id).First(&row).Error; err != nil {
			return false, err
		}
		if row.RegistrationHash != fingerprint || row.CreatedAt.IsZero() {
			return false, ErrCoordinationChanged
		}
		cutoff = earlierEpoch(cutoff, row.CreatedAt)
	}
	for _, writer := range coverage.Writers {
		id, fingerprint, err := writer.identity()
		if err != nil {
			return false, err
		}
		var row model.CleanupReferenceWriter
		if err := tx.Where("id = ?", id).First(&row).Error; err != nil {
			return false, err
		}
		if row.Fingerprint != fingerprint || row.CreatedAt.IsZero() {
			return false, ErrCoordinationChanged
		}
		cutoff = earlierEpoch(cutoff, row.CreatedAt)
	}
	if cutoff.IsZero() {
		return false, ErrCoordinationChanged
	}
	var operations []model.CleanupOperation
	if err := tx.Where("storage_id = ? AND state <> ?", binding.StorageID, "finished").Limit(4097).Find(&operations).Error; err != nil {
		return false, err
	}
	if len(operations) == 0 {
		return false, nil
	}
	if len(operations) > 4096 {
		return false, ErrCoordinationBusy
	}
	ids := make([]string, 0, len(operations))
	for _, operation := range operations {
		if operation.Generation != binding.Generation {
			return false, ErrCoordinationChanged
		}
		if operation.Kind != "producer" || !reconcilableProducerOwner(operation.Owner) || operation.UpdatedAt.IsZero() || !operation.UpdatedAt.Before(cutoff) {
			return false, nil
		}
		ids = append(ids, operation.OperationID)
	}
	result := tx.Model(&model.CleanupOperation{}).Where("operation_id IN (?) AND state <> ?", ids, "finished").Updates(map[string]interface{}{"state": "finished", "outcome": "reconciled"})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected != int64(len(ids)) {
		return false, ErrCoordinationChanged
	}
	if err := advanceCleanupRevision(tx, *store); err != nil {
		return false, err
	}
	store.Revision++
	return true, nil
}
