package cleanup

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	"k8s.io/apimachinery/pkg/util/validation"
)

// ErrGCJobNotPrepared means a draining operation has no persisted Job intent.
var ErrGCJobNotPrepared = errors.New("GC job intent is not prepared")

// GCJobIntent records a server-generated Job specification before submission.
// A repeated preparation is inspection, never permission to recreate the Job.
type GCJobIntent struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	SpecHash  string `json:"spec_hash"`
}

type GCJobBinding struct {
	GCJobIntent
	JobUID  string `json:"job_uid"`
	PodName string `json:"pod_name"`
	PodUID  string `json:"pod_uid"`
}

func hasGCJobBinding(op *model.CleanupOperation) bool {
	return op.GCJobNamespace != "" || op.GCJobName != "" || op.GCJobSpecHash != "" || op.GCJobUID != "" || op.GCPodName != "" || op.GCPodUID != ""
}
func gcJobName(r CoordinationRequest) string {
	sum := sha256.Sum256([]byte(r.StorageID + "\x00" + r.Generation + "\x00" + r.OperationID))
	return "rainbond-gc-" + hex.EncodeToString(sum[:16])
}
func validGCIntent(r CoordinationRequest, intent GCJobIntent) bool {
	decoded, err := hex.DecodeString(intent.SpecHash)
	return r.valid() && r.Kind == "gc" && len(validation.IsDNS1123Label(intent.Namespace)) == 0 && intent.Name == gcJobName(r) && err == nil && len(decoded) == 32 && hex.EncodeToString(decoded) == intent.SpecHash
}
func validGCJobBinding(r CoordinationRequest, op *model.CleanupOperation) bool {
	if !validGCIntent(r, gcIntent(op)) {
		return false
	}
	if op.GCJobUID != "" && !coordinationIdentity.MatchString(op.GCJobUID) {
		return false
	}
	if op.GCPodUID != "" || op.GCPodName != "" {
		return op.GCJobUID != "" && coordinationIdentity.MatchString(op.GCPodUID) && len(validation.IsDNS1123Subdomain(op.GCPodName)) == 0
	}
	return true
}

func gcIntent(op *model.CleanupOperation) GCJobIntent {
	return GCJobIntent{Namespace: op.GCJobNamespace, Name: op.GCJobName, SpecHash: op.GCJobSpecHash}
}

// PrepareGCJob must commit before a trusted launcher persists a Kubernetes Job. On any
// error the launcher must stop; a lost commit acknowledgment is not a retry grant.
func PrepareGCJob(database *gorm.DB, r CoordinationRequest, namespace, specHash string) (GCJobIntent, bool, error) {
	intent := GCJobIntent{Namespace: namespace, Name: gcJobName(r), SpecHash: specHash}
	if !validGCIntent(r, intent) {
		return GCJobIntent{}, false, ErrCoordinationChanged
	}
	created := false
	err := changeMaintenance(database, r, func(tx *gorm.DB, store *model.CleanupStorage, op *model.CleanupOperation) (bool, error) {
		if hasGCJobBinding(op) {
			if !validGCJobBinding(r, op) || gcIntent(op) != intent {
				return false, ErrCoordinationChanged
			}
			return false, nil
		}
		if store.Mode != "draining" || op.State != "draining" {
			return false, ErrCoordinationChanged
		}
		err := tx.Model(&model.CleanupOperation{}).Where("operation_id = ?", r.OperationID).Updates(map[string]interface{}{"gc_job_namespace": intent.Namespace, "gc_job_name": intent.Name, "gc_job_spec_hash": intent.SpecHash}).Error
		if err != nil {
			return false, err
		}
		created = true
		return true, nil
	})
	if err != nil {
		return GCJobIntent{}, false, err
	}
	return intent, created, nil
}

