package cleanup

import (
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// QueueOperation reserves a producer before submitting work to an external queue.
// An uncertain enqueue leaves this record protective; it never expires by age.
func QueueOperation(database *gorm.DB, r CoordinationRequest) (bool, error) {
	if database == nil || r.Kind != "producer" {
		return false, ErrCoordinationChanged
	}
	return acquireOperation(database, r, true)
}

// ClaimQueuedOperation transfers an exact queued reservation into active work.
// Missing returns false for legacy callers; any existing nonqueued record rejects
// replay. An already accepted task may drain while maintenance awaits producers.
func ClaimQueuedOperation(database *gorm.DB, r CoordinationRequest) (bool, error) {
	if database == nil || !r.valid() || r.Kind != "producer" {
		return false, ErrCoordinationChanged
	}
	tx := database.Begin()
	if tx.Error != nil {
		return false, tx.Error
	}
	defer tx.Rollback()
	store, err := lockCleanupStorage(tx, r)
	if err != nil {
		return false, err
	}
	var op model.CleanupOperation
	if err := tx.Where("operation_id = ?", r.OperationID).First(&op).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return false, tx.Commit().Error
		}
		return false, err
	}
	if !r.matches(op) {
		return false, ErrCoordinationChanged
	}
	if op.State == "uncertain" {
		return false, ErrCoordinationUncertain
	}
	if op.State != "queued" || op.ParentOperationID != "" || op.ExternalKey != nil {
		return false, ErrCoordinationChanged
	}
	if store.Mode != "ready" && store.Mode != "collecting" && store.Mode != "draining" {
		return false, ErrCoordinationBusy
	}
	if err := tx.Model(&model.CleanupOperation{}).Where("operation_id = ?", r.OperationID).Update("state", "active").Error; err != nil {
		return false, err
	}
	if err := advanceCleanupRevision(tx, store); err != nil {
		return false, err
	}
	return true, tx.Commit().Error
}

// CancelUnsubmittedOperation only releases an acknowledged reservation when no
// queue call has been attempted. It cannot release active or uncertain work.
func CancelUnsubmittedOperation(database *gorm.DB, r CoordinationRequest) error {
	if database == nil || !r.valid() || r.Kind != "producer" {
		return ErrCoordinationChanged
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
	var op model.CleanupOperation
	if err := tx.Where("operation_id = ?", r.OperationID).First(&op).Error; err != nil {
		return err
	}
	if !r.matches(op) || op.State != "queued" {
		return ErrCoordinationChanged
	}
	if err := tx.Model(&model.CleanupOperation{}).Where("operation_id = ?", r.OperationID).Update("state", "finished").Error; err != nil {
		return err
	}
	if err := advanceCleanupRevision(tx, store); err != nil {
		return err
	}
	return tx.Commit().Error
}
