package handler

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
	snapshotv1 "kubevirt.io/api/snapshot/v1beta1"
)

type VMSnapshotRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type VMSnapshotStatus struct {
	SnapshotName string `json:"snapshot_name"`
}

func (s *ServiceAction) CreateVMSnapshot(serviceID string, req *VMSnapshotRequest) (*VMSnapshotStatus, error) {
	if req == nil || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("snapshot name is required")
	}
	vms, err := s.kubevirtClient.VirtualMachine("").List(context.Background(), metav1.ListOptions{
		LabelSelector: "service_id=" + serviceID,
	})
	if err != nil {
		return nil, err
	}
	if len(vms.Items) == 0 {
		return nil, fmt.Errorf("service id is %v vm is not exist", serviceID)
	}
	vm := &vms.Items[0]
	snapshot := buildVMSnapshot(vm, req.Name, req.Description, serviceID)
	_, err = s.kubevirtClient.VirtualMachineSnapshot(vm.Namespace).Create(context.Background(), snapshot, metav1.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return nil, err
	}
	return &VMSnapshotStatus{SnapshotName: snapshot.Name}, nil
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
