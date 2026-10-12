package cleanup

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/db/model"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

// capability_id: rainbond.cleanup.node-job-submission
func TestNodeSubmissionReconcilesLostCreateWithoutRecreating(t *testing.T) {
	database, _ := coordinationDB(t)
	binding, err := ProvisionManagedCacheStorage(database, "cache-volume", "/cache/build")
	if err != nil {
		t.Fatal(err)
	}
	database.Model(&model.CleanupStorage{}).Where("storage_id = ?", binding.StorageID).Update("mode", "ready")
	intent := NodeJobIntent{Namespace: "system", NodeName: "node", NodeUID: "node-uid", Entry: "selected", Fingerprint: strings.Repeat("a", 64)}
	request := operation("node-submit", "delete", nodeJobScope(intent))
	request.StorageID = binding.StorageID
	request.Generation = binding.Generation
	request.Target = nodeJobTarget(intent)
	if _, err := AcquireOperation(database, request); err != nil {
		t.Fatal(err)
	}
	template := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Namespace: "system"}, Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{NodeName: "node", RestartPolicy: corev1.RestartPolicyNever, Containers: []corev1.Container{{Name: "node-cleanup", Image: "example.test/node@sha256:" + strings.Repeat("b", 64), Command: []string{"/app/node-cleanup"}}}}}}}
	client := fake.NewSimpleClientset()
	creates := 0
	jobs := gcTestJobClient{JobInterface: client.BatchV1().Jobs("system")}
	jobs.create = func(ctx context.Context, input *batchv1.Job, options metav1.CreateOptions) (*batchv1.Job, error) {
		job := input.DeepCopy()
		if len(options.DryRun) > 0 {
			return job, nil
		}
		creates++
		original, err := ReadNodeJobBinding(database, request)
		if err != nil || original.Name != job.Name || original.JobUID != "" {
			t.Fatal("creation preceded durable intent", err)
		}
		if job.Spec.Suspend == nil || !*job.Spec.Suspend {
			t.Fatal("job created runnable")
		}
		job.UID = types.UID("original-node-job")
		if err := client.Tracker().Create(batchv1.SchemeGroupVersion.WithResource("jobs"), job, job.Namespace); err != nil {
			t.Fatal(err)
		}
		return nil, errors.New("lost reply")
	}
	if _, err := SubmitSuspendedNodeJob(context.Background(), database, jobs, request, intent, template); !errors.Is(err, ErrCoordinationUncertain) {
		t.Fatal(err)
	}
	unbound, err := client.BatchV1().Jobs("system").Get(context.Background(), nodeJobName(request), metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	no := false
	unbound.Spec.Suspend = &no
	client.Tracker().Update(batchv1.SchemeGroupVersion.WithResource("jobs"), unbound, unbound.Namespace)
	if _, err := ReconcileNodeJob(context.Background(), database, jobs, request); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("unbound runnable Job adopted", err)
	}
	yes := true
	unbound.Spec.Suspend = &yes
	client.Tracker().Update(batchv1.SchemeGroupVersion.WithResource("jobs"), unbound, unbound.Namespace)
	job, err := SubmitSuspendedNodeJob(context.Background(), database, jobs, request, intent, template)
	if err != nil || job.UID != "original-node-job" || creates != 1 {
		t.Fatal("creation replayed", creates, err)
	}
	replacement := job.DeepCopy()
	replacement.UID = "replacement"
	client.Tracker().Update(batchv1.SchemeGroupVersion.WithResource("jobs"), replacement, replacement.Namespace)
	if _, err := ReconcileNodeJob(context.Background(), database, jobs, request); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("replacement Job adopted", err)
	}
	changed := job.DeepCopy()
	changed.Spec.Template.Spec.NodeName = "different-node"
	client.Tracker().Update(batchv1.SchemeGroupVersion.WithResource("jobs"), changed, changed.Namespace)
	if _, err := ReconcileNodeJob(context.Background(), database, jobs, request); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("changed Job spec accepted", err)
	}
	client.Tracker().Delete(batchv1.SchemeGroupVersion.WithResource("jobs"), job.Namespace, job.Name)
	if _, err := SubmitSuspendedNodeJob(context.Background(), database, jobs, request, intent, template); !errors.Is(err, ErrCoordinationUncertain) || creates != 1 {
		t.Fatal("missing original job recreated", err)
	}
}

