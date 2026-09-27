package cleanup

import (
	"errors"
	"regexp"
	"strings"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// ErrCoordinationBusy means a conflicting operation or maintenance owns the scope.
var ErrCoordinationBusy = errors.New("cleanup coordination scope busy")

// ErrCoordinationChanged means the immutable operation or storage binding differs.
var ErrCoordinationChanged = errors.New("cleanup coordination identity changed")

// ErrCoordinationUncertain requires evidence-based reconciliation before release.
var ErrCoordinationUncertain = errors.New("cleanup coordination requires reconciliation")

// CoordinationRequest is constructed by authenticated internal callers. A
// successful admission does not by itself establish reference completeness.
type CoordinationRequest struct {
	StorageID   string `json:"-"`
	Generation  string `json:"generation"`
	Owner       string `json:"owner"`
	OperationID string `json:"operation_id"`
	Kind        string `json:"kind"`
	Scope       string `json:"scope"`
	Fingerprint string `json:"fingerprint"`
	Target      string `json:"target,omitempty"`
}

var coordinationIdentity = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.:-]{0,63}$`)

var coordinationScope = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]{0,254}$`)

func (r CoordinationRequest) valid() bool {
	if !coordinationIdentity.MatchString(r.StorageID) || !coordinationIdentity.MatchString(r.Generation) || !coordinationIdentity.MatchString(r.OperationID) {
		return false
	}
	for _, v := range []struct {
		value string
		max   int
	}{{r.StorageID, 64}, {r.Generation, 64}, {r.Owner, 128}, {r.OperationID, 64}, {r.Fingerprint, 128}} {
		if v.value == "" || len(v.value) > v.max || strings.ContainsRune(v.value, 0) {
			return false
		}
	}
	if len(r.Target) > 1024 || strings.ContainsRune(r.Target, 0) || (r.Kind == "delete" && r.Target == "") || (r.Kind != "delete" && r.Target != "") {
		return false
	}
	if r.Kind != "producer" && r.Kind != "delete" && r.Kind != "gc" {
		return false
	}
	if r.Scope == "*" {
		return r.Kind == "producer" || r.Kind == "gc"
	}
	if r.Kind == "gc" || !coordinationScope.MatchString(r.Scope) {
		return false
	}
	for _, segment := range strings.Split(r.Scope, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}
func (r CoordinationRequest) matches(op model.CleanupOperation) bool {
	return op.OperationID == r.OperationID && op.StorageID == r.StorageID && op.Generation == r.Generation && op.Owner == r.Owner && op.Kind == r.Kind && op.Scope == r.Scope && op.Fingerprint == r.Fingerprint && op.Target == r.Target
}

// A no-op UPDATE obtains the existing storage row's write lock before any
// reads. MySQL affected-row counts may be zero for a no-op; existence and the
// generation are checked by the subsequent read in the same transaction.
func lockCleanupStorage(tx *gorm.DB, r CoordinationRequest) (model.CleanupStorage, error) {
	var store model.CleanupStorage
	if err := tx.Model(&model.CleanupStorage{}).Where("storage_id = ?", r.StorageID).UpdateColumn("revision", gorm.Expr("revision")).Error; err != nil {
		return store, err
	}
	if err := tx.Where("storage_id = ?", r.StorageID).First(&store).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return store, ErrCoordinationChanged
		}
		return store, err
	}
	if store.Generation != r.Generation {
		return store, ErrCoordinationChanged
	}
	return store, nil
}
func advanceCleanupRevision(tx *gorm.DB, store model.CleanupStorage) error {
	if store.Revision == ^uint64(0) {
		return ErrCoordinationChanged
	}
	return tx.Model(&model.CleanupStorage{}).Where("storage_id = ? AND generation = ?", store.StorageID, store.Generation).UpdateColumn("revision", store.Revision+1).Error
}

