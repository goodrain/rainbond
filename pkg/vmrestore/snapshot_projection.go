package vmrestore

import (
	"sort"
	"time"

	csi "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
	native "kubevirt.io/api/snapshot/v1beta1"
)

// SnapshotProof is a Region receipt's already-sealed native identity set.
// It is never accepted as an arbitrary UI-provided proof.
type SnapshotProof struct {
	Sealed             bool
	SnapshotUID        string
	ContentUID         string
	VolumeSnapshotUIDs map[string]string
}

func snapshotFailureDTO(intent SnapshotIntent, operationID, coreHash string) SnapshotDTO {
	dto := SnapshotDTO{SnapshotName: intent.Name, SourceVMUID: intent.SourceVM.UID, Description: intent.Description, OperationID: operationID, CoreHash: coreHash, Phase: "creating", Consistency: "offline", Disks: []SnapshotBackup{}, Coverage: SnapshotCoverage{MissingDiskKeys: []string{}, ExcludedReadOnly: []string{}}}
	for _, disk := range intent.Disks {
		if disk.ReadOnly {
			dto.Coverage.ExcludedReadOnly = append(dto.Coverage.ExcludedReadOnly, disk.DiskKey)
		} else {
			dto.Coverage.ExpectedWritableDisks++
			dto.Coverage.MissingDiskKeys = append(dto.Coverage.MissingDiskKeys, disk.DiskKey)
		}
	}
	sort.Strings(dto.Coverage.MissingDiskKeys)
	sort.Strings(dto.Coverage.ExcludedReadOnly)
	return dto
}