// capability_id: rainbond.cleanup.executor-preview-authority
func TestNodeDryRunCannotChangeExecutorIdentity(t *testing.T) {
	for _, mutation := range []string{"image", "node", "environment", "mount", "volume", "lifecycle", "security", "envfrom"} {
		t.Run(mutation, func(t *testing.T) {
			database, _ := coordinationDB(t)
			binding, err := ProvisionManagedCacheStorage(database, "cache", "/cache/build")
			if err != nil {
				t.Fatal(err)
			}
			if err := database.Model(&model.CleanupStorage{}).Where("storage_id = ?", binding.StorageID).Update("mode", "ready").Error; err != nil {
				t.Fatal(err)
			}
			intent := NodeJobIntent{Namespace: "system", NodeName: "node", NodeUID: "uid", Entry: "entry", Fingerprint: strings.Repeat("a", 64)}
			r := operation("node-dryrun", "delete", nodeJobScope(intent))
			r.StorageID = binding.StorageID
			r.Generation = binding.Generation
			r.Target = nodeJobTarget(intent)
			if _, err := AcquireOperation(database, r); err != nil {
				t.Fatal(err)
			}
			template := suspendedGCFixture()
			template.Spec.Template.Spec.NodeName = "node"
			template.Spec.Template.Spec.Containers[0].Command = []string{"/app/node-cleanup"}
			jobs := gcTestJobClient{JobInterface: fake.NewSimpleClientset().BatchV1().Jobs("system")}
			jobs.create = func(_ context.Context, input *batchv1.Job, options metav1.CreateOptions) (*batchv1.Job, error) {
				if len(options.DryRun) == 0 {
					t.Fatal("invalid preview reached actual creation")
				}
				changed := input.DeepCopy()
				switch mutation {
				case "image":
					changed.Spec.Template.Spec.Containers[0].Image = "example.test/other@sha256:" + strings.Repeat("d", 64)
				case "node":
					changed.Spec.Template.Spec.NodeName = "other"
				case "environment":
					changed.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{{Name: "UNEXPECTED", Value: "changed"}}
				case "mount":
					changed.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{{Name: "injected", MountPath: "/node-state"}}
				case "volume":
					changed.Spec.Template.Spec.Volumes = []corev1.Volume{{Name: "injected", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "other"}}}}
				case "lifecycle":
					changed.Spec.Template.Spec.Containers[0].Lifecycle = &corev1.Lifecycle{PostStart: &corev1.LifecycleHandler{Exec: &corev1.ExecAction{Command: []string{"/bin/true"}}}}
				case "security":
					user := int64(123)
					changed.Spec.Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{RunAsUser: &user}
				case "envfrom":
					changed.Spec.Template.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "other"}}}}

				}
				return changed, nil
			}
			if _, err := SubmitSuspendedNodeJob(context.Background(), database, jobs, r, intent, template); !errors.Is(err, ErrCoordinationChanged) {
				t.Fatal("mutated preview accepted", err)
			}
			if _, err := ReadNodeJobBinding(database, r); !errors.Is(err, ErrNodeJobNotPrepared) {
				t.Fatal("rejected preview persisted intent", err)
			}
		})
	}
}

func TestNodePreviewAllowsOrdinaryKubernetesDefaults(t *testing.T) {
	original := corev1.PodSpec{Containers: []corev1.Container{{Name: "node", Image: "example.test/node@sha256:" + strings.Repeat("a", 64)}}}
	observed := original.DeepCopy()
	observed.ServiceAccountName = "default"
	observed.DeprecatedServiceAccount = "default"
	observed.SecurityContext = &corev1.PodSecurityContext{}
	c := &observed.Containers[0]
	c.ImagePullPolicy = corev1.PullIfNotPresent
	c.TerminationMessagePath = "/dev/termination-log"
	c.TerminationMessagePolicy = corev1.TerminationMessageReadFile
	c.SecurityContext = &corev1.SecurityContext{}
	if !sameExecutorPreview(original, *observed) {
		t.Fatal("ordinary defaults rejected")
	}
	observed.ServiceAccountName = "privileged"
	if sameExecutorPreview(original, *observed) {
		t.Fatal("different service account accepted")
	}
}
