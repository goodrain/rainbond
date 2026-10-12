package cleanup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	"k8s.io/apimachinery/pkg/util/validation"
)

// ErrNodeJobNotPrepared denotes absence of a durable node executor intent.
var ErrNodeJobNotPrepared = errors.New("node job intent is not prepared")

// NodeJobIntent is derived by the trusted scheduler from the saved scan and
// observed node/storage binding; it does not accept a caller filesystem path.
type NodeJobIntent struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	SpecHash    string `json:"spec_hash"`
	NodeName    string `json:"node_name"`
	NodeUID     string `json:"node_uid"`
	Entry       string `json:"entry"`
	Fingerprint string `json:"fingerprint"`
}

// NodeJobBinding records the immutable intent and observed executor identities.
type NodeJobBinding struct {
	Recovery              *NodeRecoveryBinding `json:"recovery,omitempty"`
	CanceledBeforeGrantAt *time.Time           `json:"canceled_before_grant_at,omitempty"`
	Result                *NodeExecutionResult `json:"result,omitempty"`
	FinishedAt            *time.Time           `json:"finished_at,omitempty"`
	Protocol              int                  `json:"protocol"`
	NodeJobIntent
	ContainerID string `json:"container_id"`
	ImageID     string `json:"image_id"`
	JobUID      string `json:"job_uid"`
	PodName     string `json:"pod_name"`
	PodUID      string `json:"pod_uid"`
}

func nodeJobName(r CoordinationRequest) string {
	sum := sha256.Sum256([]byte(r.StorageID + "\x00" + r.Generation + "\x00" + r.OperationID))
	return "rainbond-node-clean-" + hex.EncodeToString(sum[:16])
}
func nodeJobScope(intent NodeJobIntent) string {
	sum := sha256.Sum256([]byte(intent.Entry))
	return "managed-cache/" + hex.EncodeToString(sum[:])
}
func nodeJobTarget(intent NodeJobIntent) string {
	sum := sha256.Sum256([]byte(intent.NodeUID + "\x00" + intent.Entry + "\x00" + intent.Fingerprint))
	return "managed-cache:" + hex.EncodeToString(sum[:])
}
func validNodeJobIntent(r CoordinationRequest, intent NodeJobIntent) bool {
	validEntry := ValidManagedNodeEntry("cache", intent.Entry) || ValidManagedNodeEntry("upload_components", intent.Entry)
	if !r.valid() || r.Kind != "delete" || r.Scope != nodeJobScope(intent) || r.Target != nodeJobTarget(intent) || len(validation.IsDNS1123Label(intent.Namespace)) != 0 || intent.Name != nodeJobName(r) || len(validation.IsDNS1123Subdomain(intent.NodeName)) != 0 || !coordinationIdentity.MatchString(intent.NodeUID) || !validEntry {
		return false
	}
	for _, hash := range []string{intent.SpecHash, intent.Fingerprint} {
		decoded, err := hex.DecodeString(hash)
		if err != nil || len(decoded) != 32 || hex.EncodeToString(decoded) != hash {
			return false
		}
	}
	return true
}
func readNodeBinding(r CoordinationRequest, op model.CleanupOperation) (NodeJobBinding, error) {
	var binding NodeJobBinding
	if op.NodeExecutionJSON == "" {
		return binding, ErrNodeJobNotPrepared
	}
	if len(op.NodeExecutionJSON) > 8192 || json.Unmarshal([]byte(op.NodeExecutionJSON), &binding) != nil {
		return NodeJobBinding{}, ErrCoordinationChanged
	}
	validated := binding.NodeJobIntent
	if binding.CanceledBeforeGrantAt != nil {
		if !validCanceledNode(binding) || op.State != "finished" || op.Outcome != "canceled" {
			return NodeJobBinding{}, ErrCoordinationChanged
		}
		// No executor spec exists for cancellation before job preparation.
		if binding.JobUID == "" && validated.SpecHash == "" {
			validated.SpecHash = strings.Repeat("0", 64)
		}
	}
	if binding.Protocol != 1 || !validNodeJobIntent(r, validated) || (binding.JobUID != "" && !coordinationIdentity.MatchString(binding.JobUID)) {
		return NodeJobBinding{}, ErrCoordinationChanged
	}
	if (binding.PodName != "" || binding.PodUID != "" || binding.ContainerID != "" || binding.ImageID != "") && (binding.JobUID == "" || len(validation.IsDNS1123Subdomain(binding.PodName)) != 0 || !coordinationIdentity.MatchString(binding.PodUID) || binding.ContainerID == "" || len(binding.ContainerID) > 256 || binding.ImageID == "" || len(binding.ImageID) > 512) {
		return NodeJobBinding{}, ErrCoordinationChanged
	}
	if binding.Result != nil && !validNodeResult(*binding.Result) {
		return NodeJobBinding{}, ErrCoordinationChanged
	}
	if binding.FinishedAt != nil && (binding.FinishedAt.IsZero() || binding.PodUID == "" || binding.Result == nil) {
		return NodeJobBinding{}, ErrCoordinationChanged
	}
	if binding.Recovery != nil && !validNodeRecovery(binding, *binding.Recovery) {
		return NodeJobBinding{}, ErrCoordinationChanged
	}
	return binding, nil
}
func changeNodeJob(database *gorm.DB, r CoordinationRequest, change func(*gorm.DB, model.CleanupStorage, model.CleanupOperation) error) error {
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
	binding, err := StorageBinding(tx, r.StorageID, r.Generation)
	if err != nil {
		return err
	}
	kind, err := ManagedNodeStorageKind(binding)
	if err != nil || kind == "" {
		return ErrCoordinationChanged
	}
	var op model.CleanupOperation
	if err := tx.Where("operation_id = ?", r.OperationID).First(&op).Error; err != nil {
		return err
	}
	if !r.matches(op) || op.ExternalKey != nil || op.ParentOperationID != "" || hasGCJobBinding(&op) {
		return ErrCoordinationChanged
	}
	if err := change(tx, store, op); err != nil {
		return err
	}
	return tx.Commit().Error
}

