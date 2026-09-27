package controller

import (
	"context"
	"errors"
	"net/http"
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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// capability_id: rainbond.cleanup.node-executor-admission
// capability_id: rainbond.cleanup.node-result-finalization
// capability_id: rainbond.cleanup.node-recovery-api
func TestNodeAdmissionAPIUsesKubernetesFactsAndGrantsOnce(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.DB().SetMaxOpenConns(1)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}).Error; err != nil {
		t.Fatal(err)
	}
	kube, template, volume := gcAdmissionFixture(t)
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node", UID: "node-uid"}}
	if _, err := kube.CoreV1().Nodes().Create(context.Background(), node, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	storage, err := guard.ProvisionManagedCacheStorage(database, volume, "/cache/build")
	if err != nil {
		t.Fatal(err)
	}
	// Isolated API fixture: production enrollment never promotes readiness.
	if err := database.Model(&model.CleanupStorage{}).Where("storage_id = ?", storage.StorageID).Update("mode", "ready").Error; err != nil {
		t.Fatal(err)
	}
	intent := guard.NodeJobIntent{Namespace: "system", NodeName: "node", NodeUID: "node-uid", Entry: "entry", Fingerprint: strings.Repeat("a", 64)}
	r, err := guard.ManagedNodeRequest(storage, "executor", "node-http", "request", intent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := guard.AcquireOperation(database, r); err != nil {
		t.Fatal(err)
	}

	source, err := kube.CoreV1().Pods("system").Get(context.Background(), "chaos", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	source.Spec.NodeName = "node"
	source.Spec.Volumes = template.Spec.Template.Spec.Volumes
	source.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{{Name: "data", MountPath: "/cache/build", SubPath: "owned"}}
	kube.CoreV1().Pods("system").Update(context.Background(), source, metav1.UpdateOptions{})
	state := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "state", Namespace: "system", UID: "state-uid"}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "state-pv"}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}
	kube.CoreV1().PersistentVolumeClaims("system").Create(context.Background(), state, metav1.CreateOptions{})
	kube.CoreV1().PersistentVolumes().Create(context.Background(), &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "state-pv", UID: "state-pv-uid"}, Spec: corev1.PersistentVolumeSpec{ClaimRef: &corev1.ObjectReference{Name: "state", Namespace: "system", UID: state.UID}}}, metav1.CreateOptions{})
	settings := kubeidentity.NodeJobSettings{Region: "rainbond", Image: "example.test/plugin@sha256:" + strings.Repeat("b", 64), Endpoint: "https://core.internal:8443", CredentialSecret: "core", StateClaim: "state"}
	template, err = kubeidentity.BuildManagedNodeJob(context.Background(), kube, "chaos", "chaos-uid", storage, r, intent, settings)
	if err != nil {
		t.Fatal(err)
	}
	jobs := gcAdmissionKubeClient{Interface: kube}.BatchV1().Jobs("system")
	job, err := guard.SubmitSuspendedNodeJob(context.Background(), database, jobs, r, intent, template)
	if err != nil {
		t.Fatal(err)
	}
	job, err = guard.StartNodeJob(context.Background(), database, jobs, r)
	if err != nil {
		t.Fatal(err)
	}
	yes := true
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "node-executor", Namespace: "system", UID: "node-pod-uid", ResourceVersion: "1", OwnerReferences: []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "Job", Name: job.Name, UID: job.UID, Controller: &yes}}}, Spec: *job.Spec.Template.Spec.DeepCopy(), Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Name: "node-cleanup", ContainerID: "containerd://node-helper", ImageID: job.Spec.Template.Spec.Containers[0].Image, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}}}
	if _, err := kube.CoreV1().Pods("system").Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	h := &CleanupCoordinationHandler{nodeSettings: func() kubeidentity.NodeJobSettings { return settings }, database: func() *gorm.DB { return database }, gcTarget: func() (kubernetes.Interface, string, string, error) {
		return gcAdmissionKubeClient{Interface: kube}, "system", "rbd-hub", nil
	}}
	t.Setenv("TOKEN", "isolated-node-fixture")
	router := chi.NewRouter()
	router.Use(middleware.FullToken)
	router.Post("/v2/cleanup/stores/{storage_id}/operations/{operation_id}/node/enter-job", h.EnterNodeJob)
	router.Post("/v2/cleanup/stores/{storage_id}/operations/{operation_id}/node/result", h.RecordNodeJobResult)
	router.Post("/v2/cleanup/stores/{storage_id}/operations/{operation_id}/node/finish", h.FinishNodeJob)
	router.Post("/v2/cleanup/stores/{storage_id}/operations/{operation_id}/node/status", h.NodeJobProgress)
	router.Post("/v2/cleanup/stores/{storage_id}/operations/{operation_id}/node/recover", h.RecoverNodeJob)
	endpoint := "/v2/cleanup/stores/" + r.StorageID + "/operations/" + r.OperationID + "/node/enter-job"
	unauth := httptest.NewRecorder()
	router.ServeHTTP(unauth, httptest.NewRequest(http.MethodPost, endpoint, strings.NewReader(`{}`)))
	if unauth.Code != http.StatusUnauthorized && unauth.Code != http.StatusForbidden {
		t.Fatal("unauthenticated request accepted", unauth.Code)
	}
	claimed := httptest.NewRequest(http.MethodPost, endpoint, strings.NewReader(`{"image_id":"caller-proof"}`))
	claimed.Header.Set("Authorization", "Token isolated-node-fixture")
	denied := httptest.NewRecorder()
	router.ServeHTTP(denied, claimed)
	if denied.Code != 400 {
		t.Fatal("claimed runtime facts accepted", denied.Code)
	}
	server := httptest.NewServer(router)
	defer server.Close()
	client, err := guard.NewCoordinationClient(server.URL, "isolated-node-fixture", true)
	if err != nil {
		t.Fatal(err)
	}
	locator := guard.NodeExecutorLocator{Pod: pod.Name, PodUID: string(pod.UID)}
	wrong := locator
	wrong.PodUID = "replacement"
	if err := client.EnterNodeJob(context.Background(), r, wrong); err == nil {
		t.Fatal("wrong pod admitted")
	}

	originalImage := settings.Image
	settings.Image = "example.test/plugin@sha256:" + strings.Repeat("d", 64)
	if err := client.EnterNodeJob(context.Background(), r, locator); err == nil {
		t.Fatal("changed source granted native deletion")
	}
	settings.Image = originalImage
	if err := client.EnterNodeJob(context.Background(), r, locator); err != nil {
		t.Fatal(err)
	}
	if err := client.EnterNodeJob(context.Background(), r, locator); err == nil {
		t.Fatal("execution grant repeated")
	}
	before := guard.NodeStorageMeasurement{RootDevice: 1, RootInode: 2, TotalBytes: 1000, FreeBytes: 100, AvailableBytes: 80, TotalInodes: 100, FreeInodes: 20, ObservedAt: time.Now().UTC()}
	after := before
	after.FreeBytes = 120
	after.AvailableBytes = 100
	after.ObservedAt = after.ObservedAt.Add(time.Second)
	result := guard.NodeExecutionResult{State: "deleted", Before: &before, After: &after}
	if err := client.RecoverNodeJob(context.Background(), r); !errors.Is(err, guard.ErrCoordinationBusy) {
		t.Fatal("live original accepted for recovery", err)
	}
	if err := client.FinishNodeJob(context.Background(), r, locator); err == nil {
		t.Fatal("running helper released scope")
	}
	pod.Status.Phase = corev1.PodSucceeded
	pod.Status.ContainerStatuses[0].State = corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{FinishedAt: metav1.NewTime(after.ObservedAt.Add(time.Second)), ExitCode: 0}}
	if _, err := kube.CoreV1().Pods("system").Update(context.Background(), pod, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := client.RecoverNodeJob(context.Background(), r); err != nil {
		t.Fatal("receipt helper not launched", err)
	}
	recoveredBinding, err := guard.ReadNodeJobBinding(database, r)
	if err != nil || recoveredBinding.Recovery == nil || recoveredBinding.Recovery.JobUID == "" {
		t.Fatal("missing durable recovery identity", err)
	}
	helper, err := kube.BatchV1().Jobs("system").Get(context.Background(), recoveredBinding.Recovery.Name, metav1.GetOptions{})
	if err != nil || *helper.Spec.Suspend || len(helper.Spec.Template.Spec.Volumes) != 2 {
		t.Fatal("invalid recovery startup", err)
	}
	if err := client.RecoverNodeJob(context.Background(), r); err != nil {
		t.Fatal("repeat helper request failed", err)
	}
	jobList, err := kube.BatchV1().Jobs("system").List(context.Background(), metav1.ListOptions{})
	if err != nil || len(jobList.Items) != 2 {
		t.Fatal("duplicate recovery job", err)
	}
	operationState, err := guard.InspectOperation(database, r)
	if err != nil || operationState != "executing" {
		t.Fatal("helper launch released native protection", operationState, err)
	}
	if err := client.RecordNodeJobResult(context.Background(), r, locator, result); err != nil {
		t.Fatal(err)
	}
	if err := client.FinishNodeJob(context.Background(), r, locator); err != nil {
		t.Fatal(err)
	}
	progress, err := client.NodeJobProgress(context.Background(), r)
	if err != nil || progress.State != "finished" || progress.Outcome != "deleted" || progress.Execution.Result.After.AvailableBytes != 100 {
		t.Fatal(progress, err)
	}
	kube.CoreV1().Pods("system").Delete(context.Background(), pod.Name, metav1.DeleteOptions{})
	if err := client.RecordNodeJobResult(context.Background(), r, locator, result); err != nil {
		t.Fatal("lost result acknowledgment failed after pod removal", err)
	}
	if err := client.FinishNodeJob(context.Background(), r, locator); err != nil {
		t.Fatal("lost finish acknowledgment failed after pod removal", err)
	}

}
