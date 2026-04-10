package conversion

import (
	"testing"

	kubevirtv1 "kubevirt.io/api/core/v1"
)

func TestBuildVMRuntimeConfigRandomNetwork(t *testing.T) {
	cfg, err := buildVMRuntimeConfig(nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.Networks) != 1 {
		t.Fatalf("expected 1 network, got %d", len(cfg.Networks))
	}
	if cfg.Networks[0].Pod == nil {
		t.Fatalf("expected default pod network")
	}
	if len(cfg.Interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(cfg.Interfaces))
	}
	if cfg.Interfaces[0].Masquerade == nil {
		t.Fatalf("expected default masquerade interface")
	}
	if len(cfg.Volumes) != 0 {
		t.Fatalf("expected no extra volumes for random network, got %d", len(cfg.Volumes))
	}
	if len(cfg.Disks) != 0 {
		t.Fatalf("expected no extra disks for random network, got %d", len(cfg.Disks))
	}
}

func TestBuildVMRuntimeConfigFixedNetwork(t *testing.T) {
	cfg, err := buildVMRuntimeConfig(map[string]string{
		"vm_network_mode": "fixed",
		"vm_network_name": "default/bridge-test",
		"vm_fixed_ip":     "10.250.250.10/24",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.Networks) != 1 {
		t.Fatalf("expected 1 network, got %d", len(cfg.Networks))
	}
	if cfg.Networks[0].Multus == nil {
		t.Fatalf("expected multus network")
	}
	if cfg.Networks[0].Multus.NetworkName != "default/bridge-test" {
		t.Fatalf("expected network name default/bridge-test, got %s", cfg.Networks[0].Multus.NetworkName)
	}
	if !cfg.Networks[0].Multus.Default {
		t.Fatalf("expected fixed network to be marked as default")
	}
	if len(cfg.Interfaces) != 1 || cfg.Interfaces[0].Bridge == nil {
		t.Fatalf("expected bridge interface for fixed network")
	}
	if len(cfg.Volumes) != 1 {
		t.Fatalf("expected 1 cloud-init volume, got %d", len(cfg.Volumes))
	}
	if cfg.Volumes[0].CloudInitNoCloud == nil {
		t.Fatalf("expected cloud-init network volume")
	}
	if got := cfg.Volumes[0].CloudInitNoCloud.NetworkData; got == "" || got == "10.250.250.10/24" {
		t.Fatalf("expected rendered cloud-init network data, got %q", got)
	}
	if len(cfg.Disks) != 1 {
		t.Fatalf("expected 1 cloud-init disk, got %d", len(cfg.Disks))
	}
	if cfg.Disks[0].CDRom == nil {
		t.Fatalf("expected cloud-init disk to be attached as cdrom")
	}
}

func TestBuildVMRuntimeConfigRequiresNetworkForFixedIP(t *testing.T) {
	_, err := buildVMRuntimeConfig(map[string]string{
		"vm_network_mode": "fixed",
		"vm_fixed_ip":     "10.250.250.10/24",
	})
	if err == nil {
		t.Fatal("expected error when fixed network name is missing")
	}
}

func TestBuildVMGPUDevices(t *testing.T) {
	cfg, err := buildVMRuntimeConfig(map[string]string{
		"vm_gpu_enabled":   "true",
		"vm_gpu_resources": "nvidia.com/TU104GL_Tesla_T4,gpu.example.com/A10",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.GPUs) != 2 {
		t.Fatalf("expected 2 gpus, got %d", len(cfg.GPUs))
	}
	if cfg.GPUs[0].Name != "gpu-0" || cfg.GPUs[0].DeviceName != "nvidia.com/TU104GL_Tesla_T4" {
		t.Fatalf("unexpected first gpu: %#v", cfg.GPUs[0])
	}
	if cfg.GPUs[1].Name != "gpu-1" || cfg.GPUs[1].DeviceName != "gpu.example.com/A10" {
		t.Fatalf("unexpected second gpu: %#v", cfg.GPUs[1])
	}
}

func TestBuildVMHostDevices(t *testing.T) {
	cfg, err := buildVMRuntimeConfig(map[string]string{
		"vm_usb_enabled":   "true",
		"vm_usb_resources": "[\"kubevirt.io/usb-a\",\"kubevirt.io/usb-b\"]",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.HostDevices) != 2 {
		t.Fatalf("expected 2 host devices, got %d", len(cfg.HostDevices))
	}
	if cfg.HostDevices[0].Name != "usb-0" || cfg.HostDevices[0].DeviceName != "kubevirt.io/usb-a" {
		t.Fatalf("unexpected first host device: %#v", cfg.HostDevices[0])
	}
	if cfg.HostDevices[1].Name != "usb-1" || cfg.HostDevices[1].DeviceName != "kubevirt.io/usb-b" {
		t.Fatalf("unexpected second host device: %#v", cfg.HostDevices[1])
	}
}

func TestBuildVMDiskLayoutParsesJSON(t *testing.T) {
	layout, err := buildVMDiskLayout(map[string]string{
		"vm_disk_layout": `[{"disk_key":"rootdisk","disk_role":"root","order_index":0,"boot":true},{"disk_key":"data-1","disk_role":"data","order_index":1,"boot":false}]`,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(layout) != 2 {
		t.Fatalf("expected 2 layout items, got %d", len(layout))
	}
	if layout[0].DiskRole != "root" || layout[1].DiskRole != "data" {
		t.Fatalf("unexpected layout order: %#v", layout)
	}
}

func TestBuildVMDiskLayoutRejectsInvalidJSON(t *testing.T) {
	_, err := buildVMDiskLayout(map[string]string{
		"vm_disk_layout": `{"invalid":true}`,
	})
	if err == nil {
		t.Fatal("expected error for invalid vm_disk_layout json")
	}
}

func TestApplyVMDiskLayoutAssignsRootAndDataBootOrders(t *testing.T) {
	dataDisks := []kubevirtv1.Disk{
		{
			Name: "manual1",
			DiskDevice: kubevirtv1.DiskDevice{
				Disk: &kubevirtv1.DiskTarget{Bus: kubevirtv1.DiskBusSATA},
			},
		},
		{
			Name: "manual2",
			DiskDevice: kubevirtv1.DiskDevice{
				Disk: &kubevirtv1.DiskTarget{Bus: kubevirtv1.DiskBusSATA},
			},
		},
	}

	applied, rootBootOrder, err := applyVMDiskLayout(map[string]string{
		"vm_disk_layout": `[{"disk_key":"rootdisk","disk_role":"root","order_index":0,"boot":true},{"disk_key":"data-1","disk_role":"data","order_index":1,"boot":false},{"disk_key":"data-2","disk_role":"data","order_index":2,"boot":false}]`,
	}, dataDisks)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if rootBootOrder == nil || *rootBootOrder != 1 {
		t.Fatalf("expected root boot order 1, got %#v", rootBootOrder)
	}
	if applied[0].BootOrder == nil || *applied[0].BootOrder != 2 {
		t.Fatalf("expected first data disk boot order 2, got %#v", applied[0].BootOrder)
	}
	if applied[1].BootOrder == nil || *applied[1].BootOrder != 3 {
		t.Fatalf("expected second data disk boot order 3, got %#v", applied[1].BootOrder)
	}
}
