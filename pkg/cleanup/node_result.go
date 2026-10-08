package cleanup

import (
	"encoding/json"
	"time"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// NodeStorageMeasurement reports the actual opened root's filesystem counters.
// A shared filesystem's observed change is not per-resource reclaimed size.
type NodeStorageMeasurement struct {
	RootDevice     uint64    `json:"rootDevice"`
	RootInode      uint64    `json:"rootInode"`
	TotalBytes     uint64    `json:"totalBytes"`
	FreeBytes      uint64    `json:"freeBytes"`
	AvailableBytes uint64    `json:"availableBytes"`
	TotalInodes    uint64    `json:"totalInodes"`
	FreeInodes     uint64    `json:"freeInodes"`
	ObservedAt     time.Time `json:"observedAt"`
}

// NodeExecutionResult is an immutable receipt from the original native helper.
type NodeExecutionResult struct {
	State  string                  `json:"state"`
	Before *NodeStorageMeasurement `json:"before,omitempty"`
	After  *NodeStorageMeasurement `json:"after,omitempty"`
}

func validNodeMeasurement(m *NodeStorageMeasurement) bool {
	return m != nil && m.RootInode != 0 && m.TotalBytes > 0 && m.FreeBytes <= m.TotalBytes && m.AvailableBytes <= m.FreeBytes && m.FreeInodes <= m.TotalInodes && !m.ObservedAt.IsZero()
}
func validNodeResult(result NodeExecutionResult) bool {
	if result.State != "deleted" && result.State != "failed" && result.State != "unknown" {
		return false
	}
	if result.Before != nil && !validNodeMeasurement(result.Before) {
		return false
	}
	if result.After != nil {
		if !validNodeMeasurement(result.After) || result.Before == nil || result.Before.RootDevice != result.After.RootDevice || result.Before.RootInode != result.After.RootInode || result.After.ObservedAt.Before(result.Before.ObservedAt) {
			return false
		}
	}
	if result.State == "deleted" {
		return result.Before != nil && result.After != nil
	}
	if result.State == "failed" {
		return (result.Before == nil) == (result.After == nil)
	}
	return true
}
func matchesNodeExecutor(binding NodeJobBinding, observed NodeExecutorIdentity) bool {
	return binding.PodUID != "" && observed.Namespace == binding.Namespace && observed.JobName == binding.Name && observed.JobUID == binding.JobUID && observed.PodName == binding.PodName && observed.PodUID == binding.PodUID && observed.NodeName == binding.NodeName && observed.NodeUID == binding.NodeUID && observed.SpecHash == binding.SpecHash && observed.ContainerID == binding.ContainerID && observed.ImageID == binding.ImageID
}

// RecordNodeResult persists native effects but does not release the scope. A
// conflicting receipt cannot replace the original outcome after a lost reply.
func RecordNodeResult(database *gorm.DB, r CoordinationRequest, observed NodeExecutorIdentity, result NodeExecutionResult) error {
	if !validNodeResult(result) {
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
		if !matchesNodeExecutor(binding, observed) || observed.VolumeUID != storage.VolumeUID {
			return ErrCoordinationChanged
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return err
		}
		if binding.Result != nil {
			original, err := json.Marshal(binding.Result)
			if err != nil || string(original) != string(encoded) {
				return ErrCoordinationChanged
			}
			return nil
		}
		if op.State != "executing" {
			return ErrCoordinationChanged
		}
		binding.Result = &result
		raw, err := json.Marshal(binding)
		if err != nil {
			return err
		}
		state := "applied"
		if result.State == "failed" {
			state = "rejected"
		} else if result.State == "unknown" {
			state = "uncertain"
		}
		if err := tx.Model(&model.CleanupOperation{}).Where("operation_id = ?", r.OperationID).Updates(map[string]interface{}{"node_execution_json": string(raw), "state": state, "outcome": result.State}).Error; err != nil {
			return err
		}
		return advanceCleanupRevision(tx, store)
	})
}

