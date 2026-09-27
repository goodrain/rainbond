package kubeidentity

import (
	"context"
	"reflect"
	"strings"
	"time"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// InspectNodeExecutor derives native admission facts from the bound Job and
// current node/Pod/mount, never from claims in an executor request.
func InspectNodeExecutor(ctx context.Context, client kubernetes.Interface, job *batchv1.Job, podName, podUID string, binding coordination.StorageRegistration, execution coordination.NodeJobBinding) (coordination.NodeExecutorIdentity, error) {
	return inspectNodeExecutor(ctx, client, job, podName, podUID, binding, execution, false)
}

// InspectTerminatedNodeExecutor verifies the original native process has exited.
func InspectTerminatedNodeExecutor(ctx context.Context, client kubernetes.Interface, job *batchv1.Job, podName, podUID string, binding coordination.StorageRegistration, execution coordination.NodeJobBinding) (coordination.NodeExecutorIdentity, error) {
	return inspectNodeExecutor(ctx, client, job, podName, podUID, binding, execution, true)
}

func inspectNodeExecutor(ctx context.Context, client kubernetes.Interface, job *batchv1.Job, podName, podUID string, binding coordination.StorageRegistration, execution coordination.NodeJobBinding, terminal bool) (coordination.NodeExecutorIdentity, error) {
	denied := coordination.NodeExecutorIdentity{}
	if client == nil || job == nil || job.UID == "" || job.Namespace == "" || job.Spec.Suspend == nil || *job.Spec.Suspend || job.DeletionTimestamp != nil || podName == "" || podUID == "" {
		return denied, ErrBinding
	}
	if coordination.ValidateBoundNodeJob(job, execution) != nil || binding.RootPath != "/cache/build" {
		return denied, ErrBinding
	}
	if _, err := binding.Fingerprint(); err != nil {
		return denied, ErrBinding
	}
	pod, err := client.CoreV1().Pods(job.Namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil || string(pod.UID) != podUID || pod.DeletionTimestamp != nil || len(pod.OwnerReferences) != 1 {
		return denied, ErrBinding
	}
	owner := pod.OwnerReferences[0]
	if owner.APIVersion != "batch/v1" || owner.Kind != "Job" || owner.Name != job.Name || owner.UID != job.UID || owner.Controller == nil || !*owner.Controller {
		return denied, ErrBinding
	}
	spec := &pod.Spec
	expected := &job.Spec.Template.Spec
	if spec.NodeName == "" || expected.NodeName != "" && spec.NodeName != expected.NodeName || spec.HostNetwork || spec.HostIPC || spec.HostPID || spec.RestartPolicy != corev1.RestartPolicyNever || spec.AutomountServiceAccountToken == nil || *spec.AutomountServiceAccountToken || len(spec.Containers) != 1 || len(spec.EphemeralContainers) > 0 {
		return denied, ErrBinding
	}
	// Do not accept admission-injected commands, environment, mounts or sidecars.
	if !reflect.DeepEqual(spec.Containers, expected.Containers) || !reflect.DeepEqual(spec.InitContainers, expected.InitContainers) || !reflect.DeepEqual(spec.Volumes, expected.Volumes) || !reflect.DeepEqual(spec.SecurityContext, expected.SecurityContext) || !reflect.DeepEqual(spec.ImagePullSecrets, expected.ImagePullSecrets) {
		return denied, ErrBinding
	}
	account := expected.ServiceAccountName
	if account == "" {
		account = "default"
	}
	actualAccount := spec.ServiceAccountName
	if actualAccount == "" {
		actualAccount = "default"
	}
	if actualAccount != account {
		return denied, ErrBinding
	}
	container := &spec.Containers[0]
	if len(container.Command) != 1 || container.Command[0] != "/app/node-cleanup" || !strings.Contains(container.Image, "@sha256:") {
		return denied, ErrBinding
	}
	if len(pod.Status.ContainerStatuses) != 1 {
		return denied, ErrBinding
	}
	status := pod.Status.ContainerStatuses[0]
	if status.Name != container.Name || status.RestartCount != 0 || status.ContainerID == "" || status.LastTerminationState.Terminated != nil {
		return denied, ErrBinding
	}
	running := pod.Status.Phase == corev1.PodRunning && status.State.Running != nil && status.State.Terminated == nil
	if terminal && !running {
		if (pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed) || status.State.Terminated == nil || status.State.Running != nil || status.State.Terminated.FinishedAt.IsZero() {
			return denied, ErrBinding
		}
	} else if !terminal && !running {
		return denied, ErrBinding
	}
	imageID := strings.TrimPrefix(strings.TrimPrefix(status.ImageID, "docker-pullable://"), "containerd://")
	// Runtime may report only the digest rather than the repository-qualified ID.
	digest := container.Image[strings.LastIndex(container.Image, "@")+1:]
	if imageID != container.Image && imageID != digest {
		return denied, ErrBinding
	}
	node, err := client.CoreV1().Nodes().Get(ctx, spec.NodeName, metav1.GetOptions{})
	if err != nil || node.DeletionTimestamp != nil || string(node.UID) != execution.NodeUID || spec.NodeName != execution.NodeName {
		return denied, ErrBinding
	}
	if execution.PodUID != "" && (execution.PodUID != podUID || execution.PodName != podName || execution.ContainerID != status.ContainerID || execution.ImageID != container.Image) {
		return denied, ErrBinding
	}
	mount, relative, err := effectiveMount(container, binding.RootPath)
	if err != nil || mount.ReadOnly || mount.MountPropagation != nil && *mount.MountPropagation != corev1.MountPropagationNone {
		return denied, ErrBinding
	}
	observed, err := inspectVolume(ctx, client, pod, mount.Name, relative)
	if err != nil || observed.VolumeUID != binding.VolumeUID {
		return denied, ErrBinding
	}
	// Reject an object changing across this multi-object observation. The native node helper
	// additionally pins and verifies the actual directory descriptor at execution.
	current, err := client.CoreV1().Pods(job.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil || current.UID != pod.UID || current.ResourceVersion != pod.ResourceVersion {
		return denied, ErrBinding
	}
	currentJob, err := client.BatchV1().Jobs(job.Namespace).Get(ctx, job.Name, metav1.GetOptions{})
	if err != nil || currentJob.UID != job.UID || currentJob.ResourceVersion != job.ResourceVersion {
		return denied, ErrBinding
	}
	if terminal && running {
		return denied, ErrExecutorRunning
	}
	var finishedAt time.Time
	if terminal {
		finishedAt = status.State.Terminated.FinishedAt.Time
	}
	return coordination.NodeExecutorIdentity{FinishedAt: finishedAt, Namespace: job.Namespace, JobName: job.Name, JobUID: string(job.UID), PodName: podName, PodUID: podUID, NodeName: spec.NodeName, NodeUID: string(node.UID), VolumeUID: observed.VolumeUID, SpecHash: execution.SpecHash, ContainerID: status.ContainerID, ImageID: container.Image}, nil
}
