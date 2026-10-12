package cleanup

import (
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// RequestMaintenance closes admission before waiting for already accepted
// writers. Only the explicit owner can progress this durable maintenance record.
func RequestMaintenance(database *gorm.DB, r CoordinationRequest) (bool, error) {
	if !r.valid() || r.Kind != "gc" {
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
	var existing model.CleanupOperation
	err = tx.Where("operation_id = ?", r.OperationID).First(&existing).Error
	if err == nil {
		if !r.matches(existing) {
			return false, ErrCoordinationChanged
		}
		if existing.State == "uncertain" {
			return false, ErrCoordinationUncertain
		}
		return false, tx.Commit().Error
	}
	if !gorm.IsRecordNotFoundError(err) {
		return false, err
	}
	if store.Mode != "ready" || store.MaintenanceOperationID != "" {
		return false, ErrCoordinationBusy
	}
	op := model.CleanupOperation{OperationID: r.OperationID, StorageID: r.StorageID, Generation: r.Generation, Owner: r.Owner, Kind: r.Kind, Scope: r.Scope, Fingerprint: r.Fingerprint, State: "draining"}
	if err := tx.Create(&op).Error; err != nil {
		return false, err
	}
	if err := tx.Model(&model.CleanupStorage{}).Where("storage_id = ?", r.StorageID).Updates(map[string]interface{}{"mode": "draining", "previous_mode": store.Mode, "maintenance_operation_id": r.OperationID}).Error; err != nil {
		return false, err
	}
	if err := advanceCleanupRevision(tx, store); err != nil {
		return false, err
	}
	if err := tx.Commit().Error; err != nil {
		return false, err
	}
	return true, nil
}

type maintenanceTransition func(*gorm.DB, *model.CleanupStorage, *model.CleanupOperation) (bool, error)

func changeMaintenance(database *gorm.DB, r CoordinationRequest, change maintenanceTransition) error {
	if !r.valid() || r.Kind != "gc" {
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
	if !r.matches(op) || (op.State != "finished" && store.MaintenanceOperationID != r.OperationID) {
		return ErrCoordinationChanged
	}
	changed, err := change(tx, &store, &op)
	if err != nil {
		return err
	}
	if changed {
		if err := tx.Model(&model.CleanupStorage{}).Where("storage_id = ?", r.StorageID).Updates(map[string]interface{}{"mode": store.Mode, "previous_mode": store.PreviousMode, "maintenance_operation_id": store.MaintenanceOperationID}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.CleanupOperation{}).Where("operation_id = ?", r.OperationID).Updates(map[string]interface{}{"state": op.State, "outcome": op.Outcome}).Error; err != nil {
			return err
		}
		if err := advanceCleanupRevision(tx, store); err != nil {
			return err
		}
	}
	return tx.Commit().Error
}

// EnterMaintenance grants the GC executor a single entry only after all other
// operations have finished. A lost response must be inspected, never replayed.
func EnterMaintenance(database *gorm.DB, r CoordinationRequest) error {
	return changeMaintenance(database, r, func(tx *gorm.DB, store *model.CleanupStorage, op *model.CleanupOperation) (bool, error) {
		if hasGCJobBinding(op) {
			return false, ErrCoordinationChanged
		}
		return enterMaintenanceExclusive(tx, store, op, r)
	})
}

func maintenanceDrained(tx *gorm.DB, store *model.CleanupStorage, op *model.CleanupOperation, r CoordinationRequest) error {
	if op.State == "uncertain" {
		return ErrCoordinationUncertain
	}
	if store.Mode != "draining" || op.State != "draining" {
		return ErrCoordinationChanged
	}
	var outstanding int
	if err := tx.Model(&model.CleanupOperation{}).Where("storage_id = ? AND operation_id <> ? AND state <> ?", r.StorageID, r.OperationID, "finished").Count(&outstanding).Error; err != nil {
		return err
	}
	if outstanding != 0 {
		return ErrCoordinationBusy
	}
	return nil
}

func enterMaintenanceExclusive(tx *gorm.DB, store *model.CleanupStorage, op *model.CleanupOperation, r CoordinationRequest) (bool, error) {
	if err := maintenanceDrained(tx, store, op, r); err != nil {
		return false, err
	}
	store.Mode = "maintenance"
	op.State = "exclusive"
	return true, nil
}

// CompleteMaintenanceWork is called after the owned GC process has exited and
// its outcome is known. Unknown status cannot open the restore path.
func CompleteMaintenanceWork(database *gorm.DB, r CoordinationRequest, outcome string) error {
	if outcome != "succeeded" && outcome != "failed" && outcome != "unknown" {
		return ErrCoordinationChanged
	}
	return changeMaintenance(database, r, func(_ *gorm.DB, store *model.CleanupStorage, op *model.CleanupOperation) (bool, error) {
		if op.Outcome == outcome && (op.State == "restore_pending" || op.State == "uncertain") {
			return false, nil
		}
		if op.State == "uncertain" {
			return false, ErrCoordinationUncertain
		}
		if store.Mode != "maintenance" || op.State != "exclusive" {
			return false, ErrCoordinationChanged
		}
		op.Outcome = outcome
		op.State = "restore_pending"
		if outcome == "unknown" {
			op.State = "uncertain"
			store.Mode = "recovery_required"
		}
		return true, nil
	})
}

// BeginMaintenanceRestore persists restore intent after a known GC outcome.
func BeginMaintenanceRestore(database *gorm.DB, r CoordinationRequest) error {
	return changeMaintenance(database, r, func(_ *gorm.DB, store *model.CleanupStorage, op *model.CleanupOperation) (bool, error) {
		if hasGCJobBinding(op) {
			return false, ErrCoordinationChanged
		}
		if op.State == "uncertain" {
			return false, ErrCoordinationUncertain
		}
		if store.Mode != "maintenance" || op.State != "restore_pending" {
			return false, ErrCoordinationChanged
		}
		store.Mode = "restoring"
		op.State = "restoring"
		return true, nil
	})
}

// FinishMaintenanceRestore must follow verification of the native storage's
// original configuration. Database readiness is not that external verification.
func FinishMaintenanceRestore(database *gorm.DB, r CoordinationRequest, confirmed bool) error {
	return changeMaintenance(database, r, func(_ *gorm.DB, store *model.CleanupStorage, op *model.CleanupOperation) (bool, error) {
		if hasGCJobBinding(op) {
			return false, ErrCoordinationChanged
		}
		if op.State == "finished" && confirmed {
			return false, nil
		}
		if op.State == "uncertain" {
			return false, ErrCoordinationUncertain
		}
		if store.Mode != "restoring" || op.State != "restoring" {
			return false, ErrCoordinationChanged
		}
		if !confirmed {
			store.Mode = "recovery_required"
			op.State = "uncertain"
			return true, nil
		}
		if store.PreviousMode != "ready" {
			return false, ErrCoordinationChanged
		}
		store.Mode = store.PreviousMode
		store.PreviousMode = ""
		store.MaintenanceOperationID = ""
		op.State = "finished"
		return true, nil
	})
}

// CancelMaintenanceDrain is safe only before the GC execution grant. It cannot
// cancel an active GC process or recover an uncertain operation.
func CancelMaintenanceDrain(database *gorm.DB, r CoordinationRequest) error {
	return changeMaintenance(database, r, func(_ *gorm.DB, store *model.CleanupStorage, op *model.CleanupOperation) (bool, error) {
		if op.State == "finished" && op.Outcome == "canceled" {
			return false, nil
		}
		if store.Mode != "draining" || op.State != "draining" || store.PreviousMode != "ready" {
			return false, ErrCoordinationChanged
		}
		store.Mode = store.PreviousMode
		store.PreviousMode = ""
		store.MaintenanceOperationID = ""
		op.State = "finished"
		op.Outcome = "canceled"
		return true, nil
	})
}
