package kubeidentity

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// capability_id: rainbond.cleanup.managed-cache-binding
func TestManagedCachePreparationUsesActualNodeAndVolume(t *testing.T) {
	_, _, pvc, pv := bindingObjects()
	pod := gcCleanerFixture()
	pod.Spec.NodeName = "node"
	pod.Spec.Volumes = []corev1.Volume{{Name: "cache", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvc.Name}}}}
	pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{{Name: "cache", MountPath: "/cache", SubPath: "owned"}}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node", UID: "node-uid"}}
	client := fake.NewSimpleClientset(pod, pvc, pv, node)
	observed, err := InspectManagedBuildCache(context.Background(), client, "system", pod.Name, string(pod.UID))
	if err != nil || observed.Root != "/cache/build" || observed.NodeUID != "node-uid" || observed.Mount.VolumeUID != volumeIdentity("pvc", "system", string(pvc.UID), string(pv.UID), "owned/build") {
		t.Fatal(observed, err)
	}
	if _, err := InspectManagedBuildCache(context.Background(), client, "system", pod.Name, "replacement"); err == nil {
		t.Fatal("replaced Pod accepted")
	}
	pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{Name: "nested", MountPath: "/cache/build/other"})
	client.CoreV1().Pods("system").Update(context.Background(), pod, metav1.UpdateOptions{})
	if _, err := InspectManagedBuildCache(context.Background(), client, "system", pod.Name, string(pod.UID)); err == nil {
		t.Fatal("nested mount accepted")
	}
}

// capability_id: rainbond.cleanup.cache-observation-before-readiness
func TestReadOnlyCacheEnrollmentDoesNotRequireDeletionReadiness(t *testing.T) {
	_, _, pvc, pv := bindingObjects()
	pod := gcCleanerFixture()
	pod.Spec.NodeName = "node"
	pod.Spec.Containers[0].Image = "example.test/chaos:legacy"
	pod.Spec.Containers[0].Args = []string{"--clean-up=true"}
	pod.Spec.Volumes = []corev1.Volume{{Name: "cache", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvc.Name}}}}
	pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{{Name: "cache", MountPath: "/cache", SubPath: "owned"}}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node", UID: "node-uid"}}
	client := fake.NewSimpleClientset(pod, pvc, pv, node)
	if _, err := InspectManagedBuildCacheSource(context.Background(), client, "system", pod.Name, string(pod.UID)); err != nil {
		t.Fatal("read-only identity blocked by write readiness", err)
	}
	if _, err := InspectManagedBuildCache(context.Background(), client, "system", pod.Name, string(pod.UID)); err == nil {
		t.Fatal("legacy writer enabled native deletion")
	}
}
