package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

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

// capability_id: rainbond.cleanup.node-job-launch-api
// capability_id: rainbond.cleanup.node-cancel-before-grant
func TestNodeLaunchAPIRechecksSourceBeforeStarting(t *testing.T) {
	ctx := context.Background()
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "launch.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.DB().SetMaxOpenConns(1)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}).Error; err != nil {
		t.Fatal(err)
	}
	kube, base, _ := gcAdmissionFixture(t)
	source, err := kube.CoreV1().Pods("system").Get(ctx, "chaos", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	source.Spec.NodeName = "node"
	source.Spec.Volumes = base.Spec.Template.Spec.Volumes
	source.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{{Name: "data", MountPath: "/cache/build", SubPath: "owned"}}
	kube.CoreV1().Pods("system").Update(ctx, source, metav1.UpdateOptions{})
	kube.CoreV1().Nodes().Create(ctx, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node", UID: "node-uid"}}, metav1.CreateOptions{})
	state := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "state", Namespace: "system", UID: "state-uid"}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "state-pv"}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}
	kube.CoreV1().PersistentVolumeClaims("system").Create(ctx, state, metav1.CreateOptions{})
	kube.CoreV1().PersistentVolumes().Create(ctx, &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "state-pv", UID: "state-pv-uid"}, Spec: corev1.PersistentVolumeSpec{ClaimRef: &corev1.ObjectReference{Name: "state", Namespace: "system", UID: state.UID}}}, metav1.CreateOptions{})
	observed, err := kubeidentity.InspectManagedBuildCache(ctx, kube, "system", "chaos", "chaos-uid")
	if err != nil {
		t.Fatal(err)
	}
	storage, err := guard.ProvisionManagedCacheStorage(database, observed.Mount.VolumeUID, observed.Root)
	if err != nil {
		t.Fatal(err)
	}
	// This isolated fixture has no application writers. Production enrollment remains collecting.
	if err := database.Model(&model.CleanupStorage{}).Where("storage_id = ?", storage.StorageID).Update("mode", "ready").Error; err != nil {
		t.Fatal(err)
	}
	intent := guard.NodeJobIntent{Namespace: "system", NodeName: "node", NodeUID: "node-uid", Entry: "selected", Fingerprint: strings.Repeat("a", 64)}
	request, err := guard.ManagedNodeRequest(storage, "owner", "launch", "selection", intent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := guard.AcquireOperation(database, request); err != nil {
		t.Fatal(err)
	}
	settings := kubeidentity.NodeJobSettings{Region: "rainbond", Image: "example.test/plugin@sha256:" + strings.Repeat("b", 64), Endpoint: "https://core.internal:8443", CredentialSecret: "core", StateClaim: "state"}
	configureCacheCoverageFixture(t, database, kube, storage, request)
	h := &CleanupCoordinationHandler{database: func() *gorm.DB { return database }, nodeSettings: func() kubeidentity.NodeJobSettings { return settings }, gcTarget: func() (kubernetes.Interface, string, string, error) {
		return gcAdmissionKubeClient{Interface: kube}, "system", "rbd-hub", nil
	}}
	t.Setenv("TOKEN", "isolated-launch-fixture")
	router := chi.NewRouter()
	router.Use(middleware.FullToken)
	router.Post("/stores/{storage_id}/operations/{operation_id}/node/cancel-before-grant", h.CancelNodeBeforeGrant)
	router.Post("/stores/{storage_id}/operations/{operation_id}/node/submit-job", h.SubmitNodeJob)
	router.Post("/stores/{storage_id}/operations/{operation_id}/node/start-job", h.StartNodeJob)
	endpoint := "/stores/" + request.StorageID + "/operations/" + request.OperationID + "/node/"
	post := func(action string, body interface{}, authenticated bool) int {
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, endpoint+action, strings.NewReader(string(raw)))
		if authenticated {
			req.Header.Set("Authorization", "Token isolated-launch-fixture")
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code
	}
	body := NodeJobSelection{CoordinationRequest: request, SourcePod: "chaos", SourceUID: "chaos-uid", NodeName: "node", NodeUID: "node-uid", Entry: "selected", EntryFingerprint: intent.Fingerprint}
	if status := post("submit-job", body, false); status != 401 && status != 403 {
		t.Fatal("unauthenticated launch", status)
	}
	if status := post("submit-job", body, true); status != 200 {
		t.Fatal("submit", status)
	}
	binding, err := guard.ReadNodeJobBinding(database, request)
	if err != nil {
		t.Fatal(err)
	}
	jobs := kube.BatchV1().Jobs("system")
	job, err := jobs.Get(ctx, binding.Name, metav1.GetOptions{})
	if err != nil || !*job.Spec.Suspend {
		t.Fatal("unexpected startup", err)
	}
	if status := post("submit-job", body, true); status != 200 {
		t.Fatal("repeat creates new task", status)
	}
	settings.Image = "example.test/plugin@sha256:" + strings.Repeat("c", 64)
	if status := post("start-job", request, true); status != 409 {
		t.Fatal("changed executor source accepted", status)
	}
	settings.Image = job.Spec.Template.Spec.Containers[0].Image
	if status := post("start-job", request, true); status != 200 {
		t.Fatal("start", status)
	}
	job, _ = jobs.Get(ctx, binding.Name, metav1.GetOptions{})
	if *job.Spec.Suspend {
		t.Fatal("original Job not started")
	}

	if status := post("cancel-before-grant", body, false); status != 401 && status != 403 {
		t.Fatal("unauthenticated cancellation", status)
	}
	stateName, err := guard.InspectOperation(database, request)
	if err != nil || stateName != "active" {
		t.Fatal("startup granted native deletion", stateName, err)
	}

	if status := post("cancel-before-grant", body, true); status != 200 {
		t.Fatal("ungranted task cancellation failed", status)
	}
	if status := post("cancel-before-grant", body, true); status != 200 {
		t.Fatal("lost cancellation response not reconciled", status)
	}
	progress, err := guard.ReadNodeJobProgress(database, request)
	if err != nil || progress.Outcome != "canceled" || progress.Execution.Result != nil {
		t.Fatal("cancellation invented native outcome", progress, err)
	}
}
