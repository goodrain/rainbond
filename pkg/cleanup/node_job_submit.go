package cleanup

import (
	"context"
	"errors"
	"strings"

	"github.com/jinzhu/gorm"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeJobClient is the trusted namespace's typed Kubernetes Job client.
type NodeJobClient = GCJobClient

// NodeJobSpecHash validates and fingerprints the immutable executor spec.
func NodeJobSpecHash(job *batchv1.Job, intent NodeJobIntent) (string, error) {
	if job == nil || job.Namespace != intent.Namespace || job.Spec.Template.Spec.NodeName != intent.NodeName || len(job.Spec.Template.Spec.InitContainers) != 0 || len(job.Spec.Template.Spec.EphemeralContainers) != 0 || len(job.Spec.Template.Spec.Containers) != 1 {
		return "", ErrCoordinationChanged
	}
	command := job.Spec.Template.Spec.Containers[0].Command
	if len(command) != 1 || command[0] != "/app/node-cleanup" {
		return "", ErrCoordinationChanged
	}
	// This canonicalizer covers Kubernetes defaults and controller-owned labels,
	// not GC operation state. Node execution retains its separate durable binding.
	return gcSubmissionHash(job)
}

// SubmitSuspendedNodeJob persists one server-built execution intent before the
// actual Create. It never starts the executor or grants native deletion.
func SubmitSuspendedNodeJob(ctx context.Context, database *gorm.DB, client NodeJobClient, r CoordinationRequest, intent NodeJobIntent, template *batchv1.Job) (*batchv1.Job, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if client == nil || template == nil {
		return nil, ErrCoordinationChanged
	}
	if intent.Name != "" && intent.Name != nodeJobName(r) {
		return nil, ErrCoordinationChanged
	}
	intent.Name = nodeJobName(r)
	probe := intent
	if probe.SpecHash == "" {
		probe.SpecHash = strings.Repeat("0", 64)
	}
	if !validNodeJobIntent(r, probe) {
		return nil, ErrCoordinationChanged
	}
	if previous, err := ReadNodeJobBinding(database, r); err == nil {
		if intent.SpecHash == "" {
			intent.SpecHash = previous.SpecHash
		}
		if intent != previous.NodeJobIntent {
			return nil, ErrCoordinationChanged
		}
		return ReconcileNodeJob(ctx, database, client, r)
	} else if !errors.Is(err, ErrNodeJobNotPrepared) {
		return nil, err
	}
	job := template.DeepCopy()
	if job.Namespace != intent.Namespace || (job.Name != "" && job.Name != intent.Name) || job.UID != "" || job.ResourceVersion != "" || job.GenerateName != "" || job.DeletionTimestamp != nil || len(job.OwnerReferences) > 0 || job.Spec.Selector != nil || job.Spec.ManualSelector != nil {
		return nil, ErrCoordinationChanged
	}
	job.Name = intent.Name
	yes, no := true, false
	zero, one := int32(0), int32(1)
	if job.Spec.BackoffLimit == nil {
		job.Spec.BackoffLimit = &zero
	}
	if job.Spec.Parallelism == nil {
		job.Spec.Parallelism = &one
	}
	if job.Spec.Completions == nil {
		job.Spec.Completions = &one
	}
	job.Spec.Suspend = &yes
	if job.Spec.Template.Spec.AutomountServiceAccountToken == nil {
		job.Spec.Template.Spec.AutomountServiceAccountToken = &no
	}
	if _, err := NodeJobSpecHash(job, intent); err != nil {
		return nil, err
	}
	defaulted, err := client.Create(ctx, job, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
	if err != nil {
		return nil, err
	}
	if defaulted == nil || defaulted.Name != job.Name || defaulted.Spec.Suspend == nil || !*defaulted.Spec.Suspend {
		return nil, ErrCoordinationChanged
	}
	hash, err := NodeJobSpecHash(defaulted, intent)
	if err != nil {
		return nil, err
	}
	if !sameExecutorPreview(job.Spec.Template.Spec, defaulted.Spec.Template.Spec) {
		return nil, ErrCoordinationChanged
	}
	if intent.SpecHash != "" && intent.SpecHash != hash {
		return nil, ErrCoordinationChanged
	}
	intent.SpecHash = hash
	prepared, created, err := PrepareNodeJob(database, r, intent)
	if err != nil {
		return nil, err
	}
	if !created {
		return ReconcileNodeJob(ctx, database, client, r)
	}
	submitted := defaulted.DeepCopy()
	submitted.ObjectMeta = metav1.ObjectMeta{Name: prepared.Name, Namespace: prepared.Namespace, Labels: defaulted.Labels, Annotations: defaulted.Annotations}
	submitted.Status = batchv1.JobStatus{}
	stripGCControllerFields(submitted)
	observedJob, err := client.Create(ctx, submitted, metav1.CreateOptions{})
	if err != nil {
		return nil, ErrCoordinationUncertain
	}
	return bindObservedNodeJob(database, r, NodeJobBinding{Protocol: 1, NodeJobIntent: prepared}, observedJob)
}

// ReconcileNodeJob only observes the original Job, including after response loss.
func ReconcileNodeJob(ctx context.Context, database *gorm.DB, client NodeJobClient, r CoordinationRequest) (*batchv1.Job, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if client == nil {
		return nil, ErrCoordinationChanged
	}
	binding, err := ReadNodeJobBinding(database, r)
	if err != nil {
		return nil, err
	}
	job, err := client.Get(ctx, binding.Name, metav1.GetOptions{})
	if err != nil {
		return nil, ErrCoordinationUncertain
	}
	return bindObservedNodeJob(database, r, binding, job)
}
func bindObservedNodeJob(database *gorm.DB, r CoordinationRequest, binding NodeJobBinding, job *batchv1.Job) (*batchv1.Job, error) {
	if job == nil || job.Name != binding.Name || job.Namespace != binding.Namespace || job.UID == "" || job.DeletionTimestamp != nil || len(job.OwnerReferences) > 0 {
		return nil, ErrCoordinationChanged
	}
	if binding.JobUID != "" && binding.JobUID != string(job.UID) {
		return nil, ErrCoordinationChanged
	}
	if binding.JobUID == "" && (job.Spec.Suspend == nil || !*job.Spec.Suspend) {
		return nil, ErrCoordinationChanged
	}
	hash, err := NodeJobSpecHash(job, binding.NodeJobIntent)
	if err != nil || hash != binding.SpecHash {
		return nil, ErrCoordinationChanged
	}
	if err := BindNodeJob(database, r, binding.NodeJobIntent, string(job.UID)); err != nil {
		return nil, err
	}
	return job, nil
}

// ValidateBoundNodeJob verifies an already-bound original Job without adoption.
func ValidateBoundNodeJob(job *batchv1.Job, binding NodeJobBinding) error {
	if job == nil || binding.Protocol != 1 || binding.JobUID == "" || string(job.UID) != binding.JobUID || job.Name != binding.Name || job.Namespace != binding.Namespace || job.DeletionTimestamp != nil || len(job.OwnerReferences) != 0 {
		return ErrCoordinationChanged
	}
	hash, err := NodeJobSpecHash(job, binding.NodeJobIntent)
	if err != nil || hash != binding.SpecHash {
		return ErrCoordinationChanged
	}
	return nil
}
