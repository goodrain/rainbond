package kubeidentity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// InspectCacheBuilder derives startup membership from the actual coordinated
// builder process and cache mount. Enrollment alone never certifies readiness.
func InspectCacheBuilder(ctx context.Context, client kubernetes.Interface, namespace, podName, podUID string, binding coordination.StorageRegistration) (coordination.ParticipantRegistration, error) {
	var denied coordination.ParticipantRegistration
	observed, err := InspectManagedBuildCache(ctx, client, namespace, podName, podUID)
	if err != nil || observed.Mount.VolumeUID != binding.VolumeUID || binding.RootPath != "/cache/build" {
		return denied, ErrBinding
	}
	fingerprint, err := binding.Fingerprint()
	if err != nil {
		return denied, err
	}
	domain := sha256.Sum256([]byte("managed-build-cache\x00" + binding.VolumeUID))
	if binding.StorageID != hex.EncodeToString(domain[:]) {
		return denied, ErrBinding
	}
	pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil || string(pod.UID) != podUID || pod.ResourceVersion != observed.Mount.PodVersion || validateDisabledCleaner(pod) != nil {
		return denied, ErrBinding
	}
	c := pod.Spec.Containers[0]
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == c.Name {
			identity := sha256.Sum256([]byte(podUID + "\x00" + status.ContainerID))
			return coordination.ParticipantRegistration{StorageID: binding.StorageID, Generation: binding.Generation, Owner: "cache-builder:" + hex.EncodeToString(identity[:]), Role: "cache-builder", PodUID: podUID, ContainerID: status.ContainerID, ImageID: c.Image, BindingFingerprint: fingerprint}, nil
		}
	}
	return denied, ErrBinding
}
