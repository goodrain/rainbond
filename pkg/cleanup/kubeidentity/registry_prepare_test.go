package kubeidentity

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPreparationUsesNativeConfigurationNotCallerRoot(t *testing.T) {
	service, pod, pvc, pv := bindingObjects()
	pod.Spec.Containers = pod.Spec.Containers[:1]
	pod.Spec.Containers[0].Env = []corev1.EnvVar{{Name: "REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY", Value: "/var/lib/registry"}}
	result, err := InspectNativeRegistry(context.Background(), fake.NewSimpleClientset(service, pod, pvc, pv), "system", "rbd-hub", "hub", "pod-uid")
	if err != nil || result.Container != "registry" || result.Root != "/var/lib/registry" || result.Mount.VolumeUID == "" {
		t.Fatal(result, err)
	}
	pod.Spec.Containers[0].Env = nil
	if _, err := InspectNativeRegistry(context.Background(), fake.NewSimpleClientset(service, pod, pvc, pv), "system", "rbd-hub", "hub", "pod-uid"); err == nil {
		t.Fatal("guessed unobserved native storage root")
	}
}
