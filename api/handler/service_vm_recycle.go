package handler

import (
	"context"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kv "kubevirt.io/api/core/v1"
)

// VMRecycleRecovery is a read-only proof that recovery can retain original disks.
type VMRecycleRecovery struct {
	ServiceID        string          `json:"service_id"`
	VMUID            string          `json:"vm_uid"`
	VMName           string          `json:"vm_name"`
	K8sComponentName string          `json:"k8s_component_name"`
	StorageReady     bool            `json:"storage_ready"`
	Disks            []VMRecycleDisk `json:"disks"`
}

type VMRecycleDisk struct {
	DiskKey string `json:"disk_key"`
	PVCUID  string `json:"pvc_uid"`
}

func (s *ServiceAction) GetVMRecycleRecovery(ctx context.Context, serviceID string) (*VMRecycleRecovery, error) {
	scope, err := s.snapshotScope(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	service, err := s.getDBManager().TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		return nil, err
	}
	consumer, err := s.vmVolumes(service)
	if err != nil {
		return nil, err
	}
	current, err := consumer.store.Read(scope, "")
	if err != nil {
		return nil, err
	}
	if current.State.Fence != nil || current.State.Admission != nil {
		return nil, vmrestore.ErrBusy
	}
	vm, err := s.getVirtualMachineByServiceID(serviceID)
	if err != nil {
		return nil, err
	}
	if vm == nil || vm.UID == "" || vm.Namespace != scope.Namespace || vm.Spec.Template == nil || vm.Labels["service_id"] != serviceID {
		return nil, vmrestore.ErrConflict
	}
	if vm.Spec.Running == nil && vm.Spec.RunStrategy == nil || vm.Spec.Running != nil && *vm.Spec.Running ||
		vm.Spec.RunStrategy != nil && *vm.Spec.RunStrategy != kv.RunStrategyHalted {
		return nil, vmrestore.ErrBusy
	}
	client := vmHotplugDynamicClient()
	if client == nil {
		return nil, vmrestore.ErrReferenceUnavailable
	}
	if _, err := client.Resource(vmDiskVMIGVR).Namespace(scope.Namespace).Get(ctx, vm.Name, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		if err != nil {
			return nil, vmrestore.ErrReferenceUnavailable
		}
		return nil, vmrestore.ErrBusy
	}
	volumes, err := s.getDBManager().TenantServiceVolumeDao().GetTenantServiceVolumesByServiceID(serviceID)
	if err != nil {
		return nil, err
	}
	result := &VMRecycleRecovery{ServiceID: serviceID, VMUID: string(vm.UID), VMName: vm.Name, K8sComponentName: service.K8sComponentName, Disks: []VMRecycleDisk{}}
	roots := 0
	for _, volume := range volumes {
		if volume.VolumeType == dbmodel.ConfigFileVolumeType.String() || volume.IsReadOnly {
			continue
		}
		resolved, err := consumer.resolver.ResolveVolume(ctx, scope, volume.ID)
		if err != nil {
			return nil, err
		}
		if !resolved.Attached {
			continue
		}
		if resolved.Provisioning != "bound" || resolved.ActivePVC == nil || resolved.ActivePVC.UID == "" {
			return nil, vmrestore.ErrReferenceUnavailable
		}
		matched := false
		for _, device := range vm.Spec.Template.Spec.Volumes {
			if device.PersistentVolumeClaim != nil && device.PersistentVolumeClaim.ClaimName == resolved.ActivePVC.Name ||
				device.DataVolume != nil && resolved.ActiveDV != nil && device.DataVolume.Name == resolved.ActiveDV.Name {
				matched = true
			}
		}
		if !matched {
			return nil, vmrestore.ErrConflict
		}
		if resolved.Role == "root" {
			roots++
		}
		result.Disks = append(result.Disks, VMRecycleDisk{DiskKey: resolved.DiskKey, PVCUID: resolved.ActivePVC.UID})
	}
	if roots != 1 {
		return nil, vmrestore.ErrConflict
	}
	result.StorageReady = true
	return result, nil
}
