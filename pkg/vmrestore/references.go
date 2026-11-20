package vmrestore

import (
	"context"
	"strconv"
	"time"

	dbmodel "github.com/goodrain/rainbond/db/model"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// VolumeReference separates logical metadata protection from current-PVC dependency.
// Domain subjects have no fabricated Kubernetes UID.
type VolumeReference struct {
	OperationID string     `json:"operation_id,omitempty"`
	RelationID  uint       `json:"relation_id,omitempty"`
	Object      ObjectRef  `json:"object"`
	Reason      string     `json:"reason"`
	Logical     bool       `json:"logical"`
	Physical    bool       `json:"physical"`
	PVC         *ObjectRef `json:"pvc,omitempty"`
}

// ReferenceSummary contains facts, not a mutation permit or backend capability claim.
type ReferenceSummary struct {
	VolumeID          uint              `json:"volume_id"`
	BindingGeneration uint64            `json:"binding_generation"`
	SnapshotRefs      []VolumeReference `json:"snapshot_refs"`
	RollbackRefs      []VolumeReference `json:"rollback_refs"`
	InflightRefs      []VolumeReference `json:"inflight_refs"`
	ExternalRefs      []VolumeReference `json:"external_refs"`
	OwnAttachments    []VolumeReference `json:"own_attachments"`
	CanDetach         bool              `json:"can_detach"`
	CanDelete         bool              `json:"can_delete"`
	CanExpand         bool              `json:"can_expand"`
	BlockingReasons   []string          `json:"blocking_reasons"`
}

var (
	virtualMachineResource         = schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachines"}
	virtualMachineInstanceResource = schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachineinstances"}
	vmSnapshotResource             = schema.GroupVersionResource{Group: "snapshot.kubevirt.io", Version: "v1beta1", Resource: "virtualmachinesnapshots"}
	vmSnapshotContentResource      = schema.GroupVersionResource{Group: "snapshot.kubevirt.io", Version: "v1beta1", Resource: "virtualmachinesnapshotcontents"}
	csiSnapshotResource            = schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshots"}
	csiSnapshotContentResource     = schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshotcontents"}
	dataSourceResource             = schema.GroupVersionResource{Group: "cdi.kubevirt.io", Version: "v1beta1", Resource: "datasources"}
)

func textAt(obj map[string]interface{}, path ...string) string {
	value, _, _ := unstructured.NestedString(obj, path...)
	return value
}
func sliceAt(obj map[string]interface{}, path ...string) []interface{} {
	value, _, _ := unstructured.NestedSlice(obj, path...)
	return value
}
func asMap(value interface{}) map[string]interface{} {
	result, _ := value.(map[string]interface{})
	return result
}
func reasonOnce(reasons []string, next string) []string {
	for _, reason := range reasons {
		if reason == next {
			return reasons
		}
	}
	return append(reasons, next)
}
func pushReference(refs []VolumeReference, next VolumeReference) []VolumeReference {
	for _, ref := range refs {
		if sameObject(ref.Object, next.Object) && ref.Reason == next.Reason && ref.Logical == next.Logical && ref.Physical == next.Physical {
			return refs
		}
	}
	return append(refs, next)
}

// Optional means the whole API group is absent, never an unreadable namespace
// or an unsupported version of a group that does exist.
func (r *Resolver) objects(ctx context.Context, gvr schema.GroupVersionResource, namespace string, optional bool) ([]unstructured.Unstructured, error) {
	result, err := r.dynamic.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		return result.Items, nil
	}
	if optional && apierrors.IsNotFound(err) {
		resources, discoveryErr := r.core.Discovery().ServerResourcesForGroupVersion(gvr.GroupVersion().String())
		if discoveryErr == nil {
			for _, resource := range resources.APIResources {
				if resource.Name == gvr.Resource {
					return nil, ErrReferenceUnavailable
				}
			}
			return nil, nil
		}
		if !apierrors.IsNotFound(discoveryErr) {
			return nil, ErrReferenceUnavailable
		}
		groups, groupErr := r.core.Discovery().ServerGroups()
		if groupErr != nil {
			return nil, ErrReferenceUnavailable
		}
		for _, group := range groups.Groups {
			if group.Name == gvr.Group {
				return nil, ErrReferenceUnavailable
			}
		}
		return nil, nil
	}

	return nil, ErrReferenceUnavailable
}
func vmVolumes(obj map[string]interface{}, instance bool) []interface{} {
	if instance {
		return sliceAt(obj, "spec", "volumes")
	}
	return sliceAt(obj, "spec", "template", "spec", "volumes")
}
func volumeUses(volume map[string]interface{}, resolved ResolvedVolume) bool {
	if resolved.ActivePVC != nil && textAt(volume, "persistentVolumeClaim", "claimName") == resolved.ActivePVC.Name {
		return true
	}
	return resolved.ActiveDV != nil && textAt(volume, "dataVolume", "name") == resolved.ActiveDV.Name
}
func (r *Resolver) activePVC(ctx context.Context, resolved ResolvedVolume) (*corev1.PersistentVolumeClaim, error) {
	if resolved.ActivePVC == nil {
		return nil, nil
	}
	pvc, err := r.core.CoreV1().PersistentVolumeClaims(resolved.Scope.Namespace).Get(ctx, resolved.ActivePVC.Name, metav1.GetOptions{})
	if err != nil {
		return nil, ErrReferenceUnavailable
	}
	if CheckLivePVC(*resolved.ActivePVC, pvc.Namespace, pvc.Name, string(pvc.UID)) != nil {
		return nil, ErrConflict
	}
	return pvc, nil
}
func (r *Resolver) activeHandle(ctx context.Context, resolved ResolvedVolume) (string, error) {
	pvc, err := r.activePVC(ctx, resolved)
	if err != nil || pvc == nil {
		return "", err
	}
	if pvc.Spec.VolumeName == "" {
		return "", nil
	}
	pv, err := r.core.CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
	if err != nil {
		return "", ErrReferenceUnavailable
	}
	if pv.Spec.ClaimRef == nil || pv.Spec.ClaimRef.Namespace != pvc.Namespace || pv.Spec.ClaimRef.Name != pvc.Name || pv.Spec.ClaimRef.UID != pvc.UID {
		return "", ErrConflict
	}
	if pv.Spec.CSI == nil {
		return "", nil
	}
	return pv.Spec.CSI.Driver + "\x00" + pv.Spec.CSI.VolumeHandle, nil
}
func physicalPVC(resolved ResolvedVolume, namespace, name, uid string) (*ObjectRef, bool) {
	if resolved.ActivePVC != nil && namespace == resolved.ActivePVC.Namespace && name == resolved.ActivePVC.Name && uid == resolved.ActivePVC.UID {
		return resolved.ActivePVC, true
	}
	for _, ref := range resolved.Retained {
		if ref.Kind == "PersistentVolumeClaim" && ref.Namespace == namespace && ref.Name == name && ref.UID == uid {
			copy := ref
			return &copy, false
		}
	}
	return nil, false
}

