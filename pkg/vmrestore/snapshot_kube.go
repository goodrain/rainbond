package vmrestore

import (
	"context"
	"strings"

	"github.com/goodrain/rainbond/pkg/vmstorage"
	csi "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kv "kubevirt.io/api/core/v1"
	native "kubevirt.io/api/snapshot/v1beta1"
	"kubevirt.io/client-go/kubecli"
)

// KubeSnapshotBackend is the concrete adapter; it never resolves PVC aliases itself.
type KubeSnapshotBackend struct {
	resolver *Resolver
	client   kubecli.KubevirtClient
}

// NewKubeSnapshotBackend reuses the frozen shared live resolver.
func NewKubeSnapshotBackend(resolver *Resolver, client kubecli.KubevirtClient) *KubeSnapshotBackend {
	return &KubeSnapshotBackend{resolver: resolver, client: client}
}
func (b *KubeSnapshotBackend) vm(ctx context.Context, scope Scope) (*kv.VirtualMachine, error) {
	list, err := b.client.VirtualMachine(scope.Namespace).List(ctx, metav1.ListOptions{LabelSelector: "service_id=" + scope.ServiceID})
	if err != nil {
		return nil, ErrReferenceUnavailable
	}
	if len(list.Items) != 1 {
		return nil, ErrConflict
	}
	vm := &list.Items[0]
	if vm.Namespace != scope.Namespace || !validUID(string(vm.UID)) || vm.Labels["service_id"] != scope.ServiceID || vm.Spec.Template == nil {
		return nil, ErrConflict
	}
	if tenant := vm.Labels["tenant_id"]; tenant != "" && tenant != scope.TenantID {
		return nil, ErrConflict
	}
	return vm, nil
}
func vmWantsRunning(vm *kv.VirtualMachine) bool {
	if vm.Spec.Running != nil {
		return *vm.Spec.Running
	}
	if vm.Spec.RunStrategy != nil {
		return *vm.Spec.RunStrategy != kv.RunStrategyHalted && *vm.Spec.RunStrategy != kv.RunStrategyManual
	}
	return false
}
func sourceUnlocked(vm *kv.VirtualMachine) bool {
	if vm.Status.SnapshotInProgress != nil {
		return false
	}
	for _, finalizer := range vm.Finalizers {
		if finalizer == "snapshot.kubevirt.io/snapshot-source-protection" {
			return false
		}
	}
	return true
}
func (b *KubeSnapshotBackend) power(ctx context.Context, scope Scope, intent SnapshotIntent) (bool, bool, bool, error) {
	vm, err := b.client.VirtualMachine(scope.Namespace).Get(ctx, intent.SourceVM.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, false, true, nil
	}
	if err != nil {
		return false, false, false, ErrReferenceUnavailable
	}
	if string(vm.UID) != intent.SourceVM.UID {
		return false, false, true, nil
	}
	instance, err := b.client.VirtualMachineInstance(scope.Namespace).Get(ctx, vm.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		instance = nil
	} else if err != nil {
		return false, false, false, ErrReferenceUnavailable
	}
	running := instance != nil && instance.Status.Phase == kv.Running
	stopped := instance == nil && !vmWantsRunning(vm)
	if stopped {
		names := map[string]bool{}
		for _, disk := range intent.Disks {
			names[disk.SourcePVC.Name] = true
		}
		pods, err := b.client.CoreV1().Pods(scope.Namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, false, false, ErrReferenceUnavailable
		}
		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
				continue
			}
			for _, volume := range pod.Spec.Volumes {
				if volume.PersistentVolumeClaim != nil && names[volume.PersistentVolumeClaim.ClaimName] {
					stopped = false
				}
			}
		}
	}
	return stopped, running, sourceUnlocked(vm), nil
}
func (b *KubeSnapshotBackend) checkStorage(ctx context.Context, scope Scope, disk SnapshotDisk) error {
	pvc, err := b.client.CoreV1().PersistentVolumeClaims(scope.Namespace).Get(ctx, disk.SourcePVC.Name, metav1.GetOptions{})
	if err != nil {
		return ErrReferenceUnavailable
	}
	if CheckLivePVC(disk.SourcePVC, pvc.Namespace, pvc.Name, string(pvc.UID)) != nil {
		return ErrConflict
	}
	if pvc.Status.Phase != corev1.ClaimBound || pvc.Spec.StorageClassName == nil || pvc.Spec.VolumeMode != nil && *pvc.Spec.VolumeMode != corev1.PersistentVolumeFilesystem {
		return ErrSnapshotStorage
	}
	storage, err := b.client.StorageV1().StorageClasses().Get(ctx, *pvc.Spec.StorageClassName, metav1.GetOptions{})
	if err != nil {
		return ErrReferenceUnavailable
	}
	pv, err := b.client.CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
	if err != nil {
		return ErrReferenceUnavailable
	}
	if pv.Spec.CSI == nil || pv.Spec.CSI.Driver != storage.Provisioner || pv.Spec.ClaimRef == nil || pv.Spec.ClaimRef.UID != pvc.UID {
		return ErrConflict
	}
	classes, err := b.client.KubernetesSnapshotClient().SnapshotV1().VolumeSnapshotClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return ErrReferenceUnavailable
	}
	classEvidence := make([]unstructured.Unstructured, 0, len(classes.Items))
	for _, class := range classes.Items {
		object, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&class)
		if err != nil {
			return ErrReferenceUnavailable
		}
		classEvidence = append(classEvidence, unstructured.Unstructured{Object: object})
	}
	selected := vmstorage.SelectSnapshotClass(ctx, b.client.DynamicClient(), storage, classEvidence)
	if selected.Status == "unknown" {
		return ErrReferenceUnavailable
	}
	if !vmstorage.ManagedSnapshotCapability(selected).Supported {
		return ErrSnapshotStorage
	}
	return nil
}

