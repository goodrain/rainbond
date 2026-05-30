package conversion

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/goodrain/rainbond/worker/appm/volume"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

const (
	vmOSNameKey               = "vm_os_name"
	vmGPUEnabledKey           = "vm_gpu_enabled"
	vmGPUResourcesKey         = "vm_gpu_resources"
	vmGPUCountKey             = "vm_gpu_count"
	vmUSBEnabledKey           = "vm_usb_enabled"
	vmUSBResourcesKey         = "vm_usb_resources"
	vmDiskLayoutKey           = "vm_disk_layout"
	vmDiskRootKey             = "disk"
	vmDiskInstallerKey        = "vmimage"
	vmDiskRoleRoot            = "root"
	vmDiskRoleData            = "data"
	vmDiskRoleInstaller       = "installer"
	vmDiskSourceInstaller     = "installer_media"
	vmDiskSourceContainerDisk = "container_disk"
	vmDiskDeviceDisk          = "disk"
	vmDiskDeviceCDRom         = "cdrom"
	vmDiskDeviceLUN           = "lun"

	vmPrimaryNetworkName = "default"
	vmEnvVolumeName      = "rainbond-env"
	vmEnvFileName        = "rainbond.env"
	vmEnvVolumeLabel     = "RBDENV"
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
	DeviceType string `json:"device_type"`
	SourceKind string `json:"source_kind"`
	Image      string `json:"image"`
	OrderIndex int    `json:"order_index"`
	Boot       bool   `json:"boot"`
}

func buildVMRuntimeConfig(extensionSet map[string]string, envs []corev1.EnvVar) (vmRuntimeConfig, error) {
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
	if envConfigMap := buildVMEnvConfigMap(extensionSet, envs); envConfigMap != nil {
		cfg.ConfigMaps = append(cfg.ConfigMaps, envConfigMap)
		cfg.Volumes = append(cfg.Volumes, kubevirtv1.Volume{
			Name: vmEnvVolumeName,
			VolumeSource: kubevirtv1.VolumeSource{
				ConfigMap: &kubevirtv1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: envConfigMap.Name},
					VolumeLabel:          vmEnvVolumeLabel,
				},
			},
		})
		cfg.Disks = append(cfg.Disks, kubevirtv1.Disk{
			Name: vmEnvVolumeName,
			DiskDevice: kubevirtv1.DiskDevice{
				CDRom: &kubevirtv1.CDRomTarget{
					Bus: kubevirtv1.DiskBusSATA,
				},
			},
		})
	}
	return cfg, nil
}

