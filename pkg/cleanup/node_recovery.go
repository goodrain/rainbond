package cleanup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// NodeRecoveryBinding records the original receipt-only helper independently
// from native permission. Creating it never changes the parent operation state.
type NodeRecoveryBinding struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	SpecHash  string `json:"spec_hash"`
	JobUID    string `json:"job_uid,omitempty"`
}

// NodeRecoveryName is stable across controller and plugin restarts.
func NodeRecoveryName(parent NodeJobBinding) string {
	sum := sha256.Sum256([]byte(parent.JobUID + "\x00" + parent.PodUID))
	return "rainbond-node-recover-" + hex.EncodeToString(sum[:16])
}
func validNodeRecovery(parent NodeJobBinding, recovery NodeRecoveryBinding) bool {
	hash, err := hex.DecodeString(recovery.SpecHash)
	return parent.JobUID != "" && parent.PodUID != "" && parent.CanceledBeforeGrantAt == nil && recovery.Namespace == parent.Namespace && recovery.Name == NodeRecoveryName(parent) && err == nil && len(hash) == 32 && hex.EncodeToString(hash) == recovery.SpecHash && (recovery.JobUID == "" || coordinationIdentity.MatchString(recovery.JobUID))
}

// PrepareNodeRecovery grants only one helper creation after durable intent.
func PrepareNodeRecovery(database *gorm.DB, r CoordinationRequest, intent NodeRecoveryBinding) (NodeRecoveryBinding, bool, error) {
	var saved NodeRecoveryBinding
	created := false
	if intent.JobUID != "" {
		return saved, false, ErrCoordinationChanged
	}
	err := changeNodeJob(database, r, func(tx *gorm.DB, store model.CleanupStorage, op model.CleanupOperation) error {
		parent, err := readNodeBinding(r, op)
		if err != nil {
			return err
		}
		if !validNodeRecovery(parent, intent) {
			return ErrCoordinationChanged
		}
		if parent.Recovery != nil {
			old := *parent.Recovery
			old.JobUID = ""
			if old != intent {
				return ErrCoordinationChanged
			}
			saved = *parent.Recovery
			return nil
		}
		if op.State != "executing" && op.State != "applied" && op.State != "rejected" && op.State != "uncertain" {
			return ErrCoordinationChanged
		}
		parent.Recovery = &intent
		raw, err := json.Marshal(parent)
		if err != nil {
			return err
		}
		if err := tx.Model(&model.CleanupOperation{}).Where("operation_id = ?", r.OperationID).UpdateColumn("node_execution_json", string(raw)).Error; err != nil {
			return err
		}
		if err := advanceCleanupRevision(tx, store); err != nil {
			return err
		}
		saved = intent
		created = true
		return nil
	})
	return saved, created, err
}

// BindNodeRecovery cannot adopt a replacement for a lost original helper.
func BindNodeRecovery(database *gorm.DB, r CoordinationRequest, intent NodeRecoveryBinding, uid string) error {
	if intent.JobUID != "" || !coordinationIdentity.MatchString(uid) {
		return ErrCoordinationChanged
	}
	return changeNodeJob(database, r, func(tx *gorm.DB, store model.CleanupStorage, op model.CleanupOperation) error {
		parent, err := readNodeBinding(r, op)
		if err != nil {
			return err
		}
		if parent.Recovery == nil || !validNodeRecovery(parent, intent) {
			return ErrCoordinationChanged
		}
		old := *parent.Recovery
		old.JobUID = ""
		if old != intent {
			return ErrCoordinationChanged
		}
		if parent.Recovery.JobUID != "" {
			if parent.Recovery.JobUID != uid {
				return ErrCoordinationChanged
			}
			return nil
		}
		parent.Recovery.JobUID = uid
		raw, err := json.Marshal(parent)
		if err != nil {
			return err
		}
		if err := tx.Model(&model.CleanupOperation{}).Where("operation_id = ?", r.OperationID).UpdateColumn("node_execution_json", string(raw)).Error; err != nil {
			return err
		}
		return advanceCleanupRevision(tx, store)
	})
}
