package handler

import (
	"context"
	"fmt"

	dbmodel "github.com/goodrain/rainbond/db/model"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/version"
	v1 "kubevirt.io/api/core/v1"
)

const vmExpansionGuestMessage = "PVC容量状态；来宾块设备容量、分区和文件系统扩容仍需确认"

func (s *ServiceAction) vmDiskExpansionCapability(ctx context.Context) (string, error) {
	if s.kubevirtClient == nil {
		return "KubeVirt capability client is unavailable", nil
	}
	list, err := s.kubevirtClient.KubeVirt("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("read KubeVirt disk expansion capability: %w", err)
	}
	if len(list.Items) != 1 {
		return "KubeVirt configuration could not be uniquely identified", nil
	}
	kv := list.Items[0]
	if kv.Status.Phase != v1.KubeVirtPhaseDeployed {
		return "KubeVirt is not fully deployed", nil
	}
	// Observed version is the running capability; target version may still be rolling out.
	observed, parseErr := version.ParseSemantic(kv.Status.ObservedKubeVirtVersion)
	if parseErr == nil && observed.AtLeast(version.MustParseSemantic("v1.8.0")) {
		return "", nil
	}
	if config := kv.Spec.Configuration.DeveloperConfiguration; config != nil {
		for _, gate := range config.FeatureGates {
			if gate == "ExpandDisks" {
				return "", nil
			}
		}
	}
	return "KubeVirt requires observed version 1.8 or newer, or the ExpandDisks feature gate", nil
}

func (s *ServiceAction) inspectVMExpansionClaims(ctx context.Context, service *dbmodel.TenantServices, volume *dbmodel.TenantServiceVolume, namespace string) ([]*corev1.PersistentVolumeClaim, string, error) {
	if reason, err := s.vmDiskExpansionCapability(ctx); reason != "" || err != nil {
		return nil, reason, err
	}
	relations, err := s.getDBManager().TenantServiceMountRelationDao().GetTenantServiceMountRelationsByDepServiceAndVolumeName(service.ServiceID, volume.VolumeName)
	if err != nil {
		return nil, "", fmt.Errorf("inspect disk shared mounts: %w", err)
	}
	if len(relations) > 0 {
		return nil, "组件存储已被共享挂载，不允许扩容", nil
	}
	client := vmHotplugDynamicClient()
	if client == nil {
		return nil, "", fmt.Errorf("dynamic Kubernetes client is required to identify VM disk")
	}
	vm, err := s.getVirtualMachineByServiceID(service.ServiceID)
	if err != nil {
		return nil, "", err
	}
	backingName := vmHotplugBackingName(volume)
	claimName := backingName
	dataVolumeName := backingName
	if vm != nil {
		if vm.Namespace != namespace {
			return nil, "虚拟机与组件命名空间不一致，不允许扩容", nil
		}
		if vm.UID == "" || vm.Labels["service_id"] != service.ServiceID {
			return nil, "虚拟机归属无法确认，不允许扩容", nil
		}
		if vm.Spec.Template == nil {
			return nil, "虚拟机存储模板不可用，不允许扩容", nil
		}
	}
	if vm != nil && vm.Spec.Template != nil {
		deviceNames := map[string]bool{}
		matches := 0
		for _, v := range vm.Spec.Template.Spec.Volumes {
			if v.Name != backingName && !vmDiskVolumeUses(v, backingName) {
				continue
			}
			matches++
			deviceNames[v.Name] = true
			switch {
			case v.DataVolume != nil:
				dataVolumeName = v.DataVolume.Name
				claimName = dataVolumeName
			case v.PersistentVolumeClaim != nil:
				dataVolumeName = ""
				claimName = v.PersistentVolumeClaim.ClaimName
			default:
				return nil, "数据盘未引用可扩容的PVC", nil
			}
		}
		if matches != 1 {
			return nil, "数据盘与当前虚拟机存储映射无法唯一确认", nil
		}
		for _, disk := range vm.Spec.Template.Spec.Domain.Devices.Disks {
			if !deviceNames[disk.Name] {
				continue
			}
			if disk.BootOrder != nil && *disk.BootOrder == 1 {
				return nil, "当前盘是虚拟机启动根盘，此操作仅支持数据盘扩容", nil
			}
			if disk.CDRom != nil {
				return nil, "当前盘是CDROM，不支持扩容", nil
			}
		}

	}
	var dv *unstructured.Unstructured
	if dataVolumeName != "" {
		dv, err = client.Resource(dataVolumeGVR).Namespace(namespace).Get(ctx, dataVolumeName, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, "", fmt.Errorf("read VM disk DataVolume: %w", err)
		}
		if dv != nil {
			if dv.GetUID() == "" || dv.GetLabels()["service_id"] != service.ServiceID {
				return nil, "数据盘DataVolume归属无法确认，不允许扩容", nil
			}
			if current, _, _ := unstructured.NestedString(dv.Object, "status", "claimName"); current != "" {
				claimName = current
			}
		}
	}
	pvc, err := s.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, claimName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, "数据盘尚未绑定PVC，请等待存储准备完成后重试", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("read current VM disk PVC: %w", err)
	}
	owned := pvc.Labels["service_id"] == service.ServiceID
	for _, owner := range pvc.OwnerReferences {
		if dv != nil && owner.Kind == "DataVolume" && owner.Name == dataVolumeName && owner.UID == dv.GetUID() {
			owned = true
		}
	}
	if pvc.UID == "" || !owned || pvc.DeletionTimestamp != nil {
		return nil, "数据盘PVC归属无法确认或正在删除，不允许扩容", nil
	}
	for _, name := range []string{claimName, dataVolumeName} {
		if name == "" {
			continue
		}
		if err := s.checkVMDataDiskReferences(client, namespace, name, vm, true); err != nil {
			if apierrors.IsConflict(err) {
				return nil, err.Error(), nil
			}
			return nil, "", err
		}
	}
	instances, err := client.Resource(vmDiskVMIGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("inspect VM disk consumers: %w", err)
	}
	ownedInstances := map[types.UID]bool{}
	for _, obj := range instances.Items {
		if vmDiskOwnedInstance(&obj, vm, service.ServiceID) {
			ownedInstances[obj.GetUID()] = true
		}
	}
	pods, err := s.kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("inspect VM disk pod consumers: %w", err)
	}
	for _, pod := range pods.Items {
		for _, v := range pod.Spec.Volumes {
			if v.PersistentVolumeClaim == nil || v.PersistentVolumeClaim.ClaimName != claimName {
				continue
			}
			ownPod := false
			for _, owner := range pod.OwnerReferences {
				if owner.Kind == "VirtualMachineInstance" && ownedInstances[owner.UID] {
					ownPod = true
				}
			}
			if !ownPod {
				return nil, "数据盘被其他工作负载引用，不允许扩容", nil
			}
		}
	}
	return []*corev1.PersistentVolumeClaim{pvc}, "", nil
}
