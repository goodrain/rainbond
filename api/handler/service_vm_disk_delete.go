package handler

import (
	"context"
	"encoding/json"
	"fmt"

	dbmodel "github.com/goodrain/rainbond/db/model"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	v1 "kubevirt.io/api/core/v1"
)

var vmDiskVMGVR = schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachines"}
var vmDiskVMIGVR = schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachineinstances"}

func vmDiskDeleteConflict(name, message string) error {
	return apierrors.NewConflict(dataVolumeGVR.GroupResource(), name, fmt.Errorf("%s", message))
}

// removeVMDataDisk completes reclaim before its database record can be removed.
// A pending hot-unplug or Kubernetes finalizer is a retryable conflict, not success.
func (s *ServiceAction) removeVMDataDisk(service *dbmodel.TenantServices, volume *dbmodel.TenantServiceVolume) error {
	name := vmHotplugBackingName(volume)
	if volume.ID == 0 || volume.VolumeName == "disk" {
		return vmDiskDeleteConflict(name, "系统根盘不能通过数据盘删除操作移除")
	}
	client := vmHotplugDynamicClient()
	if client == nil || s.kubeClient == nil {
		return fmt.Errorf("Kubernetes clients are required to reclaim VM disks")
	}
	vm, err := s.getVirtualMachineByServiceID(volume.ServiceID)
	if err != nil {
		return err
	}
	namespace, err := s.resolveVolumeExpansionNamespace(service)
	if err != nil {
		return err
	}
	if vm != nil {
		if vm.Namespace != namespace {
			return vmDiskDeleteConflict(name, "虚拟机与组件命名空间不一致，原配置、DataVolume和PVC均保留；请核对归属后重试删除")
		}
		if vm.Spec.Template != nil {
			for _, v := range vm.Spec.Template.Spec.Volumes {
				if v.Name == name && !vmDiskVolumeUses(v, name) {
					return vmDiskDeleteConflict(name, "当前磁盘映射已变更，原配置、DataVolume和PVC均保留；请先核验恢复卷后再删除")
				}
			}
		}
		if vm.Spec.Template != nil {
			for _, disk := range vm.Spec.Template.Spec.Domain.Devices.Disks {
				targetDisk := false
				for _, v := range vm.Spec.Template.Spec.Volumes {
					if v.Name == disk.Name && vmDiskVolumeUses(v, name) {
						targetDisk = true
					}
				}
				if targetDisk && disk.BootOrder != nil && *disk.BootOrder == 1 {
					return vmDiskDeleteConflict(name, "系统根盘不能通过数据盘删除操作移除")
				}
			}
		}
	}
	if namespace == "" {
		return fmt.Errorf("component namespace is unavailable for disk reclamation")
	}
	identity, err := s.checkVMDataDiskOwnership(client, namespace, volume.ServiceID, name)
	if err != nil {
		return err
	}
	// Other workloads, pending snapshots and clones must be checked before detaching.
	if err := s.checkVMDataDiskReferences(client, namespace, name, vm, true); err != nil {
		return err
	}
	instances, err := client.Resource(vmDiskVMIGVR).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("inspect VM disk instances: %w", err)
	}
	for _, obj := range instances.Items {
		var instance v1.VirtualMachineInstance
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &instance); err != nil {
			return err
		}
		for _, v := range instance.Spec.Volumes {
			if !vmDiskVolumeUses(v, name) {
				continue
			}
			if !vmDiskOwnedInstance(&obj, vm, service.ServiceID) {
				return vmDiskDeleteConflict(name, "磁盘仍被其他虚拟机引用，不能删除")
			}
			hotpluggable := v.DataVolume != nil && v.DataVolume.Hotpluggable || v.PersistentVolumeClaim != nil && v.PersistentVolumeClaim.Hotpluggable
			if !hotpluggable {
				return vmDiskDeleteConflict(name, "请先关闭虚拟机再删除该磁盘")
			}
			if s.kubevirtClient == nil {
				return fmt.Errorf("KubeVirt client is required to detach VM disk")
			}
			if err := s.kubevirtClient.VirtualMachine(namespace).RemoveVolume(context.Background(), vm.Name, buildVMHotplugRemoveVolumeOptions(name)); err != nil {
				return err
			}
			return vmDiskDeleteConflict(name, "磁盘正在卸载，请稍后重试删除；存储尚未回收")
		}
	}
	if vm != nil && vm.Spec.Template != nil {
		updated := vm.DeepCopy()
		targetNames := map[string]bool{}
		for _, v := range vm.Spec.Template.Spec.Volumes {
			if vmDiskVolumeUses(v, name) {
				targetNames[v.Name] = true
			}
		}
		volumes := updated.Spec.Template.Spec.Volumes[:0]
		for _, v := range updated.Spec.Template.Spec.Volumes {
			if !vmDiskVolumeUses(v, name) {
				volumes = append(volumes, v)
			}
		}
		disks := updated.Spec.Template.Spec.Domain.Devices.Disks[:0]
		for _, d := range updated.Spec.Template.Spec.Domain.Devices.Disks {
			if !targetNames[d.Name] {
				disks = append(disks, d)
			}
		}
		templates := updated.Spec.DataVolumeTemplates[:0]
		for _, dv := range updated.Spec.DataVolumeTemplates {
			if dv.Name != name {
				templates = append(templates, dv)
			}
		}
		changed := len(volumes) != len(vm.Spec.Template.Spec.Volumes) || len(templates) != len(vm.Spec.DataVolumeTemplates) || len(disks) != len(vm.Spec.Template.Spec.Domain.Devices.Disks)
		if changed {
			if vm.UID == "" || vm.ResourceVersion == "" {
				return vmDiskDeleteConflict(name, "虚拟机标识不完整，请刷新后重试")
			}
			updated.Spec.Template.Spec.Volumes = volumes
			updated.Spec.Template.Spec.Domain.Devices.Disks = disks
			updated.Spec.DataVolumeTemplates = templates
			raw, err := json.Marshal(updated)
			if err != nil {
				return err
			}
			obj := &unstructured.Unstructured{}
			if err := obj.UnmarshalJSON(raw); err != nil {
				return err
			}
			if _, err = client.Resource(vmDiskVMGVR).Namespace(namespace).Update(context.Background(), obj, metav1.UpdateOptions{}); err != nil {
				return err
			}
		}
	}
	if err := s.checkVMDataDiskReferences(client, namespace, name, nil, false); err != nil {
		return err
	}
	return s.reclaimVMDataDisk(client, namespace, volume.ServiceID, name, identity)
}

