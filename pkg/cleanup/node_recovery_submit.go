package cleanup

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jinzhu/gorm"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// NodeRecoverySpecHash accepts only the fixed receipt-only invocation with no
// business cache volume. Journal/source identity is checked by the template builder.
func NodeRecoverySpecHash(job *batchv1.Job, parent NodeJobBinding) (string, error) {
	if job == nil || job.Name != NodeRecoveryName(parent) || job.Namespace != parent.Namespace {
		return "", ErrCoordinationChanged
	}
	if _, err := NodeJobSpecHash(job, parent.NodeJobIntent); err != nil {
		return "", err
	}
	container := job.Spec.Template.Spec.Containers[0]
	required := map[string]int{"--recover": 0, "--original-pod=" + parent.PodName: 0, "--original-pod-uid=" + parent.PodUID: 0}
	for _, arg := range container.Args {
		if _, ok := required[arg]; ok {
			required[arg]++
		} else if strings.HasPrefix(strings.TrimLeft(arg, "-"), "recover") || strings.HasPrefix(strings.TrimLeft(arg, "-"), "original-pod") {
			return "", ErrCoordinationChanged
		}
	}
	for _, count := range required {
		if count != 1 {
			return "", ErrCoordinationChanged
		}
	}
	if len(container.VolumeMounts) != 2 || len(job.Spec.Template.Spec.Volumes) != 2 {
		return "", ErrCoordinationChanged
	}
	mounts := map[string]bool{}
	for _, mount := range container.VolumeMounts {
		if mounts[mount.Name] || mount.SubPath != "" || mount.SubPathExpr != "" {
			return "", ErrCoordinationChanged
		}
		if mount.Name == "node-state" && mount.MountPath == "/node-state" {
			mounts[mount.Name] = true
			continue
		}
		if mount.Name == "node-control" && mount.MountPath == "/node-control" && mount.ReadOnly {
			mounts[mount.Name] = true
			continue
		}
		return "", ErrCoordinationChanged
	}
	for _, volume := range job.Spec.Template.Spec.Volumes {
		if volume.Name == "node-state" && volume.PersistentVolumeClaim != nil {
			continue
		}
		if volume.Name == "node-control" && volume.Secret != nil {
			continue
		}
		return "", ErrCoordinationChanged
	}
	return gcSubmissionHash(job)
}

// SubmitSuspendedNodeRecovery persists recovery intent before its only Create.
func SubmitSuspendedNodeRecovery(ctx context.Context, database *gorm.DB, client NodeJobClient, r CoordinationRequest, template *batchv1.Job) (*batchv1.Job, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	parent, err := ReadNodeJobBinding(database, r)
	if err != nil {
		return nil, err
	}
	if client == nil || template == nil {
		return nil, ErrCoordinationChanged
	}
	if parent.Recovery != nil {
		return ReconcileNodeRecovery(ctx, database, client, r)
	}
	if template.UID != "" || template.ResourceVersion != "" || template.GenerateName != "" || len(template.OwnerReferences) > 0 || template.DeletionTimestamp != nil || template.Spec.Selector != nil || template.Spec.ManualSelector != nil || template.Spec.Suspend == nil || !*template.Spec.Suspend {
		return nil, ErrCoordinationChanged
	}
	if _, err := NodeRecoverySpecHash(template, parent); err != nil {
		return nil, err
	}
	preview, err := client.Create(ctx, template.DeepCopy(), metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
	if err != nil {
		return nil, err
	}
	if preview == nil || preview.Spec.Suspend == nil || !*preview.Spec.Suspend || !sameExecutorPreview(template.Spec.Template.Spec, preview.Spec.Template.Spec) {
		return nil, ErrCoordinationChanged
	}
	hash, err := NodeRecoverySpecHash(preview, parent)
	if err != nil {
		return nil, err
	}
	intent := NodeRecoveryBinding{Namespace: preview.Namespace, Name: preview.Name, SpecHash: hash}
	_, created, err := PrepareNodeRecovery(database, r, intent)
	if err != nil {
		return nil, err
	}
	if !created {
		return ReconcileNodeRecovery(ctx, database, client, r)
	}
	submitted := preview.DeepCopy()
	submitted.ObjectMeta = metav1.ObjectMeta{Name: intent.Name, Namespace: intent.Namespace, Labels: preview.Labels, Annotations: preview.Annotations}
	submitted.Status = batchv1.JobStatus{}
	stripGCControllerFields(submitted)
	job, err := client.Create(ctx, submitted, metav1.CreateOptions{})
	if err != nil {
		return nil, ErrCoordinationUncertain
	}
	return bindRecoveryJob(database, r, job)
}

// ReconcileNodeRecovery never recreates a missing or replaced recovery Job.
func ReconcileNodeRecovery(ctx context.Context, database *gorm.DB, client NodeJobClient, r CoordinationRequest) (*batchv1.Job, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	parent, err := ReadNodeJobBinding(database, r)
	if err != nil {
		return nil, err
	}
	if client == nil || parent.Recovery == nil {
		return nil, ErrCoordinationChanged
	}
	job, err := client.Get(ctx, parent.Recovery.Name, metav1.GetOptions{})
	if err != nil {
		return nil, ErrCoordinationUncertain
	}
	return bindRecoveryJob(database, r, job)
}
func bindRecoveryJob(database *gorm.DB, r CoordinationRequest, job *batchv1.Job) (*batchv1.Job, error) {
	parent, err := ReadNodeJobBinding(database, r)
	if err != nil {
		return nil, err
	}
	if parent.Recovery == nil || job == nil || job.UID == "" || job.DeletionTimestamp != nil || len(job.OwnerReferences) > 0 {
		return nil, ErrCoordinationChanged
	}
	intent := *parent.Recovery
	if intent.JobUID != "" && intent.JobUID != string(job.UID) {
		return nil, ErrCoordinationChanged
	}
	if intent.JobUID == "" && (job.Spec.Suspend == nil || !*job.Spec.Suspend) {
		return nil, ErrCoordinationChanged
	}
	hash, err := NodeRecoverySpecHash(job, parent)
	if err != nil || hash != intent.SpecHash {
		return nil, ErrCoordinationChanged
	}
	intent.JobUID = ""
	if err := BindNodeRecovery(database, r, intent, string(job.UID)); err != nil {
		return nil, err
	}
	return job, nil
}

// StartNodeRecovery starts only the already-bound original recovery helper.
// Callers must revalidate original termination and journal identity first.
func StartNodeRecovery(ctx context.Context, database *gorm.DB, client NodeJobStartClient, r CoordinationRequest) (*batchv1.Job, error) {
	job, err := ReconcileNodeRecovery(ctx, database, client, r)
	if err != nil {
		return nil, err
	}
	if job.Spec.Suspend == nil || job.ResourceVersion == "" || job.Status.Failed > 0 || job.Status.Succeeded > 0 {
		return nil, ErrCoordinationChanged
	}
	if !*job.Spec.Suspend {
		return job, nil
	}
	patch, _ := json.Marshal([]map[string]interface{}{{"op": "test", "path": "/metadata/uid", "value": job.UID}, {"op": "test", "path": "/metadata/resourceVersion", "value": job.ResourceVersion}, {"op": "test", "path": "/spec/suspend", "value": true}, {"op": "replace", "path": "/spec/suspend", "value": false}})
	if _, err := client.Patch(ctx, job.Name, types.JSONPatchType, patch, metav1.PatchOptions{}); err != nil {
		return nil, ErrCoordinationUncertain
	}
	return ReconcileNodeRecovery(ctx, database, client, r)
}
