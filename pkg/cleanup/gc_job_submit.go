package cleanup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jinzhu/gorm"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

// GCJobClient is a namespace-scoped Kubernetes Job client. Callers supply the
// client for the trusted executor namespace, never one selected by a browser.
type GCJobClient interface {
	Create(context.Context, *batchv1.Job, metav1.CreateOptions) (*batchv1.Job, error)
	Get(context.Context, string, metav1.GetOptions) (*batchv1.Job, error)
}

// SubmitSuspendedGCJob accepts only a server-built executor template. It does
// not authorize execution: volume verification and the one-use execution gate
// are still required. Dry-run defaulting cannot persist or start a Job.
func SubmitSuspendedGCJob(ctx context.Context, database *gorm.DB, client GCJobClient, r CoordinationRequest, template *batchv1.Job) (*batchv1.Job, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if client == nil {
		return nil, ErrCoordinationChanged
	}
	job, err := prepareGCSubmission(r, template)
	if err != nil {
		return nil, err
	}
	if _, err = ReadGCJobBinding(database, r); err == nil {
		return ReconcileGCJob(ctx, database, client, r)
	} else if !errors.Is(err, ErrGCJobNotPrepared) {
		return nil, err
	}
	defaulted, err := client.Create(ctx, job, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
	if err != nil {
		return nil, err
	}
	// Validate admission's result as well as the local template.
	if defaulted == nil || defaulted.Namespace != job.Namespace || defaulted.Name != job.Name || defaulted.Spec.Suspend == nil || !*defaulted.Spec.Suspend {
		return nil, ErrCoordinationChanged
	}
	if !sameExecutorPreview(job.Spec.Template.Spec, defaulted.Spec.Template.Spec) {
		return nil, ErrCoordinationChanged
	}
	hash, err := gcSubmissionHash(defaulted)
	if err != nil {
		return nil, err
	}
	intent, created, err := PrepareGCJob(database, r, job.Namespace, hash)
	if err != nil {
		return nil, err
	}
	if !created {
		return ReconcileGCJob(ctx, database, client, r)
	}
	// The intent is now durable. Any lost response, including cancellation, must
	// be reconciled by GET; it is never permission to issue another Create.
	submitted := defaulted.DeepCopy()
	submitted.ObjectMeta = metav1.ObjectMeta{Name: intent.Name, Namespace: intent.Namespace, Labels: defaulted.Labels, Annotations: defaulted.Annotations}
	submitted.Status = batchv1.JobStatus{}
	stripGCControllerFields(submitted)
	observed, err := client.Create(ctx, submitted, metav1.CreateOptions{})
	if err != nil {
		return nil, ErrCoordinationUncertain
	}
	return bindObservedGCJob(database, r, GCJobBinding{GCJobIntent: intent}, observed)
}

// ReconcileGCJob never creates, replaces, resumes or deletes a Kubernetes Job.
func ReconcileGCJob(ctx context.Context, database *gorm.DB, client GCJobClient, r CoordinationRequest) (*batchv1.Job, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if client == nil {
		return nil, ErrCoordinationChanged
	}
	binding, err := ReadGCJobBinding(database, r)
	if err != nil {
		return nil, err
	}
	job, err := client.Get(ctx, binding.Name, metav1.GetOptions{})
	if err != nil {
		return nil, ErrCoordinationUncertain
	}
	return bindObservedGCJob(database, r, binding, job)
}

func bindObservedGCJob(database *gorm.DB, r CoordinationRequest, binding GCJobBinding, job *batchv1.Job) (*batchv1.Job, error) {
	if job == nil || job.Namespace != binding.Namespace || job.Name != binding.Name || job.UID == "" || job.DeletionTimestamp != nil || len(job.OwnerReferences) > 0 {
		return nil, ErrCoordinationChanged
	}
	if binding.JobUID != "" && binding.JobUID != string(job.UID) {
		return nil, ErrCoordinationChanged
	}
	if binding.JobUID == "" && (job.Spec.Suspend == nil || !*job.Spec.Suspend) {
		return nil, ErrCoordinationChanged
	}
	hash, err := gcSubmissionHash(job)
	if err != nil || hash != binding.SpecHash {
		return nil, ErrCoordinationChanged
	}
	if err := BindGCJob(database, r, binding.GCJobIntent, string(job.UID)); err != nil {
		return nil, err
	}
	return job, nil
}

func prepareGCSubmission(r CoordinationRequest, template *batchv1.Job) (*batchv1.Job, error) {
	if !r.valid() || r.Kind != "gc" || template == nil {
		return nil, ErrCoordinationChanged
	}
	job := template.DeepCopy()
	if len(validation.IsDNS1123Label(job.Namespace)) > 0 || job.UID != "" || job.ResourceVersion != "" || job.GenerateName != "" || job.DeletionTimestamp != nil || len(job.OwnerReferences) > 0 || job.Spec.Selector != nil || job.Spec.ManualSelector != nil {
		return nil, ErrCoordinationChanged
	}
	if job.Name != "" && job.Name != gcJobName(r) {
		return nil, ErrCoordinationChanged
	}
	job.Name = gcJobName(r)
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
	if _, err := gcSubmissionHash(job); err != nil {
		return nil, err
	}
	return job, nil
}

var gcControllerLabels = []string{"controller-uid", "batch.kubernetes.io/controller-uid", "job-name", "batch.kubernetes.io/job-name"}

func stripGCControllerFields(job *batchv1.Job) {
	job.Spec.Selector = nil
	for _, key := range gcControllerLabels {
		delete(job.Spec.Template.Labels, key)
	}
	if len(job.Spec.Template.Labels) == 0 {
		job.Spec.Template.Labels = nil
	}
}

func gcSubmissionHash(job *batchv1.Job) (string, error) {
	spec := &job.Spec
	pod := &spec.Template.Spec
	if spec.BackoffLimit == nil || *spec.BackoffLimit != 0 || spec.Parallelism == nil || *spec.Parallelism != 1 || spec.Completions == nil || *spec.Completions != 1 || spec.TTLSecondsAfterFinished != nil || spec.ManualSelector != nil && *spec.ManualSelector || pod.RestartPolicy != corev1.RestartPolicyNever || pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken || pod.HostNetwork || pod.HostPID || pod.HostIPC || len(pod.Containers) != 1 {
		return "", ErrCoordinationChanged
	}
	for _, c := range append(append([]corev1.Container{}, pod.Containers...), pod.InitContainers...) {
		parts := strings.Split(c.Image, "@sha256:")
		if len(parts) != 2 || parts[0] == "" || len(parts[1]) != 64 {
			return "", ErrCoordinationChanged
		}
		raw, err := hex.DecodeString(parts[1])
		if err != nil || hex.EncodeToString(raw) != parts[1] {
			return "", ErrCoordinationChanged
		}
		if c.SecurityContext != nil && (c.SecurityContext.Privileged != nil && *c.SecurityContext.Privileged || c.SecurityContext.AllowPrivilegeEscalation != nil && *c.SecurityContext.AllowPrivilegeEscalation) {
			return "", ErrCoordinationChanged
		}
	}
	allowed := map[string]string{gcControllerLabels[0]: string(job.UID), gcControllerLabels[1]: string(job.UID), gcControllerLabels[2]: job.Name, gcControllerLabels[3]: job.Name}
	for key, want := range allowed {
		if got, ok := spec.Template.Labels[key]; ok && (want == "" || got != want) {
			return "", ErrCoordinationChanged
		}
	}
	if spec.Selector != nil {
		if len(spec.Selector.MatchExpressions) > 0 {
			return "", ErrCoordinationChanged
		}
		for key, got := range spec.Selector.MatchLabels {
			want, ok := allowed[key]
			if !ok || want == "" || got != want {
				return "", ErrCoordinationChanged
			}
		}
	}
	copy := job.DeepCopy()
	stripGCControllerFields(copy)
	yes := true
	copy.Spec.Suspend = &yes
	// API defaulting may express manualSelector=false explicitly.
	copy.Spec.ManualSelector = nil
	data, err := json.Marshal(copy.Spec)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
