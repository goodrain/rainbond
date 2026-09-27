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

// GCJobStartClient updates only the original namespace-scoped Job.
type GCJobStartClient interface {
	GCJobClient
	Patch(context.Context, string, types.PatchType, []byte, metav1.PatchOptions, ...string) (*batchv1.Job, error)
}

// StartGCJob unsuspends the original recorded Job after writers drain. Starting
// a Pod is not a native execution grant: the helper must still pass EnterGCJob.
// A retry observes an already-started Job without another mutation or creation.
func StartGCJob(ctx context.Context, database *gorm.DB, client GCJobStartClient, r CoordinationRequest) (*batchv1.Job, error) {
	job, err := ReconcileGCJob(ctx, database, client, r)
	if err != nil {
		return nil, err
	}
	if job.Spec.Suspend != nil && !*job.Spec.Suspend {
		return job, nil
	}
	if job.ResourceVersion == "" {
		return nil, ErrCoordinationChanged
	}
	err = changeMaintenance(database, r, func(tx *gorm.DB, store *model.CleanupStorage, op *model.CleanupOperation) (bool, error) {
		if !validGCJobBinding(r, op) || op.GCJobUID != string(job.UID) {
			return false, ErrCoordinationChanged
		}
		return false, maintenanceDrained(tx, store, op, r)
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
	return ReconcileGCJob(ctx, database, client, r)
}
