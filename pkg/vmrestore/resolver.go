package vmrestore

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	dbmodel "github.com/goodrain/rainbond/db/model"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

var (
	// ErrReferenceUnavailable means an actual identity or reference dependency could not be read.
	ErrReferenceUnavailable = errors.New("VM disk references could not be verified")
	// ErrNotPVCDisk excludes non-PVC volumes from disk identity operations.
	ErrNotPVCDisk      = errors.New("VM volume is not a PVC disk")
	dataVolumeResource = schema.GroupVersionResource{Group: "cdi.kubevirt.io", Version: "v1beta1", Resource: "datavolumes"}
)

// ResolvedVolume is the unique Region-authoritative disk identity projection.
// An absent PVC is nil, never a fabricated UID or a different same-name object.
type ResolvedVolume struct {
	RestoreID          string      `json:"restore_id,omitempty"`
	SnapshotUID        string      `json:"snapshot_uid,omitempty"`
	PreviousPVC        *ObjectRef  `json:"previous_pvc,omitempty"`
	PreviousDV         *ObjectRef  `json:"previous_dv,omitempty"`
	Scope              Scope       `json:"scope"`
	VolumeID           uint        `json:"volume_id"`
	DiskKey            string      `json:"disk_key"`
	CanonicalName      string      `json:"canonical_name"`
	Role               string      `json:"role"`
	Attached           bool        `json:"attached"`
	Mapped             bool        `json:"mapped"`
	ControlRevision    uint64      `json:"control_revision"`
	BindingGeneration  uint64      `json:"binding_generation"`
	ActivePVC          *ObjectRef  `json:"active_pvc,omitempty"`
	ActiveDV           *ObjectRef  `json:"active_dv,omitempty"`
	Retained           []ObjectRef `json:"retained"`
	Provisioning       string      `json:"provisioning"`
	StorageClass       string      `json:"storage_class,omitempty"`
	VolumeMode         string      `json:"volume_mode,omitempty"`
	RequestedBytes     int64       `json:"requested_bytes,omitempty"`
	CapacityBytes      int64       `json:"capacity_bytes,omitempty"`
	PVCResourceVersion string      `json:"pvc_resource_version,omitempty"`
	ReadOnly           bool        `json:"read_only"`
}

// BindingFenceView excludes internal tokens and resource commands.
type BindingFenceView struct {
	OperationID string `json:"operation_id"`
	Kind        string `json:"kind"`
	Phase       string `json:"phase"`
}

// BindingView is the safe complete-set projection for Console/profile.
type BindingView struct {
	Scope              Scope             `json:"scope"`
	ControlRevision    uint64            `json:"control_revision"`
	BindingGeneration  uint64            `json:"binding_generation"`
	Fence              *BindingFenceView `json:"fence,omitempty"`
	MutationInProgress bool              `json:"mutation_in_progress"`
	Volumes            []ResolvedVolume  `json:"volumes"`
}

// Resolver performs reads only. It is not itself permission to mutate resources.
type Resolver struct {
	store   *Store
	core    kubernetes.Interface
	dynamic dynamic.Interface
}

// NewResolver uses the single Region state store and existing Kubernetes clients.
func NewResolver(store *Store, core kubernetes.Interface, dynamicClient dynamic.Interface) *Resolver {
	return &Resolver{store: store, core: core, dynamic: dynamicClient}
}

func pvcDisk(v dbmodel.TenantServiceVolume) bool {
	return v.VolumeType != "" && v.VolumeType != dbmodel.ConfigFileVolumeType.String() && v.VolumeType != dbmodel.MemoryFSVolumeType.String() && v.VolumeType != dbmodel.LocalVolumeType.String()
}
func platformDiskRole(v dbmodel.TenantServiceVolume) string {
	if v.VolumeName == "disk" {
		return "root"
	}
	return "data"
}
func refOf(kind string, m metav1.Object) ObjectRef {
	return ObjectRef{Kind: kind, Namespace: m.GetNamespace(), Name: m.GetName(), UID: string(m.GetUID())}
}
func sameObject(a, b ObjectRef) bool {
	return a.Kind == b.Kind && a.Namespace == b.Namespace && a.Name == b.Name && a.UID == b.UID
}
func addUniqueRef(refs []ObjectRef, next ObjectRef) []ObjectRef {
	for _, ref := range refs {
		if sameObject(ref, next) {
			return refs
		}
	}
	return append(refs, next)
}

