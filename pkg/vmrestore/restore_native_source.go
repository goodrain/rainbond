package vmrestore

import (
	"context"

	dbmodel "github.com/goodrain/rainbond/db/model"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	kv "kubevirt.io/api/core/v1"
	native "kubevirt.io/api/snapshot/v1beta1"
)

// NativeRestorePreparation consumes existing managed native snapshot proofs.
// Certification is supplied by the storage owner; this does not infer support
// from a storage name or implement another storage capability checker.
type NativeRestorePreparation struct {
	Store           *Store
	Resolver        *Resolver
	Snapshots       *SnapshotEngine
	Dynamic         dynamic.Interface
	Certification   CertifiedRestoreStorage
	CurrentStartup  func(context.Context, Scope, *kv.VirtualMachine, string) (RestoreStartupPolicy, error)
	DeadlineSeconds int64
	MaxDisks        int
}

// NativeRestoreConfig obtains access mode and quota from the exact native
// backup PVC metadata. Missing or ambiguous modes are never filled from B.
func NativeRestoreConfig(dto SnapshotDTO, content *native.VirtualMachineSnapshotContent) (RestoreConfig, error) {
	if !dto.ReadyToUse || !dto.Coverage.Complete || dto.Config == nil || content == nil || string(content.UID) != dto.ContentUID {
		return RestoreConfig{}, ErrRestoreCapability
	}
	config := RestoreConfig{Hardware: *dto.Config, Volumes: []RestoreVolumeConfig{}}
	for _, layout := range config.Hardware.DiskLayout {
		var found *corev1.PersistentVolumeClaim
		for _, backup := range content.Spec.VolumeBackups {
			if backup.VolumeName != layout.CanonicalName {
				continue
			}
			if found != nil {
				return RestoreConfig{}, ErrConflict
			}
			pvc := backup.PersistentVolumeClaim
			found = &corev1.PersistentVolumeClaim{ObjectMeta: pvc.ObjectMeta, Spec: pvc.Spec}
		}
		if found == nil || len(found.Spec.AccessModes) != 1 || found.Spec.StorageClassName == nil || !validName(*found.Spec.StorageClassName) {
			return RestoreConfig{}, ErrRestoreCapability
		}
		mode := string(found.Spec.AccessModes[0])
		if mode != "ReadWriteOnce" && mode != "ReadWriteMany" && mode != "ReadOnlyMany" {
			return RestoreConfig{}, ErrRestoreCapability
		}
		quota := found.Spec.Resources.Requests[corev1.ResourceStorage]
		bytes := quota.Value()
		if bytes < 1<<30 || bytes%(1<<30) != 0 {
			return RestoreConfig{}, ErrRestoreCapability
		}
		matched := false
		for _, backup := range dto.Disks {
			if backup.VolumeID == layout.VolumeID && backup.CanonicalName == layout.CanonicalName && backup.SourcePVC.Name == found.Name && backup.SourcePVC.UID == string(found.UID) {
				matched = true
			}
		}
		if !matched {
			return RestoreConfig{}, ErrConflict
		}
		config.Volumes = append(config.Volumes, RestoreVolumeConfig{VolumeID: layout.VolumeID, DiskKey: layout.DiskKey, CapacityGiB: bytes >> 30, StorageClass: *found.Spec.StorageClassName, ReadOnly: layout.ReadOnly, AccessMode: mode})
	}
	return config, nil
}

