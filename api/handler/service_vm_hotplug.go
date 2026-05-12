package handler

import (
	"context"
	"fmt"

	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/component/k8s"
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
	if volume.VolumeType != dbmodel.VMVolumeType.String() || volume.VolumeName == "disk" {
		return nil
	}

	service, err := db.GetManager().TenantServiceDao().GetServiceByID(volume.ServiceID)
	if err != nil || service == nil || !service.IsVM() {
		return err
	}
	vm, err := s.getVirtualMachineByServiceID(volume.ServiceID)
	if err != nil || vm == nil {
		return err
	}
	if vm.Status.PrintableStatus != v1.VirtualMachineStatusRunning {
		return s.syncVirtualMachineSpecForService(volume.ServiceID)
	}
	backingName := fmt.Sprintf("manual%d", volume.ID)
	if err := ensureVMHotplugDataVolume(vm.Namespace, tenantID, volume, backingName); err != nil {
		return err
	}

	opts := &v1.AddVolumeOptions{
		Name: backingName,
		Disk: &v1.Disk{
			Serial: backingName,
			DiskDevice: v1.DiskDevice{
				Disk: &v1.DiskTarget{
					Bus: v1.DiskBusSATA,
				},
			},
		},
		VolumeSource: &v1.HotplugVolumeSource{
			DataVolume: &v1.DataVolumeSource{
				Name: backingName,
			},
		},
	}
	return s.kubevirtClient.VirtualMachine(vm.Namespace).AddVolume(context.Background(), vm.Name, opts)
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
	accessMode := volume.AccessMode
	if accessMode == "" {
		accessMode = "ReadWriteMany"
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
					"accessModes": []any{accessMode},
					"resources": map[string]any{
						"requests": map[string]any{
							"storage": storageQty.String(),
						},
					},
					"storageClassName": volume.VolumeProviderName,
					"volumeMode":       string(corev1.PersistentVolumeFilesystem),
				},
			},
		},
	}
	_, err := resourceIf.Create(context.Background(), obj, metav1.CreateOptions{})
	return err
}
