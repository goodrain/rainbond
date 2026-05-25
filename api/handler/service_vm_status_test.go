package handler

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

// capability_id: rainbond.vm-template-import.status-restoring
func TestResolveVMTransitionStatusReturnsRestoringForDataVolumeImport(t *testing.T) {
	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Status: kubevirtv1.VirtualMachineStatus{
			PrintableStatus: kubevirtv1.VirtualMachineStatusStopped,
		},
	}
	vmi := &kubevirtv1.VirtualMachineInstance{
		Status: kubevirtv1.VirtualMachineInstanceStatus{
			Phase: kubevirtv1.Pending,
			Conditions: []kubevirtv1.VirtualMachineInstanceCondition{
				{
					Type:   kubevirtv1.VirtualMachineInstanceProvisioning,
					Status: "True",
				},
				{
					Type:   kubevirtv1.VirtualMachineInstanceDataVolumesReady,
					Status: "False",
				},
			},
		},
	}

	got, ok := resolveVMTransitionStatus(vm, vmi)
	if !ok {
		t.Fatal("expected importing data volumes to override vm closed status")
	}
	if got != "restoring" {
		t.Fatalf("expected importing data volumes to map to %q, got %q", "restoring", got)
	}
}

// capability_id: rainbond.vm-template-import.status-restoring
func TestResolveVMServiceRuntimeStatusReturnsRestoringWhenDataVolumeImportsBeforeVMIExists(t *testing.T) {
	action := &ServiceAction{
		getVirtualMachineByServiceIDHook: func(serviceID string) (*kubevirtv1.VirtualMachine, error) {
			return &kubevirtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "demo-vm",
					Namespace: "demo-ns",
				},
				Spec: kubevirtv1.VirtualMachineSpec{
					DataVolumeTemplates: []kubevirtv1.DataVolumeTemplateSpec{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "manual133",
							},
						},
					},
				},
				Status: kubevirtv1.VirtualMachineStatus{
					PrintableStatus: kubevirtv1.VirtualMachineStatusStopped,
				},
			}, nil
		},
		getVirtualMachineInstanceByServiceIDHook: func(serviceID string) (*kubevirtv1.VirtualMachineInstance, error) {
			return nil, nil
		},
		getDataVolumePhasesByNamesHook: func(namespace string, names []string) (map[string]string, error) {
			if namespace != "demo-ns" {
				t.Fatalf("expected namespace %q, got %q", "demo-ns", namespace)
			}
			if len(names) != 1 || names[0] != "manual133" {
				t.Fatalf("expected data volume names [manual133], got %#v", names)
			}
			return map[string]string{
				"manual133": "ImportInProgress",
			}, nil
		},
	}

	got, ok := action.resolveVMServiceRuntimeStatus("service-a")
	if !ok {
		t.Fatal("expected importing data volume to override vm status before vmi exists")
	}
	if got != "restoring" {
		t.Fatalf("expected importing data volume to map to %q, got %q", "restoring", got)
	}
}

func TestResolveVMTransitionStatusReturnsStartingForPendingProvisioningVMI(t *testing.T) {
	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Status: kubevirtv1.VirtualMachineStatus{
			PrintableStatus: kubevirtv1.VirtualMachineStatusStopped,
		},
	}
	vmi := &kubevirtv1.VirtualMachineInstance{
		Status: kubevirtv1.VirtualMachineInstanceStatus{
			Phase: kubevirtv1.Pending,
			Conditions: []kubevirtv1.VirtualMachineInstanceCondition{
				{
					Type:   kubevirtv1.VirtualMachineInstanceProvisioning,
					Status: "True",
				},
			},
		},
	}

	got, ok := resolveVMTransitionStatus(vm, vmi)
	if !ok {
		t.Fatal("expected pending provisioning vmi to override vm closed status")
	}
	if got != "starting" {
		t.Fatalf("expected pending provisioning vmi to map to %q, got %q", "starting", got)
	}
}

func TestResolveVMTransitionStatusKeepsStoppedVMClosedWhenNoVMIExists(t *testing.T) {
	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Status: kubevirtv1.VirtualMachineStatus{
			PrintableStatus: kubevirtv1.VirtualMachineStatusStopped,
		},
	}

	if got, ok := resolveVMTransitionStatus(vm, nil); ok {
		t.Fatalf("expected no override for stopped vm without vmi, got status %q", got)
	}
}

func TestResolveVMTransitionStatusReturnsAbnormalForDataVolumeError(t *testing.T) {
	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Status: kubevirtv1.VirtualMachineStatus{
			PrintableStatus: kubevirtv1.VirtualMachineStatusDataVolumeError,
		},
	}

	got, ok := resolveVMTransitionStatus(vm, nil)
	if !ok {
		t.Fatal("expected data volume error to override vm status")
	}
	if got != "abnormal" {
		t.Fatalf("expected data volume error to map to %q, got %q", "abnormal", got)
	}
}
