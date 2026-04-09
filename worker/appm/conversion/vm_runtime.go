package conversion

import (
	"encoding/json"
	"fmt"
	"strings"

	kubevirtv1 "kubevirt.io/api/core/v1"
)

const (
	vmNetworkModeKey   = "vm_network_mode"
	vmNetworkNameKey   = "vm_network_name"
	vmFixedIPKey       = "vm_fixed_ip"
	vmGPUEnabledKey    = "vm_gpu_enabled"
	vmGPUResourcesKey  = "vm_gpu_resources"
	vmUSBEnabledKey    = "vm_usb_enabled"
	vmUSBResourcesKey  = "vm_usb_resources"
	vmNetworkModeFixed = "fixed"

	vmPrimaryNetworkName   = "default"
	vmCloudInitVolumeName  = "cloudinitnetwork"
	vmCloudInitAddressName = "eth0"
)

type vmRuntimeConfig struct {
	Networks    []kubevirtv1.Network
	Interfaces  []kubevirtv1.Interface
	Volumes     []kubevirtv1.Volume
	Disks       []kubevirtv1.Disk
	GPUs        []kubevirtv1.GPU
	HostDevices []kubevirtv1.HostDevice
}

func buildVMRuntimeConfig(extensionSet map[string]string) (vmRuntimeConfig, error) {
	cfg := vmRuntimeConfig{
		Networks: []kubevirtv1.Network{
			{
				Name: vmPrimaryNetworkName,
				NetworkSource: kubevirtv1.NetworkSource{
					Pod: &kubevirtv1.PodNetwork{},
				},
			},
		},
		Interfaces: []kubevirtv1.Interface{
			{
				Name:  vmPrimaryNetworkName,
				Model: "virtio",
				InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
					Masquerade: &kubevirtv1.InterfaceMasquerade{},
				},
			},
		},
		GPUs:        buildVMGPUDevices(extensionSet),
		HostDevices: buildVMHostDevices(extensionSet),
	}

	if !strings.EqualFold(strings.TrimSpace(extensionSet[vmNetworkModeKey]), vmNetworkModeFixed) {
		return cfg, nil
	}

	networkName := strings.TrimSpace(extensionSet[vmNetworkNameKey])
	if networkName == "" {
		return vmRuntimeConfig{}, fmt.Errorf("fixed vm network mode requires vm_network_name")
	}
	fixedIP := strings.TrimSpace(extensionSet[vmFixedIPKey])
	if fixedIP == "" {
		return vmRuntimeConfig{}, fmt.Errorf("fixed vm network mode requires vm_fixed_ip")
	}

	cfg.Networks = []kubevirtv1.Network{
		{
			Name: vmPrimaryNetworkName,
			NetworkSource: kubevirtv1.NetworkSource{
				Multus: &kubevirtv1.MultusNetwork{
					NetworkName: networkName,
					Default:     true,
				},
			},
		},
	}
	cfg.Interfaces = []kubevirtv1.Interface{
		{
			Name:  vmPrimaryNetworkName,
			Model: "virtio",
			InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
				Bridge: &kubevirtv1.InterfaceBridge{},
			},
		},
	}
	cfg.Volumes = append(cfg.Volumes, kubevirtv1.Volume{
		Name: vmCloudInitVolumeName,
		VolumeSource: kubevirtv1.VolumeSource{
			CloudInitNoCloud: &kubevirtv1.CloudInitNoCloudSource{
				NetworkData: buildVMFixedIPNetworkData(fixedIP),
			},
		},
	})
	cfg.Disks = append(cfg.Disks, kubevirtv1.Disk{
		Name: vmCloudInitVolumeName,
		DiskDevice: kubevirtv1.DiskDevice{
			CDRom: &kubevirtv1.CDRomTarget{
				Bus: kubevirtv1.DiskBusSATA,
			},
		},
	})
	return cfg, nil
}

func buildVMGPUDevices(extensionSet map[string]string) []kubevirtv1.GPU {
	if !extensionEnabled(extensionSet[vmGPUEnabledKey]) {
		return nil
	}
	resourceNames := parseExtensionList(extensionSet[vmGPUResourcesKey])
	if len(resourceNames) == 0 {
		return nil
	}
	devices := make([]kubevirtv1.GPU, 0, len(resourceNames))
	for i, resourceName := range resourceNames {
		devices = append(devices, kubevirtv1.GPU{
			Name:       fmt.Sprintf("gpu-%d", i),
			DeviceName: resourceName,
		})
	}
	return devices
}

func buildVMHostDevices(extensionSet map[string]string) []kubevirtv1.HostDevice {
	if !extensionEnabled(extensionSet[vmUSBEnabledKey]) {
		return nil
	}
	resourceNames := parseExtensionList(extensionSet[vmUSBResourcesKey])
	if len(resourceNames) == 0 {
		return nil
	}
	devices := make([]kubevirtv1.HostDevice, 0, len(resourceNames))
	for i, resourceName := range resourceNames {
		devices = append(devices, kubevirtv1.HostDevice{
			Name:       fmt.Sprintf("usb-%d", i),
			DeviceName: resourceName,
		})
	}
	return devices
}

func buildVMFixedIPNetworkData(fixedIP string) string {
	return fmt.Sprintf(
		"version: 2\nethernets:\n  %s:\n    dhcp4: false\n    addresses:\n      - %s\n",
		vmCloudInitAddressName,
		fixedIP,
	)
}

func extensionEnabled(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "enabled", "on":
		return true
	default:
		return false
	}
}

func parseExtensionList(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var items []string
		if err := json.Unmarshal([]byte(trimmed), &items); err == nil {
			return normalizeExtensionItems(items)
		}
	}
	items := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == ',' || r == '\n'
	})
	return normalizeExtensionItems(items)
}

func normalizeExtensionItems(items []string) []string {
	normalized := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return normalized
}
