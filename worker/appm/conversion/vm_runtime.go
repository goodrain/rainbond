package conversion

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

const (
	vmNetworkModeKey   = "vm_network_mode"
	vmNetworkNameKey   = "vm_network_name"
	vmFixedIPKey       = "vm_fixed_ip"
	vmGatewayKey       = "vm_gateway"
	vmDNSServersKey    = "vm_dns_servers"
	vmOSFamilyKey      = "vm_os_family"
	vmOSNameKey        = "vm_os_name"
	vmGPUEnabledKey    = "vm_gpu_enabled"
	vmGPUResourcesKey  = "vm_gpu_resources"
	vmGPUCountKey      = "vm_gpu_count"
	vmUSBEnabledKey    = "vm_usb_enabled"
	vmUSBResourcesKey  = "vm_usb_resources"
	vmDiskLayoutKey    = "vm_disk_layout"
	vmNetworkModeFixed = "fixed"

	vmPrimaryNetworkName   = "default"
	vmCloudInitVolumeName  = "cloudinitnetwork"
	vmCloudInitAddressName = "eth0"
	vmSysprepVolumeName    = "sysprepnetwork"
	vmSysprepScriptName    = "set-static-ip.ps1"
	vmSysprepTemplateVer   = "v2"
)

type vmRuntimeConfig struct {
	Networks    []kubevirtv1.Network
	Interfaces  []kubevirtv1.Interface
	Volumes     []kubevirtv1.Volume
	Disks       []kubevirtv1.Disk
	ConfigMaps  []*corev1.ConfigMap
	GPUs        []kubevirtv1.GPU
	HostDevices []kubevirtv1.HostDevice
}

type vmDiskLayoutItem struct {
	DiskKey    string `json:"disk_key"`
	DiskName   string `json:"disk_name"`
	DiskRole   string `json:"disk_role"`
	OrderIndex int    `json:"order_index"`
	Boot       bool   `json:"boot"`
}

