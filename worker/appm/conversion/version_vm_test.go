package conversion

import (
	"testing"

	kubevirtv1 "kubevirt.io/api/core/v1"
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
	found := false
	for _, name := range names {
		if name == "vm_boot_mode" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected vm_boot_mode in runtime attribute names, got %#v", names)
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