// PrepareNodeJob grants one creation attempt only after committing its intent.
// Lost responses must be reconciled; an existing intent never grants recreation.
func PrepareNodeJob(database *gorm.DB, r CoordinationRequest, intent NodeJobIntent) (NodeJobIntent, bool, error) {
	if intent.Name != "" && intent.Name != nodeJobName(r) {
		return NodeJobIntent{}, false, ErrCoordinationChanged
	}
	intent.Name = nodeJobName(r)
	if !validNodeJobIntent(r, intent) {
		return NodeJobIntent{}, false, ErrCoordinationChanged
	}
	binding, err := StorageBinding(database, r.StorageID, r.Generation)
	if err != nil {
		return NodeJobIntent{}, false, err
	}
	kind, err := ManagedNodeStorageKind(binding)
	if err != nil || !ValidManagedNodeEntry(kind, intent.Entry) {
		return NodeJobIntent{}, false, ErrCoordinationChanged
	}
	created := false
	err = changeNodeJob(database, r, func(tx *gorm.DB, store model.CleanupStorage, op model.CleanupOperation) error {
		if op.NodeExecutionJSON != "" {
			previous, err := readNodeBinding(r, op)
			if err != nil {
				return err
			}
			if previous.CanceledBeforeGrantAt != nil || previous.NodeJobIntent != intent {
				return ErrCoordinationChanged
			}
			return nil
		}
		if store.Mode != "ready" || op.State != "active" {
			return ErrCoordinationBusy
		}
		raw, err := json.Marshal(NodeJobBinding{Protocol: 1, NodeJobIntent: intent})
		if err != nil {
			return err
		}
		if err := tx.Model(&model.CleanupOperation{}).Where("operation_id = ?", r.OperationID).UpdateColumn("node_execution_json", string(raw)).Error; err != nil {
			return err
		}
		created = true
		return advanceCleanupRevision(tx, store)
	})
	if err != nil {
		return NodeJobIntent{}, false, err
	}
	return intent, created, nil
}

// BindNodeJob accepts only the original observed Job UID after spec verification.
func BindNodeJob(database *gorm.DB, r CoordinationRequest, intent NodeJobIntent, jobUID string) error {
	if !validNodeJobIntent(r, intent) || !coordinationIdentity.MatchString(jobUID) {
		return ErrCoordinationChanged
	}
	return changeNodeJob(database, r, func(tx *gorm.DB, store model.CleanupStorage, op model.CleanupOperation) error {
		previous, err := readNodeBinding(r, op)
		if err != nil {
			return err
		}
		if previous.CanceledBeforeGrantAt != nil || previous.NodeJobIntent != intent {
			return ErrCoordinationChanged
		}
		if previous.JobUID != "" {
			if previous.JobUID != jobUID {
				return ErrCoordinationChanged
			}
			return nil
		}
		if store.Mode != "ready" || op.State != "active" {
			return ErrCoordinationBusy
		}
		previous.JobUID = jobUID
		raw, err := json.Marshal(previous)
		if err != nil {
			return err
		}
		if err := tx.Model(&model.CleanupOperation{}).Where("operation_id = ?", r.OperationID).UpdateColumn("node_execution_json", string(raw)).Error; err != nil {
			return err
		}
		return advanceCleanupRevision(tx, store)
	})
}

// ReadNodeJobBinding reads original executor identity without granting work.
func ReadNodeJobBinding(database *gorm.DB, r CoordinationRequest) (NodeJobBinding, error) {
	if !r.valid() || r.Kind != "delete" {
		return NodeJobBinding{}, ErrCoordinationChanged
	}
	var op model.CleanupOperation
	if err := database.Where("operation_id = ?", r.OperationID).First(&op).Error; err != nil {
		return NodeJobBinding{}, err
	}
	if !r.matches(op) {
		return NodeJobBinding{}, ErrCoordinationChanged
	}
	return readNodeBinding(r, op)
}

// ManagedNodeRequest derives the scope/target of a trusted saved selection.
// It constructs a request only; it never registers or acquires an operation.
func ManagedNodeRequest(storage StorageRegistration, owner, operationID, fingerprint string, intent NodeJobIntent) (CoordinationRequest, error) {
	kind, err := ManagedNodeStorageKind(storage)
	if err != nil {
		return CoordinationRequest{}, err
	}
	if !ValidManagedNodeEntry(kind, intent.Entry) {
		return CoordinationRequest{}, ErrCoordinationChanged
	}
	r := CoordinationRequest{StorageID: storage.StorageID, Generation: storage.Generation, Owner: owner, OperationID: operationID, Kind: "delete", Scope: nodeJobScope(intent), Target: nodeJobTarget(intent), Fingerprint: fingerprint}
	if intent.Name != "" && intent.Name != nodeJobName(r) {
		return CoordinationRequest{}, ErrCoordinationChanged
	}
	intent.Name = nodeJobName(r)
	if intent.SpecHash == "" {
		intent.SpecHash = strings.Repeat("0", 64)
	}
	if !validNodeJobIntent(r, intent) {
		return CoordinationRequest{}, ErrCoordinationChanged
	}
	return r, nil
}
