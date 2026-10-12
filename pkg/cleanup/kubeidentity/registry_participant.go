package kubeidentity

import (
	"context"
	"strings"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// InspectRegistryParticipant verifies the instance and deployment arguments
// against a previously provisioned binding. It never accepts caller image IDs.
func InspectRegistryParticipant(ctx context.Context, client kubernetes.Interface, namespace, serviceName, podName, podUID, owner string, binding coordination.StorageRegistration) (coordination.ParticipantRegistration, error) {
	denied := coordination.ParticipantRegistration{}
	prepared, err := InspectNativeRegistry(ctx, client, namespace, serviceName, podName, podUID)
	if err != nil || prepared.Root != binding.RootPath || prepared.Mount.VolumeUID != binding.VolumeUID {
		return denied, ErrBinding
	}
	pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil || string(pod.UID) != podUID || pod.ResourceVersion != prepared.Mount.PodVersion {
		return denied, ErrBinding
	}
	var sidecar *corev1.Container
	for i := range pod.Spec.Containers {
		candidate := &pod.Spec.Containers[i]
		if len(candidate.Command) == 1 && candidate.Command[0] == "/registry-coordinator" {
			if sidecar != nil {
				return denied, ErrBinding
			}
			sidecar = candidate
		}
	}
	if sidecar == nil {
		return denied, ErrBinding
	}
	args, err := explicitArguments(sidecar)
	if err != nil {
		return denied, err
	}
	if args["storage-id"] != binding.StorageID || args["storage-generation"] != binding.Generation || args["volume-uid"] != binding.VolumeUID || args["registry-path"] != binding.RootPath || args["owner"] == "" || !strings.HasPrefix(owner, args["owner"]+":") {
		return denied, ErrBinding
	}
	nonce := strings.TrimPrefix(owner, args["owner"]+":")
	if len(nonce) != 32 {
		return denied, ErrBinding
	}
	for _, character := range nonce {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return denied, ErrBinding
		}
	}
	request := RegistryMountRequest{Namespace: namespace, Service: serviceName, Pod: podName, PodUID: podUID, RegistryContainer: prepared.Container, CoordinatorContainer: sidecar.Name, RegistryRoot: binding.RootPath, CoordinatorRoot: args["storage-root"]}
	service, err := InspectRegistryService(ctx, client, request)
	if err != nil || service.VolumeUID != binding.VolumeUID {
		return denied, ErrBinding
	}
	var status *corev1.ContainerStatus
	for i := range pod.Status.ContainerStatuses {
		if pod.Status.ContainerStatuses[i].Name == sidecar.Name {
			if status != nil {
				return denied, ErrBinding
			}
			status = &pod.Status.ContainerStatuses[i]
		}
	}
	if status == nil || status.State.Running == nil || status.ContainerID == "" || status.ImageID == "" {
		return denied, ErrBinding
	}
	current, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil || current.UID != pod.UID || current.ResourceVersion != pod.ResourceVersion {
		return denied, ErrBinding
	}
	fingerprint, err := binding.Fingerprint()
	if err != nil {
		return denied, ErrBinding
	}
	return coordination.ParticipantRegistration{StorageID: binding.StorageID, Generation: binding.Generation, Owner: owner, Role: "registry-ingress", PodUID: podUID, ContainerID: status.ContainerID, ImageID: status.ImageID, BindingFingerprint: fingerprint}, nil
}
