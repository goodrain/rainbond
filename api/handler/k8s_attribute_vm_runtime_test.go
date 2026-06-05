package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/worker/appm/conversion"
	"k8s.io/apimachinery/pkg/api/resource"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

func TestApplyVirtualMachineSpecPreservesRunStrategyWhenSyncingSpec(t *testing.T) {
	existing := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			RunStrategy: pointerToRunStrategy(kubevirtv1.RunStrategyHalted),
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Memory: &kubevirtv1.Memory{},
					},
				},
			},
		},
	}
	targetMemory := resourceMustParse(t, "4Gi")
	desired := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			RunStrategy: pointerToRunStrategy(kubevirtv1.RunStrategyAlways),
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Memory: &kubevirtv1.Memory{
							Guest: &targetMemory,
						},
					},
				},
			},
		},
	}

	updated, changed := applyVirtualMachineSpec(existing, desired)
	if !changed {
		t.Fatal("expected virtual machine spec diff to be detected")
	}
	if updated.Spec.RunStrategy == nil || *updated.Spec.RunStrategy != kubevirtv1.RunStrategyHalted {
		t.Fatalf("expected run strategy to be preserved, got %#v", updated.Spec.RunStrategy)
	}
	if updated.Spec.Template == nil || updated.Spec.Template.Spec.Domain.Memory == nil || updated.Spec.Template.Spec.Domain.Memory.Guest == nil {
		t.Fatalf("expected target memory to be synced, got %#v", updated.Spec.Template)
	}
	if updated.Spec.Template.Spec.Domain.Memory.Guest.Cmp(targetMemory) != 0 {
		t.Fatalf("expected target memory to be synced, got %#v", updated.Spec.Template.Spec.Domain.Memory.Guest)
	}
}

func TestApplyVirtualMachineSpecPreservesLegacyRunningFlagWhenSyncingSpec(t *testing.T) {
	existing := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Running: pointerToBool(false),
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Memory: &kubevirtv1.Memory{},
					},
				},
			},
		},
	}
	targetMemory := resourceMustParse(t, "2Gi")
	desired := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			RunStrategy: pointerToRunStrategy(kubevirtv1.RunStrategyAlways),
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Memory: &kubevirtv1.Memory{
							Guest: &targetMemory,
						},
					},
				},
			},
		},
	}

	updated, changed := applyVirtualMachineSpec(existing, desired)
	if !changed {
		t.Fatal("expected virtual machine spec diff to be detected")
	}
	if updated.Spec.Running == nil || *updated.Spec.Running {
		t.Fatalf("expected legacy running flag to stay false, got %#v", updated.Spec.Running)
	}
	if updated.Spec.RunStrategy != nil {
		t.Fatalf("expected legacy running flag sync to avoid forcing runStrategy, got %#v", updated.Spec.RunStrategy)
	}
	if updated.Spec.Template == nil || updated.Spec.Template.Spec.Domain.Memory == nil || updated.Spec.Template.Spec.Domain.Memory.Guest == nil {
		t.Fatalf("expected target memory to be synced, got %#v", updated.Spec.Template)
	}
	if updated.Spec.Template.Spec.Domain.Memory.Guest.Cmp(targetMemory) != 0 {
		t.Fatalf("expected target memory to be synced, got %#v", updated.Spec.Template.Spec.Domain.Memory.Guest)
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
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Memory: &kubevirtv1.Memory{},
					},
				},
			},
		},
	}
	targetMemory := resourceMustParse(t, "4Gi")
	desired := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			RunStrategy: pointerToRunStrategy(kubevirtv1.RunStrategyAlways),
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Memory: &kubevirtv1.Memory{
							Guest: &targetMemory,
						},
					},
				},
			},
		},
	}

	mockClient.EXPECT().VirtualMachine("demo-ns").Return(mockVMInterface).Times(2)
	mockVMInterface.EXPECT().Get(gomock.Any(), "demo-vm", gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, _ metav1.GetOptions) (*kubevirtv1.VirtualMachine, error) {
			return existing.DeepCopy(), nil
		},
	)
	mockVMInterface.EXPECT().Update(gomock.Any(), gomock.AssignableToTypeOf(&kubevirtv1.VirtualMachine{}), gomock.Any()).DoAndReturn(
		func(_ context.Context, updated *kubevirtv1.VirtualMachine, _ metav1.UpdateOptions) (*kubevirtv1.VirtualMachine, error) {
			if updated.Spec.RunStrategy == nil || *updated.Spec.RunStrategy != kubevirtv1.RunStrategyHalted {
				t.Fatalf("expected updated vm run strategy to stay halted, got %#v", updated.Spec.RunStrategy)
			}
			if updated.Spec.Template == nil || updated.Spec.Template.Spec.Domain.Memory == nil || updated.Spec.Template.Spec.Domain.Memory.Guest == nil {
				t.Fatalf("expected updated vm to carry memory config, got %#v", updated.Spec.Template)
			}
			if updated.Spec.Template.Spec.Domain.Memory.Guest.Cmp(targetMemory) != 0 {
				t.Fatalf("expected updated vm to carry memory config, got %#v", updated.Spec.Template.Spec.Domain.Memory.Guest)
			}
			return updated, nil
		},
	)

	action := &ServiceAction{kubevirtClient: mockClient}
	if err := action.syncVirtualMachineSpec(existing, desired); err != nil {
		t.Fatalf("expected virtual machine spec sync to succeed, got %v", err)
	}
}

