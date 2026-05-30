package conversion

// capability_id: rainbond.worker.appm.vm-container-disk-cdrom

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"github.com/goodrain/rainbond/worker/appm/volume"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

func TestBuildVMRuntimeConfigRandomNetwork(t *testing.T) {
	cfg, err := buildVMRuntimeConfig(nil, nil)
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
	if cfg.Interfaces[0].Model != "e1000" {
		t.Fatalf("expected unknown guest network to default to e1000, got %q", cfg.Interfaces[0].Model)
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

func TestBuildVMRuntimeConfigRandomWindowsNetworkUsesE1000(t *testing.T) {
	cfg, err := buildVMRuntimeConfig(map[string]string{
		"vm_os_name": "Windows Server 2022",
	}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.Interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(cfg.Interfaces))
	}
	if cfg.Interfaces[0].Model != "e1000" {
		t.Fatalf("expected windows random network to use e1000, got %q", cfg.Interfaces[0].Model)
	}
	if cfg.Interfaces[0].Masquerade == nil {
		t.Fatalf("expected windows random network to keep masquerade binding")
	}
}

func TestBuildVMRuntimeConfigRecognizedLinuxNameUsesVirtio(t *testing.T) {
	cfg, err := buildVMRuntimeConfig(map[string]string{
		"vm_os_name": "Ubuntu 22.04.5 LTS",
	}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.Interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(cfg.Interfaces))
	}
	if cfg.Interfaces[0].Model != "virtio" {
		t.Fatalf("expected recognized linux guest to use virtio, got %q", cfg.Interfaces[0].Model)
	}
	if cfg.Interfaces[0].Masquerade == nil {
		t.Fatalf("expected recognized linux guest to keep masquerade binding")
	}
}

func TestBuildVMRuntimeConfigIgnoresRemovedNetworkFields(t *testing.T) {
	cfg, err := buildVMRuntimeConfig(map[string]string{
		"vm_network_mode": "fixed",
		"vm_network_name": "default/bridge-test",
		"vm_fixed_ip":     "10.250.250.10/24",
		"vm_gateway":      "10.250.250.1",
		"vm_dns_servers":  "223.5.5.5,8.8.8.8",
	}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.Networks) != 1 {
		t.Fatalf("expected 1 network, got %d", len(cfg.Networks))
	}
	if cfg.Networks[0].Pod == nil {
		t.Fatalf("expected removed network fields to keep default pod network")
	}
	if len(cfg.Interfaces) != 1 || cfg.Interfaces[0].Masquerade == nil {
		t.Fatalf("expected removed network fields to keep masquerade interface")
	}
	if len(cfg.Volumes) != 0 {
		t.Fatalf("expected no network helper volumes once fixed ip is removed, got %d", len(cfg.Volumes))
	}
	if len(cfg.Disks) != 0 {
		t.Fatalf("expected no network helper disks once fixed ip is removed, got %d", len(cfg.Disks))
	}
}

func TestBuildVMGPUDevices(t *testing.T) {
	cfg, err := buildVMRuntimeConfig(map[string]string{
		"vm_gpu_enabled":   "true",
		"vm_gpu_resources": "nvidia.com/TU104GL_Tesla_T4,gpu.example.com/A10",
	}, nil)
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
	}, nil)
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

