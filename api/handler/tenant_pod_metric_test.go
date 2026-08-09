package handler

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestPodResourceInformationFromPodPreservesPodIdentity(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "component-a-7f9d8",
			Namespace:       "tenant-a",
			UID:             types.UID("b073e66c-9430-41a6-bf9f-a93817d8c5b7"),
			ResourceVersion: "12345",
			Labels: map[string]string{
				"creator":    "Rainbond",
				"app_id":     "app-a",
				"service_id": "service-a",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: "node-a",
			Containers: []corev1.Container{{
				Name: "component",
				Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("100m"),
					corev1.ResourceMemory:           resource.MustParse("128Mi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
				}},
			}},
		},
	}

	got := podResourceInformationFromPod(pod)
	if got.PodName != pod.Name {
		t.Fatalf("pod name = %q, want %q", got.PodName, pod.Name)
	}
	if got.PodUID != string(pod.UID) {
		t.Fatalf("pod UID = %q, want %q", got.PodUID, pod.UID)
	}
}