// Context performs preflight while the VM may still be Running.
func (b *KubeSnapshotBackend) Context(ctx context.Context, scope Scope) (SnapshotContext, error) {
	result := SnapshotContext{}
	vm, err := b.vm(ctx, scope)
	if err != nil {
		return result, err
	}
	view, err := b.resolver.ReadBindingView(ctx, scope)
	if err != nil {
		return result, err
	}
	for _, volume := range view.Volumes {
		if !volume.Attached {
			continue
		}
		if volume.ActivePVC == nil || volume.Provisioning != "bound" {
			return result, ErrSnapshotPending
		}
		matched := false
		for _, device := range vm.Spec.Template.Spec.Volumes {
			if device.Name != volume.CanonicalName {
				continue
			}
			matched = true
			if device.DataVolume != nil {
				if volume.ActiveDV == nil || device.DataVolume.Name != volume.ActiveDV.Name {
					return result, ErrConflict
				}
			} else if device.PersistentVolumeClaim == nil || device.PersistentVolumeClaim.ClaimName != volume.ActivePVC.Name {
				return result, ErrConflict
			}
		}
		if !matched {
			return result, ErrConflict
		}
		disk := SnapshotDisk{VolumeID: volume.VolumeID, DiskKey: volume.DiskKey, CanonicalName: volume.CanonicalName, Role: volume.Role, SourcePVC: *volume.ActivePVC, SourceDV: volume.ActiveDV, ReadOnly: volume.ReadOnly, StorageClass: volume.StorageClass, RequestedBytes: volume.RequestedBytes}
		if !disk.ReadOnly {
			if err := b.checkStorage(ctx, scope, disk); err != nil {
				return result, err
			}
		}
		result.Disks = append(result.Disks, disk)
	}
	result.Config, err = SnapshotSafeConfig(vm, result.Disks)
	if err != nil {
		return result, err
	}
	result.SourceVM = refOf("VirtualMachine", vm)
	instance, err := b.client.VirtualMachineInstance(scope.Namespace).Get(ctx, vm.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		instance = nil
	} else if err != nil {
		return result, ErrReferenceUnavailable
	}
	result.WasRunning = instance != nil || vmWantsRunning(vm)
	result.Stopped = instance == nil && !vmWantsRunning(vm)
	return result, nil
}

