package vmrestore

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/goodrain/rainbond/pkg/vmstorage"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	kv "kubevirt.io/api/core/v1"
)

var errNativeCloneFailed = errors.New("native restore storage clone failed")
var nativeLonghornVolumeResource = schema.GroupVersionResource{Group: "longhorn.io", Version: "v1beta2", Resource: "volumes"}

func (m *ManagedNativeRestore) preserveNativeSources(ctx context.Context, scope Scope, current Current, vm *kv.VirtualMachine) (*kv.VirtualMachine, bool, error) {
	result := vm.DeepCopy()
	retained := map[string]bool{}
	changed := false
	for _, layout := range current.Receipt.NativeRestore.Config.Hardware.DiskLayout {
		if layout.ReadOnly {
			continue
		}
		binding, exists := current.Receipt.OldBindings[strconv.FormatUint(uint64(layout.VolumeID), 10)]
		if !exists || !binding.Attached {
			return nil, false, ErrConflict
		}
		if binding.ActiveDV == nil {
			continue
		}
		dv, err := m.Resolver.dv(ctx, scope, binding.ActiveDV.Name, binding.ActiveDV)
		if err != nil || dv == nil {
			return nil, false, ErrReferenceUnavailable
		}
		claim, _, _ := unstructured.NestedString(dv.Object, "status", "claimName")
		pvc, err := m.Core.CoreV1().PersistentVolumeClaims(scope.Namespace).Get(ctx, binding.ActivePVC.Name, metav1.GetOptions{})
		if err != nil {
			return nil, false, ErrReferenceUnavailable
		}
		if claim != binding.ActivePVC.Name || !sameObject(binding.ActivePVC, refOf("PersistentVolumeClaim", pvc)) || pvc.DeletionTimestamp != nil || pvc.Status.Phase != corev1.ClaimBound || !pvcOwnedBy(scope, pvc, dv) {
			return nil, false, ErrConflict
		}
		matches := 0
		for index := range result.Spec.Template.Spec.Volumes {
			volume := &result.Spec.Template.Spec.Volumes[index]
			if volume.Name != binding.CanonicalName {
				continue
			}
			matches++
			if volume.DataVolume != nil && volume.DataVolume.Name == binding.ActiveDV.Name {
				hotpluggable := volume.DataVolume.Hotpluggable
				volume.DataVolume = nil
				volume.PersistentVolumeClaim = &kv.PersistentVolumeClaimVolumeSource{PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{ClaimName: binding.ActivePVC.Name}, Hotpluggable: hotpluggable}
				changed = true
			} else if volume.PersistentVolumeClaim == nil || volume.PersistentVolumeClaim.ClaimName != binding.ActivePVC.Name {
				return nil, false, ErrConflict
			}
		}
		if matches != 1 {
			return nil, false, ErrConflict
		}
		retained[binding.ActiveDV.Name] = true
	}
	templates := []kv.DataVolumeTemplateSpec{}
	for _, template := range result.Spec.DataVolumeTemplates {
		if retained[template.Name] {
			changed = true
			continue
		}
		templates = append(templates, template)
	}
	if changed {
		result.Spec.DataVolumeTemplates = templates
	}
	return result, changed, nil
}

func (m *ManagedNativeRestore) nativeCloneReady(ctx context.Context, pv *corev1.PersistentVolume, observed SnapshotObservation, backup SnapshotBackup) error {
	if pv.Spec.CSI.Driver != vmstorage.LonghornSnapshotDriver {
		return nil
	}
	snapshot := observed.Volumes[backup.VolumeSnapshot.Name]
	if snapshot == nil || snapshot.Status == nil || snapshot.Status.BoundVolumeSnapshotContentName == nil {
		return ErrReferenceUnavailable
	}
	content := observed.Contents[*snapshot.Status.BoundVolumeSnapshotContentName]
	if content == nil || content.Spec.Driver != pv.Spec.CSI.Driver || content.Status == nil || content.Status.SnapshotHandle == nil {
		return ErrConflict
	}
	handle := *content.Status.SnapshotHandle
	if !strings.HasPrefix(handle, "snap://") {
		return nil
	}
	volume, err := readLonghornVolume(ctx, m.Dynamic, pv.Spec.CSI.VolumeHandle)
	if err != nil {
		return err
	}
	source, _, _ := unstructured.NestedString(volume.Object, "spec", "dataSource")
	if source != handle {
		return ErrConflict
	}
	state, _, _ := unstructured.NestedString(volume.Object, "status", "cloneStatus", "state")
	switch state {
	case "completed":
		sourceVolume, snapshotName, valid := strings.Cut(strings.TrimPrefix(handle, "snap://"), "/")
		clonedSource, _, _ := unstructured.NestedString(volume.Object, "status", "cloneStatus", "sourceVolume")
		clonedSnapshot, _, _ := unstructured.NestedString(volume.Object, "status", "cloneStatus", "snapshot")
		if !valid || sourceVolume == "" || snapshotName == "" || clonedSource != sourceVolume || clonedSnapshot != snapshotName {
			return ErrConflict
		}
		return nil
	case "failed":
		return errNativeCloneFailed
	default:
		return ErrSnapshotPending
	}
}

func readLonghornVolume(ctx context.Context, client dynamic.Interface, handle string) (*unstructured.Unstructured, error) {
	if handle == "" {
		return nil, ErrConflict
	}
	volumes, err := client.Resource(nativeLonghornVolumeResource).List(ctx, metav1.ListOptions{FieldSelector: "metadata.name=" + handle})
	if err != nil {
		return nil, ErrReferenceUnavailable
	}
	var volume *unstructured.Unstructured
	for index := range volumes.Items {
		candidate := &volumes.Items[index]
		if candidate.GetName() != handle {
			continue
		}
		if volume != nil || !validUID(string(candidate.GetUID())) || candidate.GetDeletionTimestamp() != nil {
			return nil, ErrConflict
		}
		volume = candidate
	}
	if volume == nil {
		return nil, ErrReferenceUnavailable
	}
	return volume, nil
}
