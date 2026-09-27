package kubeidentity

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ManagedCachePreparation contains observed system builder storage identity,
// not arbitrary caller paths or proof that deletion is safe.
type ManagedCachePreparation struct {
	Root    string
	NodeUID string
	Mount   RegistryMountObservation
}

// InspectManagedBuildCache binds the fixed Rainbond build cache to a live
// system builder and actual PVC/PV or node hostPath. It never reads secrets.
func InspectManagedBuildCache(ctx context.Context, client kubernetes.Interface, namespace, podName, podUID string) (ManagedCachePreparation, error) {
	return inspectManagedBuildCache(ctx, client, namespace, podName, podUID, true)
}

// InspectManagedBuildCacheSource observes storage for read-only enrollment and
// collection. It does not attest that the writer is coordinated or safe to delete.
func InspectManagedBuildCacheSource(ctx context.Context, client kubernetes.Interface, namespace, podName, podUID string) (ManagedCachePreparation, error) {
	return inspectManagedBuildCache(ctx, client, namespace, podName, podUID, false)
}

func inspectManagedBuildCache(ctx context.Context, client kubernetes.Interface, namespace, podName, podUID string, requireCoordinated bool) (ManagedCachePreparation, error) {
	denied := ManagedCachePreparation{}
	if client == nil || namespace == "" || podName == "" || podUID == "" {
		return denied, ErrBinding
	}
	pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil || string(pod.UID) != podUID || pod.Labels["name"] != "rbd-chaos" || pod.Spec.NodeName == "" || pod.ResourceVersion == "" || pod.DeletionTimestamp != nil || pod.Status.Phase != corev1.PodRunning || len(pod.Spec.Containers) != 1 || (requireCoordinated && validateDisabledCleaner(pod) != nil) {
		return denied, ErrBinding
	}
	node, err := client.CoreV1().Nodes().Get(ctx, pod.Spec.NodeName, metav1.GetOptions{})
	if err != nil || node.UID == "" || node.DeletionTimestamp != nil {
		return denied, ErrBinding
	}
	const root = "/cache/build"
	mount, relative, err := effectiveMount(&pod.Spec.Containers[0], root)
	if err != nil || mount.ReadOnly || (mount.MountPropagation != nil && *mount.MountPropagation != corev1.MountPropagationNone) {
		return denied, ErrBinding
	}
	observed, err := inspectVolume(ctx, client, pod, mount.Name, relative)
	if err != nil {
		return denied, err
	}
	current, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil || current.UID != pod.UID || current.ResourceVersion != pod.ResourceVersion {
		return denied, ErrBinding
	}
	return ManagedCachePreparation{Root: root, NodeUID: string(node.UID), Mount: observed}, nil
}