// Exists only checks whether the scoped native resource name is occupied.
func (b *KubeSnapshotBackend) Exists(ctx context.Context, scope Scope, name string) (bool, error) {
	_, err := b.client.VirtualMachineSnapshot(scope.Namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, ErrReferenceUnavailable
	}
	return true, nil
}
func expectedSnapshot(scope Scope, intent SnapshotIntent, operationID, coreHash string) *native.VirtualMachineSnapshot {
	group := kv.VirtualMachineGroupVersionKind.Group
	policy := native.VirtualMachineSnapshotContentDelete
	return &native.VirtualMachineSnapshot{ObjectMeta: metav1.ObjectMeta{Name: intent.Name, Namespace: scope.Namespace, Labels: map[string]string{"service_id": scope.ServiceID, "tenant_id": scope.TenantID, SnapshotOperationLabel: operationID}, Annotations: map[string]string{SnapshotCoreAnnotation: coreHash, SnapshotSourceUIDAnnotation: intent.SourceVM.UID, "description": intent.Description}}, Spec: native.VirtualMachineSnapshotSpec{Source: corev1.TypedLocalObjectReference{APIGroup: &group, Kind: "VirtualMachine", Name: intent.SourceVM.Name}, DeletionPolicy: &policy}}
}
func ownedSnapshot(scope Scope, intent SnapshotIntent, snapshot *native.VirtualMachineSnapshot) bool {
	core, err := SnapshotCoreHash(scope, intent)
	return err == nil && snapshot != nil && validUID(string(snapshot.UID)) && snapshot.Namespace == scope.Namespace && snapshot.Name == intent.Name && snapshot.Labels["service_id"] == scope.ServiceID && snapshot.Labels["tenant_id"] == scope.TenantID && snapshot.Annotations[SnapshotCoreAnnotation] == core && snapshot.Annotations[SnapshotSourceUIDAnnotation] == intent.SourceVM.UID && snapshot.Labels[SnapshotOperationLabel] == intent.OperationID && snapshot.Spec.Source.APIGroup != nil && *snapshot.Spec.Source.APIGroup == kv.VirtualMachineGroupVersionKind.Group && snapshot.Spec.Source.Kind == "VirtualMachine" && snapshot.Spec.Source.Name == intent.SourceVM.Name
}

// Create is idempotent only for the exact same server-planned operation/core.
func (b *KubeSnapshotBackend) Create(ctx context.Context, scope Scope, intent SnapshotIntent, operationID, coreHash string) (ObjectRef, error) {
	snapshot, err := b.client.VirtualMachineSnapshot(scope.Namespace).Create(ctx, expectedSnapshot(scope, intent, operationID, coreHash), metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		snapshot, err = b.client.VirtualMachineSnapshot(scope.Namespace).Get(ctx, intent.Name, metav1.GetOptions{})
	}
	if err != nil {
		return ObjectRef{}, ErrReferenceUnavailable
	}
	if !ownedSnapshot(scope, intent, snapshot) || !validUID(string(snapshot.UID)) {
		return ObjectRef{}, ErrConflict
	}
	return refOf("VirtualMachineSnapshot", snapshot), nil
}

// Halt requests the normal graceful platform stop for the exact source VM.
func (b *KubeSnapshotBackend) Halt(ctx context.Context, scope Scope, intent SnapshotIntent) error {
	vm, err := b.vm(ctx, scope)
	if err != nil {
		return err
	}
	if string(vm.UID) != intent.SourceVM.UID {
		return ErrConflict
	}
	stopped, _, _, err := b.power(ctx, scope, intent)
	if err != nil {
		return err
	}
	if stopped {
		return nil
	}
	if err := b.client.VirtualMachine(scope.Namespace).Stop(ctx, vm.Name, &kv.StopOptions{}); err != nil {
		return ErrReferenceUnavailable
	}
	return nil
}

