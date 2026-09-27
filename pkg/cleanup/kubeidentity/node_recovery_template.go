package kubeidentity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// BuildNodeRecoveryJob constructs a suspended receipt-only helper after proving
// original termination and journal identity. It does not create or start a Job.
// The helper has no cache volume and cannot replay the native deletion.
func BuildNodeRecoveryJob(ctx context.Context, client kubernetes.Interface, original *batchv1.Job, storage coordination.StorageRegistration, binding coordination.NodeJobBinding) (*batchv1.Job, error) {
	if binding.PodUID == "" || binding.CanceledBeforeGrantAt != nil {
		return nil, ErrBinding
	}
	if _, err := InspectTerminatedNodeExecutor(ctx, client, original, binding.PodName, binding.PodUID, storage, binding); err != nil {
		return nil, err
	}
	if err := InspectNodeJournalVolume(ctx, client, original, binding); err != nil {
		return nil, err
	}
	spec := original.Spec.Template.Spec.DeepCopy()
	container := &spec.Containers[0]
	if container.Lifecycle != nil || len(container.EnvFrom) > 0 || len(container.VolumeDevices) > 0 {
		return nil, ErrBinding
	}
	for _, arg := range container.Args {
		if arg == "--" || strings.HasPrefix(arg, "--recover") || strings.HasPrefix(arg, "--original-pod") {
			return nil, ErrBinding
		}
	}
	keptMounts := []corev1.VolumeMount{}
	for _, mount := range container.VolumeMounts {
		if mount.Name == "node-state" && mount.MountPath == "/node-state" {
			keptMounts = append(keptMounts, mount)
		}
		if mount.Name == "node-control" && mount.MountPath == "/node-control" && mount.ReadOnly && mount.SubPath == "" && mount.SubPathExpr == "" {
			keptMounts = append(keptMounts, mount)
		}
	}
	if len(keptMounts) != 2 {
		return nil, ErrBinding
	}
	keptVolumes := []corev1.Volume{}
	for _, volume := range spec.Volumes {
		if volume.Name == "node-state" {
			keptVolumes = append(keptVolumes, volume)
		}
		if volume.Name == "node-control" && volume.Secret != nil {
			keptVolumes = append(keptVolumes, volume)
		}
	}
	if len(keptVolumes) != 2 {
		return nil, ErrBinding
	}
	spec.Volumes = keptVolumes
	container.VolumeMounts = keptMounts
	container.Args = append(container.Args, "--recover", "--original-pod="+binding.PodName, "--original-pod-uid="+binding.PodUID)
	// Preserve the original user/group and pinned image, and disallow writes to
	// any image filesystem path even if an older source omitted this setting.
	no, yes := false, true
	if container.SecurityContext == nil {
		container.SecurityContext = &corev1.SecurityContext{}
	}
	container.SecurityContext.AllowPrivilegeEscalation = &no
	container.SecurityContext.ReadOnlyRootFilesystem = &yes
	container.SecurityContext.Capabilities = &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}
	container.LivenessProbe = nil
	container.ReadinessProbe = nil
	container.StartupProbe = nil
	spec.AutomountServiceAccountToken = &no
	sum := sha256.Sum256([]byte(binding.JobUID + "\x00" + binding.PodUID))
	zero, one := int32(0), int32(1)
	return &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "rainbond-node-recover-" + hex.EncodeToString(sum[:16]), Namespace: original.Namespace}, Spec: batchv1.JobSpec{Suspend: &yes, BackoffLimit: &zero, Parallelism: &one, Completions: &one, Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"rainbond.io/task": "node-recovery"}, Annotations: map[string]string{nodeJournalPVCUID: original.Spec.Template.Annotations[nodeJournalPVCUID], nodeJournalPVUID: original.Spec.Template.Annotations[nodeJournalPVUID]}}, Spec: *spec}}}, nil
}
