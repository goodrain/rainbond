package handler

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

// capability_id: rainbond.vm-template-import.status-building
func TestResolveVMTransitionStatusReturnsBuildingForDataVolumeImport(t *testing.T) {
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
	if got != "building" {
		t.Fatalf("expected importing data volumes to map to %q, got %q", "building", got)
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
