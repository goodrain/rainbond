package kubeidentity

import (
	"context"
	"strings"
	"testing"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestParticipantIdentityComesFromCurrentContainerStatus(t *testing.T) {
	service, pod, pvc, pv := bindingObjects()
	service.Spec.Ports = []corev1.ServicePort{{Port: 5000, TargetPort: intstr.FromInt(5001)}}
	pod.Spec.Containers[0].Env = []corev1.EnvVar{{Name: "REGISTRY_HTTP_ADDR", Value: "127.0.0.1:5000"}, {Name: "REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY", Value: "/var/lib/registry"}}
	pod.Spec.Containers[1].Command = []string{"/registry-coordinator"}
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "coordinator", ContainerID: "containerd://actual", ImageID: "actual-image", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}
	client := fake.NewSimpleClientset(service, pod, pvc, pv)
	preparation, err := InspectNativeRegistry(context.Background(), client, "system", "rbd-hub", "hub", "pod-uid")
	if err != nil {
		t.Fatal(err)
	}
	binding := coordination.StorageRegistration{StorageID: "store", Generation: "one", VolumeUID: preparation.Mount.VolumeUID, RootPath: preparation.Root}
	pod.Spec.Containers[1].Args = []string{"--listen=:5001", "--upstream=http://127.0.0.1:5000", "--storage-id=store", "--storage-generation=one", "--volume-uid=" + binding.VolumeUID, "--registry-path=/var/lib/registry", "--storage-root=/registry", "--owner=registry-pod"}
	if _, err := client.CoreV1().Pods("system").Update(context.Background(), pod, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	client.PrependReactor("list", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
		value, err := client.Tracker().List(corev1.SchemeGroupVersion.WithResource("pods"), corev1.SchemeGroupVersion.WithKind("Pod"), action.GetNamespace())
		if err == nil {
			value.(*corev1.PodList).ResourceVersion = "snapshot"
		}
		return true, value, err
	})
	owner := "registry-pod:" + strings.Repeat("a", 32)
	observed, err := InspectRegistryParticipant(context.Background(), client, "system", "rbd-hub", "hub", "pod-uid", owner, binding)
	if err != nil || observed.ContainerID != "containerd://actual" || observed.ImageID != "actual-image" || observed.Owner != owner {
		t.Fatal(observed, err)
	}
	if _, err := InspectRegistryParticipant(context.Background(), client, "system", "rbd-hub", "hub", "pod-uid", "forged-owner", binding); err == nil {
		t.Fatal("unbound owner accepted")
	}
	binding.VolumeUID = "forged-volume"
	if _, err := InspectRegistryParticipant(context.Background(), client, "system", "rbd-hub", "hub", "pod-uid", owner, binding); err == nil {
		t.Fatal("caller volume claim accepted")
	}
}
