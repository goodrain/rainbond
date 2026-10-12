package controller

import (
	"context"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/middleware"
	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/cleanup/kubeidentity"
	"github.com/jinzhu/gorm"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	typedbatch "k8s.io/client-go/kubernetes/typed/batch/v1"
	ktesting "k8s.io/client-go/testing"
)

// capability_id: rainbond.cleanup.gc-job-execution-api
// capability_id: rainbond.cleanup.gc-job-restore
func TestGCJobAdmissionAPIRequiresVerifiedExecutorAndGrantsOnce(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "gc.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.DB().SetMaxOpenConns(1)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}).Error; err != nil {
		t.Fatal(err)
	}
	kube, _, volume := gcAdmissionFixture(t)
	binding := guard.StorageRegistration{StorageID: "store", Generation: "gen", VolumeUID: volume, RootPath: "/registry"}
	if err := guard.RegisterStorage(database, binding); err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&model.CleanupStorage{}).Where("storage_id = ?", "store").Update("mode", "ready").Error; err != nil {
		t.Fatal(err)
	}
	r := guard.CoordinationRequest{StorageID: "store", Generation: "gen", OperationID: "gc-op", Kind: "gc", Scope: "*", Owner: "executor", Fingerprint: "request"}
	if _, err := guard.RequestMaintenance(database, r); err != nil {
		t.Fatal(err)
	}
	// An unprepared/foreign pod must fail before any execution grant.
	h := &CleanupCoordinationHandler{database: func() *gorm.DB { return database }, gcTarget: func() (kubernetes.Interface, string, string, error) {
		return gcAdmissionKubeClient{Interface: kube}, "system", "rbd-hub", nil
	}}
	t.Setenv("TOKEN", "gc-isolated-fixture")
	router := chi.NewRouter()
	router.Use(middleware.FullToken)
	router.Post("/v2/cleanup/stores/{storage_id}/operations/{operation_id}/maintenance/enter-job", h.EnterGCJob)
	router.Post("/v2/cleanup/stores/{storage_id}/operations/{operation_id}/maintenance/job", h.SubmitGCJob)
	router.Post("/v2/cleanup/stores/{storage_id}/operations/{operation_id}/maintenance/job/start", h.StartGCJob)
	router.Post("/v2/cleanup/stores/{storage_id}/operations/{operation_id}/maintenance/job/restore", h.RestoreGCJob)
	router.Post("/v2/cleanup/stores/{storage_id}/operations/{operation_id}/maintenance/job/status", h.GCJobProgress)
	router.Post("/v2/cleanup/stores/{storage_id}/operations/{operation_id}/maintenance/measurement", h.RecordMaintenanceMeasurement)
	router.Post("/v2/cleanup/stores/{storage_id}/operations/{operation_id}/maintenance/complete", h.CompleteMaintenanceWork)
	server := httptest.NewServer(router)
	defer server.Close()
	client, err := guard.NewCoordinationClient(server.URL, "gc-isolated-fixture", true)
	if err != nil {
		t.Fatal(err)
	}
	locator := guard.GCExecutorLocator{Pod: "gc-pod", PodUID: "gc-pod-uid"}
	if err := client.EnterGCJob(context.Background(), r, locator); err == nil {
		t.Fatal("unprepared executor admitted")
	}
	state, err := guard.InspectOperation(database, r)
	if err != nil || state != "draining" {
		t.Fatal(state, err)
	}
	// The route must be protected even when ordinary region token auth is optional.
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("POST", "/v2/cleanup/stores/store/operations/gc-op/maintenance/enter-job", strings.NewReader("{}")))
	if response.Code != 401 {
		t.Fatal("unauthenticated admission", response.Code)
	}
	// Runtime overrides are not part of the API contract, even for an authenticated caller.
	injected := httptest.NewRequest("POST", "/v2/cleanup/stores/store/operations/gc-op/maintenance/job", strings.NewReader(`{"image":"caller-image"}`))
	injected.Header.Set("Authorization", "Token gc-isolated-fixture")
	invalid := httptest.NewRecorder()
	router.ServeHTTP(invalid, injected)
	if invalid.Code != 400 {
		t.Fatal("caller executable override accepted", invalid.Code)
	}
	// Store and job identity validation still runs even with valid internal auth.
	for i := 0; i < 2; i++ {
		if err := client.SubmitGCJob(context.Background(), r); err != nil {
			t.Fatal("submit failed", err)
		}
	}
	recorded, err := guard.ReadGCJobBinding(database, r)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.StartGCJob(context.Background(), r); err != nil {
		t.Fatal("start failed", err)
	}
	job, err := kube.BatchV1().Jobs("system").Get(context.Background(), recorded.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	yes := true
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: locator.Pod, Namespace: "system", UID: "gc-pod-uid", ResourceVersion: "1", OwnerReferences: []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "Job", Name: job.Name, UID: job.UID, Controller: &yes}}}, Spec: *job.Spec.Template.Spec.DeepCopy(), Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Name: "gc", ImageID: job.Spec.Template.Spec.Containers[0].Image, ContainerID: "containerd://owned", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}}}
	if _, err := kube.CoreV1().Pods("system").Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	writer := guard.CoordinationRequest{StorageID: "store", Generation: "gen", OperationID: "old-writer", Owner: "builder", Kind: "producer", Scope: "*", Fingerprint: "old-request"}
	// Simulate an operation accepted before the maintenance request.
	if err := database.Create(&model.CleanupOperation{StorageID: writer.StorageID, Generation: writer.Generation, OperationID: writer.OperationID, Owner: writer.Owner, Kind: writer.Kind, Scope: writer.Scope, Fingerprint: writer.Fingerprint, State: "active"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := client.EnterGCJob(context.Background(), r, locator); err == nil {
		t.Fatal("active writer was ignored")
	}
	if err := guard.FinishOperation(database, writer, true); err != nil {
		t.Fatal(err)
	}
	if err := client.EnterGCJob(context.Background(), r, guard.GCExecutorLocator{Pod: locator.Pod, PodUID: "replacement"}); err == nil {
		t.Fatal("foreign executor admitted")
	}
	if err := client.EnterGCJob(context.Background(), r, locator); err != nil {
		t.Fatal(err)
	}
	if err := client.EnterGCJob(context.Background(), r, locator); err == nil {
		t.Fatal("execution grant replayed")
	}
	if err := guard.EnterMaintenance(database, r); err == nil {
		t.Fatal("generic admission bypassed Job binding")
	}
	state, err = guard.InspectOperation(database, r)
	if err != nil || state != "exclusive" {
		t.Fatal(state, err)
	}
	fingerprint, _ := binding.Fingerprint()
	before := guard.StorageMeasurement{Protocol: 1, StorageID: r.StorageID, Generation: r.Generation, BindingFingerprint: fingerprint, FilesystemID: "owned-fs", ObservedAt: time.Now().UTC(), TotalBytes: 1000, FreeBytes: 100, AvailableBytes: 50}
	if err := client.RecordMaintenanceMeasurement(context.Background(), r, "before", before); err != nil {
		t.Fatal(err)
	}
	if err := client.CompleteMaintenanceWork(context.Background(), r, "succeeded"); err != nil {
		t.Fatal(err)
	}
	after := before
	after.ObservedAt = before.ObservedAt.Add(time.Second)
	after.FreeBytes = 200
	after.AvailableBytes = 150
	if err := client.RecordMaintenanceMeasurement(context.Background(), r, "after", after); err != nil {
		t.Fatal(err)
	}
	if err := client.RestoreGCJob(context.Background(), r); !errors.Is(err, guard.ErrCoordinationBusy) {
		t.Fatal("still-running executor not reported as pending", err)
	}
	pod.Status.Phase = corev1.PodSucceeded
	pod.Status.ContainerStatuses[0].State = corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0, FinishedAt: metav1.Now()}}
	if _, err := kube.CoreV1().Pods("system").Update(context.Background(), pod, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	hub, err := kube.CoreV1().Pods("system").Get(context.Background(), "hub", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	original := hub.DeepCopy()
	hub.Spec.Containers[0].Env = append(hub.Spec.Containers[0].Env, corev1.EnvVar{Name: "REGISTRY_STORAGE_MAINTENANCE_READONLY_ENABLED", Value: "true"})
	if _, err := kube.CoreV1().Pods("system").Update(context.Background(), hub, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := client.RestoreGCJob(context.Background(), r); err == nil {
		t.Fatal("changed registry configuration accepted")
	}
	if _, err := kube.CoreV1().Pods("system").Update(context.Background(), original, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := client.RestoreGCJob(context.Background(), r); err != nil {
		t.Fatal("verified restoration failed", err)
	}
	if err := kube.CoreV1().Pods("system").Delete(context.Background(), pod.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := client.RestoreGCJob(context.Background(), r); err != nil {
		t.Fatal("lost restoration acknowledgment not recoverable", err)
	}
	state, err = guard.InspectOperation(database, r)
	if err != nil || state != "finished" {
		t.Fatal(state, err)
	}
	progress, err := client.GCJobProgress(context.Background(), r)
	if err != nil || progress.State != "finished" || progress.Outcome != "succeeded" || progress.Before == nil || progress.After == nil || progress.After.AvailableBytes != 150 {
		t.Fatal("missing original outcome and measurements", progress, err)
	}
}

func gcAdmissionFixture(t *testing.T) (*fake.Clientset, *batchv1.Job, string) {
	t.Helper()
	no := false
	native := corev1.Container{Name: "gc", Image: "example.test/owned@sha256:" + strings.Repeat("a", 64), Command: []string{"/registry-gc"}, VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/registry", SubPath: "owned"}}}
	spec := corev1.PodSpec{NodeName: "node", RestartPolicy: corev1.RestartPolicyNever, AutomountServiceAccountToken: &no, Containers: []corev1.Container{native}, Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data"}}}}}
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Namespace: "system"}, Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: spec}}}
	hub := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "hub", Namespace: "system", UID: "hub-uid", Labels: map[string]string{"app": "hub"}}, Spec: *spec.DeepCopy()}
	hub.ResourceVersion = "1"
	hub.Annotations = map[string]string{"rainbond.io/registry-gc-executor": "v1"}
	hub.Spec.Containers[0].Name = "registry"
	hub.Spec.Containers[0].Env = []corev1.EnvVar{{Name: "REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY", Value: "/registry"}, {Name: "REGISTRY_HTTP_ADDR", Value: "127.0.0.1:5000"}, {Name: "REGISTRY_STORAGE_MAINTENANCE_UPLOADPURGING_ENABLED", Value: "false"}}
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "rbd-hub", Namespace: "system", UID: "service-uid", ResourceVersion: "1"}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "hub"}, Ports: []corev1.ServicePort{{Port: 5000, TargetPort: intstr.FromInt(5001)}}}}
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data", Namespace: "system", UID: "pvc-uid"}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "pv"}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}
	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "pv", UID: "pv-uid"}, Spec: corev1.PersistentVolumeSpec{ClaimRef: &corev1.ObjectReference{Namespace: "system", Name: "data", UID: "pvc-uid"}}, Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound}}
	image := "example.test/chaos@sha256:" + strings.Repeat("c", 64)
	cleaner := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "chaos", Namespace: "system", UID: "chaos-uid", ResourceVersion: "1", Labels: map[string]string{"name": "rbd-chaos"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "chaos", Image: image, Command: []string{"/run/rainbond-chaos"}, Args: []string{"--clean-up=false"}}}}, Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Name: "chaos", ImageID: image, ContainerID: "containerd://chaos", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}}}
	client := fake.NewSimpleClientset(hub, svc, pvc, pv, cleaner)
	observed, err := kubeidentity.InspectNativeRegistry(context.Background(), client, "system", "rbd-hub", "hub", "hub-uid")
	if err != nil {
		t.Fatal(err)
	}
	sidecar := corev1.Container{Name: "coordinator", Image: native.Image, Command: []string{"/registry-coordinator"}, VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/registry", SubPath: "owned", ReadOnly: true}, {Name: "control", MountPath: "/control", ReadOnly: true}}, Args: []string{"--listen=:5001", "--upstream=http://127.0.0.1:5000", "--storage-id=store", "--storage-generation=gen", "--volume-uid=" + observed.Mount.VolumeUID, "--registry-path=/registry", "--storage-root=/registry", "--coordination-api=http://rbd-api.system:8443", "--allow-internal-http=true", "--credential-file=/control/token"}}
	hub.Spec.Containers = append(hub.Spec.Containers, sidecar)
	hub.Spec.Volumes = append(hub.Spec.Volumes, corev1.Volume{Name: "control", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "coordination"}}})
	hub.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "registry", ImageID: "example.test/native@sha256:" + strings.Repeat("b", 64), State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}, {Name: "coordinator", ImageID: native.Image, ContainerID: "containerd://coordinator", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}
	if _, err := client.CoreV1().Pods("system").Update(context.Background(), hub, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	client.PrependReactor("list", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
		result, err := client.Tracker().List(corev1.SchemeGroupVersion.WithResource("pods"), corev1.SchemeGroupVersion.WithKind("Pod"), action.GetNamespace())
		if err == nil {
			list := result.(*corev1.PodList)
			selected := action.(ktesting.ListAction).GetListRestrictions().Labels
			filtered := []corev1.Pod{}
			for _, item := range list.Items {
				if selected.Matches(labels.Set(item.Labels)) {
					filtered = append(filtered, item)
				}
			}
			list.Items = filtered
			list.ResourceVersion = "snapshot"
		}
		return true, result, err
	})
	return client, job, observed.Mount.VolumeUID
}

type gcAdmissionJobClient struct{ typedbatch.JobInterface }

func (c gcAdmissionJobClient) Create(ctx context.Context, job *batchv1.Job, options metav1.CreateOptions) (*batchv1.Job, error) {
	observed := job.DeepCopy()
	if len(options.DryRun) > 0 {
		return observed, nil
	}
	observed.UID = "job-uid"
	observed.ResourceVersion = "1"
	return c.JobInterface.Create(ctx, observed, options)
}

type gcAdmissionKubeClient struct{ kubernetes.Interface }

func (c gcAdmissionKubeClient) BatchV1() typedbatch.BatchV1Interface {
	return gcAdmissionBatchClient{BatchV1Interface: c.Interface.BatchV1()}
}

type gcAdmissionBatchClient struct{ typedbatch.BatchV1Interface }

func (c gcAdmissionBatchClient) Jobs(namespace string) typedbatch.JobInterface {
	return gcAdmissionJobClient{JobInterface: c.BatchV1Interface.Jobs(namespace)}
}