func vmDiskOwnedInstance(obj *unstructured.Unstructured, vm *v1.VirtualMachine, serviceID string) bool {
	if vm == nil || vm.UID == "" || obj.GetName() != vm.Name || obj.GetLabels()["service_id"] != serviceID || serviceID == "" {
		return false
	}
	for _, owner := range obj.GetOwnerReferences() {
		if owner.Kind == "VirtualMachine" && owner.Name == vm.Name && owner.UID == vm.UID {
			return true
		}
	}
	return false
}

func vmDiskVolumeUses(v v1.Volume, name string) bool {
	return v.DataVolume != nil && v.DataVolume.Name == name || v.PersistentVolumeClaim != nil && v.PersistentVolumeClaim.ClaimName == name
}

type vmDiskStorageIdentity struct {
	dataVolumeUID types.UID
	pvcUID        types.UID
}

func (s *ServiceAction) checkVMDataDiskOwnership(client dynamic.Interface, namespace, serviceID, name string) (vmDiskStorageIdentity, error) {
	identity := vmDiskStorageIdentity{}
	dv, err := client.Resource(dataVolumeGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return identity, err
	}
	if dv != nil && (dv.GetUID() == "" || dv.GetLabels()["service_id"] != serviceID) {
		return identity, vmDiskDeleteConflict(name, "磁盘资源归属无法确认，禁止删除")
	}
	if dv != nil {
		claimName, _, mappingErr := unstructured.NestedString(dv.Object, "status", "claimName")
		if mappingErr != nil {
			return identity, fmt.Errorf("read VM disk claim mapping: %w", mappingErr)
		}
		if claimName != "" && claimName != name {
			return identity, vmDiskDeleteConflict(name, "当前磁盘映射已变更，原配置、DataVolume和PVC均保留；请先核验恢复卷后再删除")
		}
		identity.dataVolumeUID = dv.GetUID()
	}
	pvc, err := s.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return identity, err
	}
	if pvc != nil {
		owned := pvc.Labels["service_id"] == serviceID
		for _, owner := range pvc.OwnerReferences {
			if dv != nil && owner.Kind == "DataVolume" && owner.Name == name && owner.UID == dv.GetUID() {
				owned = true
			}
		}
		if pvc.UID == "" || !owned {
			return identity, vmDiskDeleteConflict(name, "磁盘资源归属无法确认，禁止删除")
		}
	}
	if pvc != nil {
		identity.pvcUID = pvc.UID
	}
	return identity, nil
}

