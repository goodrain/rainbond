package cleanup

import (
	"context"
	"encoding/json"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// NodeJobStartClient updates only the namespace-scoped original Job.
type NodeJobStartClient = GCJobStartClient

// StartNodeJob unsuspends the recorded Job only while the original deletion
// admission remains active. The helper must still pass a separate one-use
// node/storage/Pod identity gate before touching any filesystem entry.
func StartNodeJob(ctx context.Context, database *gorm.DB, client NodeJobStartClient, r CoordinationRequest) (*batchv1.Job, error) {
	job, err := ReconcileNodeJob(ctx, database, client, r)
	if err != nil {
		return nil, err
	}
	if job.Spec.Suspend == nil || job.ResourceVersion == "" {
		return nil, ErrCoordinationChanged
	}
	if job.Status.Succeeded > 0 || job.Status.Failed > 0 {
		return nil, ErrCoordinationChanged
	}
	if !*job.Spec.Suspend {
		return job, nil
	}
	err = changeNodeJob(database, r, func(tx *gorm.DB, store model.CleanupStorage, op model.CleanupOperation) error {
		binding, err := readNodeBinding(r, op)
		if err != nil {
			return err
		}
		if binding.JobUID != string(job.UID) {
			return ErrCoordinationChanged
		}
		if store.Mode != "ready" || store.MaintenanceOperationID != "" || op.State != "active" || binding.PodUID != "" {
			return ErrCoordinationBusy
		}
		return nodeWritersIdle(tx, r)
	})
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	patch, _ := json.Marshal([]map[string]interface{}{
		{"op": "test", "path": "/metadata/uid", "value": job.UID},
		{"op": "test", "path": "/metadata/resourceVersion", "value": job.ResourceVersion},
		{"op": "test", "path": "/spec/suspend", "value": true},
		{"op": "replace", "path": "/spec/suspend", "value": false},
	})
	if _, err := client.Patch(ctx, job.Name, types.JSONPatchType, patch, metav1.PatchOptions{}); err != nil {
		return nil, ErrCoordinationUncertain
	}
	return ReconcileNodeJob(ctx, database, client, r)
}
