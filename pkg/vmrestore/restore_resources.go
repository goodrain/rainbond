package vmrestore

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/goodrain/rainbond/pkg/vmrestore/copyio"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// RestoreCoreAnnotation carries a full SHA only as an annotation, never a label.
	RestoreCoreAnnotation = "rainbond.io/vm-restore-core-hash"
	// RestoreFrameAnnotation pins immutable resource planning inputs.
	RestoreFrameAnnotation = "rainbond.io/vm-restore-frame-hash"
	// RestorePurposeLabel marks ownership purpose, not permission to adopt an object.
	RestorePurposeLabel = "rainbond.io/vm-restore-purpose"
)

// RestoreDiskResource pins each native CSI snapshot and original B claim identity.
type RestoreDiskResource struct {
	VolumeID           uint      `json:"volume_id"`
	SnapshotContent    ObjectRef `json:"snapshot_content"`
	Snapshot           ObjectRef `json:"snapshot"`
	PreviousMaxBytes   int64     `json:"previous_max_bytes"`
	PreviousPVC        ObjectRef `json:"previous_pvc"`
	ROXClass           string    `json:"rox_class"`
	ROXClassUID        string    `json:"rox_class_uid"`
	TargetClass        string    `json:"target_class"`
	TargetAccessMode   string    `json:"target_access_mode"`
	TargetClassUID     string    `json:"target_class_uid"`
	ROXRequestBytes    int64     `json:"rox_request_bytes"`
	TargetRequestBytes int64     `json:"target_request_bytes"`
}

// RestoreResourcePlan is a bounded Region resource frame, not a second manifest.
// All fields derive from trusted certification, native projection and Region state.
type RestoreResourcePlan struct {
	Scope            Scope                 `json:"scope"`
	OperationID      string                `json:"operation_id"`
	CoreHash         string                `json:"core_hash"`
	FrameHash        string                `json:"frame_hash"`
	SourceGeneration uint64                `json:"source_generation"`
	Attempt          uint                  `json:"attempt"`
	WorkerImage      string                `json:"worker_image"`
	RuntimeImageID   string                `json:"runtime_image_id"`
	DeadlineSeconds  int64                 `json:"deadline_seconds"`
	TargetUID        uint32                `json:"target_uid"`
	TargetGID        uint32                `json:"target_gid"`
	MaxDisks         int                   `json:"max_disks"`
	Disks            []RestoreDiskResource `json:"disks"`
}

func validPinnedImage(image string) bool {
	index := strings.LastIndex(image, "@sha256:")
	return index > 0 && hashPattern.MatchString(image[index+8:]) && !strings.ContainsAny(image, "\r\n\x00 \t")
}
func validRuntimeImageID(value string) bool {
	index := strings.LastIndex(value, "sha256:")
	return index >= 0 && len(value) <= 1024 && hashPattern.MatchString(value[index+7:]) && !strings.ContainsAny(value, "\r\n\x00 \t")
}

