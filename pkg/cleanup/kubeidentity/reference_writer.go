package kubeidentity

import (
	"context"

	guard "github.com/goodrain/rainbond/pkg/cleanup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// InspectReferenceWriter binds a trusted startup protocol announcement to the
// current Kubernetes runtime. It does not infer software support from an image
// tag, accept caller image IDs, register evidence, or promote storage readiness.
func InspectReferenceWriter(ctx context.Context, client kubernetes.Interface, namespace, podName, podUID, role string) (guard.ReferenceWriter, error) {
	denied := guard.ReferenceWriter{}
	names := map[string]string{"api": "rbd-api", "worker": "rbd-worker", "builder": "rbd-chaos", "console": "rbd-app-ui"}
	name := names[role]
	if client == nil || namespace == "" || podName == "" || podUID == "" || name == "" {
		return denied, ErrBinding
	}
	pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil || pod == nil || string(pod.UID) != podUID || pod.Namespace != namespace || pod.ResourceVersion == "" || pod.DeletionTimestamp != nil || pod.Status.Phase != corev1.PodRunning || pod.Labels["name"] != name {
		return denied, ErrBinding
	}
	found := 0
	for _, container := range pod.Spec.Containers {
		if container.Name == name {
			found++
		}
	}
	if found != 1 {
		return denied, ErrBinding
	}
	var runtime *corev1.ContainerStatus
	for i := range pod.Status.ContainerStatuses {
		status := &pod.Status.ContainerStatuses[i]
		if status.Name == name {
			if runtime != nil {
				return denied, ErrBinding
			}
			runtime = status
		}
	}
	if runtime == nil || runtime.State.Running == nil || runtime.ContainerID == "" || runtime.ImageID == "" {
		return denied, ErrBinding
	}
	current, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil || current == nil || current.UID != pod.UID || current.ResourceVersion != pod.ResourceVersion {
		return denied, ErrBinding
	}
	return guard.ReferenceWriter{Namespace: namespace, PodName: podName, PodUID: podUID, ContainerName: name, ContainerID: runtime.ContainerID, ImageID: runtime.ImageID, Role: role, Protocol: guard.ReferenceWriterProtocol}, nil
}