// receipts returns bounded records without leaking them into the public DTO.
func (r *Resolver) receipts(scope Scope, state State) ([]Receipt, error) {
	rows, err := r.store.db.Model(&dbmodel.ComponentK8sAttributes{}).Where("component_id = ? AND name LIKE ? ESCAPE '='", scope.ServiceID, "vm=_restore=_receipt=_%").Order("name, ID").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	result := []Receipt{}
	for rows.Next() {
		var row dbmodel.ComponentK8sAttributes
		if err := r.store.db.ScanRows(rows, &row); err != nil {
			return nil, err
		}
		if seen[row.Name] || row.TenantID != scope.TenantID || row.ComponentID != scope.ServiceID || !strings.HasPrefix(row.Name, ReceiptPrefix) {
			return nil, ErrCorrupt
		}
		seen[row.Name] = true
		operationID := strings.TrimPrefix(row.Name, ReceiptPrefix)
		receipt, err := decodeStoredReceipt(row, scope, operationID)
		if err != nil {
			return nil, err
		}
		if err := validateSuccessfulSteps(receipt, state); err != nil {
			return nil, err
		}
		result = append(result, *receipt)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
func (r *Resolver) validateBindingRecords(scope Scope, state State) error {
	var rows []dbmodel.TenantServiceVolume
	if err := r.store.db.Where("service_id = ?", scope.ServiceID).Find(&rows).Error; err != nil {
		return err
	}
	records := map[uint]dbmodel.TenantServiceVolume{}
	for _, row := range rows {
		if pvcDisk(row) {
			records[row.ID] = row
		}
	}
	for _, binding := range state.Bindings {
		row, ok := records[binding.VolumeID]
		if !ok || row.VolumeName != binding.DiskKey || platformDiskRole(row) != binding.Role {
			return ErrCorrupt
		}
	}
	return nil
}

func (r *Resolver) volume(scope Scope, id uint) (dbmodel.TenantServiceVolume, error) {
	var rows []dbmodel.TenantServiceVolume
	if id == 0 {
		return dbmodel.TenantServiceVolume{}, ErrNotFound
	}
	if err := r.store.db.Where("ID = ? AND service_id = ?", id, scope.ServiceID).Find(&rows).Error; err != nil {
		return dbmodel.TenantServiceVolume{}, err
	}
	if len(rows) != 1 {
		return dbmodel.TenantServiceVolume{}, ErrNotFound
	}
	if !pvcDisk(rows[0]) {
		return rows[0], ErrNotPVCDisk
	}
	return rows[0], nil
}
func (r *Resolver) stable(scope Scope, state State) error {
	current, err := r.store.ReadOnly(scope, "")
	if err != nil {
		return err
	}
	if current.State.ControlRevision != state.ControlRevision || current.State.BindingGeneration != state.BindingGeneration {
		return ErrConflict
	}
	return nil
}
func (r *Resolver) dv(ctx context.Context, scope Scope, name string, expected *ObjectRef) (*unstructured.Unstructured, error) {
	obj, err := r.dynamic.Resource(dataVolumeResource).Namespace(scope.Namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		if expected != nil {
			return nil, ErrConflict
		}
		return nil, nil
	}
	if err != nil {
		return nil, ErrReferenceUnavailable
	}
	if obj.GetNamespace() != scope.Namespace || !validUID(string(obj.GetUID())) || obj.GetDeletionTimestamp() != nil || obj.GetLabels()["service_id"] != scope.ServiceID {
		return nil, ErrConflict
	}
	if tenant := obj.GetLabels()["tenant_id"]; tenant != "" && tenant != scope.TenantID {
		return nil, ErrConflict
	}
	if expected != nil && !sameObject(*expected, refOf("DataVolume", obj)) {
		return nil, ErrConflict
	}
	return obj, nil
}
func pvcOwnedBy(scope Scope, pvc *corev1.PersistentVolumeClaim, dv *unstructured.Unstructured) bool {
	if tenant := pvc.Labels["tenant_id"]; tenant != "" && tenant != scope.TenantID {
		return false
	}
	if service := pvc.Labels["service_id"]; service != "" && service != scope.ServiceID {
		return false
	}
	ownedByDV := false
	for _, owner := range pvc.OwnerReferences {
		if owner.Kind != "DataVolume" {
			continue
		}
		if dv == nil || owner.Name != dv.GetName() || owner.UID != dv.GetUID() {
			return false
		}
		ownedByDV = true
	}
	return pvc.Labels["service_id"] == scope.ServiceID || ownedByDV
}
func (r *Resolver) resolve(ctx context.Context, scope Scope, state State, volume dbmodel.TenantServiceVolume, receipts []Receipt) (ResolvedVolume, error) {
	result := ResolvedVolume{Scope: scope, VolumeID: volume.ID, DiskKey: volume.VolumeName, CanonicalName: "manual" + strconv.FormatUint(uint64(volume.ID), 10), Role: platformDiskRole(volume), Attached: true, ControlRevision: state.ControlRevision, BindingGeneration: state.BindingGeneration, Provisioning: "not_provisioned", Retained: []ObjectRef{}, ReadOnly: volume.IsReadOnly}
	binding, mapped := state.Bindings[strconv.FormatUint(uint64(volume.ID), 10)]
	result.Mapped = mapped
	claimName := result.CanonicalName
	var dv *unstructured.Unstructured
	var err error
	if mapped {
		if binding.VolumeID != volume.ID || binding.DiskKey != volume.VolumeName || binding.Role != result.Role || binding.CanonicalName != result.CanonicalName {
			return result, ErrConflict
		}
		result.Attached = binding.Attached
		result.RestoreID = binding.RestoreID
		result.SnapshotUID = binding.SnapshotUID
		result.PreviousPVC = binding.PreviousPVC
		result.PreviousDV = binding.PreviousDV
		claimName = binding.ActivePVC.Name
		if binding.ActiveDV != nil {
			dv, err = r.dv(ctx, scope, binding.ActiveDV.Name, binding.ActiveDV)
			if err != nil {
				return result, err
			}
			result.ActiveDV = binding.ActiveDV
		}
		if binding.PreviousPVC != nil {
			result.Retained = addUniqueRef(result.Retained, *binding.PreviousPVC)
		}
		if binding.PreviousDV != nil {
			result.Retained = addUniqueRef(result.Retained, *binding.PreviousDV)
		}
	} else {
		if err := r.fallbackFootprints(ctx, scope, state, volume.ID, receipts); err != nil {
			return result, err
		}
		dv, err = r.dv(ctx, scope, result.CanonicalName, nil)
		if err != nil {
			return result, err
		}
		if dv != nil {
			ref := refOf("DataVolume", dv)
			result.ActiveDV = &ref
		}
	}
	if dv != nil {
		current, found, fieldErr := unstructured.NestedString(dv.Object, "status", "claimName")
		if fieldErr != nil {
			return result, ErrCorrupt
		}
		if mapped {
			if !found || current != claimName {
				return result, ErrConflict
			}
		} else if current != "" {
			claimName = current
		}
		result.Provisioning = "pending"
	}
	pvc, err := r.core.CoreV1().PersistentVolumeClaims(scope.Namespace).Get(ctx, claimName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		if mapped {
			return result, ErrConflict
		}
		return result, nil
	}
	if err != nil {
		return result, ErrReferenceUnavailable
	}
	if pvc.Namespace != scope.Namespace || !validUID(string(pvc.UID)) || pvc.DeletionTimestamp != nil || !pvcOwnedBy(scope, pvc, dv) {
		return result, ErrConflict
	}
	ref := refOf("PersistentVolumeClaim", pvc)
	if mapped && !sameObject(binding.ActivePVC, ref) {
		return result, ErrConflict
	}
	result.ActivePVC = &ref
	result.PVCResourceVersion = pvc.ResourceVersion
	result.Provisioning = "pending"
	if pvc.Status.Phase == corev1.ClaimBound {
		result.Provisioning = "bound"
	}
	if pvc.Spec.StorageClassName != nil {
		result.StorageClass = *pvc.Spec.StorageClassName
	}
	result.VolumeMode = string(corev1.PersistentVolumeFilesystem)
	if pvc.Spec.VolumeMode != nil {
		result.VolumeMode = string(*pvc.Spec.VolumeMode)
	}
	requested := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	capacity := pvc.Status.Capacity[corev1.ResourceStorage]
	result.RequestedBytes = requested.Value()
	result.CapacityBytes = capacity.Value()
	for _, receipt := range receipts {
		if !receipt.RetainOldPVCs {
			continue
		}
		if old, ok := receipt.OldBindings[strconv.FormatUint(uint64(volume.ID), 10)]; ok {
			for _, candidate := range []*ObjectRef{&old.ActivePVC, old.ActiveDV, old.PreviousPVC, old.PreviousDV} {
				if candidate != nil && !sameObject(*candidate, ref) && (result.ActiveDV == nil || !sameObject(*candidate, *result.ActiveDV)) {
					result.Retained = addUniqueRef(result.Retained, *candidate)
				}
			}
		}
	}
	return result, nil
}

// ResolveVolume uses persisted bindings first, then canonical/DV claimName only
// for a never-mapped volume. No UID mismatch can become canonical fallback.
func (r *Resolver) ResolveVolume(ctx context.Context, scope Scope, volumeID uint) (ResolvedVolume, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if r == nil || r.store == nil || r.store.db == nil || r.core == nil || r.dynamic == nil {
		return ResolvedVolume{}, ErrReferenceUnavailable
	}
	current, err := r.store.Read(scope, "")
	if err != nil {
		return ResolvedVolume{}, err
	}
	if err := r.validateBindingRecords(scope, current.State); err != nil {
		return ResolvedVolume{}, err
	}
	volume, err := r.volume(scope, volumeID)
	if err != nil {
		return ResolvedVolume{}, err
	}
	receipts, err := r.receipts(scope, current.State)
	if err != nil {
		return ResolvedVolume{}, err
	}
	result, err := r.resolve(ctx, scope, current.State, volume, receipts)
	if err != nil {
		return result, err
	}
	if err = r.stable(scope, current.State); err != nil {
		return result, err
	}
	return result, nil
}

// ReadBindingView shares one consistent generation for the complete disk set.
func (r *Resolver) ReadBindingView(ctx context.Context, scope Scope) (BindingView, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	view := BindingView{Scope: scope, Volumes: []ResolvedVolume{}}
	if r == nil || r.store == nil || r.store.db == nil || r.core == nil || r.dynamic == nil {
		return view, ErrReferenceUnavailable
	}
	current, err := r.store.ReadOnly(scope, "")
	if err != nil {
		return view, err
	}
	state := current.State
	if err := r.validateBindingRecords(scope, state); err != nil {
		return view, err
	}
	view.ControlRevision = state.ControlRevision
	view.BindingGeneration = state.BindingGeneration
	view.MutationInProgress = state.Admission != nil
	if state.Fence != nil {
		view.Fence = &BindingFenceView{OperationID: state.Fence.OperationID, Kind: state.Fence.Kind, Phase: state.Fence.Phase}
	}
	var volumes []dbmodel.TenantServiceVolume
	if err := r.store.db.Where("service_id = ?", scope.ServiceID).Order("ID").Find(&volumes).Error; err != nil {
		return view, err
	}
	receipts, err := r.receipts(scope, state)
	if err != nil {
		return view, err
	}
	for _, volume := range volumes {
		if !pvcDisk(volume) {
			continue
		}
		resolved, err := r.resolve(ctx, scope, state, volume, receipts)
		if err != nil {
			return view, err
		}
		view.Volumes = append(view.Volumes, resolved)
	}
	sort.Slice(view.Volumes, func(i, j int) bool { return view.Volumes[i].VolumeID < view.Volumes[j].VolumeID })
	if err := r.stable(scope, state); err != nil {
		return view, err
	}
	return view, nil
}

// MaterializedBinding captures actual canonical identities for rollback. It is
// a mapped identity retaining known provenance or an empty canonical-origin pair.
func (v ResolvedVolume) MaterializedBinding() (Binding, error) {
	if v.ActivePVC == nil || v.Provisioning != "bound" {
		return Binding{}, ErrConflict
	}
	result := Binding{VolumeID: v.VolumeID, DiskKey: v.DiskKey, CanonicalName: v.CanonicalName, Role: v.Role, Attached: v.Attached, ActivePVC: *v.ActivePVC, ActiveDV: v.ActiveDV, PreviousPVC: v.PreviousPVC, PreviousDV: v.PreviousDV, RestoreID: v.RestoreID, SnapshotUID: v.SnapshotUID}
	state := NewState(v.Scope)
	state.Bindings[strconv.FormatUint(uint64(v.VolumeID), 10)] = result
	if err := state.Validate(v.Scope); err != nil {
		return Binding{}, err
	}
	return result, nil
}