// FinishNodeExecution releases only a known original outcome after the trusted
// inspector observed the original container's termination. It never changes the
// storage readiness mode or fabricates a reclaimed-space count.
func FinishNodeExecution(database *gorm.DB, r CoordinationRequest, observed NodeExecutorIdentity) error {
	if observed.FinishedAt.IsZero() {
		return ErrCoordinationBusy
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
		if !matchesNodeExecutor(binding, observed) || observed.VolumeUID != storage.VolumeUID {
			return ErrCoordinationChanged
		}
		if binding.Result == nil || !validNodeResult(*binding.Result) || binding.Result.State == "unknown" {
			return ErrCoordinationUncertain
		}
		if op.State == "finished" {
			if binding.FinishedAt == nil || !binding.FinishedAt.Equal(observed.FinishedAt) {
				return ErrCoordinationChanged
			}
			return nil
		}
		if (op.State != "applied" && op.State != "rejected") || op.Outcome != binding.Result.State {
			return ErrCoordinationChanged
		}
		binding.FinishedAt = &observed.FinishedAt
		raw, err := json.Marshal(binding)
		if err != nil {
			return err
		}
		if err := tx.Model(&model.CleanupOperation{}).Where("operation_id = ?", r.OperationID).Updates(map[string]interface{}{"node_execution_json": string(raw), "state": "finished"}).Error; err != nil {
			return err
		}
		return advanceCleanupRevision(tx, store)
	})
}

// NodeJobProgress reports one immutable task receipt without enabling work.
type NodeJobProgress struct {
	StorageID   string         `json:"storage_id"`
	Generation  string         `json:"generation"`
	OperationID string         `json:"operation_id"`
	State       string         `json:"state"`
	Outcome     string         `json:"outcome"`
	Execution   NodeJobBinding `json:"execution"`
}

// ReadNodeJobProgress reads binding and outcome in a single database snapshot.
func ReadNodeJobProgress(database *gorm.DB, r CoordinationRequest) (NodeJobProgress, error) {
	if !r.valid() || r.Kind != "delete" {
		return NodeJobProgress{}, ErrCoordinationChanged
	}
	var op model.CleanupOperation
	if err := database.Where("operation_id = ?", r.OperationID).First(&op).Error; err != nil {
		return NodeJobProgress{}, err
	}
	if !r.matches(op) {
		return NodeJobProgress{}, ErrCoordinationChanged
	}
	binding, err := readNodeBinding(r, op)
	if err != nil {
		return NodeJobProgress{}, err
	}
	if op.State == "finished" && !validCanceledNode(binding) && (binding.FinishedAt == nil || binding.Result == nil || op.Outcome != binding.Result.State) {
		return NodeJobProgress{}, ErrCoordinationChanged
	}
	return NodeJobProgress{StorageID: r.StorageID, Generation: r.Generation, OperationID: r.OperationID, State: op.State, Outcome: op.Outcome, Execution: binding}, nil
}

// NodeResultRecorded acknowledges only an identical receipt already verified
// and committed for the original executor, even after its Pod is removed.
func NodeResultRecorded(database *gorm.DB, r CoordinationRequest, locator NodeExecutorLocator, result NodeExecutionResult) (bool, error) {
	if !validNodeResult(result) {
		return false, ErrCoordinationChanged
	}
	progress, err := ReadNodeJobProgress(database, r)
	if err != nil {
		return false, err
	}
	binding := progress.Execution
	if binding.Result == nil {
		return false, nil
	}
	if binding.PodName != locator.Pod || binding.PodUID != locator.PodUID {
		return false, ErrCoordinationChanged
	}
	original, _ := json.Marshal(binding.Result)
	submitted, _ := json.Marshal(result)
	if string(original) != string(submitted) {
		return false, ErrCoordinationChanged
	}
	return true, nil
}
