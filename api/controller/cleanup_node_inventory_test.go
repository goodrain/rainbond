package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/api/middleware"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/cleanup/kubeidentity"
	"github.com/jinzhu/gorm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// capability_id: rainbond.cleanup.node-inventory-api
func TestInventoryAPIStartsReadOnlyJobWithoutDeletionReadiness(t *testing.T) {
	ctx := context.Background()
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "inventory.db"))
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
	claim := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "reports", Namespace: "system", UID: "report-uid"}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "report-pv"}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}
	kube.CoreV1().PersistentVolumeClaims("system").Create(ctx, claim, metav1.CreateOptions{})
	kube.CoreV1().PersistentVolumes().Create(ctx, &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "report-pv", UID: "report-pv-uid"}, Spec: corev1.PersistentVolumeSpec{ClaimRef: &corev1.ObjectReference{Name: claim.Name, Namespace: "system", UID: claim.UID}}}, metav1.CreateOptions{})
	h := &CleanupCoordinationHandler{database: func() *gorm.DB { return database }, inventorySettings: func() kubeidentity.NodeInventorySettings {
		return kubeidentity.NodeInventorySettings{Region: "rainbond", Image: "example.test/plugin@sha256:" + strings.Repeat("b", 64)}
	}, gcTarget: func() (kubernetes.Interface, string, string, error) {
		return gcAdmissionKubeClient{Interface: kube}, "system", "rbd-hub", nil
	}}
	t.Setenv("TOKEN", "isolated-inventory-fixture")
	post := func(auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/managed-cache/inventory", strings.NewReader(`{"pod":"chaos","pod_uid":"chaos-uid","scan_id":"scan"}`))
		if auth {
			r.Header.Set("Authorization", "Token isolated-inventory-fixture")
		}
		w := httptest.NewRecorder()
		middleware.FullToken(http.HandlerFunc(h.CollectManagedCache)).ServeHTTP(w, r)
		return w
	}
	if w := post(false); w.Code != 401 && w.Code != 403 {
		t.Fatal("unauthenticated scan accepted")
	}
	for i := 0; i < 2; i++ {
		w := post(true)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var data struct {
			Bean struct {
				State string `json:"state"`
			} `json:"bean"`
		}
		json.Unmarshal(w.Body.Bytes(), &data)
		if data.Bean.State != "submitted" {
			t.Fatal("admission reported as completed scan")
		}
	}
	jobs, err := kube.BatchV1().Jobs("system").List(ctx, metav1.ListOptions{})
	if err != nil || len(jobs.Items) != 1 || *jobs.Items[0].Spec.Suspend {
		t.Fatal("scan did not reuse started job", err)
	}
	var storage model.CleanupStorage
	database.First(&storage)
	if storage.Mode != "collecting" {
		t.Fatal("scan promoted deletion readiness")
	}
	var count int
	database.Model(&model.CleanupOperation{}).Count(&count)
	if count != 0 {
		t.Fatal("scan granted native deletion")
	}
}