// BindGCJob is called only after the trusted launcher verifies the observed
// Kubernetes Job and its immutable specification. It never replaces a UID.
func BindGCJob(database *gorm.DB, r CoordinationRequest, intent GCJobIntent, jobUID string) error {
	if !validGCIntent(r, intent) || !coordinationIdentity.MatchString(jobUID) {
		return ErrCoordinationChanged
	}
	return changeMaintenance(database, r, func(tx *gorm.DB, store *model.CleanupStorage, op *model.CleanupOperation) (bool, error) {
		if !validGCJobBinding(r, op) || gcIntent(op) != intent {
			return false, ErrCoordinationChanged
		}
		if op.GCJobUID != "" {
			if op.GCJobUID != jobUID {
				return false, ErrCoordinationChanged
			}
			return false, nil
		}
		if store.Mode != "draining" || op.State != "draining" || op.GCPodUID != "" || op.GCPodName != "" {
			return false, ErrCoordinationChanged
		}
		if err := tx.Model(&model.CleanupOperation{}).Where("operation_id = ?", r.OperationID).Update("gc_job_uid", jobUID).Error; err != nil {
			return false, err
		}
		return true, nil
	})
}

// BindGCExecutor records only a control-plane-verified Pod owned by the bound
// Job. Kubernetes ownership/image/mount verification belongs to the launcher;
// browser-supplied names or UIDs must never be passed directly to this function.
func BindGCExecutor(database *gorm.DB, r CoordinationRequest, jobUID, podName, podUID string) error {
	if !coordinationIdentity.MatchString(jobUID) || !coordinationIdentity.MatchString(podUID) || len(validation.IsDNS1123Subdomain(podName)) != 0 {
		return ErrCoordinationChanged
	}
	return changeMaintenance(database, r, func(tx *gorm.DB, store *model.CleanupStorage, op *model.CleanupOperation) (bool, error) {
		if !validGCJobBinding(r, op) || op.GCJobUID != jobUID {
			return false, ErrCoordinationChanged
		}
		if op.GCPodUID != "" || op.GCPodName != "" {
			if op.GCPodUID != podUID || op.GCPodName != podName {
				return false, ErrCoordinationChanged
			}
			return false, nil
		}
		if store.Mode != "draining" || op.State != "draining" {
			return false, ErrCoordinationChanged
		}
		if err := tx.Model(&model.CleanupOperation{}).Where("operation_id = ?", r.OperationID).Updates(map[string]interface{}{"gc_pod_name": podName, "gc_pod_uid": podUID}).Error; err != nil {
			return false, err
		}
		return true, nil
	})
}

func ReadGCJobBinding(database *gorm.DB, r CoordinationRequest) (GCJobBinding, error) {
	if !r.valid() || r.Kind != "gc" {
		return GCJobBinding{}, ErrCoordinationChanged
	}
	var op model.CleanupOperation
	if err := database.Where("operation_id = ?", r.OperationID).First(&op).Error; err != nil {
		return GCJobBinding{}, err
	}
	if !r.matches(op) {
		return GCJobBinding{}, ErrCoordinationChanged
	}
	if !hasGCJobBinding(&op) && op.State == "draining" {
		return GCJobBinding{}, ErrGCJobNotPrepared
	}
	if !validGCJobBinding(r, &op) {
		return GCJobBinding{}, ErrCoordinationChanged
	}
	return GCJobBinding{GCJobIntent: gcIntent(&op), JobUID: op.GCJobUID, PodName: op.GCPodName, PodUID: op.GCPodUID}, nil
}

// EnterGCJobExecution grants at most one native execution, after the same
// verified Job/Pod is bound and the existing writer-drain checks have passed.
func EnterGCJobExecution(database *gorm.DB, r CoordinationRequest, jobUID, podUID string) error {
	if !coordinationIdentity.MatchString(jobUID) || !coordinationIdentity.MatchString(podUID) {
		return ErrCoordinationChanged
	}
	return changeMaintenance(database, r, func(tx *gorm.DB, store *model.CleanupStorage, op *model.CleanupOperation) (bool, error) {
		if !validGCJobBinding(r, op) || op.GCJobUID != jobUID || op.GCPodUID != podUID || op.GCPodName == "" {
			return false, ErrCoordinationChanged
		}
		return enterMaintenanceExclusive(tx, store, op, r)
	})
}