// AcquireOperation returns true only for a newly persisted admission. A retry
// returns false; callers must not execute a destructive operation a second time.
func AcquireOperation(database *gorm.DB, r CoordinationRequest) (bool, error) {
	if !r.valid() || r.Kind == "gc" {
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
		if existing.State != "active" && existing.State != "finished" {
			return false, ErrCoordinationChanged
		}
		return false, tx.Commit().Error
	}
	if !gorm.IsRecordNotFoundError(err) {
		return false, err
	}
	if store.Mode != "ready" && !(store.Mode == "collecting" && r.Kind == "producer") {
		return false, ErrCoordinationBusy
	}
	var active []model.CleanupOperation
	if err := tx.Where("storage_id = ? AND state <> ?", r.StorageID, "finished").Limit(4097).Find(&active).Error; err != nil {
		return false, err
	}
	if len(active) > 4096 {
		return false, ErrCoordinationBusy
	}
	for _, op := range active {
		if op.Generation != r.Generation {
			return false, ErrCoordinationChanged
		}
		overlap := op.Scope == "*" || r.Scope == "*" || op.Scope == r.Scope || op.Scope == ""
		if overlap && (op.Kind != "producer" || r.Kind != "producer") {
			return false, ErrCoordinationBusy
		}
	}
	op := model.CleanupOperation{OperationID: r.OperationID, StorageID: r.StorageID, Generation: r.Generation, Owner: r.Owner, Kind: r.Kind, Scope: r.Scope, Fingerprint: r.Fingerprint, Target: r.Target, State: "active"}
	if err := tx.Create(&op).Error; err != nil {
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

// FinishOperation may mark an active operation finished only when its trusted
// owner has confirmed the actual work (including metadata writes) is complete.
// It cannot unlock uncertain work; reconciliation is a separate, stricter path.
func FinishOperation(database *gorm.DB, r CoordinationRequest, confirmed bool) error {
	if !r.valid() || r.Kind == "gc" {
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
	if !r.matches(op) {
		return ErrCoordinationChanged
	}
	if op.NodeExecutionJSON != "" {
		return ErrCoordinationChanged
	}
	if confirmed && (op.ExternalKey != nil || op.ParentOperationID != "") {
		return ErrCoordinationChanged
	}
	if op.State == "uncertain" {
		if confirmed {
			return ErrCoordinationUncertain
		}
		return tx.Commit().Error
	}
	if op.State == "finished" {
		if !confirmed {
			return ErrCoordinationChanged
		}
		return tx.Commit().Error
	}
	if op.State == "executing" && confirmed {
		return ErrCoordinationUncertain
	}
	if op.State != "active" && !(r.Kind == "delete" && (op.State == "executing" || op.State == "applied" || op.State == "rejected")) {
		return ErrCoordinationChanged
	}
	state := "uncertain"
	if confirmed {
		state = "finished"
	}
	if err := tx.Model(&model.CleanupOperation{}).Where("operation_id = ?", r.OperationID).Update("state", state).Error; err != nil {
		return err
	}
	if err := advanceCleanupRevision(tx, store); err != nil {
		return err
	}
	return tx.Commit().Error
}

// InspectOperation returns a receipt only for the original immutable binding.
func InspectOperation(database *gorm.DB, r CoordinationRequest) (string, error) {
	if !r.valid() {
		return "", ErrCoordinationChanged
	}
	tx := database.Begin()
	if tx.Error != nil {
		return "", tx.Error
	}
	defer tx.Rollback()
	if _, err := lockCleanupStorage(tx, r); err != nil {
		return "", err
	}
	var op model.CleanupOperation
	if err := tx.Where("operation_id = ?", r.OperationID).First(&op).Error; err != nil {
		return "", err
	}
	if !r.matches(op) {
		return "", ErrCoordinationChanged
	}
	if err := tx.Commit().Error; err != nil {
		return "", err
	}
	return op.State, nil
}