// capability_id: rainbond.vm-runtime-spec-sync-conflict-retry
func TestSyncVirtualMachineSpecRetriesOnConflict(t *testing.T) {
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
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Memory: &kubevirtv1.Memory{},
					},
				},
			},
		},
	}
	desiredMemory := resourceMustParse(t, "4Gi")
	desired := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			RunStrategy: pointerToRunStrategy(kubevirtv1.RunStrategyAlways),
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Memory: &kubevirtv1.Memory{
							Guest: &desiredMemory,
						},
					},
				},
			},
		},
	}

	mockClient.EXPECT().VirtualMachine("demo-ns").Return(mockVMInterface).Times(4)
	mockVMInterface.EXPECT().Get(gomock.Any(), "demo-vm", gomock.Any()).Times(2).DoAndReturn(
		func(_ context.Context, _ string, _ metav1.GetOptions) (*kubevirtv1.VirtualMachine, error) {
			return existing.DeepCopy(), nil
		},
	)
	attempts := 0
	mockVMInterface.EXPECT().Update(gomock.Any(), gomock.AssignableToTypeOf(&kubevirtv1.VirtualMachine{}), gomock.Any()).Times(2).DoAndReturn(
		func(_ context.Context, updated *kubevirtv1.VirtualMachine, _ metav1.UpdateOptions) (*kubevirtv1.VirtualMachine, error) {
			attempts++
			if updated.Spec.Template == nil || updated.Spec.Template.Spec.Domain.Memory == nil || updated.Spec.Template.Spec.Domain.Memory.Guest == nil {
				t.Fatalf("expected updated vm to carry memory config, got %#v", updated.Spec.Template)
			}
			if updated.Spec.Template.Spec.Domain.Memory.Guest.Cmp(desiredMemory) != 0 {
				t.Fatalf("expected updated vm to carry target memory, got %#v", updated.Spec.Template.Spec.Domain.Memory.Guest)
			}
			if attempts == 1 {
				return nil, k8serrors.NewConflict(
					schema.GroupResource{Group: "kubevirt.io", Resource: "virtualmachines"},
					"demo-vm",
					errors.New("the object has been modified"),
				)
			}
			return updated, nil
		},
	)

	action := &ServiceAction{kubevirtClient: mockClient}
	if err := action.syncVirtualMachineSpec(existing, desired); err != nil {
		t.Fatalf("expected conflict retry to succeed, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected vm spec update to retry once after conflict, got %d attempts", attempts)
	}
}

func TestIsVMRuntimeSpecAttributeOnlyIncludesSupportedVMFields(t *testing.T) {
	for _, name := range []string{
		"vm_os_name",
	} {
		if !isVMRuntimeSpecAttribute(name) {
			t.Fatalf("expected %s to trigger vm spec sync", name)
		}
	}
	for _, name := range []string{
		"vm_network_mode",
		"vm_network_name",
		"vm_fixed_ip",
		"vm_gateway",
		"vm_dns_servers",
	} {
		if isVMRuntimeSpecAttribute(name) {
			t.Fatalf("did not expect removed vm network field %s to trigger vm spec sync", name)
		}
	}
}

// capability_id: rainbond.vm-runtime.disk-layout-attr-triggers-spec-sync
func TestIsVMRuntimeSpecAttributeIncludesDiskLayout(t *testing.T) {
	if !isVMRuntimeSpecAttribute("vm_disk_layout") {
		t.Fatal("expected vm_disk_layout to trigger vm spec sync")
	}
}

// capability_id: rainbond.vm-probe-attribute-syncs-spec
func TestIsVMRuntimeSpecAttributeIncludesProbeAttributes(t *testing.T) {
	for _, name := range []string{
		"livenessProbe",
		"readinessProbe",
	} {
		if !isVMRuntimeSpecAttribute(name) {
			t.Fatalf("expected %s to trigger vm spec sync", name)
		}
	}
}

func pointerToRunStrategy(strategy kubevirtv1.VirtualMachineRunStrategy) *kubevirtv1.VirtualMachineRunStrategy {
	return &strategy
}

func pointerToBool(value bool) *bool {
	return &value
}

func resourceMustParse(t *testing.T, value string) resource.Quantity {
	t.Helper()
	return resource.MustParse(value)
}
