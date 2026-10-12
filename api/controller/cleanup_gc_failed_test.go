package controller

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/middleware"
	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// capability_id: rainbond.cleanup.gc-failed-before-admission
func TestFailedGCBeforeAdmissionCanEndMaintenanceWithoutDeletion(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "failed.db"))
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
	h := &CleanupCoordinationHandler{database: func() *gorm.DB { return database }, gcTarget: func() (kubernetes.Interface, string, string, error) {
		return gcAdmissionKubeClient{Interface: kube}, "system", "rbd-hub", nil
	}}
	t.Setenv("TOKEN", "isolated-failed-fixture")
	router := chi.NewRouter()
	router.Use(middleware.FullToken)
	router.Post("/v2/cleanup/stores/{storage_id}/operations/{operation_id}/maintenance/job", h.SubmitGCJob)
	router.Post("/v2/cleanup/stores/{storage_id}/operations/{operation_id}/maintenance/job/start", h.StartGCJob)
	router.Post("/v2/cleanup/stores/{storage_id}/operations/{operation_id}/maintenance/job/status", h.GCJobProgress)
	router.Post("/v2/cleanup/stores/{storage_id}/operations/{operation_id}/maintenance/job/cancel-failed", h.CancelFailedGCJob)
	server := httptest.NewServer(router)
	defer server.Close()
	client, err := guard.NewCoordinationClient(server.URL, "isolated-failed-fixture", true)
	if err != nil {
		t.Fatal(err)
	}
	request := guard.CoordinationRequest{StorageID: "store", Generation: "gen", OperationID: "failed-gc", Owner: "gc", Kind: "gc", Scope: "*", Fingerprint: "confirmed"}
	ctx := context.Background()
	if err := client.SubmitGCJob(ctx, request); err != nil {
		t.Fatal(err)
	}
	if err := client.StartGCJob(ctx, request); err != nil {
		t.Fatal(err)
	}
	if err := client.CancelFailedGCJob(ctx, request); err == nil {
		t.Fatal("nonterminal Job canceled")
	}
	recorded, err := guard.ReadGCJobBinding(database, request)
	if err != nil {
		t.Fatal(err)
	}
	job, err := kube.BatchV1().Jobs("system").Get(ctx, recorded.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue}}
	if _, err := kube.BatchV1().Jobs("system").UpdateStatus(ctx, job, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	progress, err := client.GCJobProgress(ctx, request)
	if err != nil || progress.ExecutorState != "failed" {
		t.Fatal(progress, err)
	}
	if err := client.CancelFailedGCJob(ctx, request); err != nil {
		t.Fatal(err)
	}
	progress, err = client.GCJobProgress(ctx, request)
	if err != nil || progress.State != "finished" || progress.Outcome != "canceled" || progress.Before != nil || progress.After != nil {
		t.Fatal("invented a deletion receipt", progress, err)
	}
	if err := guard.EnterGCJobExecution(database, request, string(job.UID), "late-pod"); err == nil {
		t.Fatal("late executor admitted")
	}
	request.OperationID = "already-admitted"
	if err := client.SubmitGCJob(ctx, request); err != nil {
		t.Fatal(err)
	}
	if err := client.StartGCJob(ctx, request); err != nil {
		t.Fatal(err)
	}
	next, err := guard.ReadGCJobBinding(database, request)
	if err != nil {
		t.Fatal(err)
	}
	if err := guard.BindGCExecutor(database, request, next.JobUID, "original-pod", "original-pod-uid"); err != nil {
		t.Fatal(err)
	}
	if err := guard.EnterGCJobExecution(database, request, next.JobUID, "original-pod-uid"); err != nil {
		t.Fatal(err)
	}
	admitted, err := kube.BatchV1().Jobs("system").Get(ctx, next.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	admitted.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue}}
	if _, err := kube.BatchV1().Jobs("system").UpdateStatus(ctx, admitted, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := client.CancelFailedGCJob(ctx, request); err == nil {
		t.Fatal("admitted GC was canceled")
	}
	state, err := guard.InspectOperation(database, request)
	if err != nil || state != "exclusive" {
		t.Fatal("execution protection released", state, err)
	}

}