// Resume starts only the same source VM after native locks were released.
func (b *KubeSnapshotBackend) Resume(ctx context.Context, scope Scope, intent SnapshotIntent) error {
	vm, err := b.vm(ctx, scope)
	if err != nil {
		return err
	}
	if string(vm.UID) != intent.SourceVM.UID || !sourceUnlocked(vm) {
		return ErrConflict
	}
	_, running, _, err := b.power(ctx, scope, intent)
	if err != nil {
		return err
	}
	if running {
		return nil
	}
	if err := b.client.VirtualMachine(scope.Namespace).Start(ctx, vm.Name, &kv.StartOptions{}); err != nil {
		return ErrReferenceUnavailable
	}
	return nil
}

// Observe reads native parents and all CSI descendants, including retained CSI
// contents, so parent disappearance alone cannot be mistaken for complete cleanup.
func (b *KubeSnapshotBackend) Observe(ctx context.Context, scope Scope, intent SnapshotIntent, known []ResourceRef) (SnapshotObservation, error) {
	observed := SnapshotObservation{Volumes: map[string]*csi.VolumeSnapshot{}, Contents: map[string]*csi.VolumeSnapshotContent{}, Resources: []ResourceRef{}}
	var err error
	observed.SourceStopped, observed.SourceRunning, observed.SourceLockClear, err = b.power(ctx, scope, intent)
	if err != nil {
		return observed, err
	}
	snapshot, err := b.client.VirtualMachineSnapshot(scope.Namespace).Get(ctx, intent.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		snapshot = nil
	} else if err != nil {
		return observed, ErrReferenceUnavailable
	}
	nativeUID := ""
	if snapshot != nil {
		if !ownedSnapshot(scope, intent, snapshot) {
			return observed, ErrConflict
		}
		observed.Snapshot = snapshot
		nativeUID = string(snapshot.UID)
		observed.Resources = resourceOnce(observed.Resources, ResourceRef{ObjectRef: refOf("VirtualMachineSnapshot", snapshot), Purpose: "snapshot"})
	}
	for _, ref := range known {
		if ref.Kind == "VirtualMachineSnapshot" {
			if nativeUID != "" && ref.UID != nativeUID {
				return observed, ErrConflict
			}
			nativeUID = ref.UID
		}
	}
	if nativeUID == "" {
		observed.CleanupComplete = snapshot == nil
		return observed, nil
	}
	contentName := "vmsnapshot-content-" + nativeUID
	expectedContentUID := ""
	for _, ref := range known {
		if ref.Kind == "VirtualMachineSnapshotContent" && ref.Name == contentName {
			expectedContentUID = ref.UID
		}
	}
	content, err := b.client.VirtualMachineSnapshotContent(scope.Namespace).Get(ctx, contentName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		content = nil
	} else if err != nil {
		return observed, ErrReferenceUnavailable
	}
	if content != nil {
		if content.Namespace != scope.Namespace || content.Spec.VirtualMachineSnapshotName == nil || *content.Spec.VirtualMachineSnapshotName != intent.Name || content.Spec.Source.VirtualMachine == nil || string(content.Spec.Source.VirtualMachine.UID) != intent.SourceVM.UID {
			return observed, ErrConflict
		}
		for _, ref := range known {
			if ref.Kind == "VirtualMachineSnapshotContent" && ref.Name == content.Name && ref.UID != string(content.UID) {
				return observed, ErrConflict
			}
		}
		expectedContentUID = string(content.UID)
		observed.Content = content
		observed.Resources = resourceOnce(observed.Resources, ResourceRef{ObjectRef: refOf("VirtualMachineSnapshotContent", content), Purpose: "snapshot-content"})
	}
	children, err := b.client.KubernetesSnapshotClient().SnapshotV1().VolumeSnapshots(scope.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return observed, ErrReferenceUnavailable
	}
	names := map[string]bool{}
	volumeUIDs := map[string]string{}
	for _, disk := range intent.Disks {
		names["vmsnapshot-"+nativeUID+"-volume-"+disk.CanonicalName] = true
	}
	for _, ref := range known {
		if ref.Kind == "VolumeSnapshot" {
			names[ref.Name] = true
			volumeUIDs[ref.Name] = ref.UID
		}
	}
	for _, child := range children.Items {
		owned := false
		for _, owner := range child.OwnerReferences {
			if owner.APIVersion == native.SchemeGroupVersion.String() && owner.Kind == "VirtualMachineSnapshotContent" && owner.Name == contentName && (expectedContentUID != "" && string(owner.UID) == expectedContentUID) {
				owned = true
			}
		}
		if !owned {
			if names[child.Name] {
				return observed, ErrConflict
			}
			continue
		}
		copy := child
		observed.Volumes[child.Name] = &copy
		names[child.Name] = true
		volumeUIDs[child.Name] = string(child.UID)
		for _, ref := range known {
			if ref.Kind == "VolumeSnapshot" && ref.Name == child.Name && ref.UID != string(child.UID) {
				return observed, ErrConflict
			}
		}
		observed.Resources = resourceOnce(observed.Resources, ResourceRef{ObjectRef: refOf("VolumeSnapshot", &child), Purpose: "volume-snapshot"})
	}
	contents, err := b.client.KubernetesSnapshotClient().SnapshotV1().VolumeSnapshotContents().List(ctx, metav1.ListOptions{})
	if err != nil {
		return observed, ErrReferenceUnavailable
	}
	for _, child := range contents.Items {
		if child.Spec.VolumeSnapshotRef.Namespace != scope.Namespace || !names[child.Spec.VolumeSnapshotRef.Name] {
			continue
		}
		if uid := volumeUIDs[child.Spec.VolumeSnapshotRef.Name]; uid != "" && uid != string(child.Spec.VolumeSnapshotRef.UID) {
			return observed, ErrConflict
		}
		for _, ref := range known {
			if ref.Kind == "VolumeSnapshotContent" && ref.Name == child.Name && ref.UID != string(child.UID) {
				return observed, ErrConflict
			}
		}
		observed.Resources = resourceOnce(observed.Resources, ResourceRef{ObjectRef: refOf("VolumeSnapshotContent", &child), Purpose: "csi-snapshot-content"})
		copy := child
		observed.Contents[child.Name] = &copy
	}
	observed.CleanupComplete = observed.Snapshot == nil && observed.Content == nil && len(observed.Resources) == 0
	return observed, nil
}

