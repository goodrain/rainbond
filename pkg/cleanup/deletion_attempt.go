package cleanup

import (
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

func changeDeletionAttempt(database *gorm.DB, r CoordinationRequest, observation bool, change func(*model.CleanupOperation) error) error {
	if !r.valid() || r.Kind != "delete" {
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
	if !observation && store.Mode != "ready" && store.Mode != "draining" {
		return ErrCoordinationBusy
	}
	var op model.CleanupOperation
	if err := tx.Where("operation_id = ?", r.OperationID).First(&op).Error; err != nil {
		return err
	}
	if !r.matches(op) || op.NodeExecutionJSON != "" {
		return ErrCoordinationChanged
	}
	if op.State == "uncertain" {
		return ErrCoordinationUncertain
	}
	previous := op.State
	if err := change(&op); err != nil {
		return err
	}
	result := tx.Model(&model.CleanupOperation{}).Where("operation_id = ? AND state = ?", r.OperationID, previous).Updates(map[string]interface{}{"state": op.State, "outcome": op.Outcome})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrCoordinationChanged
	}
	if err := advanceCleanupRevision(tx, store); err != nil {
		return err
	}
	return tx.Commit().Error
}

// BeginDeletionAttempt consumes the original target's single execution grant.
// Once consumed, losing a response never authorizes a second DELETE request.
func BeginDeletionAttempt(database *gorm.DB, r CoordinationRequest) error {
	return changeDeletionAttempt(database, r, false, func(op *model.CleanupOperation) error {
		if op.State == "executing" || op.State == "applied" || op.State == "rejected" {
			return ErrCoordinationUncertain
		}
		if op.State != "active" {
			return ErrCoordinationChanged
		}
		op.State = "executing"
		return nil
	})
}

// CompleteDeletionAttempt records only the transport result. Applied or rejected
// attempts keep the scope until the caller verifies the source and finishes it.
func CompleteDeletionAttempt(database *gorm.DB, r CoordinationRequest, outcome string) error {
	if outcome != "applied" && outcome != "rejected" && outcome != "unknown" {
		return ErrCoordinationChanged
	}
	return changeDeletionAttempt(database, r, true, func(op *model.CleanupOperation) error {
		if op.State != "executing" {
			return ErrCoordinationChanged
		}
		op.State = outcome
		op.Outcome = outcome
		if outcome == "unknown" {
			op.State = "uncertain"
		}
		return nil
	})
}