// RestoreResourceFrameHash seals all resource names, snapshot/class identities and tool inputs.
func RestoreResourceFrameHash(plan RestoreResourcePlan) (string, error) {
	plan.FrameHash = ""
	return snapshotDigest(plan)
}
func (p RestoreResourcePlan) validate() error {
	want, err := RestoreResourceFrameHash(p)
	if err != nil || want != p.FrameHash || !validScope(p.Scope) || !idPattern.MatchString(p.OperationID) || !hashPattern.MatchString(p.CoreHash) || !validPinnedImage(p.WorkerImage) || !validRuntimeImageID(p.RuntimeImageID) || p.SourceGeneration == math.MaxUint64 || p.Attempt > 9999 || p.DeadlineSeconds < 1 || p.DeadlineSeconds > 86400 || p.MaxDisks < 1 || p.MaxDisks > 128 || len(p.Disks) == 0 || len(p.Disks) > p.MaxDisks {
		return ErrCorrupt
	}
	seen := map[uint]bool{}
	for _, d := range p.Disks {
		if d.VolumeID == 0 || seen[d.VolumeID] || d.SnapshotContent.Kind != "VolumeSnapshotContent" || d.SnapshotContent.Namespace != "" || !validName(d.SnapshotContent.Name) || !validUID(d.SnapshotContent.UID) || d.Snapshot.Kind != "VolumeSnapshot" || d.Snapshot.Namespace != p.Scope.Namespace || !validName(d.Snapshot.Name) || !validUID(d.Snapshot.UID) || !validRef(d.PreviousPVC) || d.PreviousPVC.Namespace != p.Scope.Namespace || !validName(d.ROXClass) || !validUID(d.ROXClassUID) || !validName(d.TargetClass) || !validUID(d.TargetClassUID) || d.ROXClass == d.TargetClass || d.ROXRequestBytes < 1 || d.TargetRequestBytes < d.ROXRequestBytes || d.PreviousMaxBytes < 1 || (d.TargetAccessMode != "ReadWriteMany" && d.TargetAccessMode != "ReadWriteOnce") {
			return ErrCorrupt
		}
		seen[d.VolumeID] = true
	}
	return nil
}
func (p RestoreResourcePlan) name(volumeID uint, purpose string) string {
	sum, _ := snapshotDigest(struct {
		Scope      Scope
		Operation  string
		Generation uint64
		Attempt    uint
	}{p.Scope, p.OperationID, p.SourceGeneration, p.Attempt})
	if volumeID == 0 {
		return fmt.Sprintf("vmr-%s-%s", sum[:16], purpose)
	}
	return fmt.Sprintf("vmr-%s-%d-%s", sum[:16], volumeID, purpose)
}
func (p RestoreResourcePlan) metadata(name, purpose string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: name, Namespace: p.Scope.Namespace, Labels: map[string]string{"tenant_id": p.Scope.TenantID, "service_id": p.Scope.ServiceID, RestoreOperationLabel: p.OperationID, RestorePurposeLabel: purpose}, Annotations: map[string]string{RestoreCoreAnnotation: p.CoreHash, RestoreFrameAnnotation: p.FrameHash}}
}

// RestoreClaims builds a ROX view and a genuinely blank target. No DataVolume,
// writable source clone, VM owner reference or source-volume mutation is created.
func RestoreClaims(plan RestoreResourcePlan, volumeID uint) (*corev1.PersistentVolumeClaim, *corev1.PersistentVolumeClaim, error) {
	if err := plan.validate(); err != nil {
		return nil, nil, err
	}
	var d RestoreDiskResource
	found := false
	for _, disk := range plan.Disks {
		if disk.VolumeID == volumeID {
			d = disk
			found = true
		}
	}
	if !found {
		return nil, nil, ErrConflict
	}
	mode := corev1.PersistentVolumeFilesystem
	group := "snapshot.storage.k8s.io"
	class := d.ROXClass
	build := func(name, purpose string, bytes int64, access corev1.PersistentVolumeAccessMode) *corev1.PersistentVolumeClaim {
		return &corev1.PersistentVolumeClaim{ObjectMeta: plan.metadata(name, purpose), Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: &class, VolumeMode: &mode, AccessModes: []corev1.PersistentVolumeAccessMode{access}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: *resource.NewQuantity(bytes, resource.BinarySI)}}}}
	}
	rox := build(plan.name(d.VolumeID, "rox"), "restore-rox", d.ROXRequestBytes, corev1.ReadOnlyMany)
	rox.Spec.DataSource = &corev1.TypedLocalObjectReference{APIGroup: &group, Kind: "VolumeSnapshot", Name: d.Snapshot.Name}
	target := build(plan.name(d.VolumeID, "target"), "restore-target", d.TargetRequestBytes, corev1.PersistentVolumeAccessMode(d.TargetAccessMode))
	targetClass := d.TargetClass
	target.Spec.StorageClassName = &targetClass
	if err := MarkRestorePVC(rox, plan.Scope, d.VolumeID, plan.SourceGeneration+1, plan.OperationID); err != nil {
		return nil, nil, err
	}
	if err := MarkRestorePVC(target, plan.Scope, d.VolumeID, plan.SourceGeneration+1, plan.OperationID); err != nil {
		return nil, nil, err
	}
	return rox, target, nil
}

// RestoreMounts contains actual server-read identities after PVC binding.
type RestoreMounts struct {
	VolumeID uint
	ROX      ObjectRef
	Previous ObjectRef
	Target   *ObjectRef
}