func (s *ServiceAction) checkVMDataDiskReferences(client dynamic.Interface, namespace, name string, ownVM *v1.VirtualMachine, allowOwnVM bool) error {
	resources := []schema.GroupVersionResource{vmDiskVMGVR, vmDiskVMIGVR,
		{Group: "snapshot.kubevirt.io", Version: "v1beta1", Resource: "virtualmachinesnapshots"},
		{Group: "snapshot.kubevirt.io", Version: "v1beta1", Resource: "virtualmachinesnapshotcontents"},
		{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshots"}, dataVolumeGVR,
		{Group: "cdi.kubevirt.io", Version: "v1beta1", Resource: "datasources"}}
	needsDataSourceLookup := false
	for _, gvr := range resources {
		listNamespace := namespace
		if gvr == dataVolumeGVR || gvr.Resource == "datasources" {
			listNamespace = ""
		}
		list, err := client.Resource(gvr).Namespace(listNamespace).List(context.Background(), metav1.ListOptions{})
		if apierrors.IsNotFound(err) && (gvr.Group == "snapshot.kubevirt.io" || gvr.Group == "snapshot.storage.k8s.io") {
			continue // Optional snapshot CRDs may not be installed.
		}
		if apierrors.IsNotFound(err) && gvr.Resource == "datasources" && !needsDataSourceLookup {
			continue
		}

		if err != nil {
			return fmt.Errorf("inspect disk references in %s: %w", gvr.Resource, err)
		}
		for _, obj := range list.Items {
			if gvr == dataVolumeGVR && obj.GetName() == name && obj.GetNamespace() == namespace {
				continue
			}
			if gvr == vmDiskVMGVR {
				pending, _, _ := unstructured.NestedSlice(obj.Object, "status", "volumeRequests")
				if vmDiskObjectReferences(pending, name) {
					return vmDiskDeleteConflict(name, "磁盘仍有待处理的挂载请求，不能删除或扩容")
				}
			}
			if gvr.Resource == "datasources" {
				for _, state := range []string{"spec", "status"} {
					sourceName, _, _ := unstructured.NestedString(obj.Object, state, "source", "pvc", "name")
					sourceNamespace, _, _ := unstructured.NestedString(obj.Object, state, "source", "pvc", "namespace")
					if sourceNamespace == "" {
						sourceNamespace = obj.GetNamespace()
					}
					if sourceName == name && sourceNamespace == namespace {
						return vmDiskDeleteConflict(name, "磁盘仍被数据源或克隆引用，不能删除或扩容")
					}
				}
				continue
			}

			if allowOwnVM && ownVM != nil && (gvr == vmDiskVMGVR && obj.GetUID() == ownVM.UID || gvr == vmDiskVMIGVR && vmDiskOwnedInstance(&obj, ownVM, ownVM.Labels["service_id"])) {
				continue
			}
			if gvr == dataVolumeGVR {
				kind, _, _ := unstructured.NestedString(obj.Object, "spec", "sourceRef", "kind")
				if kind == "DataSource" {
					needsDataSourceLookup = true
				}

				sourceName, _, _ := unstructured.NestedString(obj.Object, "spec", "source", "pvc", "name")
				sourceNamespace, _, _ := unstructured.NestedString(obj.Object, "spec", "source", "pvc", "namespace")
				if sourceNamespace == "" {
					sourceNamespace = obj.GetNamespace()
				}
				if sourceName == name && sourceNamespace == namespace {
					return vmDiskDeleteConflict(name, "磁盘仍被克隆引用，不能删除")
				}
				continue
			}
			// A snapshot may not have created its content yet; protect its source VM.
			source, _, _ := unstructured.NestedString(obj.Object, "spec", "source", "name")
			if (gvr.Resource == "virtualmachinesnapshots" && ownVM != nil && source == ownVM.Name) || vmDiskObjectReferences(obj.Object["spec"], name) {
				return vmDiskDeleteConflict(name, "磁盘仍被虚拟机、快照或克隆引用，不能删除")
			}
		}
	}
	if allowOwnVM {
		return nil
	}
	pods, err := s.kubeClient.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("inspect disk pod references: %w", err)
	}
	for _, pod := range pods.Items {
		for _, v := range pod.Spec.Volumes {
			if v.PersistentVolumeClaim != nil && v.PersistentVolumeClaim.ClaimName == name {
				return vmDiskDeleteConflict(name, "磁盘仍被工作负载引用，不能删除")
			}
		}
	}
	return nil
}

