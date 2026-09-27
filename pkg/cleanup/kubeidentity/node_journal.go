package kubeidentity

import (
	"context"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const nodeJournalPVCUID = "rainbond.io/node-journal-pvc-uid"
const nodeJournalPVUID = "rainbond.io/node-journal-pv-uid"

// InspectNodeJournalVolume requires the original journal's concrete PVC and PV,
// not merely a same-name replacement. UID annotations are covered by the saved
// executor spec hash. It reads references only, never Secret contents or files.
func InspectNodeJournalVolume(ctx context.Context, client kubernetes.Interface, job *batchv1.Job, binding coordination.NodeJobBinding) error {
	if client == nil || coordination.ValidateBoundNodeJob(job, binding) != nil {
		return ErrBinding
	}
	expectedPVC := job.Spec.Template.Annotations[nodeJournalPVCUID]
	expectedPV := job.Spec.Template.Annotations[nodeJournalPVUID]
	if expectedPVC == "" || expectedPV == "" {
		return ErrBinding
	}
	container := &job.Spec.Template.Spec.Containers[0]
	mount, relative, err := effectiveMount(container, "/node-state")
	if err != nil || mount.Name != "node-state" || mount.MountPath != "/node-state" || relative != "" || mount.ReadOnly || (mount.MountPropagation != nil && *mount.MountPropagation != corev1.MountPropagationNone) {
		return ErrBinding
	}
	claimName := ""
	for _, volume := range job.Spec.Template.Spec.Volumes {
		if volume.Name == mount.Name {
			if volume.PersistentVolumeClaim == nil || volume.PersistentVolumeClaim.ReadOnly || claimName != "" {
				return ErrBinding
			}
			claimName = volume.PersistentVolumeClaim.ClaimName
		}
	}
	if claimName == "" {
		return ErrBinding
	}
	claim, err := client.CoreV1().PersistentVolumeClaims(job.Namespace).Get(ctx, claimName, metav1.GetOptions{})
	if err != nil || string(claim.UID) != expectedPVC || claim.DeletionTimestamp != nil || claim.Status.Phase != corev1.ClaimBound || claim.Spec.VolumeName == "" {
		return ErrBinding
	}
	volume, err := client.CoreV1().PersistentVolumes().Get(ctx, claim.Spec.VolumeName, metav1.GetOptions{})
	if err != nil || string(volume.UID) != expectedPV || volume.DeletionTimestamp != nil || volume.Spec.ClaimRef == nil || volume.Spec.ClaimRef.UID != claim.UID || volume.Spec.ClaimRef.Name != claim.Name || volume.Spec.ClaimRef.Namespace != job.Namespace {
		return ErrBinding
	}
	return ctx.Err()
}
