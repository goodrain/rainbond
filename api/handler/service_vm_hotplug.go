package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	appmvolume "github.com/goodrain/rainbond/worker/appm/volume"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	v1 "kubevirt.io/api/core/v1"
)

var dataVolumeGVR = schema.GroupVersionResource{
	Group:    "cdi.kubevirt.io",
	Version:  "v1beta1",
	Resource: "datavolumes",
}

func (s *ServiceAction) hotplugVMDataDisk(tenantID string, volume *dbmodel.TenantServiceVolume) error {
	if s != nil && s.hotplugVMDataDiskHook != nil {
		return s.hotplugVMDataDiskHook(tenantID, volume)
	}
	if volume == nil {
		return nil
	}
	if volume.VolumeName == "disk" || volume.VolumeType == dbmodel.ConfigFileVolumeType.String() {
		return nil
	}

	service, err := db.GetManager().TenantServiceDao().GetServiceByID(volume.ServiceID)
	if err != nil || service == nil || !service.IsVM() {
		return err
	}
	if resolveVMHotplugDeviceType(volume.VolumePath) != "disk" {
		return s.syncVirtualMachineSpecAfterResourceUpdate(volume.ServiceID)
	}
	vm, err := s.getVirtualMachineByServiceID(volume.ServiceID)
	if err != nil || vm == nil {
		return err
	}
	if vm.Status.PrintableStatus != v1.VirtualMachineStatusRunning {
		return s.syncVirtualMachineSpecAfterResourceUpdate(volume.ServiceID)
	}
	backingName := fmt.Sprintf("manual%d", volume.ID)
	if err := ensureVMHotplugDataVolume(vm.Namespace, tenantID, volume, backingName); err != nil {
		return err
	}

	opts := buildVMHotplugAddVolumeOptions(backingName, volume.VolumePath)
	return s.kubevirtClient.VirtualMachine(vm.Namespace).AddVolume(context.Background(), vm.Name, opts)
}

func buildVMHotplugAddVolumeOptions(backingName, volumePath string) *v1.AddVolumeOptions {
	return &v1.AddVolumeOptions{
		Name: backingName,
		Disk: &v1.Disk{
			Serial: backingName,
			DiskDevice: v1.DiskDevice{
				Disk: &v1.DiskTarget{
					Bus: appmvolume.VMVolumeDiskBus(volumePath),
				},
			},
		},
		VolumeSource: &v1.HotplugVolumeSource{
			DataVolume: &v1.DataVolumeSource{
				Name: backingName,
			},
		},
	}
}

func ensureVMHotplugDataVolume(namespace, tenantID string, volume *dbmodel.TenantServiceVolume, backingName string) error {
	dynamicClient := k8s.Default().DynamicClient
	if dynamicClient == nil {
		return fmt.Errorf("dynamic client is required for vm hotplug volume")
	}
	resourceIf := dynamicClient.Resource(dataVolumeGVR).Namespace(namespace)
	if _, err := resourceIf.Get(context.Background(), backingName, metav1.GetOptions{}); err == nil {
		return nil
	}

	storageQty := resource.NewScaledQuantity(volume.VolumeCapacity, resource.Mega)
	accessModes := resolveVMHotplugAccessModes(volume)
	accessModeValues := make([]any, 0, len(accessModes))
	for _, mode := range accessModes {
		accessModeValues = append(accessModeValues, string(mode))
	}

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "cdi.kubevirt.io/v1beta1",
			"kind":       "DataVolume",
			"metadata": map[string]any{
				"name":      backingName,
				"namespace": namespace,
				"labels": map[string]any{
					"service_id":  volume.ServiceID,
					"tenant_id":   tenantID,
					"volume_name": backingName,
					"stateless":   "",
				},
				"annotations": map[string]any{
					"volume_name": volume.VolumeName,
				},
			},
			"spec": map[string]any{
				"source": map[string]any{
					"blank": map[string]any{},
				},
				"storage": map[string]any{
					"accessModes": accessModeValues,
					"resources": map[string]any{
						"requests": map[string]any{
							"storage": storageQty.String(),
						},
					},
					"storageClassName": resolveVMHotplugStorageClassName(volume),
					"volumeMode":       string(corev1.PersistentVolumeFilesystem),
				},
			},
		},
	}
	_, err := resourceIf.Create(context.Background(), obj, metav1.CreateOptions{})
	return err
}

func resolveVMHotplugAccessModes(volume *dbmodel.TenantServiceVolume) []corev1.PersistentVolumeAccessMode {
	defaultModes := []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
	if volume == nil {
		return defaultModes
	}

	seen := make(map[corev1.PersistentVolumeAccessMode]struct{})
	modes := make([]corev1.PersistentVolumeAccessMode, 0, 1)
	for _, item := range strings.Split(volume.AccessMode, ",") {
		mode, ok := normalizeVMHotplugAccessMode(item)
		if !ok {
			continue
		}
		if _, exists := seen[mode]; exists {
			continue
		}
		seen[mode] = struct{}{}
		modes = append(modes, mode)
	}
	if len(modes) == 0 {
		return defaultModes
	}
	return modes
}

func normalizeVMHotplugAccessMode(mode string) (corev1.PersistentVolumeAccessMode, bool) {
	switch strings.ToUpper(strings.TrimSpace(mode)) {
	case "RWO", "READWRITEONCE":
		return corev1.ReadWriteOnce, true
	case "ROX", "READONLYMANY":
		return corev1.ReadOnlyMany, true
	case "RWX", "READWRITEMANY":
		return corev1.ReadWriteMany, true
	case "READWRITEONCEPOD":
		return corev1.ReadWriteOncePod, true
	default:
		return "", false
	}
}

func resolveVMHotplugDeviceType(volumePath string) string {
	return appmvolume.VMVolumeDeviceType(volumePath)
}

func resolveVMHotplugStorageClassName(volume *dbmodel.TenantServiceVolume) string {
	if volume == nil || volume.VolumeType == "" || volume.VolumeType == dbmodel.VMVolumeType.String() {
		return "local-path"
	}
	return volume.VolumeType
}
