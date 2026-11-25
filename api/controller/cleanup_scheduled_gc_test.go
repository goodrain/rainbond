package controller

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/cleanup/kubeidentity"
	"github.com/jinzhu/gorm"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func scheduledGCFixture(t *testing.T) (*CleanupCoordinationHandler, *gorm.DB, *fake.Clientset, guard.StorageRegistration) {
	t.Helper()
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "scheduled.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	database.DB().SetMaxOpenConns(1)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}).Error; err != nil {
		t.Fatal(err)
	}
	kube, _, volume := gcAdmissionFixture(t)
	binding, err := guard.ProvisionRegistryStorage(database, volume, "/registry")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&model.CleanupStorage{}).Where("storage_id = ?", binding.StorageID).Update("mode", "ready").Error; err != nil {
		t.Fatal(err)
	}
	hub, err := kube.CoreV1().Pods("system").Get(context.Background(), "hub", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for i, arg := range hub.Spec.Containers[1].Args {
		if strings.HasPrefix(arg, "--storage-id=") {
			hub.Spec.Containers[1].Args[i] = "--storage-id=" + binding.StorageID
		}
		if strings.HasPrefix(arg, "--storage-generation=") {
			hub.Spec.Containers[1].Args[i] = "--storage-generation=" + binding.Generation
		}
	}
	if _, err := kube.CoreV1().Pods("system").Update(context.Background(), hub, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	h := &CleanupCoordinationHandler{
		database: func() *gorm.DB { return database },
		gcTarget: func() (kubernetes.Interface, string, string, error) {
			return gcAdmissionKubeClient{Interface: kube}, "system", "rbd-hub", nil
		},
		inspectRegistry: func(ctx context.Context, pod, uid string) (kubeidentity.RegistryPreparation, error) {
			return kubeidentity.InspectNativeRegistry(ctx, kube, "system", "rbd-hub", pod, uid)
		},
	}
	return h, database, kube, binding
}

func noScheduledVersions(context.Context, guard.StorageRegistration, string, uint) error { return nil }

const scheduledDailySlot = "daily:2026-11-25T03:00:00+08:00"

// capability_id: rainbond.cleanup.scheduled-gc-preflight
func TestScheduledGCRejectsUnqualifiedRegistryBeforeMutation(t *testing.T) {
	for _, scenario := range []string{"legacy-cleaner", "missing-binding", "missing-service", "zero-keep"} {
		t.Run(scenario, func(t *testing.T) {
			h, database, kube, _ := scheduledGCFixture(t)
			keep := uint(5)
			switch scenario {
			case "legacy-cleaner":
				pod, _ := kube.CoreV1().Pods("system").Get(context.Background(), "chaos", metav1.GetOptions{})
				pod.Spec.Containers[0].Args = []string{"--clean-up=true"}
				_, _ = kube.CoreV1().Pods("system").Update(context.Background(), pod, metav1.UpdateOptions{})
			case "missing-binding":
				if err := database.Delete(&model.CleanupStorage{}).Error; err != nil {
					t.Fatal(err)
				}
			case "missing-service":
				if err := kube.CoreV1().Services("system").Delete(context.Background(), "rbd-hub", metav1.DeleteOptions{}); err != nil {
					t.Fatal(err)
				}
			case "zero-keep":
				keep = 0
			}
			called := false
			err := h.runScheduledCleanup(context.Background(), scheduledDailySlot, keep, func(context.Context, guard.StorageRegistration, string, uint) error { called = true; return nil })
			if err == nil || called {
				t.Fatal("unqualified cleanup reached version mutation", err, called)
			}
			var operations int
			if err := database.Model(&model.CleanupOperation{}).Count(&operations).Error; err != nil || operations != 0 {
				t.Fatal("preflight created an operation", operations, err)
			}
			jobs, _ := kube.BatchV1().Jobs("system").List(context.Background(), metav1.ListOptions{})
			if len(jobs.Items) != 0 {
				t.Fatal("preflight created a Job")
			}
		})
	}
	t.Run("rejected-trigger-and-canceled-caller", func(t *testing.T) {
		h, database, _, _ := scheduledGCFixture(t)
		if err := h.RunScheduledCleanup(context.Background(), scheduledDailySlot, 0); err == nil {
			t.Fatal("zero retention accepted")
		}
		for _, trigger := range []string{"", "daily:invalid", "startup:untrusted-owner"} {
			if err := h.runScheduledCleanup(context.Background(), trigger, 5, noScheduledVersions); err == nil {
				t.Fatal("untrusted trigger accepted", trigger)
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := h.runScheduledCleanup(ctx, scheduledDailySlot, 5, noScheduledVersions); !errors.Is(err, context.Canceled) {
			t.Fatal("canceled caller created cleanup", err)
		}
		if err := h.runScheduledCleanup(context.Background(), scheduledDailySlot, 5, func(context.Context, guard.StorageRegistration, string, uint) error { return guard.ErrCoordinationBusy }); !errors.Is(err, guard.ErrCoordinationBusy) {
			t.Fatal("version-retention refusal ignored", err)
		}
		ctx, cancel = context.WithCancel(context.Background())
		if err := h.runScheduledCleanup(ctx, scheduledDailySlot, 5, func(context.Context, guard.StorageRegistration, string, uint) error { cancel(); return nil }); !errors.Is(err, context.Canceled) {
			t.Fatal("cancellation after version cleanup ignored", err)
		}
		var operations int
		if err := database.Model(&model.CleanupOperation{}).Count(&operations).Error; err != nil || operations != 0 {
			t.Fatal("rejected work created a maintenance drain", operations, err)
		}
	})
}

// capability_id: rainbond.cleanup.scheduled-gc-slots
func TestScheduledGCStartupSharesDaemonSetRolloutIdentity(t *testing.T) {
	h, _, kube, binding := scheduledGCFixture(t)
	controller := true
	ds := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "rbd-chaos", Namespace: "system", UID: "chaos-daemonset", ResourceVersion: "1", Generation: 4}}
	if _, err := kube.AppsV1().DaemonSets("system").Create(context.Background(), ds, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	pod, _ := kube.CoreV1().Pods("system").Get(context.Background(), "chaos", metav1.GetOptions{})
	pod.OwnerReferences = []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "DaemonSet", Name: ds.Name, UID: ds.UID, Controller: &controller}}
	_, _ = kube.CoreV1().Pods("system").Update(context.Background(), pod, metav1.UpdateOptions{})
	first, err := h.scheduledGCTrigger(context.Background(), "startup")
	if err != nil {
		t.Fatal(err)
	}
	second, err := h.scheduledGCTrigger(context.Background(), "startup")
	if err != nil || first != second {
		t.Fatal("replicas did not share startup slot", first, second, err)
	}
	ds.Generation++
	_, _ = kube.AppsV1().DaemonSets("system").Update(context.Background(), ds, metav1.UpdateOptions{})
	next, err := h.scheduledGCTrigger(context.Background(), "startup")
	if err != nil || next == first {
		t.Fatal("new rollout reused startup slot", next, err)
	}
	r1 := scheduledGCRequest(binding, first, 5)
	r2 := scheduledGCRequest(binding, second, 5)
	r3 := scheduledGCRequest(binding, next, 5)
	if r1 != r2 || r1.OperationID == r3.OperationID {
		t.Fatal("unstable maintenance identity")
	}
	if daily, err := h.scheduledGCTrigger(context.Background(), scheduledDailySlot); err != nil || daily != scheduledDailySlot {
		t.Fatal("daily slot changed", daily, err)
	}
	pod.OwnerReferences[0].UID = "replaced-owner"
	_, _ = kube.CoreV1().Pods("system").Update(context.Background(), pod, metav1.UpdateOptions{})
	if _, err := h.scheduledGCTrigger(context.Background(), "startup"); err == nil {
		t.Fatal("foreign DaemonSet accepted")
	}
	pod.OwnerReferences = nil
	_, _ = kube.CoreV1().Pods("system").Update(context.Background(), pod, metav1.UpdateOptions{})
	if _, err := h.scheduledGCTrigger(context.Background(), "startup"); err == nil {
		t.Fatal("ownerless startup accepted")
	}
}