// Follow the reference fields used by VM, CDI clones and both snapshot APIs.
func vmDiskObjectReferences(value interface{}, name string) bool {
	switch v := value.(type) {
	case map[string]interface{}:
		for key, item := range v {
			if (key == "claimName" || key == "persistentVolumeClaimName") && item == name {
				return true
			}
			if key == "dataVolume" || key == "persistentVolumeClaim" || key == "pvc" {
				if ref, ok := item.(map[string]interface{}); ok {
					if ref["name"] == name {
						return true
					}
					if meta, ok := ref["metadata"].(map[string]interface{}); ok && meta["name"] == name {
						return true
					}
				}
			}
			if vmDiskObjectReferences(item, name) {
				return true
			}
		}
	case []interface{}:
		for _, item := range v {
			if vmDiskObjectReferences(item, name) {
				return true
			}
		}
	}
	return false
}

func (s *ServiceAction) reclaimVMDataDisk(client dynamic.Interface, namespace, serviceID, name string, identity vmDiskStorageIdentity) error {
	dv, err := client.Resource(dataVolumeGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if dv != nil && dv.GetUID() != identity.dataVolumeUID {
		return vmDiskDeleteConflict(name, "磁盘DataVolume标识已变更，请刷新后重试")
	}

	if dv != nil && (dv.GetUID() == "" || dv.GetLabels()["service_id"] != serviceID) {
		return vmDiskDeleteConflict(name, "磁盘资源归属无法确认，禁止删除")
	}
	if dv != nil {
		claimName, _, mappingErr := unstructured.NestedString(dv.Object, "status", "claimName")
		if mappingErr != nil {
			return fmt.Errorf("read VM disk claim mapping: %w", mappingErr)
		}
		if claimName != "" && claimName != name {
			return vmDiskDeleteConflict(name, "当前磁盘映射已变更，DataVolume和PVC均保留；请刷新并核验恢复卷后再删除")
		}
	}

	pvc, err := s.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if pvc != nil && pvc.UID != identity.pvcUID {
		return vmDiskDeleteConflict(name, "磁盘PVC标识已变更，请刷新后重试")
	}

	if pvc != nil {
		owned := pvc.Labels["service_id"] == serviceID
		for _, owner := range pvc.OwnerReferences {
			if dv != nil && owner.Kind == "DataVolume" && owner.Name == name && owner.UID == dv.GetUID() {
				owned = true
			}
		}
		if pvc.UID == "" || !owned {
			return vmDiskDeleteConflict(name, "磁盘资源归属无法确认，禁止删除")
		}
		// Reclaim the PVC first, keeping its owned DataVolume for retries while finalizers run.
		uid := pvc.UID
		if err := s.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Delete(context.Background(), name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		if _, err := s.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), name, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
			if err != nil {
				return err
			}
			return vmDiskDeleteConflict(name, "磁盘存储正在回收，请稍后重试删除")
		}
	}
	if dv != nil {
		uid := dv.GetUID()
		if err := client.Resource(dataVolumeGVR).Namespace(namespace).Delete(context.Background(), name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	if _, err := client.Resource(dataVolumeGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		if err != nil {
			return err
		}
		return vmDiskDeleteConflict(name, "磁盘存储正在回收，请稍后重试删除")
	}
	if _, err := s.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), name, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		if err != nil {
			return err
		}
		return vmDiskDeleteConflict(name, "磁盘存储正在回收，请稍后重试删除")
	}
	return nil
}
