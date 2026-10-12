package cleanup

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

var registryUploadID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._=-]{0,255}$`)

func registryUploadKey(storage, generation, repository, id string) (string, error) {
	check := CoordinationRequest{StorageID: storage, Generation: generation, OperationID: "lookup", Owner: "lookup", Kind: "producer", Scope: repository, Fingerprint: "lookup"}
	if !check.valid() || repository == "*" || !registryUploadID.MatchString(id) {
		return "", ErrCoordinationChanged
	}
	sum := sha256.Sum256([]byte("registry-upload\x00" + storage + "\x00" + generation + "\x00" + repository + "\x00" + id))
	return hex.EncodeToString(sum[:]), nil
}
func operationRequest(op model.CleanupOperation) CoordinationRequest {
	return CoordinationRequest{StorageID: op.StorageID, Generation: op.Generation, Owner: op.Owner, OperationID: op.OperationID, Kind: op.Kind, Scope: op.Scope, Fingerprint: op.Fingerprint, Target: op.Target}
}
func uploadParent(tx *gorm.DB, r CoordinationRequest) (model.CleanupOperation, error) {
	var op model.CleanupOperation
	if err := tx.Where("operation_id = ?", r.OperationID).First(&op).Error; err != nil {
		return op, err
	}
	if !r.matches(op) || r.Kind != "producer" || op.ParentOperationID != "" {
		return op, ErrCoordinationChanged
	}
	return op, nil
}

// BindRegistryUpload records the actual upload ID before it is delivered to a
// Registry client. Ordinary HTTP completion must not release this parent lease.
func BindRegistryUpload(database *gorm.DB, r CoordinationRequest, repository, id string) error {
	if !r.valid() || r.Kind != "producer" || (r.Scope != "*" && r.Scope != repository) {
		return ErrCoordinationChanged
	}
	key, err := registryUploadKey(r.StorageID, r.Generation, repository, id)
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
	if store.Mode != "ready" && store.Mode != "draining" && store.Mode != "collecting" {
		return ErrCoordinationBusy
	}
	op, err := uploadParent(tx, r)
	if err != nil {
		return err
	}
	if op.State == "uncertain" {
		return ErrCoordinationUncertain
	}
	if op.State != "active" {
		return ErrCoordinationChanged
	}
	if op.ExternalKey != nil {
		if *op.ExternalKey != key || op.ExternalScope != repository || op.ExternalID != id {
			return ErrCoordinationChanged
		}
		return tx.Commit().Error
	}
	if err := tx.Model(&model.CleanupOperation{}).Where("operation_id = ?", r.OperationID).Updates(map[string]interface{}{"external_key": key, "external_scope": repository, "external_id": id}).Error; err != nil {
		return err
	}
	if err := advanceCleanupRevision(tx, store); err != nil {
		return err
	}
	return tx.Commit().Error
}

// LookupRegistryUpload resolves the persistent identity, including after a
// sidecar restart. It does not grant permission to resume an uncertain upload.
func LookupRegistryUpload(database *gorm.DB, storage, generation, repository, id string) (CoordinationRequest, error) {
	key, err := registryUploadKey(storage, generation, repository, id)
	if err != nil {
		return CoordinationRequest{}, err
	}
	tx := database.Begin()
	if tx.Error != nil {
		return CoordinationRequest{}, tx.Error
	}
	defer tx.Rollback()
	if _, err := lockCleanupStorage(tx, CoordinationRequest{StorageID: storage, Generation: generation}); err != nil {
		return CoordinationRequest{}, err
	}
	var op model.CleanupOperation
	if err := tx.Where("external_key = ?", key).First(&op).Error; err != nil {
		return CoordinationRequest{}, err
	}
	if op.StorageID != storage || op.Generation != generation || op.ExternalScope != repository || op.ExternalID != id || op.ParentOperationID != "" || op.Kind != "producer" {
		return CoordinationRequest{}, ErrCoordinationChanged
	}
	if err := tx.Commit().Error; err != nil {
		return CoordinationRequest{}, err
	}
	return operationRequest(op), nil
}

// AcquireUploadRequest admits a part request under an already accepted upload.
// Final completion/abort first closes part admission and waits for active parts.
func AcquireUploadRequest(database *gorm.DB, parent, r CoordinationRequest, closing bool) (bool, error) {
	if !parent.valid() || !r.valid() || parent.Kind != "producer" || r.Kind != "producer" || parent.OperationID == r.OperationID || r.StorageID != parent.StorageID || r.Generation != parent.Generation || r.Scope == "*" {
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
	if store.Mode != "ready" && store.Mode != "draining" && store.Mode != "collecting" {
		return false, ErrCoordinationBusy
	}
	root, err := uploadParent(tx, parent)
	if err != nil {
		return false, err
	}
	if root.ExternalKey == nil || root.ExternalScope != r.Scope {
		return false, ErrCoordinationChanged
	}
	var existing model.CleanupOperation
	err = tx.Where("operation_id = ?", r.OperationID).First(&existing).Error
	if err == nil {
		if !r.matches(existing) || existing.ParentOperationID != parent.OperationID || existing.ClosingUpload != closing {
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
	if root.State == "uncertain" {
		return false, ErrCoordinationUncertain
	}
	if root.State != "active" {
		return false, ErrCoordinationBusy
	}
	var outstanding int
	if err := tx.Model(&model.CleanupOperation{}).Where("parent_operation_id = ? AND state <> ?", parent.OperationID, "finished").Count(&outstanding).Error; err != nil {
		return false, err
	}
	if outstanding >= 4096 || (closing && outstanding != 0) {
		return false, ErrCoordinationBusy
	}
	op := model.CleanupOperation{OperationID: r.OperationID, StorageID: r.StorageID, Generation: r.Generation, Owner: r.Owner, Kind: r.Kind, Scope: r.Scope, Fingerprint: r.Fingerprint, State: "active", ParentOperationID: parent.OperationID, ClosingUpload: closing}
	if err := tx.Create(&op).Error; err != nil {
		return false, err
	}
	if closing {
		if err := tx.Model(&model.CleanupOperation{}).Where("operation_id = ?", parent.OperationID).Update("state", "closing").Error; err != nil {
			return false, err
		}
	}
	if err := advanceCleanupRevision(tx, store); err != nil {
		return false, err
	}
	if err := tx.Commit().Error; err != nil {
		return false, err
	}
	return true, nil
}

// FinishUploadRequest leaves the parent active between parts and releases it
// only after the closing request is confirmed. Any unknown result retains it.
func FinishUploadRequest(database *gorm.DB, parent, r CoordinationRequest, confirmed bool) error {
	outcome := "unknown"
	if confirmed {
		outcome = "succeeded"
	}
	return RecordUploadRequest(database, parent, r, outcome)
}

// RecordUploadRequest distinguishes rejected completion from an unknown write.
// A confirmed rejection may reopen the upload; an unknown result never does.
func RecordUploadRequest(database *gorm.DB, parent, r CoordinationRequest, outcome string) error {
	if outcome != "succeeded" && outcome != "rejected" && outcome != "unknown" {
		return ErrCoordinationChanged
	}
	confirmed := outcome != "unknown"
	if !parent.valid() || !r.valid() || parent.Kind != "producer" || r.Kind != "producer" || r.StorageID != parent.StorageID || r.Generation != parent.Generation {
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
	root, err := uploadParent(tx, parent)
	if err != nil {
		return err
	}
	var child model.CleanupOperation
	if err := tx.Where("operation_id = ?", r.OperationID).First(&child).Error; err != nil {
		return err
	}
	if !r.matches(child) || child.ParentOperationID != parent.OperationID || root.ExternalKey == nil || root.ExternalScope != r.Scope {
		return ErrCoordinationChanged
	}
	if child.State == "uncertain" {
		return ErrCoordinationUncertain
	}
	if child.State == "finished" && confirmed && child.Outcome == outcome {
		return tx.Commit().Error
	}
	if child.State != "active" {
		return ErrCoordinationChanged
	}
	state := "finished"
	if !confirmed {
		state = "uncertain"
		root.State = "uncertain"
	} else if child.ClosingUpload && root.State != "uncertain" {
		if root.State != "closing" {
			return ErrCoordinationChanged
		}
		root.State = "finished"
		if outcome == "rejected" {
			root.State = "active"
		}
	}
	if err := tx.Model(&model.CleanupOperation{}).Where("operation_id = ?", child.OperationID).Updates(map[string]interface{}{"state": state, "outcome": outcome}).Error; err != nil {
		return err
	}
	if err := tx.Model(&model.CleanupOperation{}).Where("operation_id = ?", root.OperationID).Update("state", root.State).Error; err != nil {
		return err
	}
	if err := advanceCleanupRevision(tx, store); err != nil {
		return err
	}
	return tx.Commit().Error
}