// VolumeReferences reports a point-in-time fact set. The actual mutator must
// hold admission and revalidate identity/use UID preconditions immediately before
// changing storage. This read does not grant a lease or bypass source admission.
func (r *Resolver) VolumeReferences(ctx context.Context, scope Scope, volumeID uint) (ReferenceSummary, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	summary := ReferenceSummary{VolumeID: volumeID, SnapshotRefs: []VolumeReference{}, RollbackRefs: []VolumeReference{}, InflightRefs: []VolumeReference{}, ExternalRefs: []VolumeReference{}, OwnAttachments: []VolumeReference{}, BlockingReasons: []string{}}
	resolved, err := r.ResolveVolume(ctx, scope, volumeID)
	if err != nil {
		return summary, err
	}
	summary.BindingGeneration = resolved.BindingGeneration
	current, err := r.store.Read(scope, "")
	if err != nil {
		return summary, err
	}
	if current.State.ControlRevision != resolved.ControlRevision || current.State.BindingGeneration != resolved.BindingGeneration {
		return summary, ErrConflict
	}
	return r.volumeReferences(ctx, scope, resolved, current, false, resolved.ActivePVC != nil)
}

// volumeReferences is the single reference matrix for ordinary and admitted
// reclaim reads. Expected identities are not claims that deleted objects exist.
func (r *Resolver) volumeReferences(ctx context.Context, scope Scope, resolved ResolvedVolume, current Current, reclaiming, pvcPresent bool) (ReferenceSummary, error) {
	summary := ReferenceSummary{VolumeID: resolved.VolumeID, BindingGeneration: resolved.BindingGeneration, SnapshotRefs: []VolumeReference{}, RollbackRefs: []VolumeReference{}, InflightRefs: []VolumeReference{}, ExternalRefs: []VolumeReference{}, OwnAttachments: []VolumeReference{}, BlockingReasons: []string{}}
	receipts, err := r.receipts(scope, current.State)
	if err != nil {
		return summary, err
	}
	key := strconv.FormatUint(uint64(resolved.VolumeID), 10)
	for _, receipt := range receipts {
		old, hasOld := receipt.OldBindings[key]
		next, hasNext := receipt.NewBindings[key]
		if receipt.RetainOldPVCs && hasOld {
			candidates := []struct {
				object *ObjectRef
				pvc    *ObjectRef
			}{{&old.ActivePVC, &old.ActivePVC}, {old.ActiveDV, &old.ActivePVC}, {old.PreviousPVC, old.PreviousPVC}, {old.PreviousDV, old.PreviousPVC}}
			for _, candidate := range candidates {
				if candidate.object == nil {
					continue
				}
				physical := candidate.pvc != nil && resolved.ActivePVC != nil && sameObject(*candidate.pvc, *resolved.ActivePVC)
				summary.RollbackRefs = pushReference(summary.RollbackRefs, VolumeReference{Object: *candidate.object, Reason: "retained_rollback", Logical: true, Physical: physical, PVC: candidate.pvc})
			}
		}

		if current.State.Fence != nil && current.State.Fence.OperationID == receipt.OperationID && (hasOld || hasNext) {
			target := ObjectRef{Kind: "RestoreOperation", Namespace: scope.Namespace, Name: receipt.OperationID, UID: ""}
			summary.InflightRefs = pushReference(summary.InflightRefs, VolumeReference{Object: target, OperationID: receipt.OperationID, Reason: "active_operation", Logical: true, Physical: hasNext && resolved.ActivePVC != nil && sameObject(next.ActivePVC, *resolved.ActivePVC)})
		}
	}
	// Current binding retains its immediate previous version independently of older receipts.
	if binding, ok := current.State.Bindings[key]; ok && binding.PreviousPVC != nil {
		summary.RollbackRefs = pushReference(summary.RollbackRefs, VolumeReference{Object: *binding.PreviousPVC, Reason: "retained_previous", Logical: true, Physical: resolved.ActivePVC != nil && sameObject(*binding.PreviousPVC, *resolved.ActivePVC), PVC: binding.PreviousPVC})
	}
	var mounts []dbmodel.TenantServiceMountRelation
	if err := r.store.db.Where("dep_service_id = ? AND volume_name = ?", scope.ServiceID, resolved.DiskKey).Find(&mounts).Error; err != nil {
		return summary, err
	}
	if len(mounts) > 0 {
		summary.ExternalRefs = append(summary.ExternalRefs, VolumeReference{Object: ObjectRef{Kind: "SharedMount", Namespace: scope.Namespace, Name: resolved.DiskKey, UID: ""}, Reason: "platform_shared_mount", Logical: true, Physical: true, PVC: resolved.ActivePVC})
	}

	vms, err := r.objects(ctx, virtualMachineResource, scope.Namespace, false)
	if err != nil {
		return summary, err
	}
	ownVMs := map[string]bool{}
	ownNames := map[string]bool{}
	for _, vm := range vms {
		if vm.GetLabels()["service_id"] == scope.ServiceID {
			if vm.GetNamespace() != scope.Namespace || !validUID(string(vm.GetUID())) {
				return summary, ErrConflict
			}
			ownVMs[string(vm.GetUID())] = true
			ownNames[vm.GetName()] = true
		}
	}
	if len(ownVMs) > 1 {
		return summary, ErrConflict
	}
	for _, vm := range vms {
		own := ownVMs[string(vm.GetUID())]
		for _, value := range vmVolumes(vm.Object, false) {
			volume := asMap(value)
			if own && volume["name"] == resolved.CanonicalName && !volumeUses(volume, resolved) {
				summary.BlockingReasons = reasonOnce(summary.BlockingReasons, "runtime_mapping_conflict")
			}
			if volumeUses(volume, resolved) {
				ref := VolumeReference{Object: refOf("VirtualMachine", &vm), Reason: "vm_attachment", Physical: true, PVC: resolved.ActivePVC}
				if own {
					summary.OwnAttachments = pushReference(summary.OwnAttachments, ref)
				} else {
					summary.ExternalRefs = pushReference(summary.ExternalRefs, ref)
				}
			}
		}
		for _, request := range sliceAt(vm.Object, "status", "volumeRequests") {
			add := asMap(asMap(request)["addVolumeOptions"])
			source := asMap(add["volumeSource"])
			if volumeUses(source, resolved) {
				summary.InflightRefs = pushReference(summary.InflightRefs, VolumeReference{Object: refOf("VirtualMachine", &vm), Reason: "pending_volume_request", Physical: true, PVC: resolved.ActivePVC})
			}
		}
	}
	instances, err := r.objects(ctx, virtualMachineInstanceResource, scope.Namespace, false)
	if err != nil {
		return summary, err
	}
	ownInstances := map[string]bool{}
	ownHotplugPods := map[string]string{}
	for _, instance := range instances {
		own := false
		for _, owner := range instance.GetOwnerReferences() {
			if owner.Kind == "VirtualMachine" && ownVMs[string(owner.UID)] && instance.GetLabels()["service_id"] == scope.ServiceID {
				own = true
				ownInstances[string(instance.GetUID())] = true
			}
		}
		for _, value := range vmVolumes(instance.Object, true) {
			if volumeUses(asMap(value), resolved) {
				ref := VolumeReference{Object: refOf("VirtualMachineInstance", &instance), Reason: "vmi_attachment", Physical: true, PVC: resolved.ActivePVC}
				if own {
					summary.OwnAttachments = pushReference(summary.OwnAttachments, ref)
				} else {
					summary.ExternalRefs = pushReference(summary.ExternalRefs, ref)
				}
			}
		}
		if own && resolved.ActivePVC != nil {
			for _, value := range sliceAt(instance.Object, "status", "volumeStatus") {
				status := asMap(value)
				name := textAt(status, "hotplugVolume", "attachPodName")
				uid := textAt(status, "hotplugVolume", "attachPodUID")
				if textAt(status, "name") == resolved.CanonicalName && textAt(status, "persistentVolumeClaimInfo", "claimName") == resolved.ActivePVC.Name && validName(name) && validUID(uid) {
					ownHotplugPods[uid] = name
				}
			}
		}
	}
	pods, err := r.core.CoreV1().Pods(scope.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return summary, ErrReferenceUnavailable
	}
	ownLaunchers := map[string]string{}
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		for _, owner := range pod.OwnerReferences {
			if owner.Kind == "VirtualMachineInstance" && ownInstances[string(owner.UID)] {
				ownLaunchers[string(pod.UID)] = pod.Name
			}
		}
	}
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		own := false
		for _, owner := range pod.OwnerReferences {
			if owner.Kind == "VirtualMachineInstance" && ownInstances[string(owner.UID)] {
				own = true
			}
			// Hotplug disk Pods are owned by virt-launcher, not directly by
			// the VMI. Require both the exact live owner and VMI observation.
			if owner.Kind == "Pod" && owner.APIVersion == "v1" && ownLaunchers[string(owner.UID)] == owner.Name &&
				pod.Labels["kubevirt.io"] == "hotplug-disk" && ownHotplugPods[string(pod.UID)] == pod.Name {
				own = true
			}
		}
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil && resolved.ActivePVC != nil && volume.PersistentVolumeClaim.ClaimName == resolved.ActivePVC.Name {
				ref := VolumeReference{Object: refOf("Pod", &pod), Reason: "pod_mount", Physical: true, PVC: resolved.ActivePVC}
				if own {
					summary.OwnAttachments = pushReference(summary.OwnAttachments, ref)
				} else {
					summary.ExternalRefs = pushReference(summary.ExternalRefs, ref)
				}
			}
		}
	}
	activeHandle := ""
	if pvcPresent {
		activeHandle, err = r.activeHandle(ctx, resolved)
		if err != nil {
			return summary, err
		}
	}
	if err := r.snapshotReferences(ctx, scope, resolved, ownNames, activeHandle, &summary); err != nil {
		return summary, err
	}
	if err := r.cloneReferences(ctx, scope, resolved, &summary); err != nil {
		return summary, err
	}

	logicalBlock := len(summary.SnapshotRefs) > 0 || len(summary.RollbackRefs) > 0 || len(summary.InflightRefs) > 0 || len(summary.ExternalRefs) > 0
	physicalBlock := false
	for _, refs := range [][]VolumeReference{summary.SnapshotRefs, summary.RollbackRefs, summary.InflightRefs, summary.ExternalRefs} {
		for _, ref := range refs {
			if ref.Physical {
				physicalBlock = true
			}
		}
	}
	for _, refs := range [][]VolumeReference{summary.SnapshotRefs, summary.RollbackRefs, summary.InflightRefs, summary.ExternalRefs} {
		for _, ref := range refs {
			summary.BlockingReasons = reasonOnce(summary.BlockingReasons, ref.Reason)
		}
	}
	busy := current.State.Fence != nil || (current.State.Admission != nil && !ownsAdmissionContext(ctx, scope, current.State.Admission.Token))
	if busy {
		summary.BlockingReasons = reasonOnce(summary.BlockingReasons, "component_operation_in_progress")
	}
	if !reclaiming && resolved.Provisioning != "bound" {
		summary.BlockingReasons = reasonOnce(summary.BlockingReasons, "volume_not_bound")
	}
	runtimeConflict := false
	for _, reason := range summary.BlockingReasons {
		if reason == "runtime_mapping_conflict" {
			runtimeConflict = true
		}
	}
	eligible := !busy && !runtimeConflict && (resolved.Provisioning == "bound" || reclaiming)
	summary.CanDetach = eligible && !reclaiming && resolved.Role == "data" && !logicalBlock
	summary.CanDelete = eligible && resolved.Role == "data" && !logicalBlock && len(summary.OwnAttachments) == 0
	// Historical B rollback/snapshot references must not block expansion of independent A.
	summary.CanExpand = eligible && !reclaiming && !physicalBlock && !resolved.ReadOnly
	if err := r.stable(scope, current.State); err != nil {
		return summary, err
	}
	return summary, nil
}
