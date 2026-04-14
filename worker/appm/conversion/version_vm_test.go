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

// capability_id: rainbond.worker.appm.vm-boot-media-paths
func TestResolveVMBootPathUsesISOInstallerWhenRootDiskIsBlank(t *testing.T) {
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

	if got := resolveVMBootPath(map[string]string{"vm_boot_source_format": "iso"}, templates); got != vmBootPathISOInstaller {
		t.Fatalf("expected iso installer boot path, got %q", got)
	}
}

func TestResolveVMBootPathUsesVMImageRootDiskForQCOW2WithoutImportedRoot(t *testing.T) {
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

	if got := resolveVMBootPath(map[string]string{"vm_boot_source_format": "qcow2"}, templates); got != vmBootPathVMImageRootDisk {
		t.Fatalf("expected qcow2 root-disk boot path, got %q", got)
	}
}

func TestResolveVMBootPathUsesImportedRootDiskWhenDataVolumeAlreadyImported(t *testing.T) {
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

	if got := resolveVMBootPath(map[string]string{"vm_boot_source_format": "qcow2"}, templates); got != vmBootPathImportedRootDisk {
		t.Fatalf("expected imported root-disk boot path, got %q", got)
	}
}

func TestPrepareVMImageBootVolumesForISOInstallerKeepsBlankRootDiskAndPrependsVMImage(t *testing.T) {
	volumes := []kubevirtv1.Volume{
		{Name: "manual-root", VolumeSource: kubevirtv1.VolumeSource{DataVolume: &kubevirtv1.DataVolumeSource{Name: "manual-root"}}},
	}
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

	preparedVolumes, preparedTemplates, rootBlankName := prepareVMImageBootVolumes(
		vmBootPathISOInstaller,
		"goodrain.me/default:test",
		volumes,
		templates,
	)

	if rootBlankName != "" {
		t.Fatalf("did not expect iso installer path to remove blank root disk, got %q", rootBlankName)
	}
	if len(preparedVolumes) != 2 || preparedVolumes[0].Name != "vmimage" || preparedVolumes[1].Name != "manual-root" {
		t.Fatalf("expected vmimage volume to be prepended without dropping root disk, got %#v", preparedVolumes)
	}
	if preparedVolumes[0].ContainerDisk == nil || preparedVolumes[0].ContainerDisk.Image != "goodrain.me/default:test" {
		t.Fatalf("expected vmimage containerDisk to use runtime image, got %#v", preparedVolumes[0])
	}
	if len(preparedTemplates) != 1 || preparedTemplates[0].Name != "manual-root" {
		t.Fatalf("expected iso installer path to keep blank root datavolume template, got %#v", preparedTemplates)
	}
}

func TestPrepareVMImageBootVolumesForVMImageRootDiskDropsBlankRootDisk(t *testing.T) {
	volumes := []kubevirtv1.Volume{
		{Name: "manual-root", VolumeSource: kubevirtv1.VolumeSource{DataVolume: &kubevirtv1.DataVolumeSource{Name: "manual-root"}}},
		{Name: "data-1", VolumeSource: kubevirtv1.VolumeSource{DataVolume: &kubevirtv1.DataVolumeSource{Name: "data-1"}}},
	}
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
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "data-1",
				Annotations: map[string]string{"volume_name": "data-1"},
			},
			Spec: cdiv1.DataVolumeSpec{
				Source: &cdiv1.DataVolumeSource{
					Blank: &cdiv1.DataVolumeBlankImage{},
				},
			},
		},
	}

	preparedVolumes, preparedTemplates, rootBlankName := prepareVMImageBootVolumes(
		vmBootPathVMImageRootDisk,
		"goodrain.me/default:test",
		volumes,
		templates,
	)

	if rootBlankName != "manual-root" {
		t.Fatalf("expected qcow2 path to drop root blank datavolume manual-root, got %q", rootBlankName)
	}
	if len(preparedVolumes) != 2 || preparedVolumes[0].Name != "vmimage" || preparedVolumes[1].Name != "data-1" {
		t.Fatalf("expected qcow2 path to prepend vmimage and drop blank root disk volume, got %#v", preparedVolumes)
	}
	if len(preparedTemplates) != 1 || preparedTemplates[0].Name != "data-1" {
		t.Fatalf("expected qcow2 path to drop blank root datavolume template, got %#v", preparedTemplates)
	}
}

func TestAttachISOInstallerDiskPromotesInstallerAheadOfBlankRootDisk(t *testing.T) {
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

	updated := appendISOInstallerDisk(disks, nil)

	if len(updated) != 2 {
		t.Fatalf("expected installer media and root disk, got %#v", updated)
	}
	if updated[0].Name != "manual-root" || updated[0].BootOrder == nil || *updated[0].BootOrder != 2 {
		t.Fatalf("expected iso installer path to shift root disk boot order to 2, got %#v", updated[0])
	}
	if updated[1].Name != "vmimage" || updated[1].DiskDevice.CDRom == nil {
		t.Fatalf("expected vmimage to be appended as cdrom, got %#v", updated[1])
	}
	if updated[1].BootOrder == nil || *updated[1].BootOrder != 1 {
		t.Fatalf("expected vmimage installer media to take boot order 1, got %#v", updated[1].BootOrder)
	}
}

func TestAttachVMImageRootDiskUsesRootBootOrderForQCOW2Path(t *testing.T) {
	rootBoot := uint(1)
	disks := []kubevirtv1.Disk{
		{
			Name: "data-1",
			DiskDevice: kubevirtv1.DiskDevice{
				Disk: &kubevirtv1.DiskTarget{Bus: kubevirtv1.DiskBusSATA},
			},
		},
	}

	updated := appendVMImageRootDisk(disks, &rootBoot)

	if len(updated) != 2 {
		t.Fatalf("expected vmimage root disk to be appended, got %#v", updated)
	}
	if updated[1].Name != "vmimage" || updated[1].DiskDevice.Disk == nil {
		t.Fatalf("expected vmimage to be appended as disk, got %#v", updated[1])
	}
	if updated[1].BootOrder == nil || *updated[1].BootOrder != 1 {
		t.Fatalf("expected qcow2 root disk path to use boot order 1, got %#v", updated[1].BootOrder)
	}
	if updated[0].BootOrder != nil {
		t.Fatalf("did not expect qcow2 path to rewrite unrelated data disk boot order, got %#v", updated[0].BootOrder)
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
