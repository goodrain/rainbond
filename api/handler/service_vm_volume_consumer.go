package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/util/retry"

	"github.com/goodrain/rainbond/api/util/bcode"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	"github.com/jinzhu/gorm"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kv "kubevirt.io/api/core/v1"
)

type vmVolumeConsumer struct {
	database *gorm.DB
	store    *vmrestore.Store
	resolver *vmrestore.Resolver
	scope    vmrestore.Scope
}
type vmVolumeMutation struct {
	*vmVolumeConsumer
	current                 vmrestore.Current
	operation, action, hash string
	volume                  vmrestore.ResolvedVolume
	ctx                     context.Context
}

func (s *ServiceAction) vmVolumes(service *dbmodel.TenantServices) (*vmVolumeConsumer, error) {
	database := s.getDBManager().DB()
	if database == nil || s.kubeClient == nil || vmHotplugDynamicClient() == nil {
		return nil, vmrestore.ErrReferenceUnavailable
	}
	namespace, err := s.resolveVolumeExpansionNamespace(service)
	if err != nil {
		return nil, err
	}
	scope := vmrestore.Scope{TenantID: service.TenantID, ServiceID: service.ServiceID, Namespace: namespace}
	store := vmrestore.NewStore(database)
	return &vmVolumeConsumer{database: database, store: store, resolver: vmrestore.NewResolver(store, s.kubeClient, vmHotplugDynamicClient()), scope: scope}, nil
}
func vmVolumeError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(bcode.Coder); ok {
		return err
	}
	if errors.Is(err, vmrestore.ErrNotPVCDisk) {
		return bcode.NewBadRequest("此操作不适用于非PVC存储")
	}
	if errors.Is(err, vmrestore.ErrNotFound) {
		return &bcode.Code{Status: 404, Code: 404, Message: "所属虚拟机存储不存在"}
	}
	if errors.Is(err, vmrestore.ErrBusy) || errors.Is(err, vmrestore.ErrConflict) || errors.Is(err, vmrestore.ErrCorrupt) || errors.Is(err, vmrestore.ErrDetached) || apierrors.IsConflict(err) {
		return &bcode.Code{Status: 409, Code: 409, Message: "虚拟机磁盘操作冲突，请核对当前任务、卷身份及引用后重试"}
	}
	return err
}
func vmVolumeExpectation(c vmrestore.Current, operation, hash, step string) vmrestore.Expectation {
	return vmrestore.Expectation{ControlRevision: c.State.ControlRevision, BindingGeneration: c.State.BindingGeneration, OperationID: operation, ManifestHash: hash, Step: step, RequestHash: hash}
}
func vmVolumeResources(v vmrestore.ResolvedVolume) []vmrestore.ResourceRef {
	refs := []vmrestore.ResourceRef{}
	if v.ActivePVC != nil {
		refs = append(refs, vmrestore.ResourceRef{ObjectRef: *v.ActivePVC, Purpose: "mutation-active-pvc"})
	}
	if v.ActiveDV != nil {
		refs = append(refs, vmrestore.ResourceRef{ObjectRef: *v.ActiveDV, Purpose: "mutation-active-dv"})
	}
	return refs
}
func vmVolumeFingerprint(scope vmrestore.Scope, id uint, key, verb, request string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s/%s/%s/%d/%s/%s/%s", scope.TenantID, scope.ServiceID, scope.Namespace, id, key, verb, request)))
	return hex.EncodeToString(sum[:])
}