// ProjectSnapshot never serializes a native VM object or error text. All inputs
// are observed objects; its output is the safe Region DTO consumed by Console.
func ProjectSnapshot(scope Scope, intent SnapshotIntent, operationID, coreHash string, snapshot *native.VirtualMachineSnapshot, content *native.VirtualMachineSnapshotContent, volumes map[string]*csi.VolumeSnapshot, proof SnapshotProof) (SnapshotDTO, error) {
	dto := snapshotFailureDTO(intent, operationID, coreHash)
	if err := validateSnapshotIntent(scope, intent); err != nil {
		return dto, err
	}
	expectedHash, err := SnapshotCoreHash(scope, intent)
	if err != nil || expectedHash != coreHash {
		return dto, ErrConflict
	}
	if operationID != intent.OperationID {
		return dto, ErrConflict
	}
	dto.Namespace = intent.SourceVM.Namespace
	dto.SourceVMName = intent.SourceVM.Name
	if snapshot == nil {
		return dto, nil
	}
	if snapshot.Namespace != scope.Namespace || snapshot.Name != intent.Name || snapshot.Labels["service_id"] != scope.ServiceID || snapshot.Labels["tenant_id"] != scope.TenantID || snapshot.Labels[SnapshotOperationLabel] != operationID || snapshot.Annotations[SnapshotCoreAnnotation] != coreHash || snapshot.Annotations[SnapshotSourceUIDAnnotation] != intent.SourceVM.UID || snapshot.Spec.Source.Kind != "VirtualMachine" || snapshot.Spec.Source.Name != intent.SourceVM.Name || snapshot.Spec.Source.APIGroup == nil || *snapshot.Spec.Source.APIGroup != kubevirtv1.VirtualMachineGroupVersionKind.Group || !validUID(string(snapshot.UID)) {
		return dto, ErrConflict
	}
	if proof.SnapshotUID != "" && proof.SnapshotUID != string(snapshot.UID) {
		return dto, ErrConflict
	}
	dto.SnapshotUID = string(snapshot.UID)
	if snapshot.DeletionTimestamp != nil {
		dto.Phase = "deleting"
		return dto, nil
	}
	if snapshot.Status == nil {
		return dto, nil
	}
	if !proof.Sealed && snapshot.Status.SourceUID != nil && string(*snapshot.Status.SourceUID) != intent.SourceVM.UID {
		return dto, ErrConflict
	}
	if snapshot.Status.Phase == native.Failed || snapshot.Status.Error != nil {
		dto.Phase = "failed"
		dto.ErrorCode = "vm_snapshot_native_failed"
		return dto, nil
	}
	for _, indication := range snapshot.Status.Indications {
		if indication == native.VMSnapshotOnlineSnapshotIndication {
			return dto, ErrConflict
		}
	}
	if snapshot.Status.CreationTime != nil {
		dto.CreationTime = snapshot.Status.CreationTime.UTC().Format(time.RFC3339Nano)
	}
	if content == nil {
		return dto, nil
	}
	expectedContent := "vmsnapshot-content-" + string(snapshot.UID)
	if snapshot.Status.VirtualMachineSnapshotContentName == nil || *snapshot.Status.VirtualMachineSnapshotContentName != content.Name || content.Name != expectedContent || content.Namespace != scope.Namespace || !validUID(string(content.UID)) || content.Spec.VirtualMachineSnapshotName == nil || *content.Spec.VirtualMachineSnapshotName != snapshot.Name || content.Spec.Source.VirtualMachine == nil {
		return dto, ErrConflict
	}
	if proof.Sealed && (proof.SnapshotUID != string(snapshot.UID) || proof.ContentUID != string(content.UID)) {
		return dto, ErrConflict
	}
	source := content.Spec.Source.VirtualMachine
	if source.Namespace != scope.Namespace || source.Name != intent.SourceVM.Name || string(source.UID) != intent.SourceVM.UID || source.Labels["service_id"] != scope.ServiceID || source.Labels["tenant_id"] != scope.TenantID {
		return dto, ErrConflict
	}
	historical := &kubevirtv1.VirtualMachine{ObjectMeta: source.ObjectMeta, Spec: source.Spec}
	config, err := SnapshotSafeConfig(historical, intent.Disks)
	if err != nil {
		return dto, err
	}
	configHash, err := snapshotDigest(config)
	if err != nil || configHash != intent.ConfigHash {
		return dto, ErrConflict
	}
	dto.Config = &config
	dto.ContentName = content.Name
	dto.ContentUID = string(content.UID)
	known := map[string]SnapshotDisk{}
	for _, disk := range intent.Disks {
		known[disk.CanonicalName] = disk
	}
	backups := map[string]native.VolumeBackup{}
	for _, backup := range content.Spec.VolumeBackups {
		disk, ok := known[backup.VolumeName]
		if !ok {
			return dto, ErrConflict
		}
		if _, exists := backups[backup.VolumeName]; exists {
			return dto, ErrConflict
		}
		pvc := backup.PersistentVolumeClaim
		if pvc.Namespace != scope.Namespace || pvc.Name != disk.SourcePVC.Name || string(pvc.UID) != disk.SourcePVC.UID || pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != disk.StorageClass {
			return dto, ErrConflict
		}
		requested := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		if requested.Value() != disk.RequestedBytes {
			return dto, ErrConflict
		}
		backups[backup.VolumeName] = backup
	}
	included := map[string]bool{}
	if snapshot.Status.SnapshotVolumes != nil {
		for _, name := range snapshot.Status.SnapshotVolumes.IncludedVolumes {
			included[name] = true
		}
	}
	missing := []string{}
	for _, disk := range intent.Disks {
		if disk.ReadOnly {
			continue
		}
		backup, exists := backups[disk.CanonicalName]
		if !exists || backup.VolumeSnapshotName == nil || !included[disk.CanonicalName] {
			missing = append(missing, disk.DiskKey)
			continue
		}
		volume := volumes[*backup.VolumeSnapshotName]
		if volume == nil || volume.DeletionTimestamp != nil || volume.Status == nil || volume.Status.ReadyToUse == nil || !*volume.Status.ReadyToUse || volume.Status.Error != nil {
			missing = append(missing, disk.DiskKey)
			continue
		}
		owned := false
		for _, owner := range volume.OwnerReferences {
			if owner.APIVersion == native.SchemeGroupVersion.String() && owner.Kind == "VirtualMachineSnapshotContent" && owner.Name == content.Name && owner.UID == content.UID {
				owned = true
			}
		}
		if !owned || volume.Name != *backup.VolumeSnapshotName || volume.Namespace != scope.Namespace || volume.Spec.Source.PersistentVolumeClaimName == nil || *volume.Spec.Source.PersistentVolumeClaimName != disk.SourcePVC.Name || !validUID(string(volume.UID)) {
			return dto, ErrConflict
		}
		if proof.Sealed && proof.VolumeSnapshotUIDs[volume.Name] != string(volume.UID) {
			return dto, ErrConflict
		}
		dto.Disks = append(dto.Disks, SnapshotBackup{SnapshotDisk: disk, VolumeSnapshot: ObjectRef{Kind: "VolumeSnapshot", Namespace: scope.Namespace, Name: volume.Name, UID: string(volume.UID)}})
	}
	sort.Slice(dto.Disks, func(i, j int) bool { return dto.Disks[i].VolumeID < dto.Disks[j].VolumeID })
	sort.Strings(missing)
	dto.Coverage.MissingDiskKeys = missing
	dto.Coverage.IncludedDisks = len(dto.Disks)
	dto.Coverage.Complete = len(missing) == 0 && dto.Coverage.ExpectedWritableDisks > 0
	dto.ReadyToUse = dto.Coverage.Complete && snapshot.Status.ReadyToUse != nil && *snapshot.Status.ReadyToUse && (proof.Sealed || snapshot.Status.SourceUID != nil) && content.DeletionTimestamp == nil && content.Status != nil && content.Status.ReadyToUse != nil && *content.Status.ReadyToUse && content.Status.Error == nil
	if dto.ReadyToUse {
		dto.Phase = "ready"
	}
	return dto, nil
}
