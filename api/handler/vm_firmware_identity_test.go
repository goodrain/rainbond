package handler

import (
	"context"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
)

func stockReviewVM(uuid, mode string, firmware bool, cores uint32) *kubevirtv1.VirtualMachine {
	vm := &kubevirtv1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "same-vm-name", Namespace: "same-namespace"}, Spec: kubevirtv1.VirtualMachineSpec{Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{Spec: kubevirtv1.VirtualMachineInstanceSpec{Domain: kubevirtv1.DomainSpec{CPU: &kubevirtv1.CPU{Cores: cores}}}}}}
	if firmware {
		vm.Spec.Template.Spec.Domain.Firmware = &kubevirtv1.Firmware{UUID: types.UID(uuid)}
		if mode == "efi" {
			vm.Spec.Template.Spec.Domain.Firmware.Bootloader = &kubevirtv1.Bootloader{EFI: &kubevirtv1.EFI{}}
		} else if mode == "bios" {
			vm.Spec.Template.Spec.Domain.Firmware.Bootloader = &kubevirtv1.Bootloader{BIOS: &kubevirtv1.BIOS{}}
		}
	}
	return vm
}

// capability_id: rainbond.vm-runtime.firmware-identity-preservation
func TestSyncVirtualMachineSpecPreservesExistingFirmwareIdentity(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := kubecli.NewMockKubevirtClient(ctrl)
	virtualMachines := kubecli.NewMockVirtualMachineInterface(ctrl)
	existing := stockReviewVM("99999999-9999-4999-8999-999999999999", "bios", true, 1)
	desired := stockReviewVM("", "efi", true, 2)
	client.EXPECT().VirtualMachine(existing.Namespace).Return(virtualMachines).Times(2)
	virtualMachines.EXPECT().Get(gomock.Any(), existing.Name, gomock.Any()).Return(existing, nil)
	virtualMachines.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, updated *kubevirtv1.VirtualMachine, _ metav1.UpdateOptions) (*kubevirtv1.VirtualMachine, error) {
			if updated.Spec.Template.Spec.Domain.Firmware.UUID != existing.Spec.Template.Spec.Domain.Firmware.UUID || stockReviewMode(updated) != "efi" || updated.Spec.Template.Spec.Domain.CPU.Cores != 2 {
				t.Fatal("routine spec synchronization lost identity or new user configuration")
			}
			return updated, nil
		})
	action := &ServiceAction{kubevirtClient: client}
	if err := action.syncVirtualMachineSpec(existing, desired); err != nil {
		t.Fatal("spec synchronization failed")
	}
}
func stockReviewMode(vm *kubevirtv1.VirtualMachine) string {
	if vm == nil || vm.Spec.Template == nil || vm.Spec.Template.Spec.Domain.Firmware == nil || vm.Spec.Template.Spec.Domain.Firmware.Bootloader == nil {
		return ""
	}
	b := vm.Spec.Template.Spec.Domain.Firmware.Bootloader
	if b.EFI != nil {
		return "efi"
	}
	if b.BIOS != nil {
		return "bios"
	}
	return ""
}

// capability_id: rainbond.vm-runtime.firmware-identity-preservation
func TestVMFirmwareIdentityCompatibilityMatrix(t *testing.T) {
	const current = "99999999-9999-4999-8999-999999999999"
	const rotated = "88888888-8888-4888-8888-888888888888"
	for _, c := range []struct {
		name         string
		old, desired *kubevirtv1.VirtualMachine
		uuid, mode   string
		changed      bool
		cores        uint32
	}{
		{"nil_existing", nil, stockReviewVM("", "efi", true, 1), "", "", false, 0},
		{"nil_desired", stockReviewVM(current, "efi", true, 1), nil, current, "efi", false, 1},
		{"nil_firmware_noop", stockReviewVM("", "", false, 1), stockReviewVM("", "", false, 1), "", "", false, 1},
		{"new_default_firmware", stockReviewVM("", "", false, 1), stockReviewVM("", "efi", true, 1), "", "efi", true, 1},
		{"empty_existing_uuid", stockReviewVM("", "bios", true, 1), stockReviewVM("", "efi", true, 1), "", "efi", true, 1},
		{"existing_uuid_desired_nil_firmware", stockReviewVM(current, "bios", true, 1), stockReviewVM("", "", false, 1), current, "", true, 1},
		{"existing_uuid_keep_new_mode_and_cpu", stockReviewVM(current, "bios", true, 1), stockReviewVM("", "efi", true, 2), current, "efi", true, 2},
		{"existing_uuid_only_omitted_is_noop", stockReviewVM(current, "efi", true, 1), stockReviewVM("", "efi", true, 1), current, "efi", false, 1},
		{"explicit_desired_same_uuid", stockReviewVM(current, "efi", true, 1), stockReviewVM(current, "efi", true, 1), current, "efi", false, 1},
		{"explicit_desired_rotation", stockReviewVM(current, "efi", true, 1), stockReviewVM(rotated, "bios", true, 2), rotated, "bios", true, 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			oldBefore := c.old.DeepCopy()
			desiredBefore := c.desired.DeepCopy()
			got, changed := applyVirtualMachineSpec(c.old, c.desired)
			if !reflect.DeepEqual(c.old, oldBefore) || !reflect.DeepEqual(c.desired, desiredBefore) {
				t.Fatal("input object mutated")
			}
			if changed != c.changed {
				t.Fatalf("changed=%v, want %v", changed, c.changed)
			}
			if c.old == nil {
				if got != nil {
					t.Fatal("nil existing should remain nil")
				}
				return
			}
			uuid := ""
			if got.Spec.Template.Spec.Domain.Firmware != nil {
				uuid = string(got.Spec.Template.Spec.Domain.Firmware.UUID)
			}
			if uuid != c.uuid {
				t.Fatal("firmware identity not preserved or explicit intent overridden")
			}
			if stockReviewMode(got) != c.mode {
				t.Fatal("new boot mode overridden by old firmware")
			}
			if got.Spec.Template.Spec.Domain.CPU.Cores != c.cores {
				t.Fatal("new hardware config discarded")
			}
		})
	}
}