// capability_id: rainbond.cleanup.scheduled-gc-cancel-drain
func TestScheduledGCCanceledDrainDoesNotStartOrReleaseWriter(t *testing.T) {
	h, database, kube, binding := scheduledGCFixture(t)
	writer := guard.CoordinationRequest{StorageID: binding.StorageID, Generation: binding.Generation, OperationID: "old-push", Owner: "builder", Kind: "producer", Scope: "*", Fingerprint: "old-push"}
	if _, err := guard.AcquireOperation(database, writer); err != nil {
		t.Fatal(err)
	}
	request := scheduledGCRequest(binding, scheduledDailySlot, 5)
	if err := h.runScheduledCleanup(context.Background(), scheduledDailySlot, 5, noScheduledVersions); !errors.Is(err, guard.ErrCoordinationBusy) {
		t.Fatal("scheduled GC interrupted an admitted build", err)
	}
	if state, err := guard.InspectOperation(database, writer); err != nil || state != "active" {
		t.Fatal("existing push was released", state, err)
	}
	observation, err := guard.InspectStorage(database, binding.StorageID, binding.Generation)
	if err != nil || observation.Mode != "ready" {
		t.Fatal("busy scheduled cleanup closed write admission", observation, err)
	}
	jobs, _ := kube.BatchV1().Jobs("system").List(context.Background(), metav1.ListOptions{})
	if len(jobs.Items) != 0 {
		t.Fatal("active build caused a GC Job submission")
	}
	httpWrite := guard.CoordinationRequest{StorageID: binding.StorageID, Generation: binding.Generation, OperationID: "next-http", Owner: "proxy", Kind: "producer", Scope: "app/a", Fingerprint: "next-http"}
	if _, err := guard.AcquireOperation(database, httpWrite); err != nil {
		t.Fatal("active build could not continue Registry requests", err)
	}
	if err := guard.FinishOperation(database, httpWrite, true); err != nil {
		t.Fatal(err)
	}
	if err := guard.FinishOperation(database, writer, true); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	kube.PrependReactor("create", "jobs", func(action ktesting.Action) (bool, runtime.Object, error) {
		job := action.(ktesting.CreateAction).GetObject().(*batchv1.Job).DeepCopy()
		if err := kube.Tracker().Add(job); err != nil {
			return true, nil, err
		}
		cancel()
		return true, job, nil
	})
	if err := h.runScheduledCleanup(ctx, scheduledDailySlot, 5, noScheduledVersions); !errors.Is(err, context.Canceled) {
		t.Fatal("unstarted Job cancellation not propagated", err)
	}
	waitScheduledState(t, database, request, "finished")
	observation, err = guard.InspectStorage(database, binding.StorageID, binding.Generation)
	if err != nil || observation.Mode != "ready" {
		t.Fatal("canceled unstarted Job left writes closed", observation, err)
	}
	jobs, _ = kube.BatchV1().Jobs("system").List(context.Background(), metav1.ListOptions{})
	if len(jobs.Items) != 1 || jobs.Items[0].Spec.Suspend == nil || !*jobs.Items[0].Spec.Suspend {
		t.Fatal("writer drain started native GC")
	}
}