// Delete only deletes snapshot artifacts. Source VM/PVC/DV entries are ignored.
func (b *KubeSnapshotBackend) Delete(ctx context.Context, scope Scope, refs []ResourceRef) error {
	for _, ref := range refs {
		if ref.Kind == "VirtualMachineSnapshot" || ref.Kind == "VirtualMachineSnapshotContent" || ref.Kind == "VolumeSnapshot" {
			if ref.Namespace != scope.Namespace || !validUID(ref.UID) || !validName(ref.Name) {
				return ErrConflict
			}
		}
		uid := types.UID(ref.UID)
		options := metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}}
		var err error
		switch ref.Kind {
		case "VirtualMachineSnapshot":
			err = b.client.VirtualMachineSnapshot(scope.Namespace).Delete(ctx, ref.Name, options)
		case "VirtualMachineSnapshotContent":
			err = b.client.VirtualMachineSnapshotContent(scope.Namespace).Delete(ctx, ref.Name, options)
		case "VolumeSnapshot":
			err = b.client.KubernetesSnapshotClient().SnapshotV1().VolumeSnapshots(scope.Namespace).Delete(ctx, ref.Name, options)
		default:
			continue // CSI contents follow deletionPolicy; never force a finalizer or retained backend.
		}
		if err != nil && !apierrors.IsNotFound(err) {
			return ErrReferenceUnavailable
		}
	}
	return nil
}

