package handler

import (
	"fmt"

	dbmodel "github.com/goodrain/rainbond/db/model"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	return s.deleteBoundVM(service, volume)
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
