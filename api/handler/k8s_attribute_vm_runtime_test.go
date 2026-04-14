package handler

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/worker/appm/conversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
	kubecli "kubevirt.io/client-go/kubecli"
)

func TestApplyVMRuntimeDeviceConfigUpdatesGPUAndUSBDevices(t *testing.T) {
	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{},
					},
				},
			},
		},
	}

	updatedVM, changed := applyVMRuntimeDeviceConfig(vm, map[string]string{
		"vm_gpu_enabled":   "true",
		"vm_gpu_resources": "[\"gpu.example.com/A10\"]",
		"vm_gpu_count":     "2",
		"vm_usb_enabled":   "true",
		"vm_usb_resources": "[\"kubevirt.io/usb-a\"]",
	})

	if !changed {
		t.Fatal("expected runtime device config to change VM devices")
	}
	if len(updatedVM.Spec.Template.Spec.Domain.Devices.GPUs) != 2 {
		t.Fatalf("expected 2 gpu devices, got %#v", updatedVM.Spec.Template.Spec.Domain.Devices.GPUs)
	}
	if updatedVM.Spec.Template.Spec.Domain.Devices.GPUs[0].DeviceName != "gpu.example.com/A10" {
		t.Fatalf("unexpected gpu config: %#v", updatedVM.Spec.Template.Spec.Domain.Devices.GPUs)
	}
	if len(updatedVM.Spec.Template.Spec.Domain.Devices.HostDevices) != 1 {
		t.Fatalf("expected 1 usb host device, got %#v", updatedVM.Spec.Template.Spec.Domain.Devices.HostDevices)
	}
	if updatedVM.Spec.Template.Spec.Domain.Devices.HostDevices[0].DeviceName != "kubevirt.io/usb-a" {
		t.Fatalf("unexpected host device config: %#v", updatedVM.Spec.Template.Spec.Domain.Devices.HostDevices)
	}
}

func TestApplyVMRuntimeDeviceConfigClearsAcceleratorsWhenDisabled(t *testing.T) {
	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{
							GPUs: []kubevirtv1.GPU{
								{Name: "gpu-0", DeviceName: "gpu.example.com/A10"},
							},
							HostDevices: []kubevirtv1.HostDevice{
								{Name: "usb-0", DeviceName: "kubevirt.io/usb-a"},
							},
						},
					},
				},
			},
		},
	}

	updatedVM, changed := applyVMRuntimeDeviceConfig(vm, map[string]string{})

	if !changed {
		t.Fatal("expected runtime device config to clear VM devices")
	}
	if len(updatedVM.Spec.Template.Spec.Domain.Devices.GPUs) != 0 {
		t.Fatalf("expected gpus to be cleared, got %#v", updatedVM.Spec.Template.Spec.Domain.Devices.GPUs)
	}
	if len(updatedVM.Spec.Template.Spec.Domain.Devices.HostDevices) != 0 {
		t.Fatalf("expected usb host devices to be cleared, got %#v", updatedVM.Spec.Template.Spec.Domain.Devices.HostDevices)
	}
}

func TestSyncVMRuntimeDeviceConfigUpdatesVirtualMachineWhenDevicesChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := kubecli.NewMockKubevirtClient(ctrl)
	mockVMInterface := kubecli.NewMockVirtualMachineInterface(ctrl)

	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{},
					},
				},
			},
		},
	}

	expectedGPUs, expectedHostDevices := conversion.BuildVMAccelerationDevices(map[string]string{
		"vm_gpu_enabled":   "true",
		"vm_gpu_resources": "[\"gpu.example.com/A10\"]",
		"vm_usb_enabled":   "true",
		"vm_usb_resources": "[\"kubevirt.io/usb-a\"]",
	})

	mockClient.EXPECT().VirtualMachine("demo-ns").Return(mockVMInterface)
	mockVMInterface.EXPECT().Update(gomock.Any(), gomock.AssignableToTypeOf(&kubevirtv1.VirtualMachine{}), gomock.Any()).DoAndReturn(
		func(_ context.Context, updated *kubevirtv1.VirtualMachine, _ metav1.UpdateOptions) (*kubevirtv1.VirtualMachine, error) {
			if len(updated.Spec.Template.Spec.Domain.Devices.GPUs) != len(expectedGPUs) {
				t.Fatalf("expected updated vm to carry gpu config, got %#v", updated.Spec.Template.Spec.Domain.Devices.GPUs)
			}
			if len(updated.Spec.Template.Spec.Domain.Devices.HostDevices) != len(expectedHostDevices) {
				t.Fatalf("expected updated vm to carry usb config, got %#v", updated.Spec.Template.Spec.Domain.Devices.HostDevices)
			}
			return updated, nil
		},
	)

	action := &ServiceAction{kubevirtClient: mockClient}
	if err := action.syncVMRuntimeDeviceConfig(vm, map[string]string{
		"vm_gpu_enabled":   "true",
		"vm_gpu_resources": "[\"gpu.example.com/A10\"]",
		"vm_usb_enabled":   "true",
		"vm_usb_resources": "[\"kubevirt.io/usb-a\"]",
	}); err != nil {
		t.Fatalf("expected sync to succeed, got %v", err)
	}
}