// Read freshly verifies native A and obtains current B only for a new prepare.
// An existing operation keeps its sealed original B and power intent on replay.
func (p *NativeRestorePreparation) Read(ctx context.Context, scope Scope, op, name, power string) (RestorePreparation, error) {
	if p == nil || p.Store == nil || p.Resolver == nil || p.Snapshots == nil || p.Dynamic == nil || p.CurrentStartup == nil {
		return RestorePreparation{}, ErrRestoreCapability
	}
	if !p.Certification.Capability.ROXVerified || !p.Certification.Capability.SparsePersistenceVerified || !p.Certification.DeleteReclaimVerified {
		return RestorePreparation{}, ErrRestoreCapability
	}
	dto, err := p.Snapshots.ByName(ctx, scope, name)
	if err != nil {
		return RestorePreparation{}, err
	}
	if !dto.ReadyToUse || !dto.ManifestACKed || !dto.Finalized || dto.Cancelled || dto.Deleted {
		return RestorePreparation{}, ErrRestoreCapability
	}
	source, err := p.Store.Read(scope, dto.OperationID)
	if err != nil {
		return RestorePreparation{}, err
	}
	if source.Receipt == nil || source.Receipt.SnapshotIdentity == nil || !source.Receipt.SnapshotIdentity.Pinned || source.Receipt.SnapshotIdentity.Startup == nil {
		return RestorePreparation{}, ErrRestoreCapability
	}
	observed, err := p.Snapshots.backend.Observe(ctx, scope, *source.Receipt.Snapshot, source.Receipt.Resources)
	if err != nil {
		return RestorePreparation{}, err
	}
	fresh, err := p.Snapshots.project(source, observed)
	if err != nil {
		return RestorePreparation{}, err
	}
	if !fresh.ReadyToUse || fresh.NativeProofHash != dto.NativeProofHash || fresh.ContentUID != dto.ContentUID || fresh.SnapshotUID != dto.SnapshotUID {
		return RestorePreparation{}, ErrConflict
	}
	config, err := NativeRestoreConfig(fresh.SnapshotDTO, observed.Content)
	if err != nil {
		return RestorePreparation{}, err
	}
	for _, volume := range config.Volumes {
		if volume.StorageClass != p.Certification.TargetClassName {
			return RestorePreparation{}, ErrRestoreCapability
		}
	}
	configHash, _ := RestoreConfigHash(config)
	startup := *source.Receipt.SnapshotIdentity.Startup
	startup.BootstrapMode = "restored"
	startup.GeneratedMedia = []RestoreGeneratedMedia{}
	identityHash, _ := snapshotDigest(startup)
	current, err := p.Store.Read(scope, op)
	if err != nil {
		return RestorePreparation{}, err
	}
	var sealed *RestoreIntent
	if current.Receipt != nil {
		if current.Receipt.Restore == nil {
			return RestorePreparation{}, ErrRestoreCapability
		}
		r := current.Receipt
		if r.Restore.SourceSnapshot.UID != dto.SnapshotUID || r.Restore.SourceContent.UID != dto.ContentUID || r.Restore.SourceCoreHash != dto.CoreHash || r.Restore.SourceDocumentHash != source.Receipt.ConsoleAckHash || r.Restore.ConfigAfterHash != configHash || r.Restore.PowerAfter != power {
			return RestorePreparation{}, ErrConflict
		}
		if r.Execution != nil {
			return RestorePreparation{Intent: *r.Restore, Config: config, Frame: r.Execution.Frame, NetworkHash: r.Execution.NetworkHash, IdentityHash: identityHash, BeforeStartup: r.Execution.BeforeStartup, SourceStartup: startup}, p.Certification.validate(r.Execution.Frame)
		}
		sealed = r.Restore
	}
	view, err := p.Resolver.ReadBindingView(ctx, scope)
	if err != nil {
		return RestorePreparation{}, err
	}
	ownedPrepare := sealed != nil && view.Fence != nil && view.Fence.OperationID == op && view.Fence.Kind == "restore" && view.Fence.Phase == "prepared"
	if (view.Fence != nil && !ownedPrepare) || view.MutationInProgress {
		return RestorePreparation{}, ErrBusy
	}
	list, err := p.Dynamic.Resource(restoreVMGVR).Namespace(scope.Namespace).List(ctx, metav1.ListOptions{LabelSelector: "service_id=" + scope.ServiceID})
	if err != nil {
		return RestorePreparation{}, ErrReferenceUnavailable
	}
	if len(list.Items) != 1 {
		return RestorePreparation{}, ErrConflict
	}
	var vm kv.VirtualMachine
	if runtime.DefaultUnstructuredConverter.FromUnstructured(list.Items[0].Object, &vm) != nil || vm.Spec.Template == nil || !validUID(string(vm.UID)) {
		return RestorePreparation{}, ErrCorrupt
	}
	if vm.Namespace != scope.Namespace || vm.Labels["service_id"] != scope.ServiceID {
		return RestorePreparation{}, ErrConflict
	}
	if tenant := vm.Labels["tenant_id"]; tenant != "" && tenant != scope.TenantID {
		return RestorePreparation{}, ErrConflict
	}
	firmware := ""
	if vm.Spec.Template.Spec.Domain.Firmware != nil {
		firmware = string(vm.Spec.Template.Spec.Domain.Firmware.UUID)
	}
	beforeStartup, err := p.CurrentStartup(ctx, scope, &vm, firmware)
	if err != nil {
		return RestorePreparation{}, err
	}
	disks := []SnapshotDisk{}
	for _, volume := range view.Volumes {
		if volume.Attached {
			if volume.ActivePVC == nil {
				return RestorePreparation{}, ErrConflict
			}
			disks = append(disks, SnapshotDisk{VolumeID: volume.VolumeID, DiskKey: volume.DiskKey, CanonicalName: volume.CanonicalName, Role: volume.Role, SourcePVC: *volume.ActivePVC, SourceDV: volume.ActiveDV, ReadOnly: volume.ReadOnly})
		}
	}
	hardware, err := SnapshotSafeConfig(&vm, disks)
	if err != nil {
		return RestorePreparation{}, err
	}
	before := RestoreConfig{Hardware: hardware, Volumes: []RestoreVolumeConfig{}}
	wasRunning := vmWantsRunning(&vm)
	instance, instanceErr := p.Dynamic.Resource(restoreVMIGVR).Namespace(scope.Namespace).Get(ctx, vm.Name, metav1.GetOptions{})
	if instanceErr == nil {
		owned := false
		for _, owner := range instance.GetOwnerReferences() {
			owned = owned || (owner.Kind == "VirtualMachine" && owner.UID == vm.UID)
		}
		if !owned || !validUID(string(instance.GetUID())) {
			return RestorePreparation{}, ErrConflict
		}
		wasRunning = true
	} else if !apierrors.IsNotFound(instanceErr) {
		return RestorePreparation{}, ErrReferenceUnavailable
	}
	var rows []dbmodel.TenantServiceVolume
	if err := p.Store.db.Where("service_id = ?", scope.ServiceID).Order("ID").Find(&rows).Error; err != nil {
		return RestorePreparation{}, err
	}
	for _, row := range rows {
		if pvcDisk(row) {
			before.Volumes = append(before.Volumes, RestoreVolumeConfig{VolumeID: row.ID, DiskKey: row.VolumeName, CapacityGiB: row.VolumeCapacity, StorageClass: row.VolumeType, ReadOnly: row.IsReadOnly, AccessMode: row.AccessMode})
		}
	}
	intent := RestoreIntent{CurrentVM: refOf("VirtualMachine", &vm), SourceSnapshot: ObjectRef{Kind: "VirtualMachineSnapshot", Namespace: scope.Namespace, Name: name, UID: dto.SnapshotUID}, SourceContent: ObjectRef{Kind: "VirtualMachineSnapshotContent", Namespace: scope.Namespace, Name: dto.ContentName, UID: dto.ContentUID}, SourceVMUID: dto.SourceVMUID, SourceCoreHash: dto.CoreHash, SourceDocumentHash: source.Receipt.ConsoleAckHash, SourceGeneration: view.BindingGeneration, WasRunning: wasRunning, PowerAfter: power, Before: before, ConfigAfterHash: configHash}
	if sealed != nil {
		intent = *sealed
	}
	coreHash, _ := snapshotDigest(struct {
		Scope     Scope
		Operation string
		Intent    RestoreIntent
	}{scope, op, intent})
	frame := RestoreResourcePlan{Scope: scope, OperationID: op, CoreHash: coreHash, SourceGeneration: view.BindingGeneration, WorkerImage: p.Certification.Capability.WorkerImage, RuntimeImageID: p.Certification.RuntimeImageID, DeadlineSeconds: p.DeadlineSeconds, MaxDisks: p.MaxDisks, TargetUID: p.Certification.Capability.TargetUID, TargetGID: p.Certification.Capability.TargetGID, Disks: []RestoreDiskResource{}}
	for _, backup := range fresh.Disks {
		readOnly := false
		for _, layout := range config.Hardware.DiskLayout {
			if layout.VolumeID == backup.VolumeID {
				readOnly = layout.ReadOnly
			}
		}
		if readOnly {
			continue
		}
		var previous *ResolvedVolume
		for i := range view.Volumes {
			if view.Volumes[i].VolumeID == backup.VolumeID {
				previous = &view.Volumes[i]
			}
		}
		if previous == nil || previous.ActivePVC == nil {
			return RestorePreparation{}, ErrConflict
		}
		volume := observed.Volumes[backup.VolumeSnapshot.Name]
		if volume == nil || volume.Status == nil || volume.Status.BoundVolumeSnapshotContentName == nil {
			return RestorePreparation{}, ErrConflict
		}
		content := observed.Contents[*volume.Status.BoundVolumeSnapshotContentName]
		if content == nil {
			return RestorePreparation{}, ErrConflict
		}
		mode := ""
		for _, v := range config.Volumes {
			if v.VolumeID == backup.VolumeID {
				mode = v.AccessMode
			}
		}
		if mode == "" {
			return RestorePreparation{}, ErrConflict
		}
		frame.Disks = append(frame.Disks, RestoreDiskResource{VolumeID: backup.VolumeID, SnapshotContent: refOf("VolumeSnapshotContent", content), Snapshot: backup.VolumeSnapshot, PreviousPVC: *previous.ActivePVC, PreviousMaxBytes: previous.RequestedBytes, ROXClass: p.Certification.ROXClassName, ROXClassUID: p.Certification.ROXClassUID, TargetClass: p.Certification.TargetClassName, TargetClassUID: p.Certification.TargetClassUID, TargetAccessMode: mode, ROXRequestBytes: backup.RequestedBytes, TargetRequestBytes: backup.RequestedBytes})
	}
	frame.FrameHash, _ = RestoreResourceFrameHash(frame)
	network, err := p.Store.NetworkIdentityHash(scope)
	if err != nil {
		return RestorePreparation{}, err
	}
	result := RestorePreparation{Intent: intent, Config: config, Frame: frame, NetworkHash: network, IdentityHash: identityHash, BeforeStartup: beforeStartup, SourceStartup: startup}
	if err := result.validate(scope, op, name, power); err != nil {
		return RestorePreparation{}, err
	}
	if err := p.Certification.validate(frame); err != nil {
		return RestorePreparation{}, err
	}
	return result, nil
}
