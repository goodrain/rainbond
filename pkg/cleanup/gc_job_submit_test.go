package cleanup

import (
	"context"
	"errors"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	typedbatch "k8s.io/client-go/kubernetes/typed/batch/v1"
	ktesting "k8s.io/client-go/testing"
)

func suspendedGCFixture() *batchv1.Job {
	return &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Namespace: "system"}, Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{RestartPolicy: corev1.RestartPolicyNever, Containers: []corev1.Container{{Name: "gc", Image: "example.test/gated-executor@sha256:" + strings.Repeat("a", 64), Command: []string{"/registry-gc"}}}}}}}
}

// capability_id: rainbond.cleanup.gc-job-submission
func TestGCSubmissionPersistsBeforeCreateAndReconcilesLostResponse(t *testing.T) {
	database, _ := coordinationDB(t)
	request := operation("submitted-gc", "gc", "*")
	if _, err := RequestMaintenance(database, request); err != nil {
		t.Fatal(err)
	}
	client := fake.NewSimpleClientset()
	creates := 0
	jobClient := gcTestJobClient{JobInterface: client.BatchV1().Jobs("system")}
	jobClient.create = func(ctx context.Context, input *batchv1.Job, options metav1.CreateOptions) (*batchv1.Job, error) {
		job := input.DeepCopy()
		if len(options.DryRun) > 0 {
			return job, nil
		}
		creates++
		binding, err := ReadGCJobBinding(database, request)
		if err != nil || binding.Name != job.Name || binding.JobUID != "" {
			t.Fatal("Job submitted before durable intent", err)
		}
		if job.Spec.Suspend == nil || !*job.Spec.Suspend {
			t.Fatal("Job submitted runnable")
		}
		job.UID = types.UID("original-job")
		if err := client.Tracker().Create(batchv1.SchemeGroupVersion.WithResource("jobs"), job, job.Namespace); err != nil {
			t.Fatal(err)
		}
		return nil, errors.New("lost create response")
	}
	if _, err := SubmitSuspendedGCJob(context.Background(), database, jobClient, request, suspendedGCFixture()); !errors.Is(err, ErrCoordinationUncertain) {
		t.Fatal("lost response not uncertain", err)
	}
	job, err := SubmitSuspendedGCJob(context.Background(), database, jobClient, request, suspendedGCFixture())
	if err != nil || job.UID != "original-job" || creates != 1 {
		t.Fatal("creation replayed instead of reconciled", creates, err)
	}
	if err := client.Tracker().Delete(batchv1.SchemeGroupVersion.WithResource("jobs"), job.Namespace, job.Name); err != nil {
		t.Fatal(err)
	}
	if _, err := SubmitSuspendedGCJob(context.Background(), database, jobClient, request, suspendedGCFixture()); !errors.Is(err, ErrCoordinationUncertain) {
		t.Fatal("missing Job recreated", err)
	}
	if creates != 1 {
		t.Fatal("missing job retried")
	}
	replacement := job.DeepCopy()
	replacement.UID = "replacement"
	if err := client.Tracker().Create(batchv1.SchemeGroupVersion.WithResource("jobs"), replacement, replacement.Namespace); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileGCJob(context.Background(), database, jobClient, request); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("replacement accepted", err)
	}
}

func TestGCSubmissionRejectsUnsafeRetrySettings(t *testing.T) {
	for _, kind := range []string{"restart", "backoff", "ttl", "mutable-image"} {
		t.Run(kind, func(t *testing.T) {
			database, _ := coordinationDB(t)
			request := operation("gc", "gc", "*")
			if _, err := RequestMaintenance(database, request); err != nil {
				t.Fatal(err)
			}
			job := suspendedGCFixture()
			one := int32(1)
			switch kind {
			case "restart":
				job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyOnFailure
			case "backoff":
				job.Spec.BackoffLimit = &one
			case "ttl":
				job.Spec.TTLSecondsAfterFinished = &one
			case "mutable-image":
				job.Spec.Template.Spec.Containers[0].Image = "example.test/registry:latest"
			}
			client := fake.NewSimpleClientset()
			if _, err := SubmitSuspendedGCJob(context.Background(), database, client.BatchV1().Jobs("system"), request, job); err == nil {
				t.Fatal("unsafe template accepted")
			}
			if len(client.Actions()) != 0 {
				t.Fatal("unsafe template reached Kubernetes")
			}
		})
	}
}

// The vendored fake drops CreateOptions, so retain them at the typed boundary.
type gcTestJobClient struct {
	typedbatch.JobInterface
	create func(context.Context, *batchv1.Job, metav1.CreateOptions) (*batchv1.Job, error)
}

func (c gcTestJobClient) Create(ctx context.Context, j *batchv1.Job, o metav1.CreateOptions) (*batchv1.Job, error) {
	return c.create(ctx, j, o)
}