// RestoreCopyJob runs disks sequentially in one fixed binary. Jobs never retry
// automatically and have no TTL: evidence must be persisted before removal.
func RestoreCopyJob(frame RestoreResourcePlan, plan copyio.Plan, mounts []RestoreMounts) (*batchv1.Job, *corev1.ConfigMap, error) {
	if frame.validate() != nil || plan.Validate() != nil || plan.FrameHash != frame.FrameHash || plan.ManifestHash != frame.CoreHash || plan.OperationID != frame.OperationID || len(plan.Disks) != len(frame.Disks) || len(mounts) != len(plan.Disks) || plan.TargetUID != frame.TargetUID || plan.TargetGID != frame.TargetGID || plan.DeadlineSeconds != frame.DeadlineSeconds {
		return nil, nil, ErrConflict
	}
	body, err := json.Marshal(plan)
	if err != nil || len(body) > copyio.MaxPlanBytes {
		return nil, nil, ErrCorrupt
	}
	planMode := int32(0444)
	immutable := true
	zero := int32(0)
	one := int32(1)
	disabled := false
	enabled := true
	uid := int64(0)
	gid := int64(frame.TargetGID)
	grace := int64(30)
	purpose := "restore-" + plan.Mode
	metadata := frame.metadata(frame.name(0, plan.Mode), purpose)
	cm := &corev1.ConfigMap{ObjectMeta: frame.metadata(frame.name(0, plan.Mode+"-plan"), "restore-plan"), Immutable: &immutable, Data: map[string]string{"plan.json": string(body)}}
	volumes := []corev1.Volume{{Name: "plan", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: cm.Name}, DefaultMode: &planMode}}}, {Name: "work", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}}
	mounted := []corev1.VolumeMount{{Name: "plan", MountPath: "/plan", ReadOnly: true}, {Name: "work", MountPath: "/work"}}
	for index, disk := range plan.Disks {
		mount := mounts[index]
		expected := frame.Disks[index]
		if mount.VolumeID != disk.VolumeID || expected.VolumeID != disk.VolumeID || !validRef(mount.ROX) || mount.ROX.Namespace != frame.Scope.Namespace || mount.ROX.Name != frame.name(disk.VolumeID, "rox") || mount.ROX.UID != disk.SourcePVCUID || !sameObject(mount.Previous, expected.PreviousPVC) || mount.Previous.UID != disk.PreviousPVCUID {
			return nil, nil, ErrConflict
		}
		add := func(suffix, name, path string, readOnly bool) {
			volumeName := suffix + strconv.Itoa(index)
			volumes = append(volumes, corev1.Volume{Name: volumeName, VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: name, ReadOnly: readOnly}}})
			mounted = append(mounted, corev1.VolumeMount{Name: volumeName, MountPath: path, ReadOnly: readOnly})
		}
		sourceDir, beforeDir, targetDir := copyio.Directories("/volumes", index)
		add("input", mount.ROX.Name, sourceDir, true)
		add("previous", mount.Previous.Name, beforeDir, true)
		if plan.Mode != "inspect" {
			if mount.Target == nil || !validRef(*mount.Target) || mount.Target.Namespace != frame.Scope.Namespace || mount.Target.Name != frame.name(disk.VolumeID, "target") || mount.Target.UID != disk.TargetPVCUID {
				return nil, nil, ErrConflict
			}
			add("target", mount.Target.Name, targetDir, plan.Mode == "verify")
		} else if disk.TargetPVCUID != "" {
			return nil, nil, ErrConflict
		}
	}
	caps := &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}
	if plan.Mode == "copy" {
		caps.Add = []corev1.Capability{"CHOWN"}
	}
	job := &batchv1.Job{ObjectMeta: metadata, Spec: batchv1.JobSpec{BackoffLimit: &zero, Parallelism: &one, Completions: &one, ActiveDeadlineSeconds: &frame.DeadlineSeconds, Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: metadata.Labels, Annotations: metadata.Annotations}, Spec: corev1.PodSpec{AutomountServiceAccountToken: &disabled, RestartPolicy: corev1.RestartPolicyNever, TerminationGracePeriodSeconds: &grace, Volumes: volumes, SecurityContext: &corev1.PodSecurityContext{RunAsUser: &uid, RunAsGroup: &gid, SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}}, Containers: []corev1.Container{{Name: "copy", Image: frame.WorkerImage, ImagePullPolicy: corev1.PullIfNotPresent, Command: []string{"/vm-restore-copy"}, VolumeMounts: mounted, TerminationMessagePolicy: corev1.TerminationMessageReadFile, SecurityContext: &corev1.SecurityContext{AllowPrivilegeEscalation: &disabled, Privileged: &disabled, ReadOnlyRootFilesystem: &enabled, Capabilities: caps}, Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m"), corev1.ResourceMemory: resource.MustParse("128Mi")}, Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("256Mi")}}}}}}}}
	return job, cm, nil
}
