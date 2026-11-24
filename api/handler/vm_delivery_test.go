package handler

import (
	"testing"

	kv "kubevirt.io/api/core/v1"
)

// capability_id: rainbond.vm-delivery.current-system-disk
func TestVMDeliverySelectsInstalledDiskAndRejectsAdditionalDisks(t *testing.T) {
	order := uint(1)
	vm := &kv.VirtualMachine{Spec: kv.VirtualMachineSpec{Template: &kv.VirtualMachineInstanceTemplateSpec{Spec: kv.VirtualMachineInstanceSpec{
		Domain:  kv.DomainSpec{Devices: kv.Devices{Disks: []kv.Disk{{Name: "installer", BootOrder: &order, DiskDevice: kv.DiskDevice{CDRom: &kv.CDRomTarget{}}}, {Name: "root", DiskDevice: kv.DiskDevice{Disk: &kv.DiskTarget{}}}}}},
		Volumes: []kv.Volume{{Name: "installer", VolumeSource: kv.VolumeSource{DataVolume: &kv.DataVolumeSource{Name: "iso"}}}, {Name: "root", VolumeSource: kv.VolumeSource{DataVolume: &kv.DataVolumeSource{Name: "installed-root"}}}},
	}}}}
	root, err := resolveVMDeliveryRootPVC(vm)
	if err != nil || root != "installed-root" {
		t.Fatalf("selected installation media instead of installed OS: %q %v", root, err)
	}
	vm.Spec.Template.Spec.Domain.Devices.Disks = append(vm.Spec.Template.Spec.Domain.Devices.Disks, kv.Disk{Name: "data", DiskDevice: kv.DiskDevice{Disk: &kv.DiskTarget{}}})
	vm.Spec.Template.Spec.Volumes = append(vm.Spec.Template.Spec.Volumes, kv.Volume{Name: "data", VolumeSource: kv.VolumeSource{DataVolume: &kv.DataVolumeSource{Name: "data"}}})
	if _, err := resolveVMDeliveryRootPVC(vm); err == nil {
		t.Fatal("additional writable data disk silently omitted")
	}
}