// DeleteReferences rejects real readonly/inflight references; provenance alone is ignored.
func (b *KubeSnapshotBackend) DeleteReferences(ctx context.Context, scope Scope, resources []ResourceRef) error {
	targets := map[string]bool{}
	for _, ref := range resources {
		if ref.Kind == "VolumeSnapshot" {
			targets[ref.Name] = true
		}
	}
	claims, err := b.client.CoreV1().PersistentVolumeClaims(scope.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return ErrReferenceUnavailable
	}
	for _, claim := range claims.Items {
		referenced := false
		if source := claim.Spec.DataSource; source != nil && source.Kind == "VolumeSnapshot" && targets[source.Name] && (source.APIGroup == nil || *source.APIGroup == csi.GroupName) {
			referenced = true
		}
		if source := claim.Spec.DataSourceRef; source != nil && source.Kind == "VolumeSnapshot" && targets[source.Name] && (source.Namespace == nil || *source.Namespace == scope.Namespace) && (source.APIGroup == nil || *source.APIGroup == csi.GroupName) {
			referenced = true
		}
		if referenced {
			independent, err := b.independentClone(ctx, claim)
			if err != nil {
				return err
			}
			if !independent {
				return ErrBusy
			}
		}
	}
	return nil
}

func writableSnapshotClone(modes []corev1.PersistentVolumeAccessMode) bool {
	writable := false
	for _, mode := range modes {
		if mode == corev1.ReadOnlyMany {
			return false
		}
		if mode == corev1.ReadWriteOnce || mode == corev1.ReadWriteMany || mode == corev1.ReadWriteOncePod {
			writable = true
		}
	}
	return writable
}

// Only a completed ordinary writable CephFS allocation releases snapshot provenance.
func (b *KubeSnapshotBackend) independentClone(ctx context.Context, claim corev1.PersistentVolumeClaim) (bool, error) {
	if claim.DeletionTimestamp != nil || claim.Status.Phase != corev1.ClaimBound || !validUID(string(claim.UID)) || claim.Spec.VolumeName == "" || claim.Spec.StorageClassName == nil || !writableSnapshotClone(claim.Spec.AccessModes) || claim.Spec.VolumeMode != nil && *claim.Spec.VolumeMode != corev1.PersistentVolumeFilesystem {
		return false, nil
	}
	pv, err := b.client.CoreV1().PersistentVolumes().Get(ctx, claim.Spec.VolumeName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, ErrReferenceUnavailable
	}
	if pv.DeletionTimestamp != nil || !validUID(string(pv.UID)) || pv.Status.Phase != corev1.VolumeBound || pv.Spec.ClaimRef == nil || pv.Spec.ClaimRef.Namespace != claim.Namespace || pv.Spec.ClaimRef.Name != claim.Name || pv.Spec.ClaimRef.UID != claim.UID || pv.Spec.StorageClassName != *claim.Spec.StorageClassName || !writableSnapshotClone(pv.Spec.AccessModes) || pv.Spec.CSI == nil || pv.Spec.CSI.ReadOnly || pv.Spec.CSI.VolumeHandle == "" || pv.Spec.CSI.Driver != "cephfs.csi.ceph.com" || pv.Spec.VolumeMode != nil && *pv.Spec.VolumeMode != corev1.PersistentVolumeFilesystem {
		return false, nil
	}
	for _, option := range pv.Spec.MountOptions {
		if option == "ro" {
			return false, nil
		}
	}
	storage, err := b.client.StorageV1().StorageClasses().Get(ctx, *claim.Spec.StorageClassName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, ErrReferenceUnavailable
	}
	for _, value := range []string{storage.Parameters["backingSnapshot"], pv.Spec.CSI.VolumeAttributes["backingSnapshot"]} {
		if value != "" && !strings.EqualFold(value, "false") {
			return false, nil
		}
	}
	return storage.Provisioner == pv.Spec.CSI.Driver, nil
}