func (c *vmVolumeConsumer) begin(ctx context.Context, volume *dbmodel.TenantServiceVolume, verb, request string) (*vmVolumeMutation, error) {
	fingerprint := vmVolumeFingerprint(c.scope, volume.ID, volume.VolumeName, verb, request)
	var record dbmodel.TenantServiceVolume
	if err := c.database.Where("ID = ? AND service_id = ?", volume.ID, c.scope.ServiceID).First(&record).Error; err != nil {
		return nil, err
	}
	if record.VolumeName != volume.VolumeName || volume.ServiceID != c.scope.ServiceID || record.IsReadOnly || record.VolumeName == "disk" {
		return nil, vmrestore.ErrConflict
	}

	action := "disk-" + verb + "-" + fingerprint
	current, err := c.store.Read(c.scope, "")
	if err != nil {
		return nil, err
	}
	if current.State.Fence != nil {
		return nil, vmrestore.ErrBusy
	}
	mutation := &vmVolumeMutation{vmVolumeConsumer: c, action: action, hash: fingerprint}
	if current.State.Admission != nil {
		if current.State.Admission.Action != action || !current.State.Admission.OutcomeUnknown {
			return nil, vmrestore.ErrBusy
		}
		mutation.operation = current.State.Admission.Token
		mutation.current, err = c.store.Read(c.scope, mutation.operation)
		if err != nil {
			return nil, err
		}
		receipt := mutation.current.Receipt
		if receipt == nil || receipt.ManifestHash != fingerprint {
			return nil, vmrestore.ErrConflict
		}
		prepared, ok := receipt.Steps["prepare"]
		if !ok || !prepared.Applied || prepared.RequestHash != fingerprint || prepared.BindingGeneration != mutation.current.State.BindingGeneration {
			return nil, vmrestore.ErrConflict
		}
		binding, ok := receipt.OldBindings[strconv.FormatUint(uint64(volume.ID), 10)]
		if !ok || binding.DiskKey != volume.VolumeName {
			return nil, vmrestore.ErrConflict
		}
		mutation.volume = vmrestore.ResolvedVolume{Scope: c.scope, VolumeID: volume.ID, DiskKey: binding.DiskKey, CanonicalName: binding.CanonicalName, Role: binding.Role, Attached: binding.Attached, ActivePVC: &binding.ActivePVC, ActiveDV: binding.ActiveDV, BindingGeneration: prepared.BindingGeneration, Mapped: true, Provisioning: "bound"}
		// Unknown is retryable; an executing call is not. Claim the retry by
		// the same Store CAS before any caller can perform external work.
		claimed, claimErr := c.store.Mutate(c.scope, vmrestore.Expectation{ControlRevision: mutation.current.State.ControlRevision, BindingGeneration: prepared.BindingGeneration, AdmissionOwner: mutation.operation}, func(now vmrestore.Current) (vmrestore.Mutation, error) {
			admission := now.State.Admission
			if admission == nil || admission.Token != mutation.operation || admission.Action != action || !admission.OutcomeUnknown {
				return vmrestore.Mutation{}, vmrestore.ErrBusy
			}
			admission.OutcomeUnknown = false
			return vmrestore.Mutation{State: now.State}, nil
		})
		if claimErr != nil {
			return nil, claimErr
		}
		mutation.current.State = claimed.State
	} else {
		resolved, err := c.resolver.ResolveVolume(ctx, c.scope, volume.ID)
		if err != nil {
			return nil, err
		}
		if resolved.Role != "data" || resolved.ReadOnly {
			return nil, vmrestore.ErrConflict
		}
		if resolved.ActivePVC == nil || resolved.Provisioning != "bound" {
			return nil, vmrestore.ErrConflict
		}
		binding, err := resolved.MaterializedBinding()
		if err != nil {
			return nil, err
		}
		bytes := make([]byte, 16)
		if _, err := rand.Read(bytes); err != nil {
			return nil, err
		}
		mutation.operation = "disk-" + verb + "-" + hex.EncodeToString(bytes)
		mutation.current, err = c.store.Mutate(c.scope, vmVolumeExpectation(current, mutation.operation, fingerprint, "prepare"), func(now vmrestore.Current) (vmrestore.Mutation, error) {
			if err := now.State.AdmitMutation(mutation.operation, action); err != nil {
				return vmrestore.Mutation{}, err
			}
			if !resolved.Mapped {
				now.State.Bindings[strconv.FormatUint(uint64(volume.ID), 10)] = binding
				now.State.BindingGeneration++
			}
			receipt := vmrestore.NewReceipt(c.scope, mutation.operation, fingerprint)
			receipt.OldBindings[strconv.FormatUint(uint64(volume.ID), 10)] = binding
			receipt.NewBindings[strconv.FormatUint(uint64(volume.ID), 10)] = binding
			receipt.Resources = vmVolumeResources(resolved)
			return vmrestore.Mutation{State: now.State, Receipt: &receipt}, nil
		})
		if err != nil {
			return nil, err
		}
		resolved.BindingGeneration = mutation.current.State.BindingGeneration
		mutation.volume = resolved
	}
	mutation.ctx = vmrestore.WithAdmissionContext(ctx, mutation.operation)
	if err := mutation.check(); err != nil {
		return nil, err
	}
	return mutation, nil
}
func (m *vmVolumeMutation) check() error {
	now, err := m.store.Read(m.scope, m.operation)
	if err != nil {
		return err
	}
	if now.State.Fence != nil || now.State.Admission == nil || now.State.Admission.Token != m.operation || now.State.Admission.Action != m.action || now.State.BindingGeneration != m.volume.BindingGeneration || now.Receipt == nil || now.Receipt.ManifestHash != m.hash {
		return vmrestore.ErrConflict
	}
	m.current = now
	return nil
}
func (m *vmVolumeMutation) step(name string, change func(*vmrestore.Current) error) error {
	if err := m.check(); err != nil {
		return err
	}
	updated, err := m.store.Mutate(m.scope, vmVolumeExpectation(m.current, m.operation, m.hash, name), func(c vmrestore.Current) (vmrestore.Mutation, error) {
		if err := change(&c); err != nil {
			return vmrestore.Mutation{}, err
		}
		return vmrestore.Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err == nil {
		m.current = updated
	}
	return err
}
func (m *vmVolumeMutation) pending() {
	m.finish(false)
}
func (m *vmVolumeMutation) finish(known bool) {
	if m.check() != nil {
		return
	}
	// A retry may become unknown more than once. An immutable historical
	// "pending" step must not suppress this new execution's state update.
	updated, err := m.store.Mutate(m.scope, vmrestore.Expectation{ControlRevision: m.current.State.ControlRevision, BindingGeneration: m.volume.BindingGeneration, AdmissionOwner: m.operation}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		if c.State.Admission == nil || c.State.Admission.Token != m.operation || c.State.Admission.Action != m.action {
			return vmrestore.Mutation{}, vmrestore.ErrConflict
		}
		if err := c.State.FinishMutation(m.operation, known); err != nil {
			return vmrestore.Mutation{}, err
		}
		return vmrestore.Mutation{State: c.State}, nil
	})
	if err == nil {
		m.current.State = updated.State
	}
}
func (m *vmVolumeMutation) reject() {
	if m.check() != nil {
		return
	}
	known := !m.current.Receipt.Steps["issued"].Applied && !m.current.Receipt.Steps["detaching"].Applied && !m.current.Receipt.Steps["pending"].Applied
	m.finish(known)
}
func (m *vmVolumeMutation) refs() (vmrestore.ReferenceSummary, error) {
	if err := m.check(); err != nil {
		return vmrestore.ReferenceSummary{}, err
	}
	if m.current.Receipt.Steps["issued"].Applied && m.action == "disk-delete-"+m.hash {
		return m.resolver.VolumeReferencesForMutation(m.ctx, m.scope, m.volume.VolumeID, m.operation)
	}
	return m.resolver.VolumeReferences(m.ctx, m.scope, m.volume.VolumeID)
}
func (m *vmVolumeMutation) matches(v vmrestore.ResolvedVolume) bool {
	return v.BindingGeneration == m.volume.BindingGeneration && v.ActivePVC != nil && m.volume.ActivePVC != nil && *v.ActivePVC == *m.volume.ActivePVC && ((v.ActiveDV == nil && m.volume.ActiveDV == nil) || (v.ActiveDV != nil && m.volume.ActiveDV != nil && *v.ActiveDV == *m.volume.ActiveDV))
}

func (s *ServiceAction) resolvedVM(ctx context.Context, v vmrestore.ResolvedVolume) (*kv.VirtualMachine, error) {
	if s.getVirtualMachineByServiceIDHook != nil {
		vm, err := s.getVirtualMachineByServiceIDHook(v.Scope.ServiceID)
		if err != nil {
			return nil, err
		}
		if vm != nil && vm.Namespace != v.Scope.Namespace {
			return nil, &bcode.Code{Status: 409, Code: 409, Message: "虚拟机与组件命名空间不一致，配置及存储已保留"}
		}
		if vm != nil && (vm.UID == "" || vm.Labels["service_id"] != v.Scope.ServiceID) {
			return nil, vmrestore.ErrConflict
		}
		return vm, nil
	}

	list, err := vmHotplugDynamicClient().Resource(vmDiskVMGVR).Namespace(v.Scope.Namespace).List(ctx, metav1.ListOptions{LabelSelector: "service_id=" + v.Scope.ServiceID})
	if err != nil {
		return nil, err
	}
	if len(list.Items) > 1 {
		return nil, vmrestore.ErrConflict
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	var vm kv.VirtualMachine
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(list.Items[0].Object, &vm); err != nil {
		return nil, err
	}
	if vm.UID == "" {
		return nil, vmrestore.ErrConflict
	}
	return &vm, nil
}
func vmResolvedVolumeUses(v kv.Volume, r vmrestore.ResolvedVolume) bool {
	return v.PersistentVolumeClaim != nil && r.ActivePVC != nil && v.PersistentVolumeClaim.ClaimName == r.ActivePVC.Name || v.DataVolume != nil && r.ActiveDV != nil && v.DataVolume.Name == r.ActiveDV.Name
}
func (s *ServiceAction) checkVMDataDevice(ctx context.Context, r vmrestore.ResolvedVolume) (*kv.VirtualMachine, error) {
	vm, err := s.resolvedVM(ctx, r)
	if err != nil || vm == nil {
		return vm, err
	}
	if vm.Spec.Template == nil {
		return nil, vmrestore.ErrConflict
	}
	targets := map[string]bool{}
	for _, v := range vm.Spec.Template.Spec.Volumes {
		if vmResolvedVolumeUses(v, r) {
			targets[v.Name] = true
		}
		if v.Name == r.CanonicalName && !vmResolvedVolumeUses(v, r) {
			return nil, vmrestore.ErrConflict
		}
	}
	for _, disk := range vm.Spec.Template.Spec.Domain.Devices.Disks {
		if targets[disk.Name] && ((disk.BootOrder != nil && *disk.BootOrder == 1) || disk.CDRom != nil) {
			return nil, vmrestore.ErrConflict
		}
	}
	return vm, nil
}
func (s *ServiceAction) detachResolvedVM(m *vmVolumeMutation) error {
	refs, err := m.refs()
	if err != nil {
		return err
	}
	if !refs.CanDetach {
		return vmrestore.ErrConflict
	}
	vm, err := s.checkVMDataDevice(m.ctx, m.volume)
	if err != nil {
		return err
	}
	instances, err := vmHotplugDynamicClient().Resource(vmDiskVMIGVR).Namespace(m.scope.Namespace).List(m.ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, obj := range instances.Items {
		var instance kv.VirtualMachineInstance
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &instance); err != nil {
			return err
		}
		for _, v := range instance.Spec.Volumes {
			if !vmResolvedVolumeUses(v, m.volume) {
				continue
			}
			if !vmDiskOwnedInstance(&obj, vm, m.scope.ServiceID) {
				return vmrestore.ErrConflict
			}
			hot := v.DataVolume != nil && v.DataVolume.Hotpluggable || v.PersistentVolumeClaim != nil && v.PersistentVolumeClaim.Hotpluggable
			if !hot {
				return &bcode.Code{Status: 409, Code: 409, Message: "请先关闭虚拟机再删除该磁盘；原配置和存储均保留"}
			}
			if err := m.step("detaching", func(*vmrestore.Current) error { return nil }); err != nil {
				return err
			}
			if s.kubevirtClient == nil {
				return vmrestore.ErrReferenceUnavailable
			}
			if err := s.kubevirtClient.VirtualMachine(m.scope.Namespace).RemoveVolume(m.ctx, vm.Name, buildVMHotplugRemoveVolumeOptions(m.volume.CanonicalName)); err != nil {
				return err
			}
			return &bcode.Code{Status: 409, Code: 409, Message: "磁盘正在卸载，请稍后重试删除；存储尚未回收"}
		}
	}
	if vm != nil {
		updated := vm.DeepCopy()
		names := map[string]bool{}
		volumes := []kv.Volume{}
		for _, v := range updated.Spec.Template.Spec.Volumes {
			if vmResolvedVolumeUses(v, m.volume) {
				names[v.Name] = true
			} else {
				volumes = append(volumes, v)
			}
		}
		if len(names) > 0 {
			updated.Spec.Template.Spec.Volumes = volumes
			disks := []kv.Disk{}
			for _, disk := range updated.Spec.Template.Spec.Domain.Devices.Disks {
				if !names[disk.Name] {
					disks = append(disks, disk)
				}
			}
			updated.Spec.Template.Spec.Domain.Devices.Disks = disks
			templates := []kv.DataVolumeTemplateSpec{}
			for _, dv := range updated.Spec.DataVolumeTemplates {
				if m.volume.ActiveDV == nil || dv.Name != m.volume.ActiveDV.Name {
					templates = append(templates, dv)
				}
			}
			updated.Spec.DataVolumeTemplates = templates
			if vm.ResourceVersion == "" {
				return vmrestore.ErrConflict
			}
			if err := m.step("detaching", func(*vmrestore.Current) error { return nil }); err != nil {
				return err
			}
			raw, err := json.Marshal(updated)
			if err != nil {
				return err
			}
			obj := &unstructured.Unstructured{}
			if err := obj.UnmarshalJSON(raw); err != nil {
				return err
			}
			_, err = vmHotplugDynamicClient().Resource(vmDiskVMGVR).Namespace(m.scope.Namespace).Update(m.ctx, obj, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	}
	refs, err = m.refs()
	if err != nil {
		return err
	}
	if !refs.CanDelete {
		return vmrestore.ErrConflict
	}
	return nil
}
func (s *ServiceAction) reclaimResolvedVM(m *vmVolumeMutation) error {
	// The issued receipt was sealed while the shared reference check allowed deletion.
	// Subsequent retries use only its exact identities, never Resolve's missing-target fallback.
	if err := m.check(); err != nil {
		return err
	}
	if m.volume.ActiveDV == nil && m.volume.ActivePVC != nil && m.volume.ActivePVC.Name == m.volume.CanonicalName {
		_, err := vmHotplugDynamicClient().Resource(dataVolumeGVR).Namespace(m.scope.Namespace).Get(m.ctx, m.volume.CanonicalName, metav1.GetOptions{})
		if err == nil {
			return vmrestore.ErrConflict
		}
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	refs := vmVolumeResources(m.volume)
	alive := 0
	for _, ref := range refs {
		if ref.Kind == "PersistentVolumeClaim" {
			pvc, err := s.kubeClient.CoreV1().PersistentVolumeClaims(ref.Namespace).Get(m.ctx, ref.Name, metav1.GetOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			if err == nil {
				alive++
			}
			if err == nil && string(pvc.UID) != ref.UID {
				return vmrestore.ErrConflict
			}
		} else {
			dv, err := vmHotplugDynamicClient().Resource(dataVolumeGVR).Namespace(ref.Namespace).Get(m.ctx, ref.Name, metav1.GetOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			if err == nil {
				alive++
			}
			if err == nil && string(dv.GetUID()) != ref.UID {
				return vmrestore.ErrConflict
			}
		}
	}
	if alive > 0 {
		current, err := m.refs()
		if err != nil {
			return err
		}
		if !current.CanDelete {
			return vmrestore.ErrConflict
		}
	}

	// Remove the DV first so its controller cannot recreate the PVC during reclamation.
	if ref := m.volume.ActiveDV; ref != nil {
		if err := m.check(); err != nil {
			return err
		}
		uid := types.UID(ref.UID)
		err := vmHotplugDynamicClient().Resource(dataVolumeGVR).Namespace(ref.Namespace).Delete(m.ctx, ref.Name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		if _, err := vmHotplugDynamicClient().Resource(dataVolumeGVR).Namespace(ref.Namespace).Get(m.ctx, ref.Name, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
			if err != nil {
				return err
			}
			return &bcode.Code{Status: 409, Code: 409, Message: "磁盘存储仍在回收，请稍后重试；配置已保留"}
		}
	}
	if ref := m.volume.ActivePVC; ref != nil {
		if err := m.check(); err != nil {
			return err
		}
		uid := types.UID(ref.UID)
		err := s.kubeClient.CoreV1().PersistentVolumeClaims(ref.Namespace).Delete(m.ctx, ref.Name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		if _, err := s.kubeClient.CoreV1().PersistentVolumeClaims(ref.Namespace).Get(m.ctx, ref.Name, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
			if err != nil {
				return err
			}
			return &bcode.Code{Status: 409, Code: 409, Message: "磁盘存储仍在回收，请稍后重试；配置已保留"}
		}
	}
	return nil
}
func (s *ServiceAction) deleteBoundVM(service *dbmodel.TenantServices, volume *dbmodel.TenantServiceVolume) error {
	consumer, err := s.vmVolumes(service)
	if err != nil {
		return err
	}
	m, err := consumer.begin(context.Background(), volume, "delete", "")
	if err != nil {
		return err
	}
	issued := m.current.Receipt.Steps["issued"].Applied
	if !issued {
		if err := s.detachResolvedVM(m); err != nil {
			if m.current.Receipt.Steps["detaching"].Applied {
				m.pending()
			} else {
				m.reject()
			}
			return err
		}
		if err := m.step("issued", func(*vmrestore.Current) error { return nil }); err != nil {
			return err
		}
	}
	if err := s.reclaimResolvedVM(m); err != nil {
		m.pending()
		return err
	}
	if err := m.check(); err != nil {
		return err
	}
	tx := consumer.database.Begin()
	defer tx.Rollback()
	if tx.Error != nil {
		return tx.Error
	}
	_, err = consumer.store.MutateTx(tx, consumer.scope, vmVolumeExpectation(m.current, m.operation, m.hash, "complete"), func(c vmrestore.Current) (vmrestore.Mutation, error) {
		if c.State.Admission == nil || c.State.Admission.Token != m.operation || c.State.Admission.Action != m.action {
			return vmrestore.Mutation{}, vmrestore.ErrConflict
		}
		if err := tx.Where("ID = ? AND service_id = ?", volume.ID, service.ServiceID).Delete(&dbmodel.TenantServiceVolume{}).Error; err != nil {
			return vmrestore.Mutation{}, err
		}
		if err := tx.Where("service_id = ? AND volume_name = ?", service.ServiceID, volume.VolumeName).Delete(&dbmodel.TenantServiceConfigFile{}).Error; err != nil {
			return vmrestore.Mutation{}, err
		}
		key := strconv.FormatUint(uint64(volume.ID), 10)
		if _, ok := c.State.Bindings[key]; ok {
			delete(c.State.Bindings, key)
			c.State.BindingGeneration++
		}
		c.Receipt.NewBindings = map[string]vmrestore.Binding{}
		if err := c.State.FinishMutation(m.operation, true); err != nil {
			return vmrestore.Mutation{}, err
		}
		return vmrestore.Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		tx.Rollback()
		m.pending()
		return err
	}
	if err := tx.Commit().Error; err != nil {
		m.pending()
		return err
	}
	return nil
}

func (s *ServiceAction) resizeVMVolume(ctx context.Context, service *dbmodel.TenantServices, volume *dbmodel.TenantServiceVolume, target int64) error {
	if target <= 0 || target < volume.VolumeCapacity {
		return bcode.NewBadRequest("volume capacity can only be expanded")
	}
	consumer, err := s.vmVolumes(service)
	if err != nil {
		return err
	}
	m, err := consumer.begin(ctx, volume, "resize", strconv.FormatInt(target, 10))
	if err != nil {
		return err
	}
	refs, err := m.refs()
	if err != nil {
		m.reject()
		return err
	}
	if !refs.CanExpand {
		m.reject()
		return vmrestore.ErrConflict
	}

	status, claims, err := s.inspectVolumeExpansion(m.ctx, service, volume)
	if err != nil {
		m.reject()
		return err
	}
	if !status.AllowExpansion || len(claims) != 1 {
		m.reject()
		return bcode.NewBadRequest(status.Message)
	}
	claim := claims[0]
	size := resource.MustParse(fmt.Sprintf("%dGi", target))
	requested := claim.Spec.Resources.Requests[corev1.ResourceStorage]
	actual := claim.Status.Capacity[corev1.ResourceStorage]
	if size.Cmp(requested) < 0 || size.Cmp(actual) < 0 {
		m.reject()
		return bcode.NewBadRequest("volume capacity cannot be reduced")
	}
	if err := m.step("issued", func(*vmrestore.Current) error { return nil }); err != nil {
		return err
	}
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := m.check(); err != nil {
			return err
		}
		resolved, err := consumer.resolver.ResolveVolume(m.ctx, m.scope, volume.ID)
		if err != nil {
			return err
		}
		if !m.matches(resolved) {
			return vmrestore.ErrConflict
		}
		refs, err := m.refs()
		if err != nil {
			return err
		}
		if !refs.CanExpand {
			return vmrestore.ErrConflict
		}
		current, err := s.kubeClient.CoreV1().PersistentVolumeClaims(m.scope.Namespace).Get(m.ctx, m.volume.ActivePVC.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if err := vmrestore.CheckLivePVC(*m.volume.ActivePVC, current.Namespace, current.Name, string(current.UID)); err != nil {
			return err
		}
		quantity := current.Spec.Resources.Requests[corev1.ResourceStorage]
		capacity := current.Status.Capacity[corev1.ResourceStorage]
		if quantity.Cmp(size) > 0 || capacity.Cmp(size) > 0 {
			return vmrestore.ErrConflict
		}
		if quantity.Cmp(size) == 0 {
			return nil
		}
		next := current.DeepCopy()
		if next.Spec.Resources.Requests == nil {
			next.Spec.Resources.Requests = corev1.ResourceList{}
		}
		next.Spec.Resources.Requests[corev1.ResourceStorage] = size
		_, err = s.kubeClient.CoreV1().PersistentVolumeClaims(m.scope.Namespace).Update(m.ctx, next, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		m.pending()
		return err
	}
	if err := m.check(); err != nil {
		return err
	}
	tx := consumer.database.Begin()
	defer tx.Rollback()
	if tx.Error != nil {
		return tx.Error
	}
	_, err = consumer.store.MutateTx(tx, m.scope, vmVolumeExpectation(m.current, m.operation, m.hash, "complete"), func(c vmrestore.Current) (vmrestore.Mutation, error) {
		if c.State.Admission == nil || c.State.Admission.Token != m.operation {
			return vmrestore.Mutation{}, vmrestore.ErrConflict
		}
		result := tx.Model(&dbmodel.TenantServiceVolume{}).Where("ID = ? AND service_id = ?", volume.ID, service.ServiceID).UpdateColumn("volume_capacity", target)
		if result.Error != nil {
			return vmrestore.Mutation{}, result.Error
		}
		if result.RowsAffected != 1 {
			var stored dbmodel.TenantServiceVolume
			if err := tx.Where("ID = ? AND service_id = ?", volume.ID, service.ServiceID).First(&stored).Error; err != nil {
				return vmrestore.Mutation{}, err
			}
			if stored.VolumeCapacity != target {
				return vmrestore.Mutation{}, vmrestore.ErrConflict
			}
		}
		if err := c.State.FinishMutation(m.operation, true); err != nil {
			return vmrestore.Mutation{}, err
		}
		return vmrestore.Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		tx.Rollback()
		m.pending()
		return err
	}
	if err := tx.Commit().Error; err != nil {
		m.pending()
		return err
	}
	return nil
}
func (s *ServiceAction) updateVMVolume(service *dbmodel.TenantServices, req *apimodel.UpdVolumeReq) error {
	volume, err := s.getDBManager().TenantServiceVolumeDao().GetVolumeByServiceIDAndName(service.ServiceID, req.VolumeName)
	if err != nil {
		return err
	}
	if req.VolumePath != "" && req.VolumePath != volume.VolumePath || req.VolumeType != "" && req.VolumeType != volume.VolumeType || req.Mode != nil && (volume.Mode == nil || *req.Mode != *volume.Mode) {
		return &bcode.Code{Status: 409, Code: 409, Message: "当前仅支持保持磁盘连接配置的数据盘扩容；设备及路径变更需等待统一运行配置接线"}
	}
	if req.VolumeCapacity == nil {
		return bcode.NewBadRequest("volume capacity is required")
	}
	// Generic conversion is not yet binding-aware; never reconstruct manual B here.
	return s.resizeVMVolume(context.Background(), service, volume, *req.VolumeCapacity)
}

func (s *ServiceAction) replayVMVolumeDelete(service *dbmodel.TenantServices, name string, original error) *util.APIHandleError {
	if !errors.Is(original, gorm.ErrRecordNotFound) {
		return util.CreateAPIHandleErrorFromDBError("find volume", original)
	}
	consumer, err := s.vmVolumes(service)
	if err != nil {
		return vmDiskAPIError(err)
	}
	state, err := consumer.store.Read(consumer.scope, "")
	if err != nil {
		return vmDiskAPIError(err)
	}
	// The non-PVC delete already committed its row before legacy runtime sync.
	// Resume only this actor's recorded unknown outcome, never an in-flight caller.
	if state.State.Fence == nil && len(state.State.Bindings) == 0 && state.State.Admission != nil && state.State.Admission.OutcomeUnknown && state.State.Admission.Action == "delete-config:"+name {
		return vmDiskAPIError(s.withLegacyVMConfig(service, state.State.Admission.Action, func() error { return s.syncVirtualMachineSpecAfterResourceUpdate(service.ServiceID) }))
	}
	if state.State.Fence != nil || state.State.Admission != nil {
		return vmDiskAPIError(vmrestore.ErrBusy)
	}
	for _, binding := range state.State.Bindings {
		if binding.DiskKey == name {
			return vmDiskAPIError(vmrestore.ErrConflict)
		}
	}
	var rows []dbmodel.ComponentK8sAttributes
	if err := consumer.database.Select("name").Where("component_id = ? AND tenant_id = ? AND name LIKE ? ESCAPE '='", service.ServiceID, service.TenantID, "vm=_restore=_receipt=_%").Find(&rows).Error; err != nil {
		return vmDiskAPIError(err)
	}
	matched := 0
	for _, row := range rows {
		operation := strings.TrimPrefix(row.Name, vmrestore.ReceiptPrefix)
		current, err := consumer.store.Read(consumer.scope, operation)
		if err != nil {
			return vmDiskAPIError(err)
		}
		receipt := current.Receipt
		if receipt == nil || !strings.HasPrefix(operation, "disk-delete-") || !receipt.Steps["complete"].Applied {
			continue
		}
		for key, binding := range receipt.OldBindings {
			if binding.DiskKey != name {
				continue
			}
			hash := vmVolumeFingerprint(consumer.scope, binding.VolumeID, name, "delete", "")
			if key != strconv.FormatUint(uint64(binding.VolumeID), 10) || receipt.ManifestHash != hash || receipt.Steps["complete"].RequestHash != hash || len(receipt.NewBindings) != 0 {
				return vmDiskAPIError(vmrestore.ErrConflict)
			}
			expected := vmVolumeResources(vmrestore.ResolvedVolume{ActivePVC: &binding.ActivePVC, ActiveDV: binding.ActiveDV})
			if !reflect.DeepEqual(expected, receipt.Resources) {
				return vmDiskAPIError(vmrestore.ErrConflict)
			}
			for _, ref := range expected {
				if ref.Kind == "PersistentVolumeClaim" {
					_, err = s.kubeClient.CoreV1().PersistentVolumeClaims(ref.Namespace).Get(context.Background(), ref.Name, metav1.GetOptions{})
				} else {
					_, err = vmHotplugDynamicClient().Resource(dataVolumeGVR).Namespace(ref.Namespace).Get(context.Background(), ref.Name, metav1.GetOptions{})
				}
				if !apierrors.IsNotFound(err) {
					if err != nil {
						return vmDiskAPIError(err)
					}
					return vmDiskAPIError(vmrestore.ErrConflict)
				}
			}
			matched++
		}
	}
	// A request without an explicit Region ID cannot choose between reused names.
	if matched == 1 {
		return nil
	}
	if matched > 1 {
		return vmDiskAPIError(vmrestore.ErrConflict)
	}
	return util.CreateAPIHandleError(404, original)
}

func nonPVCDisk(volume *dbmodel.TenantServiceVolume) bool {
	return volume.VolumeType == dbmodel.ConfigFileVolumeType.String() || volume.VolumeType == dbmodel.MemoryFSVolumeType.String() || volume.VolumeType == dbmodel.LocalVolumeType.String()
}
func vmDiskAPIError(err error) *util.APIHandleError {
	if err == nil {
		return nil
	}
	if handle, ok := err.(*util.APIHandleError); ok {
		return handle
	}
	mapped := vmVolumeError(err)
	if code, ok := mapped.(bcode.Coder); ok {
		return util.CreateAPIHandleError(code.GetStatus(), mapped)
	}
	return util.CreateAPIHandleError(500, mapped)
}

func vmConfigRequestHash(scope vmrestore.Scope, volume dbmodel.TenantServiceVolume, content string) string {
	volume.Model = dbmodel.Model{}
	util.SetVolumeDefaultValue(&volume)
	data, _ := json.Marshal(struct {
		Scope   vmrestore.Scope
		Volume  dbmodel.TenantServiceVolume
		Content string
	}{scope, volume, content})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (m *vmVolumeMutation) configCommitHash(tx *gorm.DB, name string) (string, error) {
	var volumes []dbmodel.TenantServiceVolume
	if err := tx.Where("service_id = ? AND volume_name = ?", m.scope.ServiceID, name).Find(&volumes).Error; err != nil {
		return "", err
	}
	if len(volumes) != 1 {
		return "", vmrestore.ErrConflict
	}
	var configs []dbmodel.TenantServiceConfigFile
	if err := tx.Where("service_id = ? AND volume_name = ?", m.scope.ServiceID, name).Find(&configs).Error; err != nil {
		return "", err
	}
	if len(configs) > 1 {
		return "", vmrestore.ErrConflict
	}
	content, configID := "", uint(0)
	if len(configs) == 1 {
		content, configID = configs[0].FileContent, configs[0].ID
	}
	if vmConfigRequestHash(m.scope, volumes[0], content) != m.hash {
		return "", vmrestore.ErrConflict
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d/%d/%s", volumes[0].ID, configID, m.hash)))
	return hex.EncodeToString(sum[:]), nil
}

// addVMConfigVolume records the SQL commit in the existing receipt, in the same
// transaction as the real volume/config DAOs. A retry resumes only its sync.
func (s *ServiceAction) addVMConfigVolume(service *dbmodel.TenantServices, volume *dbmodel.TenantServiceVolume, tenantID, content string) error {
	consumer, err := s.vmVolumes(service)
	if err != nil {
		return err
	}
	util.SetVolumeDefaultValue(volume)
	current, err := consumer.store.Read(consumer.scope, "")
	if err != nil {
		return err
	}
	if current.State.Fence != nil || len(current.State.Bindings) > 0 {
		return vmrestore.ErrBusy
	}
	m := &vmVolumeMutation{vmVolumeConsumer: consumer, action: "add-config:" + volume.VolumeName,
		hash: vmConfigRequestHash(consumer.scope, *volume, content)}
	if admission := current.State.Admission; admission != nil {
		if admission.Action != m.action || !admission.OutcomeUnknown {
			return vmrestore.ErrBusy
		}
		m.operation = admission.Token
		m.current, err = consumer.store.Read(consumer.scope, m.operation)
		if err != nil {
			return err
		}
		if m.current.State.Fence != nil || len(m.current.State.Bindings) > 0 {
			return vmrestore.ErrBusy
		}
		receipt := m.current.Receipt
		if receipt == nil || receipt.ManifestHash != m.hash || !receipt.Steps["prepare"].Applied ||
			receipt.Steps["prepare"].RequestHash != m.hash || receipt.Steps["prepare"].BindingGeneration != m.current.State.BindingGeneration || receipt.Steps["complete"].Applied {
			return vmrestore.ErrConflict
		}
		claimed, err := consumer.store.Mutate(consumer.scope, vmrestore.Expectation{ControlRevision: m.current.State.ControlRevision, BindingGeneration: m.current.State.BindingGeneration, AdmissionOwner: m.operation}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
			if c.State.Admission == nil || c.State.Admission.Token != m.operation || c.State.Admission.Action != m.action || !c.State.Admission.OutcomeUnknown {
				return vmrestore.Mutation{}, vmrestore.ErrBusy
			}
			c.State.Admission.OutcomeUnknown = false
			return vmrestore.Mutation{State: c.State}, nil
		})
		if err != nil {
			return err
		}
		m.current.State = claimed.State
	} else {
		owner := make([]byte, 16)
		if _, err := rand.Read(owner); err != nil {
			return err
		}
		m.operation = "config-add-" + hex.EncodeToString(owner)
		m.current, err = consumer.store.Mutate(consumer.scope, vmVolumeExpectation(current, m.operation, m.hash, "prepare"), func(c vmrestore.Current) (vmrestore.Mutation, error) {
			if len(c.State.Bindings) > 0 {
				return vmrestore.Mutation{}, vmrestore.ErrConflict
			}
			if err := c.State.AdmitMutation(m.operation, m.action); err != nil {
				return vmrestore.Mutation{}, err
			}
			receipt := vmrestore.NewReceipt(consumer.scope, m.operation, m.hash)
			return vmrestore.Mutation{State: c.State, Receipt: &receipt}, nil
		})
		if err != nil {
			return err
		}
	}
	m.volume.BindingGeneration = m.current.State.BindingGeneration
	m.ctx = vmrestore.WithAdmissionContext(context.Background(), m.operation)
	if err := m.check(); err != nil {
		return err
	}
	if m.current.Receipt.Steps["config-sql"].Applied {
		proof, err := m.configCommitHash(consumer.database, volume.VolumeName)
		if err != nil || proof != m.current.Receipt.Steps["config-sql"].RequestHash {
			m.pending()
			return vmrestore.ErrConflict
		}
		if err := s.syncVirtualMachineSpecAfterResourceUpdate(service.ServiceID); err != nil {
			m.pending()
			return fmt.Errorf("sync vm config-file to virtualmachine: %w", err)
		}
	} else {
		staged := false
		failure := s.volumnVarLegacy(volume, tenantID, content, "add", false, func(tx *gorm.DB) error {
			proof, err := m.configCommitHash(tx, volume.VolumeName)
			if err != nil {
				return err
			}
			expected := vmVolumeExpectation(m.current, m.operation, m.hash, "config-sql")
			expected.RequestHash = proof
			updated, err := consumer.store.MutateTx(tx, consumer.scope, expected, func(c vmrestore.Current) (vmrestore.Mutation, error) {
				if c.State.Admission == nil || c.State.Admission.Token != m.operation || c.State.Admission.Action != m.action || c.State.Fence != nil || len(c.State.Bindings) > 0 {
					return vmrestore.Mutation{}, vmrestore.ErrConflict
				}
				return vmrestore.Mutation{State: c.State, Receipt: c.Receipt}, nil
			})
			if err == nil {
				m.current, staged = updated, true
			}
			return err
		})
		if failure != nil {
			m.finish(!staged)
			return failure
		}
	}
	if err := m.step("complete", func(c *vmrestore.Current) error { return c.State.FinishMutation(m.operation, true) }); err != nil {
		m.pending()
		return err
	}
	return nil
}

// withLegacyVMConfig holds the shared admission while the unchanged non-PVC
// flow runs. Existing physical bindings make generic conversion unsafe.
func (s *ServiceAction) withLegacyVMConfig(service *dbmodel.TenantServices, action string, change func() error) error {
	consumer, err := s.vmVolumes(service)
	if err != nil {
		return err
	}
	current, err := consumer.store.Read(consumer.scope, "")
	if err != nil {
		return err
	}
	fingerprint := vmVolumeFingerprint(consumer.scope, 0, action, "config", "")
	owner := "config-" + fingerprint[:32]
	if current.State.Fence != nil || len(current.State.Bindings) > 0 {
		return &bcode.Code{Status: 409, Code: 409, Message: "虚拟机存在恢复卷映射或进行中的任务，当前配置文件操作需等待统一运行配置接线；配置已保留"}
	}
	if admission := current.State.Admission; admission != nil && (admission.Token != owner || admission.Action != action || !admission.OutcomeUnknown) {
		return vmrestore.ErrBusy
	}
	// Claim both a new action and an unknown-outcome retry atomically. Merely
	// sharing the deterministic token must not let concurrent callers execute.
	_, err = consumer.store.Mutate(consumer.scope, vmrestore.Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration, AdmissionOwner: owner}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		if len(c.State.Bindings) > 0 {
			return vmrestore.Mutation{}, vmrestore.ErrConflict
		}
		if admission := c.State.Admission; admission != nil {
			if admission.Token != owner || admission.Action != action || !admission.OutcomeUnknown {
				return vmrestore.Mutation{}, vmrestore.ErrBusy
			}
			admission.OutcomeUnknown = false
		} else if err := c.State.AdmitMutation(owner, action); err != nil {
			return vmrestore.Mutation{}, err
		}
		return vmrestore.Mutation{State: c.State}, nil
	})
	if err != nil {
		return err
	}
	outcome := change()
	latest, readErr := consumer.store.Read(consumer.scope, "")
	if readErr != nil {
		return readErr
	}
	_, finishErr := consumer.store.Mutate(consumer.scope, vmrestore.Expectation{ControlRevision: latest.State.ControlRevision, BindingGeneration: latest.State.BindingGeneration, AdmissionOwner: owner}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		if err := c.State.FinishMutation(owner, outcome == nil); err != nil {
			return vmrestore.Mutation{}, err
		}
		return vmrestore.Mutation{State: c.State}, nil
	})
	if outcome != nil {
		return outcome
	}
	return finishErr
}

// VolumnVar preserves generic non-VM/non-PVC behavior and routes physical VM
// deletion through the durable Region identity and mutation protocol.
func (s *ServiceAction) VolumnVar(tsv *dbmodel.TenantServiceVolume, tenantID, fileContent, action string) *util.APIHandleError {
	if action != "delete" {
		if action == "add" && nonPVCDisk(tsv) {
			service, err := s.getDBManager().TenantServiceDao().GetServiceByID(tsv.ServiceID)
			if err != nil {
				return util.CreateAPIHandleErrorFromDBError("get service", err)
			}
			if service != nil && service.IsVM() {
				if tsv.VolumeType == dbmodel.ConfigFileVolumeType.String() {
					return vmDiskAPIError(s.addVMConfigVolume(service, tsv, tenantID, fileContent))
				}
				return vmDiskAPIError(s.withLegacyVMConfig(service, "add-config:"+tsv.VolumeName, func() error {
					if failure := s.volumnVarLegacy(tsv, tenantID, fileContent, action, false); failure != nil {
						return failure
					}
					return nil
				}))
			}
		}
		return s.volumnVarLegacy(tsv, tenantID, fileContent, action, false)
	}
	manager := s.getDBManager()
	service, err := manager.TenantServiceDao().GetServiceByID(tsv.ServiceID)
	if err != nil {
		return util.CreateAPIHandleErrorFromDBError("get service", err)
	}
	if service == nil || !service.IsVM() {
		return s.volumnVarLegacy(tsv, tenantID, fileContent, action, false)
	}
	if tsv.VolumeName == "" {
		return util.CreateAPIHandleError(400, fmt.Errorf("虚拟机删除磁盘必须指定 volume_name"))
	}
	volume, err := manager.TenantServiceVolumeDao().GetVolumeByServiceIDAndName(tsv.ServiceID, tsv.VolumeName)
	if err != nil {
		return s.replayVMVolumeDelete(service, tsv.VolumeName, err)
	}
	if nonPVCDisk(volume) {
		return vmDiskAPIError(s.withLegacyVMConfig(service, "delete-config:"+volume.VolumeName, func() error {
			if err := s.volumnVarLegacy(tsv, tenantID, fileContent, action, true); err != nil {
				return err
			}
			return nil
		}))
	}
	return vmDiskAPIError(s.removeVMDataDisk(service, volume))
}

// UpdVolume avoids generic VM reconstruction for capacity-only requests until
// normal-runtime conversion consumes the same binding generation.
func (s *ServiceAction) UpdVolume(sid string, req *apimodel.UpdVolumeReq) error {
	service, err := s.getDBManager().TenantServiceDao().GetServiceByID(sid)
	if err != nil {
		return err
	}
	if service == nil || !service.IsVM() {
		return s.updVolumeLegacy(sid, req)
	}
	volume, err := s.getDBManager().TenantServiceVolumeDao().GetVolumeByServiceIDAndName(sid, req.VolumeName)
	if err != nil {
		return err
	}
	if nonPVCDisk(volume) {
		return vmVolumeError(s.withLegacyVMConfig(service, "update-config:"+volume.VolumeName, func() error { return s.updVolumeLegacy(sid, req) }))
	}
	return vmVolumeError(s.updateVMVolume(service, req))
}

func vmObservedGi(bytes int64) int64 {
	if bytes <= 0 {
		return 0
	}
	return (bytes + gibibyte - 1) / gibibyte
}

// addVMSourceRelation serializes only the existing DB relation write with the
// source VM's admission. It does not reconstruct workloads or physical bindings.
func (s *ServiceAction) addVMSourceRelation(relation *dbmodel.TenantServiceMountRelation) (bool, *util.APIHandleError) {
	service, err := s.getDBManager().TenantServiceDao().GetServiceByID(relation.DependServiceID)
	if err != nil {
		return true, util.CreateAPIHandleErrorFromDBError("get source service", err)
	}
	if service == nil || !service.IsVM() {
		return false, nil
	}
	volume, err := s.getDBManager().TenantServiceVolumeDao().GetVolumeByServiceIDAndName(service.ServiceID, relation.VolumeName)
	if err != nil {
		return true, util.CreateAPIHandleErrorFromDBError("get source volume", err)
	}
	consumer, err := s.vmVolumes(service)
	if err != nil {
		return true, vmDiskAPIError(err)
	}
	current, err := consumer.store.Read(consumer.scope, "")
	if err != nil {
		return true, vmDiskAPIError(err)
	}
	transaction := consumer.database.Begin()
	defer transaction.Rollback()
	if transaction.Error != nil {
		return true, vmDiskAPIError(transaction.Error)
	}
	ownerBytes := make([]byte, 16)
	if _, err := rand.Read(ownerBytes); err != nil {
		return true, vmDiskAPIError(err)
	}
	owner := "share-" + hex.EncodeToString(ownerBytes)
	_, err = consumer.store.MutateTx(transaction, consumer.scope, vmrestore.Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		if err := c.State.AdmitMutation(owner, "share-source"); err != nil {
			return vmrestore.Mutation{}, err
		}
		relation.HostPath = volume.HostPath
		if err := s.getDBManager().TenantServiceMountRelationDaoTransactions(transaction).AddModel(relation); err != nil {
			return vmrestore.Mutation{}, err
		}
		if err := c.State.FinishMutation(owner, true); err != nil {
			return vmrestore.Mutation{}, err
		}
		return vmrestore.Mutation{State: c.State}, nil
	})
	if err != nil {
		return true, vmDiskAPIError(err)
	}
	return true, vmDiskAPIError(transaction.Commit().Error)
}
