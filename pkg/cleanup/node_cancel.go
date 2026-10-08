package cleanup

import (
	"encoding/json"
	"time"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// CancelNodeBeforeGrant ends only an original cache operation for which Core
// never issued native permission. The storage transaction serializes against
// EnterNodeExecution, so an in-flight or lost native grant cannot be canceled.
func CancelNodeBeforeGrant(database *gorm.DB, r CoordinationRequest, intent NodeJobIntent) error {
	if !r.valid() || r.Kind != "delete" || (intent.Name != "" && intent.Name != nodeJobName(r)) {
		return ErrCoordinationChanged
	}
	intent.Name = nodeJobName(r)
	tx := database.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()
	store, err := lockCleanupStorage(tx, r)
	if err != nil {
		return err
	}
	storage, err := StorageBinding(tx, r.StorageID, r.Generation)
	if err != nil {
		return err
	}
	expected, err := ManagedNodeRequest(storage, r.Owner, r.OperationID, r.Fingerprint, intent)
	if err != nil || expected != r {
		return ErrCoordinationChanged
	}
	var op model.CleanupOperation
	err = tx.Where("operation_id = ?", r.OperationID).First(&op).Error
	missing := gorm.IsRecordNotFoundError(err)
	if err != nil && !missing {
		return err
	}
	if missing {
		// An immutable canceled receipt also fences a late original Acquire request.
		// This is a tombstone, not an admission or permission to create an executor.
		op = model.CleanupOperation{OperationID: r.OperationID, StorageID: r.StorageID, Generation: r.Generation, Owner: r.Owner, Kind: r.Kind, Scope: r.Scope, Target: r.Target, Fingerprint: r.Fingerprint, State: "active"}
	} else if !r.matches(op) {
		return ErrCoordinationChanged
	}
	binding := NodeJobBinding{Protocol: 1, NodeJobIntent: intent}
	if op.NodeExecutionJSON != "" {
		binding, err = readNodeBinding(r, op)
		if err != nil {
			return err
		}
		if binding.Namespace != intent.Namespace || binding.NodeName != intent.NodeName || binding.NodeUID != intent.NodeUID || binding.Entry != intent.Entry || binding.Fingerprint != intent.Fingerprint {
			return ErrCoordinationChanged
		}
	}
	if binding.CanceledBeforeGrantAt != nil {
		return tx.Commit().Error
	}
	if op.State != "active" || op.ExternalKey != nil || op.ParentOperationID != "" || binding.PodUID != "" || binding.PodName != "" || binding.ContainerID != "" || binding.ImageID != "" || binding.Result != nil || binding.FinishedAt != nil {
		return ErrCoordinationUncertain
	}
	now := time.Now().UTC()
	binding.CanceledBeforeGrantAt = &now
	raw, err := json.Marshal(binding)
	if err != nil {
		return err
	}
	if missing {
		op.NodeExecutionJSON = string(raw)
		op.State = "finished"
		op.Outcome = "canceled"
		if err := tx.Create(&op).Error; err != nil {
			return err
		}
	} else if err := tx.Model(&model.CleanupOperation{}).Where("operation_id = ?", r.OperationID).Updates(map[string]interface{}{"node_execution_json": string(raw), "state": "finished", "outcome": "canceled"}).Error; err != nil {
		return err
	}
	if err := advanceCleanupRevision(tx, store); err != nil {
		return err
	}
	return tx.Commit().Error
}

func validCanceledNode(binding NodeJobBinding) bool {
	return binding.CanceledBeforeGrantAt != nil && !binding.CanceledBeforeGrantAt.IsZero() && binding.PodName == "" && binding.PodUID == "" && binding.ContainerID == "" && binding.ImageID == "" && binding.Result == nil && binding.FinishedAt == nil
}