func buildVMEnvConfigMap(extensionSet map[string]string, envs []corev1.EnvVar) *corev1.ConfigMap {
	if len(envs) == 0 {
		return nil
	}
	lines := make([]string, 0, len(envs))
	seen := make(map[string]struct{}, len(envs))
	for _, env := range envs {
		name := strings.TrimSpace(env.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		lines = append(lines, fmt.Sprintf("%s=%s", name, env.Value))
	}
	if len(lines) == 0 {
		return nil
	}
	sort.Strings(lines)
	name := vmEnvVolumeName
	if serviceID := strings.TrimSpace(extensionSet["service_id"]); serviceID != "" {
		name = fmt.Sprintf("vm-env-%s", serviceID)
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: map[string]string{
			vmEnvFileName: strings.Join(lines, "\n") + "\n",
		},
	}
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

func BuildVMAccelerationDevices(extensionSet map[string]string) ([]kubevirtv1.GPU, []kubevirtv1.HostDevice) {
	return buildVMGPUDevices(extensionSet), buildVMHostDevices(extensionSet)
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

func resolveVMInterfaceModel(extensionSet map[string]string) string {
	if looksLikeLinuxGuestHint(extensionSet[vmOSNameKey]) {
		return "virtio"
	}
	return "e1000"
}

func looksLikeLinuxGuestHint(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return false
	}
	for _, marker := range []string{
		"linux",
		"ubuntu",
		"debian",
		"centos",
		"fedora",
		"rhel",
		"red hat",
		"rocky",
		"almalinux",
		"opensuse",
		"suse",
		"oracle linux",
		"alpine",
		"arch",
		"kylin",
		"uos",
		"anolis",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
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
	for i := range layout {
		layout[i].DiskRole = strings.ToLower(strings.TrimSpace(layout[i].DiskRole))
		layout[i].SourceKind = strings.ToLower(strings.TrimSpace(layout[i].SourceKind))
		layout[i].DeviceType = strings.ToLower(strings.TrimSpace(layout[i].DeviceType))
		if layout[i].DiskRole == "" {
			if layout[i].DiskKey == vmDiskRootKey {
				layout[i].DiskRole = vmDiskRoleRoot
			} else {
				layout[i].DiskRole = vmDiskRoleData
			}
		}
		if layout[i].DiskRole == vmDiskRoleRoot {
			layout[i].DiskKey = vmDiskRootKey
		}
		if layout[i].DiskRole == vmDiskRoleInstaller || layout[i].SourceKind == vmDiskSourceInstaller || layout[i].DiskKey == vmDiskInstallerKey {
			layout[i].DiskKey = vmDiskInstallerKey
			layout[i].DiskRole = vmDiskRoleInstaller
			layout[i].SourceKind = vmDiskSourceInstaller
		}
		if layout[i].SourceKind == vmDiskSourceContainerDisk {
			layout[i].DiskRole = vmDiskRoleData
			layout[i].DeviceType = vmDiskDeviceCDRom
			layout[i].Image = strings.TrimSpace(layout[i].Image)
		}
	}
	return layout, nil
}

func applyVMDiskLayout(extensionSet map[string]string, disks []kubevirtv1.Disk, diskMeta map[string]volume.VMDiskMeta, bootPath vmBootPath) ([]kubevirtv1.Disk, error) {
	layout, err := buildVMDiskLayout(extensionSet)
	if err != nil {
		return nil, err
	}
	if len(layout) == 0 {
		return disks, nil
	}

	includeInstaller := shouldIncludeInstallerDisk(layout, bootPath)
	filtered := make([]kubevirtv1.Disk, 0, len(disks))
	for _, disk := range disks {
		if bootPath == vmBootPathISOInstaller && isVMInstallerDisk(disk) && !includeInstaller {
			continue
		}
		filtered = append(filtered, disk)
	}

	orderedDiskNames := resolveOrderedVMDiskNames(layout, filtered, diskMeta, bootPath)
	managedDiskNames := make(map[string]struct{}, len(orderedDiskNames))
	for _, name := range orderedDiskNames {
		managedDiskNames[name] = struct{}{}
	}
	for i := range filtered {
		if _, ok := managedDiskNames[filtered[i].Name]; ok {
			filtered[i].BootOrder = nil
		}
	}
	for index, name := range orderedDiskNames {
		order := uint(index + 1)
		for i := range filtered {
			if filtered[i].Name != name {
				continue
			}
			filtered[i].BootOrder = &order
			break
		}
	}
	return filtered, nil
}

func applyVMBootVolumeLayout(extensionSet map[string]string, volumes []kubevirtv1.Volume, bootPath vmBootPath) ([]kubevirtv1.Volume, error) {
	layout, err := buildVMDiskLayout(extensionSet)
	if err != nil {
		return nil, err
	}
	if len(layout) == 0 || shouldIncludeInstallerDisk(layout, bootPath) {
		return volumes, nil
	}

	filtered := make([]kubevirtv1.Volume, 0, len(volumes))
	for _, vmVolume := range volumes {
		if bootPath == vmBootPathISOInstaller && isVMInstallerVolume(vmVolume) {
			continue
		}
		filtered = append(filtered, vmVolume)
	}
	return filtered, nil
}

func shouldIncludeInstallerDisk(layout []vmDiskLayoutItem, bootPath vmBootPath) bool {
	if bootPath != vmBootPathISOInstaller {
		return false
	}
	for _, item := range layout {
		if item.SourceKind == vmDiskSourceInstaller || item.DiskRole == vmDiskRoleInstaller || item.DiskKey == vmDiskInstallerKey {
			return true
		}
	}
	return false
}

func resolveOrderedVMDiskNames(layout []vmDiskLayoutItem, disks []kubevirtv1.Disk, diskMeta map[string]volume.VMDiskMeta, bootPath vmBootPath) []string {
	ordered := make([]string, 0, len(layout))
	seen := make(map[string]struct{}, len(layout))
	for _, item := range layout {
		name, ok := resolveVMDiskNameForLayoutItem(item, diskMeta, bootPath)
		if !ok {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		ordered = append(ordered, name)
		seen[name] = struct{}{}
	}
	managedNames := make(map[string]struct{}, len(diskMeta))
	for _, meta := range diskMeta {
		managedNames[meta.DiskName] = struct{}{}
	}
	for _, disk := range disks {
		if _, exists := seen[disk.Name]; exists {
			continue
		}
		if _, managed := managedNames[disk.Name]; managed {
			ordered = append(ordered, disk.Name)
			seen[disk.Name] = struct{}{}
			continue
		}
		if bootPath == vmBootPathVMImageRootDisk && disk.Name == vmDiskInstallerKey && disk.DiskDevice.Disk != nil {
			ordered = append(ordered, disk.Name)
			seen[disk.Name] = struct{}{}
		}
	}
	return ordered
}

func resolveVMDiskNameForLayoutItem(item vmDiskLayoutItem, diskMeta map[string]volume.VMDiskMeta, bootPath vmBootPath) (string, bool) {
	if item.SourceKind == vmDiskSourceInstaller || item.DiskRole == vmDiskRoleInstaller || item.DiskKey == vmDiskInstallerKey {
		if bootPath != vmBootPathISOInstaller {
			return "", false
		}
		return vmDiskInstallerKey, true
	}
	if item.SourceKind == vmDiskSourceContainerDisk {
		if strings.TrimSpace(item.Image) == "" || strings.TrimSpace(item.DiskKey) == "" {
			return "", false
		}
		return item.DiskKey, true
	}
	if item.DiskRole == vmDiskRoleRoot && bootPath == vmBootPathVMImageRootDisk {
		return vmDiskInstallerKey, true
	}
	meta, ok := diskMeta[item.DiskKey]
	if !ok || meta.DiskName == "" {
		return "", false
	}
	return meta.DiskName, true
}

func isVMInstallerDisk(disk kubevirtv1.Disk) bool {
	return disk.Name == vmDiskInstallerKey && disk.DiskDevice.CDRom != nil
}

func isVMInstallerVolume(vmVolume kubevirtv1.Volume) bool {
	return vmVolume.Name == vmDiskInstallerKey && vmVolume.ContainerDisk != nil
}
