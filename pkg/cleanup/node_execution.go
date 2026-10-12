package cleanup

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	"k8s.io/apimachinery/pkg/util/validation"
)

// NodeExecutorIdentity is produced by the trusted Kubernetes inspector. HTTP
// handlers must inspect live objects, never decode these facts from a caller.
type NodeExecutorIdentity struct {
	FinishedAt                                  time.Time
	Namespace, JobName, JobUID, PodName, PodUID string
	NodeName, NodeUID, VolumeUID, SpecHash      string
	ContainerID, ImageID                        string
}

// EnterNodeExecution consumes the original Pod's only native execution grant.
// A lost acknowledgment is never permission to reissue native deletion.
func EnterNodeExecution(database *gorm.DB, r CoordinationRequest, observed NodeExecutorIdentity) error {
	if !observed.FinishedAt.IsZero() || !coordinationIdentity.MatchString(observed.PodUID) || len(validation.IsDNS1123Subdomain(observed.PodName)) != 0 || observed.ContainerID == "" || len(observed.ContainerID) > 256 || observed.ImageID == "" || len(observed.ImageID) > 512 || strings.ContainsAny(observed.ContainerID+observed.ImageID, "\x00\r\n") {
		return ErrCoordinationChanged
	}
	return changeNodeJob(database, r, func(tx *gorm.DB, store model.CleanupStorage, op model.CleanupOperation) error {
		binding, err := readNodeBinding(r, op)
		if err != nil {
			return err
		}
		storage, err := StorageBinding(tx, r.StorageID, r.Generation)
		if err != nil {
			return err
		}
		if observed.Namespace != binding.Namespace || observed.JobName != binding.Name || observed.JobUID != binding.JobUID || binding.JobUID == "" || observed.NodeName != binding.NodeName || observed.NodeUID != binding.NodeUID || observed.VolumeUID != storage.VolumeUID || observed.SpecHash != binding.SpecHash {
			return ErrCoordinationChanged
		}
		if binding.PodUID != "" || op.State == "executing" || op.State == "applied" || op.State == "uncertain" || op.State == "finished" {
			return ErrCoordinationUncertain
		}
		if store.Mode != "ready" || store.MaintenanceOperationID != "" || op.State != "active" {
			return ErrCoordinationBusy
		}
		if err := nodeWritersIdle(tx, r); err != nil {
			return err
		}
		binding.PodName = observed.PodName
		binding.PodUID = observed.PodUID
		binding.ContainerID = observed.ContainerID
		binding.ImageID = observed.ImageID
		raw, err := json.Marshal(binding)
		if err != nil {
			return err
		}
		if err := tx.Model(&model.CleanupOperation{}).Where("operation_id = ?", r.OperationID).Updates(map[string]interface{}{"node_execution_json": string(raw), "state": "executing"}).Error; err != nil {
			return err
		}
		return advanceCleanupRevision(tx, store)
	})
}

func nodeWritersIdle(tx *gorm.DB, r CoordinationRequest) error {
	var writers []model.CleanupOperation
	if err := tx.Where("storage_id = ? AND state <> ? AND kind = ?", r.StorageID, "finished", "producer").Limit(4097).Find(&writers).Error; err != nil {
		return err
	}
	if len(writers) > 4096 {
		return ErrCoordinationBusy
	}
	for _, writer := range writers {
		if writer.Generation != r.Generation {
			return ErrCoordinationChanged
		}
		if writer.Scope == "*" || writer.Scope == "" || writer.Scope == r.Scope {
			return ErrCoordinationBusy
		}
	}
	return nil
}
