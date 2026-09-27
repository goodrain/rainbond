package kubeidentity

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

// capability_id: rainbond.cleanup.registry-service-coverage
func TestRegistryServiceRejectsMixedAndEmptyDeployments(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		service, pod, pvc, pv := bindingObjects()
		service.Spec.Ports = []corev1.ServicePort{{Port: 5000, TargetPort: intstr.FromInt(5001)}}
		pod.Spec.Containers[0].Env = []corev1.EnvVar{{Name: "REGISTRY_HTTP_ADDR", Value: "127.0.0.1:5000"}}
		pod.Spec.Containers[1].Command = []string{"/registry-coordinator"}
		pod.Spec.Containers[1].Args = []string{"--listen=:5001", "--upstream=http://127.0.0.1:5000"}
		objects := []runtime.Object{service, pod, pvc, pv}
		if mixed {
			old := pod.DeepCopy()
			old.Name = "old-hub"
			old.UID = "old-pod"
			old.Spec.Containers = old.Spec.Containers[:1]
			objects = append(objects, old)
		}
		client := fake.NewSimpleClientset(objects...)
		client.PrependReactor("list", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
			result, err := client.Tracker().List(corev1.SchemeGroupVersion.WithResource("pods"), corev1.SchemeGroupVersion.WithKind("Pod"), action.GetNamespace())
			if err == nil {
				result.(*corev1.PodList).ResourceVersion = "snapshot-1"
			}
			return true, result, err
		})
		request := RegistryMountRequest{Namespace: "system", Service: "rbd-hub", RegistryContainer: "registry", CoordinatorContainer: "coordinator", RegistryRoot: "/var/lib/registry", CoordinatorRoot: "/registry"}
		observation, err := InspectRegistryService(context.Background(), client, request)
		if mixed {
			if err == nil {
				t.Fatal("legacy selected Pod ignored")
			}
		} else if err != nil || len(observation.PodUIDs) != 1 {
			t.Fatal(observation, err)
		}
		client.PrependReactor("list", "pods", func(ktesting.Action) (bool, runtime.Object, error) {
			list := &corev1.PodList{}
			list.ResourceVersion = "snapshot-2"
			return true, list, nil
		})
		if _, err := InspectRegistryService(context.Background(), client, request); err == nil {
			t.Fatal("empty service treated as fully coordinated")
		}
	}
}

func TestRegistryServiceBoundsEmptyContinuationPages(t *testing.T) {
	service, pod, pvc, pv := bindingObjects()
	client := fake.NewSimpleClientset(service, pod, pvc, pv)
	calls := 0
	client.PrependReactor("list", "pods", func(ktesting.Action) (bool, runtime.Object, error) {
		calls++
		if calls > 33 {
			return true, nil, fmt.Errorf("stop test pagination")
		}
		list := &corev1.PodList{}
		list.ResourceVersion = "snapshot"
		list.Continue = fmt.Sprint(calls)
		return true, list, nil
	})
	_, err := InspectRegistryService(context.Background(), client, RegistryMountRequest{Namespace: "system", Service: "rbd-hub"})
	if err == nil || calls > 32 {
		t.Fatalf("pagination not bounded: calls=%d", calls)
	}
}
