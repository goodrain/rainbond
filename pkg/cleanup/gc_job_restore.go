package cleanup

import (
	"encoding/json"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

func verifiedGCJobOutcome(store *model.CleanupStorage, op *model.CleanupOperation, r CoordinationRequest, jobUID, podUID string) bool {
	if !validGCJobBinding(r, op) || jobUID == "" || podUID == "" || op.GCJobUID != jobUID || op.GCPodUID != podUID || op.GCPodName == "" || (op.Outcome != "succeeded" && op.Outcome != "failed") {
		return false
	}
	var before, after StorageMeasurement
	if json.Unmarshal([]byte(op.BeforeMeasurement), &before) != nil || json.Unmarshal([]byte(op.AfterMeasurement), &after) != nil {
		return false
	}
	if !IsValidStorageMeasurement(before) || !IsValidStorageMeasurement(after) || before.StorageID != r.StorageID || after.StorageID != r.StorageID || before.Generation != r.Generation || after.Generation != r.Generation || store.RegistrationFingerprint == "" || before.BindingFingerprint != store.RegistrationFingerprint || after.BindingFingerprint != store.RegistrationFingerprint || before.FilesystemID != after.FilesystemID || after.ObservedAt.Before(before.ObservedAt) {
		return false
	}
	return true
}

// BeginGCJobRestore persists intent only after the trusted controller verifies
// the original executor terminated and the Registry source remains unchanged.
func BeginGCJobRestore(database *gorm.DB, r CoordinationRequest, jobUID, podUID string) error {
	return changeMaintenance(database, r, func(_ *gorm.DB, store *model.CleanupStorage, op *model.CleanupOperation) (bool, error) {
		if !verifiedGCJobOutcome(store, op, r, jobUID, podUID) {
			return false, ErrCoordinationChanged
		}
		if op.State == "finished" {
			return false, nil
		}
		if store.Mode == "restoring" && op.State == "restoring" {
			return false, nil
		}
		if store.Mode != "maintenance" || op.State != "restore_pending" || store.PreviousMode != "ready" {
			return false, ErrCoordinationChanged
		}
		store.Mode = "restoring"
		op.State = "restoring"
		return true, nil
	})
}

// FinishGCJobRestore requires a fresh controller verification. It restores only
// this operation's original admission mode, never a caller-supplied write flag.
func FinishGCJobRestore(database *gorm.DB, r CoordinationRequest, jobUID, podUID string) error {
	return changeMaintenance(database, r, func(_ *gorm.DB, store *model.CleanupStorage, op *model.CleanupOperation) (bool, error) {
		if !verifiedGCJobOutcome(store, op, r, jobUID, podUID) {
			return false, ErrCoordinationChanged
		}
		if op.State == "finished" {
			return false, nil
		}
		if store.Mode != "restoring" || op.State != "restoring" || store.PreviousMode != "ready" {
			return false, ErrCoordinationChanged
		}
		store.Mode = store.PreviousMode
		store.PreviousMode = ""
		store.MaintenanceOperationID = ""
		op.State = "finished"
		return true, nil
	})
}