func buildVMRuntimeConfig(extensionSet map[string]string) (vmRuntimeConfig, error) {
	interfaceModel := resolveVMInterfaceModel(extensionSet)
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
				Model: interfaceModel,
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

	fixedIP := strings.TrimSpace(extensionSet[vmFixedIPKey])
	if fixedIP == "" {
		return vmRuntimeConfig{}, fmt.Errorf("fixed vm network mode requires vm_fixed_ip")
	}
	gateway := strings.TrimSpace(extensionSet[vmGatewayKey])
	dnsServers := parseExtensionList(extensionSet[vmDNSServersKey])
	cfg.Interfaces = []kubevirtv1.Interface{
		{
			Name:  vmPrimaryNetworkName,
			Model: interfaceModel,
			InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
				Bridge: &kubevirtv1.InterfaceBridge{},
			},
		},
	}
	if useVMFixedPodNetwork(extensionSet) {
		return cfg, nil
	}

	networkName := strings.TrimSpace(extensionSet[vmNetworkNameKey])
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
	if resolveVMOSFamily(extensionSet) == "windows" {
		unattendXML := buildVMSysprepUnattendXML(vmSysprepScriptName)
		networkScript := buildVMSysprepNetworkScript(fixedIP, gateway, dnsServers)
		configMapName := buildVMSysprepConfigMapName(networkName, fixedIP, unattendXML, networkScript)
		cfg.ConfigMaps = append(cfg.ConfigMaps, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: configMapName,
			},
			Data: map[string]string{
				"autounattend.xml":  unattendXML,
				"unattend.xml":      unattendXML,
				vmSysprepScriptName: networkScript,
			},
		})
		cfg.Volumes = append(cfg.Volumes, kubevirtv1.Volume{
			Name: vmSysprepVolumeName,
			VolumeSource: kubevirtv1.VolumeSource{
				Sysprep: &kubevirtv1.SysprepSource{
					ConfigMap: &corev1.LocalObjectReference{Name: configMapName},
				},
			},
		})
		cfg.Disks = append(cfg.Disks, kubevirtv1.Disk{
			Name: vmSysprepVolumeName,
			DiskDevice: kubevirtv1.DiskDevice{
				CDRom: &kubevirtv1.CDRomTarget{
					Bus: kubevirtv1.DiskBusSATA,
				},
			},
		})
		return cfg, nil
	}
	cfg.Volumes = append(cfg.Volumes, kubevirtv1.Volume{
		Name: vmCloudInitVolumeName,
		VolumeSource: kubevirtv1.VolumeSource{
			CloudInitNoCloud: &kubevirtv1.CloudInitNoCloudSource{
				NetworkData: buildVMFixedIPNetworkData(fixedIP, gateway, dnsServers),
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

func useVMFixedPodNetwork(extensionSet map[string]string) bool {
	return strings.EqualFold(strings.TrimSpace(extensionSet[vmNetworkModeKey]), vmNetworkModeFixed) &&
		strings.TrimSpace(extensionSet[vmNetworkNameKey]) == ""
}

func resolveVMFixedPodIPAnnotationValue(extensionSet map[string]string) string {
	if !useVMFixedPodNetwork(extensionSet) {
		return ""
	}
	fixedIP := strings.TrimSpace(extensionSet[vmFixedIPKey])
	if fixedIP == "" {
		return ""
	}
	ip, _, _ := strings.Cut(fixedIP, "/")
	return strings.TrimSpace(ip)
}

func buildVMGPUDevices(extensionSet map[string]string) []kubevirtv1.GPU {
	if !extensionEnabled(extensionSet[vmGPUEnabledKey]) {
		return nil
	}
	resourceNames := parseExtensionList(extensionSet[vmGPUResourcesKey])
	if len(resourceNames) == 0 {
		return nil
	}
	resourceNames = expandVMGPUResourceNames(resourceNames, extensionSet[vmGPUCountKey])
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

func expandVMGPUResourceNames(resourceNames []string, countValue string) []string {
	gpuCount := parsePositiveInt(countValue)
	if gpuCount <= 1 || len(resourceNames) != 1 {
		return resourceNames
	}
	expanded := make([]string, 0, gpuCount)
	for i := 0; i < gpuCount; i++ {
		expanded = append(expanded, resourceNames[0])
	}
	return expanded
}

func parsePositiveInt(value string) int {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}
	count, err := strconv.Atoi(trimmed)
	if err != nil || count < 1 {
		return 0
	}
	return count
}

func buildVMFixedIPNetworkData(fixedIP, gateway string, dnsServers []string) string {
	lines := []string{
		"version: 2",
		"ethernets:",
		fmt.Sprintf("  %s:", vmCloudInitAddressName),
		"    dhcp4: false",
		"    addresses:",
		fmt.Sprintf("      - %s", fixedIP),
	}
	if gateway != "" {
		lines = append(lines, fmt.Sprintf("    gateway4: %s", gateway))
	}
	if len(dnsServers) > 0 {
		lines = append(lines, "    nameservers:", "      addresses:")
		for _, dnsServer := range dnsServers {
			lines = append(lines, fmt.Sprintf("        - %s", dnsServer))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func resolveVMOSFamily(extensionSet map[string]string) string {
	explicit := strings.ToLower(strings.TrimSpace(extensionSet[vmOSFamilyKey]))
	switch explicit {
	case "windows", "linux":
		return explicit
	}
	if strings.Contains(strings.ToLower(strings.TrimSpace(extensionSet[vmOSNameKey])), "windows") {
		return "windows"
	}
	return "linux"
}

func resolveVMInterfaceModel(extensionSet map[string]string) string {
	if resolveVMOSFamily(extensionSet) == "windows" {
		return "e1000"
	}
	return "virtio"
}

func buildVMSysprepConfigMapName(networkName, fixedIP string, payloads ...string) string {
	builder := strings.TrimSpace(networkName) + "|" + strings.TrimSpace(fixedIP) + "|" + vmSysprepTemplateVer
	for _, payload := range payloads {
		builder += "|" + payload
	}
	sum := sha1.Sum([]byte(builder))
	return fmt.Sprintf("vm-sysprep-%x", sum[:6])
}

func buildVMSysprepUnattendXML(scriptName string) string {
	command := fmt.Sprintf(
		`powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File D:\%s`,
		scriptName,
	)
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<unattend xmlns="urn:schemas-microsoft-com:unattend" xmlns:wcm="http://schemas.microsoft.com/WMIConfig/2002/State">
  <settings pass="specialize">
    <component name="Microsoft-Windows-Deployment" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS">
      <RunSynchronous>
        <RunSynchronousCommand wcm:action="add">
          <Order>1</Order>
          <Description>Configure static IPv4</Description>
          <Path>%s</Path>
        </RunSynchronousCommand>
      </RunSynchronous>
    </component>
  </settings>
</unattend>
`, command)
}

func buildVMSysprepNetworkScript(fixedIP, gateway string, dnsServers []string) string {
	address, prefixLength := splitVMFixedIPCIDR(fixedIP)
	networkCmd := fmt.Sprintf(
		`  New-NetIPAddress -InterfaceAlias $adapter -IPAddress '%s' -PrefixLength %s -Type Unicast -ErrorAction Stop`,
		address,
		prefixLength,
	)
	if gateway != "" {
		networkCmd = fmt.Sprintf(
			`  New-NetIPAddress -InterfaceAlias $adapter -IPAddress '%s' -PrefixLength %s -DefaultGateway '%s' -Type Unicast -ErrorAction Stop`,
			address,
			prefixLength,
			gateway,
		)
	}
	var dnsCmd string
	if len(dnsServers) > 0 {
		quoted := make([]string, 0, len(dnsServers))
		for _, dnsServer := range dnsServers {
			quoted = append(quoted, fmt.Sprintf("'%s'", dnsServer))
		}
		dnsCmd = fmt.Sprintf("\n  Set-DnsClientServerAddress -InterfaceAlias $adapter -ServerAddresses @(%s) -ErrorAction SilentlyContinue", strings.Join(quoted, ", "))
	}
	return fmt.Sprintf(`$adapter = Get-NetAdapter | Where-Object {$_.Status -ne 'Disabled'} | Sort-Object ifIndex | Select-Object -First 1 -ExpandProperty Name
if ($adapter) {
  Set-NetIPInterface -InterfaceAlias $adapter -AddressFamily IPv4 -Dhcp Disabled -ErrorAction SilentlyContinue
  Get-NetIPAddress -InterfaceAlias $adapter -AddressFamily IPv4 -ErrorAction SilentlyContinue | Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue
%s%s
}
`, networkCmd, dnsCmd)
}

func splitVMFixedIPCIDR(fixedIP string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(fixedIP), "/", 2)
	address := parts[0]
	if len(parts) == 2 && parts[1] != "" {
		return address, parts[1]
	}
	return address, "24"
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

func buildVMDiskLayout(extensionSet map[string]string) ([]vmDiskLayoutItem, error) {
	raw := strings.TrimSpace(extensionSet[vmDiskLayoutKey])
	if raw == "" {
		return nil, nil
	}
	var layout []vmDiskLayoutItem
	if err := json.Unmarshal([]byte(raw), &layout); err != nil {
		return nil, fmt.Errorf("invalid vm_disk_layout: %w", err)
	}
	sort.SliceStable(layout, func(i, j int) bool {
		if layout[i].OrderIndex == layout[j].OrderIndex {
			if layout[i].DiskRole == layout[j].DiskRole {
				return layout[i].DiskKey < layout[j].DiskKey
			}
			return layout[i].DiskRole == "root"
		}
		return layout[i].OrderIndex < layout[j].OrderIndex
	})
	return layout, nil
}

func applyVMDiskLayout(extensionSet map[string]string, dataDisks []kubevirtv1.Disk) ([]kubevirtv1.Disk, *uint, error) {
	layout, err := buildVMDiskLayout(extensionSet)
	if err != nil {
		return nil, nil, err
	}
	if len(layout) == 0 {
		return dataDisks, nil, nil
	}

	applied := make([]kubevirtv1.Disk, len(dataDisks))
	copy(applied, dataDisks)

	var rootBootOrder *uint
	dataBootOrders := make([]uint, 0)
	bootOrder := uint(1)
	for _, item := range layout {
		if item.DiskRole == "root" {
			order := bootOrder
			rootBootOrder = &order
		} else {
			dataBootOrders = append(dataBootOrders, bootOrder)
		}
		bootOrder++
	}

	for i := range applied {
		if i >= len(dataBootOrders) {
			break
		}
		order := dataBootOrders[i]
		applied[i].BootOrder = &order
	}

	return applied, rootBootOrder, nil
}