func TestSyncVMRuntimeDeviceConfigSkipsUpdateWhenDevicesAlreadyMatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := kubecli.NewMockKubevirtClient(ctrl)
	gpus, hostDevices := conversion.BuildVMAccelerationDevices(map[string]string{
		"vm_gpu_enabled":   "true",
		"vm_gpu_resources": "[\"gpu.example.com/A10\"]",
		"vm_usb_enabled":   "true",
		"vm_usb_resources": "[\"kubevirt.io/usb-a\"]",
	})
	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{
							GPUs:        gpus,
							HostDevices: hostDevices,
						},
					},
				},
			},
		},
	}

	action := &ServiceAction{kubevirtClient: mockClient}
	if err := action.syncVMRuntimeDeviceConfig(vm, map[string]string{
		"vm_gpu_enabled":   "true",
		"vm_gpu_resources": "[\"gpu.example.com/A10\"]",
		"vm_usb_enabled":   "true",
		"vm_usb_resources": "[\"kubevirt.io/usb-a\"]",
	}); err != nil {
		t.Fatalf("expected sync to succeed without update, got %v", err)
	}
}

func TestApplyVirtualMachineSpecReplacesSpecWhenChanged(t *testing.T) {
	existing := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			RunStrategy: pointerToRunStrategy(kubevirtv1.RunStrategyHalted),
		},
	}
	desired := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			RunStrategy: pointerToRunStrategy(kubevirtv1.RunStrategyAlways),
		},
	}

	updated, changed := applyVirtualMachineSpec(existing, desired)
	if !changed {
		t.Fatal("expected virtual machine spec diff to be detected")
	}
	if updated.Spec.RunStrategy == nil || *updated.Spec.RunStrategy != kubevirtv1.RunStrategyAlways {
		t.Fatalf("expected run strategy to be replaced, got %#v", updated.Spec.RunStrategy)
	}
}

func TestSyncVirtualMachineSpecUpdatesWhenSpecChanges(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := kubecli.NewMockKubevirtClient(ctrl)
	mockVMInterface := kubecli.NewMockVirtualMachineInterface(ctrl)

	existing := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			RunStrategy: pointerToRunStrategy(kubevirtv1.RunStrategyHalted),
		},
	}
	desired := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			RunStrategy: pointerToRunStrategy(kubevirtv1.RunStrategyAlways),
		},
	}

	mockClient.EXPECT().VirtualMachine("demo-ns").Return(mockVMInterface)
	mockVMInterface.EXPECT().Update(gomock.Any(), gomock.AssignableToTypeOf(&kubevirtv1.VirtualMachine{}), gomock.Any()).DoAndReturn(
		func(_ context.Context, updated *kubevirtv1.VirtualMachine, _ metav1.UpdateOptions) (*kubevirtv1.VirtualMachine, error) {
			if updated.Spec.RunStrategy == nil || *updated.Spec.RunStrategy != kubevirtv1.RunStrategyAlways {
				t.Fatalf("expected updated vm spec to be synced, got %#v", updated.Spec.RunStrategy)
			}
			return updated, nil
		},
	)

	action := &ServiceAction{kubevirtClient: mockClient}
	if err := action.syncVirtualMachineSpec(existing, desired); err != nil {
		t.Fatalf("expected virtual machine spec sync to succeed, got %v", err)
	}
}

func pointerToRunStrategy(strategy kubevirtv1.VirtualMachineRunStrategy) *kubevirtv1.VirtualMachineRunStrategy {
	return &strategy
}
