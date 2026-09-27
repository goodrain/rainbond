package controller

import (
	"context"
	"testing"

	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/cleanup/kubeidentity"
	"github.com/jinzhu/gorm"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func configureCacheCoverageFixture(t *testing.T, database *gorm.DB, kube *fake.Clientset, binding guard.StorageRegistration, request guard.CoordinationRequest) {
	t.Helper()
	ctx := context.Background()
	if err := database.AutoMigrate(&model.CleanupParticipant{}).Error; err != nil {
		t.Fatal(err)
	}
	source, err := kube.CoreV1().Pods("system").Get(ctx, "chaos", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	yes := true
	source.OwnerReferences = []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "DaemonSet", Name: "chaos-owner", UID: "daemon-uid", Controller: &yes}}
	if _, err := kube.CoreV1().Pods("system").Update(ctx, source, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	daemon := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "chaos-owner", Namespace: "system", UID: "daemon-uid", ResourceVersion: "1", Generation: 1}, Spec: appsv1.DaemonSetSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"name": "rbd-chaos"}}, Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"name": "rbd-chaos"}}, Spec: *source.Spec.DeepCopy()}}, Status: appsv1.DaemonSetStatus{ObservedGeneration: 1, DesiredNumberScheduled: 1, CurrentNumberScheduled: 1, UpdatedNumberScheduled: 1, NumberReady: 1, NumberAvailable: 1}}
	if _, err := kube.AppsV1().DaemonSets("system").Create(ctx, daemon, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	kube.PrependReactor("list", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
		obj, err := kube.Tracker().List(corev1.SchemeGroupVersion.WithResource("pods"), corev1.SchemeGroupVersion.WithKind("Pod"), action.GetNamespace())
		if err != nil {
			return true, nil, err
		}
		obj.(*corev1.PodList).ResourceVersion = "snapshot"
		return true, obj, nil
	})
	member, err := kubeidentity.InspectCacheBuilder(ctx, kube, "system", "chaos", string(source.UID), binding)
	if err != nil {
		t.Fatal(err)
	}
	if err := guard.RegisterParticipant(database, member); err != nil {
		t.Fatal(err)
	}
	_, ready, err := guard.CertifyManagedCacheWriters(database, binding, &request, func() ([]guard.ParticipantRegistration, error) {
		return kubeidentity.InspectCacheWriterCoverage(ctx, kube, "system", binding)
	})
	if err != nil || !ready {
		t.Fatal("fixture coverage not certified", err)
	}
}
