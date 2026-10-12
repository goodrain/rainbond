package cleanup

import (
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// GCJobProgress is a read-only receipt for one immutable GC operation. Finished
// describes that operation, not the admission state of any newer maintenance.
type GCJobProgress struct {
	StorageID     string              `json:"storage_id"`
	Generation    string              `json:"generation"`
	OperationID   string              `json:"operation_id"`
	ExecutorState string              `json:"executor_state,omitempty"`
	State         string              `json:"state"`
	Outcome       string              `json:"outcome,omitempty"`
	JobNamespace  string              `json:"job_namespace,omitempty"`
	JobName       string              `json:"job_name,omitempty"`
	JobUID        string              `json:"job_uid,omitempty"`
	PodName       string              `json:"pod_name,omitempty"`
	PodUID        string              `json:"pod_uid,omitempty"`
	Before        *StorageMeasurement `json:"before,omitempty"`
	After         *StorageMeasurement `json:"after,omitempty"`
}

// ReadGCJobProgress never creates, resumes or restores a task and discloses only
// the operation matching the complete authenticated request binding.
func ReadGCJobProgress(database *gorm.DB, r CoordinationRequest) (GCJobProgress, error) {
	if !r.valid() || r.Kind != "gc" {
		return GCJobProgress{}, ErrCoordinationChanged
	}
	var op model.CleanupOperation
	if err := database.Where("operation_id = ?", r.OperationID).First(&op).Error; err != nil {
		return GCJobProgress{}, err
	}
	if !r.matches(op) || hasGCJobBinding(&op) && !validGCJobBinding(r, &op) {
		return GCJobProgress{}, ErrCoordinationChanged
	}
	before, after, err := MaintenanceMeasurements(database, r)
	if err != nil {
		return GCJobProgress{}, err
	}
	return GCJobProgress{StorageID: r.StorageID, Generation: r.Generation, OperationID: r.OperationID, State: op.State, Outcome: op.Outcome, JobNamespace: op.GCJobNamespace, JobName: op.GCJobName, JobUID: op.GCJobUID, PodName: op.GCPodName, PodUID: op.GCPodUID, Before: before, After: after}, nil
}
