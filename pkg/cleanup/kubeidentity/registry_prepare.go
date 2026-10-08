package kubeidentity

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// RegistryPreparation identifies the native data root before adding a sidecar.
// It does not assert ingress isolation or participant readiness.
type RegistryPreparation struct {
	Mount           RegistryMountObservation
	Container, Root string
}

// InspectNativeRegistry inspects the configured system Service and selected Pod.
// Only an explicit native filesystem root is accepted here; inspecting config
// files for installations without that override is a separate trusted step.
func InspectNativeRegistry(ctx context.Context, client kubernetes.Interface, namespace, serviceName, podName, podUID string) (RegistryPreparation, error) {
	denied := RegistryPreparation{}
	if client == nil || namespace == "" || serviceName == "" || podName == "" || podUID == "" {
		return denied, ErrBinding
	}
	service, err := client.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil || len(service.Spec.Selector) == 0 || service.Spec.Type == corev1.ServiceTypeExternalName {
		return denied, ErrBinding
	}
	pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil || string(pod.UID) != podUID || pod.DeletionTimestamp != nil || !labels.SelectorFromSet(service.Spec.Selector).Matches(labels.Set(pod.Labels)) {
		return denied, ErrBinding
	}
	var selected *corev1.Container
	root := ""
	for i := range pod.Spec.Containers {
		hasRoot := false
		for _, env := range pod.Spec.Containers[i].Env {
			if env.Name == "REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY" {
				hasRoot = true
			}
		}
		if !hasRoot {
			continue
		}
		value, err := literalEnv(&pod.Spec.Containers[i], "REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY")
		if err != nil {
			return denied, ErrBinding
		}
		if selected != nil || !cleanRoot(value) {
			return denied, ErrBinding
		}
		selected = &pod.Spec.Containers[i]
		root = value
	}
	if selected == nil {
		return denied, ErrBinding
	}
	mount, relative, err := effectiveMount(selected, root)
	if err != nil || mount.ReadOnly {
		return denied, ErrBinding
	}
	observation, err := inspectVolume(ctx, client, pod, mount.Name, relative)
	if err != nil {
		return denied, err
	}
	return RegistryPreparation{Mount: observation, Container: selected.Name, Root: root}, nil
}