// capability_id: rainbond.cleanup.scheduled-gc-verified-completion
func TestScheduledGCVerifiedCompletionIsIdempotent(t *testing.T) {
	for _, outcome := range []string{"succeeded", "failed", "delayed-termination", "unknown", "canceled-after-grant", "changed-source", "failed-before-grant"} {
		t.Run(outcome, func(t *testing.T) {
			h, database, kube, binding := scheduledGCFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			request := scheduledGCRequest(binding, scheduledDailySlot, 5)
			installScheduledGCExecutor(t, database, kube, binding, request, outcome, cancel)
			calls := 0
			cleanupVersions := func(context.Context, guard.StorageRegistration, string, uint) error { calls++; return nil }
			err := h.runScheduledCleanup(ctx, scheduledDailySlot, 5, cleanupVersions)
			progress, readErr := guard.ReadGCJobProgress(database, request)
			if readErr != nil || calls != 1 {
				t.Fatal("missing durable receipt or version selection", progress, readErr, calls)
			}
			observation, readErr := guard.InspectStorage(database, binding.StorageID, binding.Generation)
			if readErr != nil {
				t.Fatal(readErr)
			}
			switch outcome {
			case "succeeded", "failed", "delayed-termination":
				expected := outcome
				if outcome == "delayed-termination" {
					expected = "succeeded"
				}
				if (err == nil) != (expected == "succeeded") || progress.State != "finished" || progress.Outcome != expected || observation.Mode != "ready" || progress.Before == nil || progress.After == nil {
					t.Fatal("unverified completion", progress, observation, err)
				}
				err = h.runScheduledCleanup(context.Background(), scheduledDailySlot, 5, cleanupVersions)
				if (err == nil) != (expected == "succeeded") || calls != 1 {
					t.Fatal("same slot repeated version cleanup or changed outcome", err, calls)
				}
			case "failed-before-grant":
				if err == nil || progress.State != "finished" || progress.Outcome != "canceled" || observation.Mode != "ready" || progress.Before != nil || progress.After != nil {
					t.Fatal("failed ungranted helper invented GC result", progress, observation, err)
				}
			default:
				if err == nil || progress.State == "finished" || observation.Mode == "ready" {
					t.Fatal("ambiguous/changed executor opened writes", progress, observation, err)
				}
			}
			jobs, _ := kube.BatchV1().Jobs("system").List(context.Background(), metav1.ListOptions{})
			if len(jobs.Items) != 1 {
				t.Fatal("same operation recreated Job", len(jobs.Items))
			}
		})
	}
}

