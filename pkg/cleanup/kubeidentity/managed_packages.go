package kubeidentity

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ManagedPackagePreparation contains the physical identity of Rainbond's fixed
// package_build subtree. It is neither a path supplied by a caller nor deletion readiness.
type ManagedPackagePreparation struct {
	Root    string
	NodeUID string
	Mount   RegistryMountObservation
}

// InspectManagedPackageSource observes the live rbd-chaos grdata mount for
// read-only enrollment. A separate check is required before destructive work.
func InspectManagedPackageSource(ctx context.Context, client kubernetes.Interface, namespace, podName, podUID string) (ManagedPackagePreparation, error) {
	return inspectManagedPackage(ctx, client, namespace, podName, podUID, false)
}

// InspectManagedPackage requires the coordinated/pinned source used to rebuild
// an original deletion Job immediately before its start.
func InspectManagedPackage(ctx context.Context, client kubernetes.Interface, namespace, podName, podUID string) (ManagedPackagePreparation, error) {
	return inspectManagedPackage(ctx, client, namespace, podName, podUID, true)
}

func inspectManagedPackage(ctx context.Context, client kubernetes.Interface, namespace, podName, podUID string, requireCoordinated bool) (ManagedPackagePreparation, error) {
	denied := ManagedPackagePreparation{}
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
	const root = "/grdata/package_build"
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
	return ManagedPackagePreparation{Root: root, NodeUID: string(node.UID), Mount: observed}, nil
}
