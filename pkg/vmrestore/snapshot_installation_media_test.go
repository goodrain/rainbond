package vmrestore

import (
	"errors"
	"testing"

	kv "kubevirt.io/api/core/v1"
)

// capability_id: rainbond.vm-snapshot.readonly-installation-media
func TestSnapshotWithReadOnlyInstallationMediaKeepsWritableCoverage(t *testing.T) {
	f := snapshotProjectionFixture(t)
	vm := f.content.Spec.Source.VirtualMachine
	for _, name := range []string{"vmimage", "virtio"} {
		vm.Spec.Template.Spec.Volumes = append(vm.Spec.Template.Spec.Volumes, kv.Volume{Name: name, VolumeSource: kv.VolumeSource{ContainerDisk: &kv.ContainerDiskSource{Image: "installation-media"}}})
		vm.Spec.Template.Spec.Domain.Devices.Disks = append(vm.Spec.Template.Spec.Domain.Devices.Disks, kv.Disk{Name: name, DiskDevice: kv.DiskDevice{CDRom: &kv.CDRomTarget{Bus: kv.DiskBusSATA}}})
	}
	dto, err := f.project(SnapshotProof{})
	if err != nil || !dto.ReadyToUse || !dto.Coverage.Complete || dto.Coverage.ExpectedWritableDisks != 1 || len(dto.Config.DiskLayout) != 1 {
		t.Fatalf("read-only installation media blocked the managed root snapshot: %+v %v", dto, err)
	}
	writable := false
	for _, device := range []kv.DiskDevice{
		{Disk: &kv.DiskTarget{Bus: kv.DiskBusSATA}},
		{CDRom: &kv.CDRomTarget{Bus: kv.DiskBusSATA, ReadOnly: &writable}},
	} {
		vm.Spec.Template.Spec.Domain.Devices.Disks[1].DiskDevice = device
		if _, err := f.project(SnapshotProof{}); !errors.Is(err, ErrSnapshotIncomplete) {
			t.Fatalf("unmanaged writable container disk was accepted: %v", err)
		}
	}
}
