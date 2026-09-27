package cleanup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// RunNodeInventoryJob starts only a server-built observational task. Kubernetes
// retains its identity across request retries. It never uses a cleanup grant or
// reports a scan as complete merely because the Job was accepted.
func RunNodeInventoryJob(ctx context.Context, client NodeJobStartClient, scanID string, template *batchv1.Job, fresh func() (*batchv1.Job, error)) (*batchv1.Job, error) {
	if client == nil || template == nil || fresh == nil || !coordinationIdentity.MatchString(scanID) || template.Namespace == "" || template.UID != "" || template.ResourceVersion != "" || template.Name != "" || template.GenerateName != "" {
		return nil, ErrCoordinationChanged
	}
	job := template.DeepCopy()
	key := sha256.Sum256([]byte(scanID + "\x00" + job.Spec.Template.Spec.NodeName))
	job.Name = "cleanup-inventory-" + hex.EncodeToString(key[:20])
	if _, err := inventoryJobHash(job); err != nil || job.Spec.Suspend == nil || !*job.Spec.Suspend {
		return nil, ErrCoordinationChanged
	}
	// A dry-run create still rejects an existing name. Use a distinct,
	// never-persisted name so completed jobs can be observed repeatedly.
	// Controller-generated labels are validated and excluded by inventoryJobHash.
	previewRequest := job.DeepCopy()
	previewRequest.Name = "cleanup-inventory-preview-" + hex.EncodeToString(key[:16])
	preview, err := client.Create(ctx, previewRequest, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
	if err != nil {
		return nil, err
	}
	if preview == nil || preview.Name != previewRequest.Name || preview.Namespace != job.Namespace || preview.Spec.Suspend == nil || !*preview.Spec.Suspend || !sameExecutorPreview(job.Spec.Template.Spec, preview.Spec.Template.Spec) {
		return nil, ErrCoordinationChanged
	}
	hash, err := inventoryJobHash(preview)
	if err != nil {
		return nil, err
	}
	read := func() (*batchv1.Job, error) {
		current, err := client.Get(ctx, job.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		actual, err := inventoryJobHash(current)
		if err != nil || actual != hash || current.Name != job.Name || current.Namespace != job.Namespace || current.UID == "" || current.ResourceVersion == "" || current.DeletionTimestamp != nil || len(current.OwnerReferences) > 0 || current.Spec.Suspend == nil {
			return nil, ErrCoordinationChanged
		}
		return current, nil
	}
	current, err := read()
	if apierrors.IsNotFound(err) {
		created := preview.DeepCopy()
		created.ObjectMeta = metav1.ObjectMeta{Name: job.Name, Namespace: job.Namespace, Labels: preview.Labels, Annotations: preview.Annotations}
		created.Status = batchv1.JobStatus{}
		stripGCControllerFields(created)
		if _, err := client.Create(ctx, created, metav1.CreateOptions{}); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return nil, ErrCoordinationUncertain
			}
		}
		current, err = read()
	}
	if err != nil {
		return nil, err
	}
	if !*current.Spec.Suspend || current.Status.Succeeded > 0 || current.Status.Failed > 0 {
		return current, nil
	}
	verified, err := fresh()
	if err != nil || verified == nil || !reflect.DeepEqual(template.Spec, verified.Spec) || !reflect.DeepEqual(template.Annotations, verified.Annotations) {
		return nil, ErrCoordinationChanged
	}
	patch, _ := json.Marshal([]map[string]interface{}{
		{"op": "test", "path": "/metadata/uid", "value": current.UID},
		{"op": "test", "path": "/metadata/resourceVersion", "value": current.ResourceVersion},
		{"op": "test", "path": "/spec/suspend", "value": true},
		{"op": "replace", "path": "/spec/suspend", "value": false},
	})
	if _, err := client.Patch(ctx, current.Name, types.JSONPatchType, patch, metav1.PatchOptions{}); err != nil {
		return nil, ErrCoordinationUncertain
	}
	return read()
}

func inventoryJobHash(job *batchv1.Job) (string, error) {
	if job == nil || len(job.Spec.Template.Spec.Containers) != 1 || len(job.Spec.Template.Spec.InitContainers) != 0 || len(job.Spec.Template.Spec.EphemeralContainers) != 0 {
		return "", ErrCoordinationChanged
	}
	c := job.Spec.Template.Spec.Containers[0]
	if len(c.Command) != 1 || c.Command[0] != "/app/node-inventory" {
		return "", ErrCoordinationChanged
	}
	for _, mount := range c.VolumeMounts {
		if mount.Name == "node-cache" && !mount.ReadOnly {
			return "", ErrCoordinationChanged
		}
	}
	return gcSubmissionHash(job)
}
