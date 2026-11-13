package handler

import (
	"context"
	"strings"

	"github.com/goodrain/rainbond/pkg/vmrestore"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
	snapshotv1 "kubevirt.io/api/snapshot/v1beta1"
)

// VMSnapshotRequest preserves legacy fields while accepting bounded operation options.
type VMSnapshotRequest = vmrestore.SnapshotCreateRequest

// VMSnapshotStatus preserves the original snapshot_name composite field.
type VMSnapshotStatus struct {
	SnapshotName string `json:"snapshot_name"`
	*vmrestore.SnapshotOperationDTO
}

// CreateVMSnapshot preserves the internal legacy entrypoint and prepares a scoped operation.
func (s *ServiceAction) CreateVMSnapshot(serviceID string, request *VMSnapshotRequest) (*VMSnapshotStatus, error) {
	return s.PrepareVMSnapshot(context.Background(), serviceID, request)
}

func buildVMSnapshot(vm *kubevirtv1.VirtualMachine, snapshotName, description, serviceID string) *snapshotv1.VirtualMachineSnapshot {
	apiGroup := kubevirtv1.VirtualMachineGroupVersionKind.Group
	annotations := map[string]string{}
	if strings.TrimSpace(description) != "" {
		annotations["description"] = description
	}
	return &snapshotv1.VirtualMachineSnapshot{
		TypeMeta: metav1.TypeMeta{
			APIVersion: snapshotv1.SchemeGroupVersion.String(),
			Kind:       "VirtualMachineSnapshot",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        strings.TrimSpace(snapshotName),
			Namespace:   vm.Namespace,
			Labels:      map[string]string{"service_id": serviceID},
			Annotations: annotations,
		},
		Spec: snapshotv1.VirtualMachineSnapshotSpec{
			Source: corev1.TypedLocalObjectReference{
				APIGroup: &apiGroup,
				Kind:     "VirtualMachine",
				Name:     vm.Name,
			},
		},
	}
}