func TestBuildVMRuntimeConfigInjectsEnvConfigMapVolume(t *testing.T) {
	cfg, err := buildVMRuntimeConfig(map[string]string{
		"vm_os_name": "Windows Server 2022",
	}, []corev1.EnvVar{
		{Name: "DEMO_HOST", Value: "demo-service"},
		{Name: "DEMO_PORT", Value: "8080"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.ConfigMaps) != 1 {
		t.Fatalf("expected one env configmap, got %d", len(cfg.ConfigMaps))
	}
	content := cfg.ConfigMaps[0].Data["rainbond.env"]
	if content == "" {
		t.Fatal("expected rainbond.env content to be generated")
	}
	if content != "DEMO_HOST=demo-service\nDEMO_PORT=8080\n" {
		t.Fatalf("unexpected rainbond.env content: %q", content)
	}
	if len(cfg.Volumes) != 1 {
		t.Fatalf("expected one injected volume, got %d", len(cfg.Volumes))
	}
	if cfg.Volumes[0].ConfigMap == nil {
		t.Fatalf("expected injected volume to use configMap source, got %#v", cfg.Volumes[0].VolumeSource)
	}
	if cfg.Volumes[0].ConfigMap.LocalObjectReference.Name != cfg.ConfigMaps[0].Name {
		t.Fatalf("expected configmap volume name %q, got %q", cfg.ConfigMaps[0].Name, cfg.Volumes[0].ConfigMap.LocalObjectReference.Name)
	}
	if cfg.Volumes[0].ConfigMap.VolumeLabel != "RBDENV" {
		t.Fatalf("expected env volume label RBDENV, got %q", cfg.Volumes[0].ConfigMap.VolumeLabel)
	}
	if len(cfg.Disks) != 1 {
		t.Fatalf("expected one injected disk, got %d", len(cfg.Disks))
	}
	if cfg.Disks[0].Name != cfg.Volumes[0].Name {
		t.Fatalf("expected disk name %q, got %q", cfg.Volumes[0].Name, cfg.Disks[0].Name)
	}
	if cfg.Disks[0].DiskDevice.CDRom == nil {
		t.Fatalf("expected env disk to be attached as cdrom, got %#v", cfg.Disks[0].DiskDevice)
	}
	if cfg.Disks[0].DiskDevice.CDRom.Bus != kubevirtv1.DiskBusSATA {
		t.Fatalf("expected env cdrom bus sata, got %q", cfg.Disks[0].DiskDevice.CDRom.Bus)
	}
}

func TestBuildVMRuntimeConfigSkipsDuplicateAndBlankEnvNames(t *testing.T) {
	cfg, err := buildVMRuntimeConfig(nil, []corev1.EnvVar{
		{Name: "DEMO_HOST", Value: "demo-service"},
		{Name: "", Value: "ignored"},
		{Name: "DEMO_HOST", Value: "overwritten"},
		{Name: "DEMO_PORT", Value: "8080"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	content := cfg.ConfigMaps[0].Data["rainbond.env"]
	if content != "DEMO_HOST=demo-service\nDEMO_PORT=8080\n" {
		t.Fatalf("unexpected deduplicated env content: %q", content)
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

func TestBuildVMDiskLayoutKeepsContainerDiskImage(t *testing.T) {
	layout, err := buildVMDiskLayout(map[string]string{
		"vm_disk_layout": `[{"disk_key":"driver-media","disk_name":"driver-media","disk_role":"data","device_type":"cdrom","source_kind":"container_disk","image":"registry.example.com/team/windows-driver:virtio","order_index":1}]`,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(layout) != 1 {
		t.Fatalf("expected one layout item, got %#v", layout)
	}
	if layout[0].SourceKind != vmDiskSourceContainerDisk {
		t.Fatalf("expected container disk source kind, got %#v", layout[0])
	}
	if layout[0].DeviceType != vmDiskDeviceCDRom {
		t.Fatalf("expected container disk to stay cdrom, got %#v", layout[0])
	}
	if layout[0].Image != "registry.example.com/team/windows-driver:virtio" {
		t.Fatalf("expected image to be preserved, got %#v", layout[0])
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
	meta := map[string]volume.VMDiskMeta{
		"disk":   {DiskName: "manual1", DiskKey: "disk"},
		"data-2": {DiskName: "manual2", DiskKey: "data-2"},
	}

	applied, err := applyVMDiskLayout(map[string]string{
		"vm_disk_layout": `[{"disk_key":"disk","disk_role":"root","source_kind":"volume","order_index":0,"boot":true},{"disk_key":"data-2","disk_role":"data","source_kind":"volume","order_index":1,"boot":false}]`,
	}, dataDisks, meta, vmBootPathImportedRootDisk)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if applied[0].BootOrder == nil || *applied[0].BootOrder != 1 {
		t.Fatalf("expected root disk boot order 1, got %#v", applied[0].BootOrder)
	}
	if applied[1].BootOrder == nil || *applied[1].BootOrder != 2 {
		t.Fatalf("expected data disk boot order 2, got %#v", applied[1].BootOrder)
	}
}

func TestApplyVMDiskLayoutDropsInstallerDiskWhenLayoutRemovesIt(t *testing.T) {
	disks := []kubevirtv1.Disk{
		{
			Name: "manual1",
			DiskDevice: kubevirtv1.DiskDevice{
				Disk: &kubevirtv1.DiskTarget{Bus: kubevirtv1.DiskBusSATA},
			},
		},
		{
			Name: "vmimage",
			DiskDevice: kubevirtv1.DiskDevice{
				CDRom: &kubevirtv1.CDRomTarget{Bus: kubevirtv1.DiskBusSATA},
			},
		},
	}
	meta := map[string]volume.VMDiskMeta{
		"disk": {DiskName: "manual1", DiskKey: "disk"},
	}

	applied, err := applyVMDiskLayout(map[string]string{
		"vm_disk_layout": `[{"disk_key":"disk","disk_role":"root","source_kind":"volume","order_index":0,"boot":true}]`,
	}, disks, meta, vmBootPathISOInstaller)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(applied) != 1 {
		t.Fatalf("expected installer disk to be removed, got %#v", applied)
	}
	if applied[0].Name != "manual1" {
		t.Fatalf("expected root disk to remain, got %#v", applied[0])
	}
	if applied[0].BootOrder == nil || *applied[0].BootOrder != 1 {
		t.Fatalf("expected root disk boot order 1 after removing installer, got %#v", applied[0].BootOrder)
	}
}

func TestAppendVMContainerDiskCDROMsCreatesContainerDiskVolumeAndDisk(t *testing.T) {
	layout := []vmDiskLayoutItem{
		{
			DiskKey:    "driver-media",
			DiskName:   "driver-media",
			DiskRole:   vmDiskRoleData,
			DeviceType: vmDiskDeviceCDRom,
			SourceKind: vmDiskSourceContainerDisk,
			Image:      "registry.example.com/team/windows-driver:virtio",
			OrderIndex: 1,
		},
	}

	volumes, disks := appendVMContainerDiskCDROMs(nil, nil, layout)

	if len(volumes) != 1 {
		t.Fatalf("expected one container disk volume, got %#v", volumes)
	}
	if volumes[0].Name != "driver-media" || volumes[0].ContainerDisk == nil {
		t.Fatalf("expected driver-media container disk volume, got %#v", volumes[0])
	}
	if volumes[0].ContainerDisk.Image != "registry.example.com/team/windows-driver:virtio" {
		t.Fatalf("unexpected container disk image: %#v", volumes[0].ContainerDisk)
	}
	if len(disks) != 1 {
		t.Fatalf("expected one cdrom disk, got %#v", disks)
	}
	if disks[0].Name != "driver-media" || disks[0].DiskDevice.CDRom == nil {
		t.Fatalf("expected driver-media cdrom disk, got %#v", disks[0])
	}
}