func TestGCSubmissionDefaultingAndMutation(t *testing.T) {
	for _, mutation := range []string{"none", "image", "selector", "namespace", "unsuspended-unbound"} {
		t.Run(mutation, func(t *testing.T) {
			database, _ := coordinationDB(t)
			r := operation("gc-defaults", "gc", "*")
			if _, err := RequestMaintenance(database, r); err != nil {
				t.Fatal(err)
			}
			client := fake.NewSimpleClientset()
			jobs := gcTestJobClient{JobInterface: client.BatchV1().Jobs("system")}
			var saved *batchv1.Job
			jobs.create = func(ctx context.Context, input *batchv1.Job, options metav1.CreateOptions) (*batchv1.Job, error) {
				job := input.DeepCopy()
				job.UID = "dry-run-uid"
				if len(options.DryRun) == 0 {
					job.UID = "actual-uid"
				}
				job.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirst
				job.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"batch.kubernetes.io/controller-uid": string(job.UID)}}
				job.Spec.Template.Labels = map[string]string{"batch.kubernetes.io/controller-uid": string(job.UID), "batch.kubernetes.io/job-name": job.Name}
				if len(options.DryRun) > 0 {
					return job, nil
				}
				saved = job.DeepCopy()
				if mutation == "unsuspended-unbound" {
					return nil, errors.New("lost response")
				}
				return job, nil
			}
			job, err := SubmitSuspendedGCJob(context.Background(), database, jobs, r, suspendedGCFixture())
			if mutation == "unsuspended-unbound" {
				if !errors.Is(err, ErrCoordinationUncertain) {
					t.Fatal(err)
				}
				job = saved
			} else if err != nil {
				t.Fatal("defaulted job rejected", err)
			}
			switch mutation {
			case "image":
				job.Spec.Template.Spec.Containers[0].Image = "example.test/other@sha256:" + strings.Repeat("b", 64)
			case "selector":
				job.Spec.Selector.MatchLabels["foreign"] = "serving"
			case "namespace":
				job.Namespace = "other"
			case "unsuspended-unbound":
				no := false
				job.Spec.Suspend = &no
			}
			// The client can return an unexpected namespace; binding must reject it.
			client.PrependReactor("get", "jobs", func(action ktesting.Action) (bool, runtime.Object, error) { return true, job.DeepCopy(), nil })
			_, err = ReconcileGCJob(context.Background(), database, jobs, r)
			if mutation == "none" && err != nil {
				t.Fatal(err)
			}
			if mutation != "none" && !errors.Is(err, ErrCoordinationChanged) {
				t.Fatal("mutation not rejected", err)
			}
		})
	}
}

func TestGCSubmissionCanceledAndDryRunFailureLeaveNoIntent(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		database, _ := coordinationDB(t)
		r := operation("gc-cancel", "gc", "*")
		if _, err := RequestMaintenance(database, r); err != nil {
			t.Fatal(err)
		}
		ctx, stop := context.WithCancel(context.Background())
		if cancel {
			stop()
		}
		calls := 0
		client := gcTestJobClient{create: func(ctx context.Context, j *batchv1.Job, o metav1.CreateOptions) (*batchv1.Job, error) {
			calls++
			return nil, errors.New("dry-run unavailable")
		}}
		if _, err := SubmitSuspendedGCJob(ctx, database, client, r, suspendedGCFixture()); err == nil {
			t.Fatal("failure ignored")
		}
		stop()
		if cancel && calls != 0 {
			t.Fatal("canceled request reached Kubernetes")
		}
		if _, err := ReadGCJobBinding(database, r); !errors.Is(err, ErrGCJobNotPrepared) {
			t.Fatal("failed dry-run persisted intent", err)
		}
	}
}

// capability_id: rainbond.cleanup.executor-preview-authority
func TestGCPreviewCannotInjectLifecycleOrStorage(t *testing.T) {
	for _, kind := range []string{"lifecycle", "mount", "volume", "environment"} {
		t.Run(kind, func(t *testing.T) {
			database, _ := coordinationDB(t)
			request := operation("preview-authority", "gc", "*")
			if _, err := RequestMaintenance(database, request); err != nil {
				t.Fatal(err)
			}
			jobs := gcTestJobClient{JobInterface: fake.NewSimpleClientset().BatchV1().Jobs("system")}
			jobs.create = func(_ context.Context, input *batchv1.Job, options metav1.CreateOptions) (*batchv1.Job, error) {
				if len(options.DryRun) == 0 {
					t.Fatal("injected executor reached creation")
				}
				changed := input.DeepCopy()
				container := &changed.Spec.Template.Spec.Containers[0]
				switch kind {
				case "lifecycle":
					container.Lifecycle = &corev1.Lifecycle{PostStart: &corev1.LifecycleHandler{Exec: &corev1.ExecAction{Command: []string{"/bin/true"}}}}
				case "mount":
					container.VolumeMounts = []corev1.VolumeMount{{Name: "other", MountPath: "/registry"}}
				case "volume":
					changed.Spec.Template.Spec.Volumes = []corev1.Volume{{Name: "other", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "other"}}}}
				case "environment":
					container.EnvFrom = []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "other"}}}}
				}
				return changed, nil
			}
			if _, err := SubmitSuspendedGCJob(context.Background(), database, jobs, request, suspendedGCFixture()); !errors.Is(err, ErrCoordinationChanged) {
				t.Fatal("injected preview accepted", err)
			}
		})
	}
}
