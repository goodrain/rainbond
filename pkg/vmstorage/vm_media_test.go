package vmstorage

import (
	"context"
	"testing"

	v1 "kubevirt.io/api/core/v1"
)

// capability_id: rainbond.vm-storage.snapshot-all-disks
func TestVirtualMachineSnapshotExcludesOnlyReadOnlyInstaller(t *testing.T) {
	for _, name := range []string{"implicit read-only CDROM", "different installer name", "explicit read-only CDROM", "writable CDROM", "regular read-only data", "missing device"} {
		t.Run(name, func(t *testing.T) {
			c, d, vm := boundVMFixture(t)
			volumeName := "vmimage"
			if name == "different installer name" {
				volumeName = "virtio"
			}
			vm.Spec.Template.Spec.Volumes = append(vm.Spec.Template.Spec.Volumes, v1.Volume{Name: volumeName, VolumeSource: v1.VolumeSource{ContainerDisk: &v1.ContainerDiskSource{Image: "installer"}}})
			device := v1.Disk{Name: volumeName, DiskDevice: v1.DiskDevice{CDRom: &v1.CDRomTarget{}}}
			readOnly := name != "writable CDROM"
			if name != "implicit read-only CDROM" {
				device.CDRom.ReadOnly = &readOnly
			}
			if name == "regular read-only data" {
				device.DiskDevice = v1.DiskDevice{Disk: &v1.DiskTarget{ReadOnly: true}}
			}
			if name != "missing device" {
				vm.Spec.Template.Spec.Domain.Devices.Disks = append(vm.Spec.Template.Spec.Domain.Devices.Disks, device)
			}
			r := NewInspector(c, d).VirtualMachine(context.Background(), vm)
			excluded := name == "implicit read-only CDROM" || name == "different installer name" || name == "explicit read-only CDROM"
			if r.Supported != excluded || (excluded && len(r.Volumes) != 1) || (!excluded && (len(r.Volumes) != 2 || r.Status != "unsupported")) {
				t.Fatalf("installer/data scope incorrect: %+v", r)
			}
		})
	}
}
