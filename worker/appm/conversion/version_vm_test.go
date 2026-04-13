package conversion

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

func TestSelectVMProbesReturnsReadinessBeforeLiveness(t *testing.T) {
	readiness, liveness := selectVMProbes(func(mode string) *kubevirtv1.Probe {
		return &kubevirtv1.Probe{
			InitialDelaySeconds: map[string]int32{
				"readiness": 11,
				"liveness":  22,
			}[mode],
		}
	})

	if readiness == nil || readiness.InitialDelaySeconds != 11 {
		t.Fatalf("expected readiness probe to come from readiness mode, got %#v", readiness)
	}
	if liveness == nil || liveness.InitialDelaySeconds != 22 {
		t.Fatalf("expected liveness probe to come from liveness mode, got %#v", liveness)
	}
}

func TestDecodeVMProbeYAMLParsesProbePayload(t *testing.T) {
	probe, err := decodeVMProbeYAML([]byte("initialDelaySeconds: 7\nperiodSeconds: 3\n"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if probe == nil {
		t.Fatal("expected probe to be decoded")
	}
	if probe.InitialDelaySeconds != 7 || probe.PeriodSeconds != 3 {
		t.Fatalf("unexpected decoded probe: %#v", probe)
	}
}

func TestVMRuntimeAttributeNamesIncludesBootMode(t *testing.T) {
	names := vmRuntimeAttributeNames()
	found := map[string]bool{
		"vm_boot_mode":          false,
		"vm_boot_source_format": false,
	}
	for _, name := range names {
		if _, ok := found[name]; ok {
			found[name] = true
		}
	}
	for name, ok := range found {
		if !ok {
			t.Fatalf("expected %s in runtime attribute names, got %#v", name, names)
		}
	}
}

func TestApplyVMBootModeSetsUEFIFirmware(t *testing.T) {
	domain := &kubevirtv1.DomainSpec{}

	applyVMBootMode(domain, map[string]string{"vm_boot_mode": "uefi"})

	if domain.Firmware == nil || domain.Firmware.Bootloader == nil || domain.Firmware.Bootloader.EFI == nil {
		t.Fatalf("expected EFI firmware to be configured, got %#v", domain.Firmware)
	}
	if domain.Firmware.Bootloader.BIOS != nil {
		t.Fatalf("did not expect BIOS bootloader when boot mode is uefi, got %#v", domain.Firmware.Bootloader)
	}
}

func TestApplyVMBootModeSetsBIOSFirmware(t *testing.T) {
	domain := &kubevirtv1.DomainSpec{}

	applyVMBootMode(domain, map[string]string{"vm_boot_mode": "bios"})

	if domain.Firmware == nil || domain.Firmware.Bootloader == nil || domain.Firmware.Bootloader.BIOS == nil {
		t.Fatalf("expected BIOS firmware to be configured, got %#v", domain.Firmware)
	}
	if domain.Firmware.Bootloader.EFI != nil {
		t.Fatalf("did not expect EFI bootloader when boot mode is bios, got %#v", domain.Firmware.Bootloader)
	}
}

func TestVMImageDiskDeviceUsesCDRomForISO(t *testing.T) {
	device := vmImageDiskDevice(map[string]string{"vm_boot_source_format": "iso"})

	if device.CDRom == nil {
		t.Fatalf("expected vmimage to be attached as cdrom, got %#v", device)
	}
	if device.Disk != nil {
		t.Fatalf("did not expect disk target for iso boot media, got %#v", device)
	}
}

func TestVMImageDiskDeviceUsesDiskForQCOW2(t *testing.T) {
	device := vmImageDiskDevice(map[string]string{"vm_boot_source_format": "qcow2"})

	if device.Disk == nil {
		t.Fatalf("expected vmimage to be attached as disk, got %#v", device)
	}
	if device.CDRom != nil {
		t.Fatalf("did not expect cdrom target for qcow2 boot media, got %#v", device)
	}
}

func TestResolveVMImageBootOrderPromotesISOInstallerAheadOfBlankRootDisk(t *testing.T) {
	rootBoot := uint(1)
	disks := []kubevirtv1.Disk{
		{
			Name:      "manual-root",
			BootOrder: &rootBoot,
			DiskDevice: kubevirtv1.DiskDevice{
				Disk: &kubevirtv1.DiskTarget{Bus: kubevirtv1.DiskBusSATA},
			},
		},
	}

	updated, vmImageBoot := resolveVMImageBootOrder(disks, nil, map[string]string{
		"vm_boot_source_format": "iso",
	}, false)

	if vmImageBoot != 1 {
		t.Fatalf("expected installer media boot order 1, got %d", vmImageBoot)
	}
	if updated[0].BootOrder == nil || *updated[0].BootOrder != 2 {
		t.Fatalf("expected blank root disk boot order to shift to 2, got %#v", updated[0].BootOrder)
	}
}

func TestResolveVMImageBootOrderKeepsQCOWRootOrder(t *testing.T) {
	rootBoot := uint(1)
	disks := []kubevirtv1.Disk{
		{
			Name: "manual-root",
			DiskDevice: kubevirtv1.DiskDevice{
				Disk: &kubevirtv1.DiskTarget{Bus: kubevirtv1.DiskBusSATA},
			},
		},
	}

	updated, vmImageBoot := resolveVMImageBootOrder(disks, &rootBoot, map[string]string{
		"vm_boot_source_format": "qcow2",
	}, true)

	if vmImageBoot != 1 {
		t.Fatalf("expected qcow root disk boot order 1, got %d", vmImageBoot)
	}
	if updated[0].BootOrder != nil {
		t.Fatalf("did not expect existing disks to be rewritten for qcow boot, got %#v", updated[0].BootOrder)
	}
}

func TestHasImportedVMRootDataVolumeDetectsHTTPRoot(t *testing.T) {
	templates := []kubevirtv1.DataVolumeTemplateSpec{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "manual-root",
				Annotations: map[string]string{"volume_name": "disk"},
			},
			Spec: cdiv1.DataVolumeSpec{
				Source: &cdiv1.DataVolumeSource{
					HTTP: &cdiv1.DataVolumeSourceHTTP{URL: "https://download/root.qcow2"},
				},
			},
		},
	}

	if !hasImportedVMRootDataVolume(templates) {
		t.Fatalf("expected imported root data volume to be detected")
	}
}

func TestVMRootBlankDataVolumeNameReturnsRootBlankTemplate(t *testing.T) {
	templates := []kubevirtv1.DataVolumeTemplateSpec{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "manual-root",
				Annotations: map[string]string{"volume_name": "disk"},
			},
			Spec: cdiv1.DataVolumeSpec{
				Source: &cdiv1.DataVolumeSource{
					Blank: &cdiv1.DataVolumeBlankImage{},
				},
			},
		},
	}

	if got := vmRootBlankDataVolumeName(templates); got != "manual-root" {
		t.Fatalf("expected blank root template manual-root, got %q", got)
	}
}