func waitScheduledState(t *testing.T, database *gorm.DB, request guard.CoordinationRequest, expected string) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		if state, err := guard.InspectOperation(database, request); err == nil && state == expected {
			return
		}
		select {
		case <-deadline:
			t.Fatal("scheduled operation did not reach", expected)
		case <-time.After(time.Millisecond):
		}
	}
}

// The fake helper uses the actual durable admission/measurement protocol. It
// cannot make the scheduler unlock a different Job or fabricate success.
func installScheduledGCExecutor(t *testing.T, database *gorm.DB, kube *fake.Clientset, binding guard.StorageRegistration, request guard.CoordinationRequest, outcome string, cancel context.CancelFunc) {
	t.Helper()
	kube.PrependReactor("patch", "jobs", func(action ktesting.Action) (bool, runtime.Object, error) {
		name := action.(ktesting.PatchAction).GetName()
		resource := batchv1.SchemeGroupVersion.WithResource("jobs")
		value, err := kube.Tracker().Get(resource, "system", name)
		if err != nil {
			return true, nil, err
		}
		job := value.(*batchv1.Job).DeepCopy()
		no := false
		job.Spec.Suspend = &no
		job.ResourceVersion = "2"
		if err := kube.Tracker().Update(resource, job, "system"); err != nil {
			return true, nil, err
		}
		if outcome == "failed-before-grant" {
			job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue}}
			_ = kube.Tracker().Update(resource, job, "system")
			return true, job, nil
		}
		yes := true
		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "scheduled-gc-pod", Namespace: "system", UID: "scheduled-gc-pod-uid", ResourceVersion: "1", OwnerReferences: []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "Job", Name: job.Name, UID: job.UID, Controller: &yes}}}, Spec: *job.Spec.Template.Spec.DeepCopy(), Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Name: "gc", ImageID: job.Spec.Template.Spec.Containers[0].Image, ContainerID: "containerd://scheduled-gc", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}}}
		if err := kube.Tracker().Add(pod); err != nil {
			t.Fatal(err)
		}
		if err := guard.BindGCExecutor(database, request, string(job.UID), pod.Name, string(pod.UID)); err != nil {
			t.Fatal(err)
		}
		if err := guard.EnterGCJobExecution(database, request, string(job.UID), string(pod.UID)); err != nil {
			t.Fatal(err)
		}
		if outcome == "canceled-after-grant" {
			cancel()
			return true, job, nil
		}
		if outcome != "unknown" {
			fingerprint, _ := binding.Fingerprint()
			before := guard.StorageMeasurement{Protocol: 1, StorageID: binding.StorageID, Generation: binding.Generation, BindingFingerprint: fingerprint, FilesystemID: "owned-filesystem", ObservedAt: time.Now().UTC(), TotalBytes: 1000, FreeBytes: 100, AvailableBytes: 50}
			if err := guard.RecordMaintenanceMeasurement(database, request, "before", before); err != nil {
				t.Fatal(err)
			}
			knownOutcome := outcome
			if outcome == "changed-source" || outcome == "delayed-termination" {
				knownOutcome = "succeeded"
			}
			if err := guard.CompleteMaintenanceWork(database, request, knownOutcome); err != nil {
				t.Fatal(err)
			}
			after := before
			after.ObservedAt = before.ObservedAt.Add(time.Second)
			after.AvailableBytes = 100
			if err := guard.RecordMaintenanceMeasurement(database, request, "after", after); err != nil {
				t.Fatal(err)
			}
		}
		terminated := pod.DeepCopy()
		terminated.Status.Phase = corev1.PodSucceeded
		terminated.Status.ContainerStatuses[0].State = corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{FinishedAt: metav1.Now()}}
		if outcome == "delayed-termination" {
			reads := 0
			kube.PrependReactor("get", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
				if action.(ktesting.GetAction).GetName() != pod.Name {
					return false, nil, nil
				}
				reads++
				resource := corev1.SchemeGroupVersion.WithResource("pods")
				observed, err := kube.Tracker().Get(resource, "system", pod.Name)
				if reads == 2 {
					if state, err := guard.InspectStorage(database, binding.StorageID, binding.Generation); err != nil || state.Mode != "maintenance" {
						t.Fatal("Job completion unlocked a running native process", state, err)
					}
					_ = kube.Tracker().Update(resource, terminated, "system")
				}
				return true, observed, err
			})
		} else if err := kube.Tracker().Update(corev1.SchemeGroupVersion.WithResource("pods"), terminated, "system"); err != nil {
			t.Fatal(err)
		}
		job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
		if outcome == "unknown" || outcome == "failed" {
			job.Status.Conditions[0].Type = batchv1.JobFailed
		}
		if err := kube.Tracker().Update(resource, job, "system"); err != nil {
			t.Fatal(err)
		}
		if outcome == "changed-source" {
			value, _ := kube.Tracker().Get(corev1.SchemeGroupVersion.WithResource("pods"), "system", "hub")
			hub := value.(*corev1.Pod).DeepCopy()
			hub.Spec.Containers[0].Env = append(hub.Spec.Containers[0].Env, corev1.EnvVar{Name: "REGISTRY_STORAGE_MAINTENANCE_READONLY_ENABLED", Value: "true"})
			_ = kube.Tracker().Update(corev1.SchemeGroupVersion.WithResource("pods"), hub, "system")
		}
		return true, job, nil
	})
}
