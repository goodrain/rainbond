package kubeidentity

import (
	"context"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type legacyCleanerObservation struct {
	UID         string
	Spec        corev1.PodSpec
	ContainerID string
	ImageID     string
}

// inspectLegacyCleaners requires every observed system chaos process to have
// explicitly disabled its pre-plugin automatic image/slug deletion and native
// GC loop. This is source evidence, not sufficient proof of producer coverage.
func inspectLegacyCleaners(ctx context.Context, client kubernetes.Interface, namespace string) ([]legacyCleanerObservation, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: "name=rbd-chaos", Limit: 65})
	if err != nil || pods.ResourceVersion == "" || pods.Continue != "" || len(pods.Items) == 0 || len(pods.Items) > 64 {
		return nil, ErrBinding
	}
	for i := range pods.Items {
		if err := validateDisabledCleaner(&pods.Items[i]); err != nil {
			return nil, err
		}
	}
	sort.Slice(pods.Items, func(i, j int) bool { return pods.Items[i].Name < pods.Items[j].Name })
	observations := make([]legacyCleanerObservation, 0, len(pods.Items))
	for _, pod := range pods.Items {
		for _, status := range pod.Status.ContainerStatuses {
			if status.Name == pod.Spec.Containers[0].Name {
				observations = append(observations, legacyCleanerObservation{string(pod.UID), pod.Spec, status.ContainerID, status.ImageID})
			}
		}
	}
	return observations, nil
}

func validateDisabledCleaner(pod *corev1.Pod) error {

	if pod.UID == "" || pod.ResourceVersion == "" || pod.DeletionTimestamp != nil || pod.Status.Phase != corev1.PodRunning || len(pod.Spec.Containers) != 1 || len(pod.Spec.EphemeralContainers) != 0 {
		return ErrBinding
	}
	c := &pod.Spec.Containers[0]
	if len(c.Command) != 1 || c.Command[0] != "/run/rainbond-chaos" || !pinnedRuntimeImage(pod, c) {
		return ErrBinding
	}
	seen := false
	for _, arg := range c.Args {
		if arg == "--" {
			return ErrBinding
		}
		if arg == "--clean-up" || strings.HasPrefix(arg, "--clean-up=") {
			if seen || arg != "--clean-up=false" {
				return ErrBinding
			}
			seen = true
		}
	}
	if !seen {
		return ErrBinding
	}
	return nil
}
