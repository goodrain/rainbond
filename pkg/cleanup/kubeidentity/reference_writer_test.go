package kubeidentity

import (
	"context"
	"testing"

	guard "github.com/goodrain/rainbond/pkg/cleanup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestReferenceWriterIdentityUsesActualRuntime(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "system", UID: "original", ResourceVersion: "1", Labels: map[string]string{"name": "rbd-api"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "rbd-api", Image: "mutable-tag"}}}, Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Name: "rbd-api", ContainerID: "containerd://actual", ImageID: "registry@sha256:actual", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}}}
	client := fake.NewSimpleClientset(pod)
	result, err := InspectReferenceWriter(context.Background(), client, "system", "api", "original", "api")
	if err != nil || result.ImageID != "registry@sha256:actual" || result.ContainerID != "containerd://actual" || result.Protocol != guard.ReferenceWriterProtocol {
		t.Fatal(result, err)
	}
	for _, input := range []struct{ uid, role string }{{"replacement", "api"}, {"original", "worker"}, {"original", "unknown"}} {
		if _, err := InspectReferenceWriter(context.Background(), client, "system", "api", input.uid, input.role); err == nil {
			t.Fatal("foreign runtime accepted")
		}
	}
	pod.Status.ContainerStatuses[0].ContainerID = ""
	client.CoreV1().Pods("system").Update(context.Background(), pod, metav1.UpdateOptions{})
	if _, err := InspectReferenceWriter(context.Background(), client, "system", "api", "original", "api"); err == nil {
		t.Fatal("missing runtime accepted")
	}
}
