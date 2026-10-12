package kubeidentity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

// capability_id: rainbond.cleanup.cache-writer-coverage
func TestCacheCoverageRejectsRollingWritersAndDetachedBuilds(t *testing.T) {
	_, _, pvc, pv := bindingObjects()
	pod := gcCleanerFixture()
	pod.Spec.NodeName = "node"
	pod.Spec.Volumes = []corev1.Volume{{Name: "cache", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvc.Name}}}}
	pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{{Name: "cache", MountPath: "/cache", SubPath: "owned"}}
	yes := true
	pod.OwnerReferences = []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "DaemonSet", Name: "chaos", UID: "daemon-uid", Controller: &yes}}
	daemon := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "chaos", Namespace: "system", UID: "daemon-uid", ResourceVersion: "1", Generation: 1}, Spec: appsv1.DaemonSetSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"name": "rbd-chaos"}}, Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"name": "rbd-chaos"}}, Spec: *pod.Spec.DeepCopy()}}, Status: appsv1.DaemonSetStatus{ObservedGeneration: 1, DesiredNumberScheduled: 1, CurrentNumberScheduled: 1, UpdatedNumberScheduled: 1, NumberReady: 1, NumberAvailable: 1}}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node", UID: "node-uid"}}
	kube := fake.NewSimpleClientset(pod, daemon, pvc, pv, node)
	kube.PrependReactor("list", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
		obj, err := kube.Tracker().List(corev1.SchemeGroupVersion.WithResource("pods"), corev1.SchemeGroupVersion.WithKind("Pod"), action.GetNamespace())
		if err != nil {
			return true, nil, err
		}
		list := obj.(*corev1.PodList)
		list.ResourceVersion = "snapshot"
		return true, list, nil
	})
	volume := volumeIdentity("pvc", "system", string(pvc.UID), string(pv.UID), "owned/build")
	sum := sha256.Sum256([]byte("managed-build-cache\x00" + volume))
	binding := coordination.StorageRegistration{StorageID: hex.EncodeToString(sum[:]), Generation: "one", VolumeUID: volume, RootPath: "/cache/build"}
	members, err := InspectCacheWriterCoverage(context.Background(), kube, "system", binding)
	if err != nil || len(members) != 1 || members[0].Role != "cache-builder" {
		t.Fatal("stable writer coverage rejected", err)
	}
	daemon.Spec.Selector.MatchLabels["name"] = "other"
	daemon.Spec.Template.Labels["name"] = "other"
	daemon.Generation = 2
	daemon.Status.ObservedGeneration = 2
	kube.AppsV1().DaemonSets("system").Update(context.Background(), daemon, metav1.UpdateOptions{})
	if _, err := InspectCacheWriterCoverage(context.Background(), kube, "system", binding); err == nil {
		t.Fatal("controller could create undiscovered writers")
	}
	daemon.Spec.Selector.MatchLabels["name"] = "rbd-chaos"
	daemon.Spec.Template.Labels["name"] = "rbd-chaos"
	daemon.Generation = 3
	daemon.Status.ObservedGeneration = 3
	kube.AppsV1().DaemonSets("system").Update(context.Background(), daemon, metav1.UpdateOptions{})
	daemon.Status.UpdatedNumberScheduled = 0
	kube.AppsV1().DaemonSets("system").UpdateStatus(context.Background(), daemon, metav1.UpdateOptions{})
	if _, err := InspectCacheWriterCoverage(context.Background(), kube, "system", binding); err == nil {
		t.Fatal("partial rollout certified")
	}
	daemon.Status.UpdatedNumberScheduled = 1
	kube.AppsV1().DaemonSets("system").UpdateStatus(context.Background(), daemon, metav1.UpdateOptions{})
	kube.CoreV1().Pods("system").Create(context.Background(), &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "old-build", Labels: map[string]string{"job": "codebuild"}}, Status: corev1.PodStatus{Phase: corev1.PodRunning}}, metav1.CreateOptions{})
	if _, err := InspectCacheWriterCoverage(context.Background(), kube, "system", binding); err == nil {
		t.Fatal("detached build ignored")
	}
}
