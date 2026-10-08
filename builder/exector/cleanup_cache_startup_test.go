package exector

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/cleanup/kubeidentity"
	"github.com/jinzhu/gorm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

// capability_id: rainbond.cleanup.cache-builder-startup
func TestCacheStartupRegistersActualInstanceWithoutEnablingDeletion(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "startup.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.DB().SetMaxOpenConns(1)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}, &model.CleanupParticipant{}).Error; err != nil {
		t.Fatal(err)
	}
	image := "example.test/chaos@sha256:" + strings.Repeat("a", 64)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "chaos", Namespace: "system", UID: "pod-uid", ResourceVersion: "1", Labels: map[string]string{"name": "rbd-chaos"}}, Spec: corev1.PodSpec{NodeName: "node", Containers: []corev1.Container{{Name: "chaos", Image: image, Args: []string{"--clean-up=false"}, VolumeMounts: []corev1.VolumeMount{{Name: "cache", MountPath: "/cache"}}}}, Volumes: []corev1.Volume{{Name: "cache", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/opt/rainbond/cache"}}}}}, Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Name: "chaos", ImageID: image, ContainerID: "containerd://one", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}}}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node", UID: "node-uid"}}
	kube := fake.NewSimpleClientset(pod, node)
	observed, err := kubeidentity.InspectManagedBuildCache(context.Background(), kube, "system", "chaos", "pod-uid")
	if err != nil {
		t.Fatal(err)
	}
	binding, err := guard.ProvisionManagedCacheStorage(database, observed.Mount.VolumeUID, observed.Root)
	if err != nil {
		t.Fatal(err)
	}
	// The process can start before the API reports its container as Running.
	pendingReads := 0
	kube.PrependReactor("get", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
		pendingReads++
		if pendingReads <= 4 {
			pending := pod.DeepCopy()
			pending.Status.Phase = corev1.PodPending
			return true, pending, nil
		}
		return false, nil, nil
	})
	for i := 0; i < 2; i++ {
		if err := registerCacheBuilderStartup(context.Background(), database, kube, "system", "chaos"); err != nil {
			t.Fatal(err)
		}
	}
	var participants []model.CleanupParticipant
	database.Find(&participants)
	if len(participants) != 1 || participants[0].Role != "cache-builder" || participants[0].ContainerID != "containerd://one" {
		t.Fatal("startup identity missing")
	}
	status, err := guard.InspectStorage(database, binding.StorageID, binding.Generation)
	if err != nil || status.Mode != "collecting" {
		t.Fatal("startup promoted deletion readiness", err)
	}
	// This isolated state models a previously certified generation, not a live switch.
	database.Model(&model.CleanupStorage{}).Where("storage_id = ?", binding.StorageID).Update("mode", "ready")
	pod.Spec.Containers[0].Args = []string{"--clean-up=true"}
	kube.CoreV1().Pods("system").Update(context.Background(), pod, metav1.UpdateOptions{})
	if err := registerCacheBuilderStartup(context.Background(), database, kube, "system", "chaos"); err != nil {
		t.Fatal(err)
	}
	status, err = guard.InspectStorage(database, binding.StorageID, binding.Generation)
	if err != nil || status.Mode != "collecting" {
		t.Fatal("unsupported startup retained readiness", err)
	}
	pending := model.CleanupOperation{OperationID: "pending-delete", StorageID: binding.StorageID, Generation: binding.Generation, Owner: "executor", Kind: "delete", Scope: "*", State: "executing"}
	if err := database.Create(&pending).Error; err != nil {
		t.Fatal(err)
	}
	if err := registerCacheBuilderStartup(context.Background(), database, kube, "system", "chaos"); err == nil {
		t.Fatal("unverified startup bypassed outstanding deletion")
	}
}
