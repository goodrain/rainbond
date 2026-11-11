package conversion

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/goodrain/rainbond/worker/appm/volume"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

const (
	vmOSNameKey               = "vm_os_name"
	vmBootSourceFormatKey     = "vm_boot_source_format"
	vmSSHUsernameKey          = "vm_ssh_username"
	vmSSHPublicKeyKey         = "vm_ssh_authorized_key"
	vmLoginPasswordHashKey    = VMLoginPasswordHashAttribute
	vmSSHAuthorizedKeyLimit   = 16 * 1024
	vmLoginPasswordHashLimit  = 256
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
	vmGuestSyncVolume    = "rainbond-guest-sync"
)

// VMLoginPasswordHashAttribute is the managed VM login verifier attribute.
const VMLoginPasswordHashAttribute = "vm_login_password_hash"

var vmLinuxUsernamePattern = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
var vmSHA512CryptPattern = regexp.MustCompile(`^\$6\$rounds=(\d{6,7})\$([./0-9A-Za-z]{16})\$([./0-9A-Za-z]{86})$`)

type vmLoginMode string

const (
	vmLoginModeNone     vmLoginMode = "none"
	vmLoginModeSSHKey   vmLoginMode = "ssh_key"
	vmLoginModePassword vmLoginMode = "password"
)

type vmLoginBootstrap struct {
	Mode          vmLoginMode
	Username      string
	AuthorizedKey string
	PasswordHash  string
}

type vmRuntimeConfig struct {
	// RootDiskBus stays SATA unless login bootstrap was validated for a Linux disk image.
	RootDiskBus kubevirtv1.DiskBus
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

func buildVMRuntimeConfig(extensionSet map[string]string, envs []corev1.EnvVar, envVarSecrets []*corev1.Secret,
	guestFiles []volume.VMGuestFile) (vmRuntimeConfig, error) {
	login, err := validateVMLoginBootstrap(extensionSet)
	if err != nil {
		return vmRuntimeConfig{}, err
	}
	if login.Mode != vmLoginModeNone && (!looksLikeLinuxGuestHint(extensionSet[vmOSNameKey]) || vmBootSourceUsesCDRom(extensionSet)) {
		return vmRuntimeConfig{}, errors.New("VM login bootstrap requires a Linux disk image")
	}
	interfaceModel := resolveVMInterfaceModel()
	cfg := vmRuntimeConfig{
		RootDiskBus: kubevirtv1.DiskBusSATA,
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
					Bridge: &kubevirtv1.InterfaceBridge{},
				},
			},
		},
		GPUs:        buildVMGPUDevices(extensionSet),
		HostDevices: buildVMHostDevices(extensionSet),
	}
	if login.Mode != vmLoginModeNone {
		cfg.RootDiskBus = kubevirtv1.DiskBusVirtio
	}
	if envConfigMap := buildVMEnvConfigMap(extensionSet, envs, envVarSecrets); envConfigMap != nil {
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
	syncVolume, err := buildVMGuestSyncVolume(extensionSet, guestFiles, login)
	if err != nil {
		return vmRuntimeConfig{}, err
	}
	if syncVolume != nil {
		cfg.Volumes = append(cfg.Volumes, *syncVolume)
		syncDiskDevice := kubevirtv1.DiskDevice{
			CDRom: &kubevirtv1.CDRomTarget{
				Bus: kubevirtv1.DiskBusSATA,
			},
		}
		if login.Mode != vmLoginModeNone {
			syncDiskDevice = kubevirtv1.DiskDevice{
				Disk: &kubevirtv1.DiskTarget{Bus: cfg.RootDiskBus},
			}
		}
		cfg.Disks = append(cfg.Disks, kubevirtv1.Disk{
			Name:       vmGuestSyncVolume,
			DiskDevice: syncDiskDevice,
		})
	}
	return cfg, nil
}

func applyVMLoginRootDiskBus(disks []kubevirtv1.Disk, diskMeta map[string]volume.VMDiskMeta,
	bootPath vmBootPath, rootDiskBus kubevirtv1.DiskBus) []kubevirtv1.Disk {
	if bootPath != vmBootPathImportedRootDisk || rootDiskBus != kubevirtv1.DiskBusVirtio {
		return disks
	}
	rootMeta, ok := diskMeta[vmDiskRootKey]
	if !ok || rootMeta.DiskKey != vmDiskRootKey || rootMeta.DeviceType != vmDiskDeviceDisk || rootMeta.DiskName == "" {
		return disks
	}

	for index := range disks {
		if disks[index].Name != rootMeta.DiskName || disks[index].DiskDevice.Disk == nil {
			continue
		}
		updated := append([]kubevirtv1.Disk(nil), disks...)
		target := *updated[index].DiskDevice.Disk
		target.Bus = rootDiskBus
		updated[index].DiskDevice.Disk = &target
		return updated
	}
	return disks
}

func buildVMEnvConfigMap(extensionSet map[string]string, envs []corev1.EnvVar, envVarSecrets []*corev1.Secret) *corev1.ConfigMap {
	if len(envs) == 0 && len(envVarSecrets) == 0 {
		return nil
	}
	values := make(map[string]string, len(envs))
	lines := make([]string, 0, len(envs)+len(envVarSecrets))
	seen := make(map[string]struct{}, len(envs)+len(envVarSecrets))
	for _, secret := range envVarSecrets {
		for key, value := range secret.Data {
			name := strings.TrimSpace(key)
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			values[name] = string(value)
			lines = append(lines, fmt.Sprintf("%s=%s", name, values[name]))
		}
	}
	for _, env := range envs {
		name := strings.TrimSpace(env.Name)
		if name == "" {
			continue
		}
		if env.ValueFrom != nil {
			continue
		}
		if _, ok := seen[name]; ok {
			for i := range lines {
				if strings.HasPrefix(lines[i], name+"=") {
					lines[i] = fmt.Sprintf("%s=%s", name, env.Value)
					break
				}
			}
			values[name] = env.Value
			continue
		}
		seen[name] = struct{}{}
		values[name] = env.Value
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

func validateVMLoginBootstrap(extensionSet map[string]string) (vmLoginBootstrap, error) {
	username := strings.TrimSpace(extensionSet[vmSSHUsernameKey])
	publicKey := strings.TrimSpace(extensionSet[vmSSHPublicKeyKey])
	passwordHash := extensionSet[vmLoginPasswordHashKey]
	if extensionSet["vm_login_password"] != "" {
		return vmLoginBootstrap{}, errors.New("invalid VM login configuration")
	}
	if username == "" && publicKey == "" && passwordHash == "" {
		return vmLoginBootstrap{Mode: vmLoginModeNone}, nil
	}
	if username == "" || !vmLinuxUsernamePattern.MatchString(username) || publicKey != "" && passwordHash != "" {
		return vmLoginBootstrap{}, errors.New("invalid VM login configuration")
	}
	if publicKey != "" {
		if len(publicKey) > vmSSHAuthorizedKeyLimit || strings.ContainsAny(publicKey, "\r\n") || strings.Contains(publicKey, "PRIVATE KEY") {
			return vmLoginBootstrap{}, errors.New("invalid VM SSH public key")
		}
		parsed, _, options, rest, err := ssh.ParseAuthorizedKey([]byte(publicKey))
		if err != nil || len(options) != 0 || len(bytes.TrimSpace(rest)) != 0 {
			return vmLoginBootstrap{}, errors.New("invalid VM SSH public key")
		}
		return vmLoginBootstrap{
			Mode:          vmLoginModeSSHKey,
			Username:      username,
			AuthorizedKey: strings.TrimSpace(string(ssh.MarshalAuthorizedKey(parsed))),
		}, nil
	}
	if passwordHash == "" || passwordHash != strings.TrimSpace(passwordHash) || len(passwordHash) > vmLoginPasswordHashLimit ||
		strings.ContainsAny(passwordHash, "\r\n\x00") || username == "root" ||
		!looksLikeLinuxGuestHint(extensionSet[vmOSNameKey]) || vmBootSourceUsesCDRom(extensionSet) {
		return vmLoginBootstrap{}, errors.New("invalid VM password bootstrap configuration")
	}
	passwordHash, err := NormalizeVMLoginPasswordHash(passwordHash)
	if err != nil {
		return vmLoginBootstrap{}, err
	}
	return vmLoginBootstrap{Mode: vmLoginModePassword, Username: username, PasswordHash: passwordHash}, nil
}

// NormalizeVMLoginPasswordHash accepts a raw or JSON-string-encoded Console value and validates
// the SHA-512-crypt verifier before it is stored or rendered into cloud-init.
func NormalizeVMLoginPasswordHash(value string) (string, error) {
	if value == "" || value != strings.TrimSpace(value) || len(value) > vmLoginPasswordHashLimit ||
		strings.ContainsAny(value, "\r\n\x00") {
		return "", errors.New("invalid VM login password hash")
	}
	logicalValue := value
	if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
		if err := json.Unmarshal([]byte(value), &logicalValue); err != nil {
			return "", errors.New("invalid VM login password hash")
		}
	}
	if logicalValue == "" || len(logicalValue) > vmLoginPasswordHashLimit || strings.ContainsAny(logicalValue, "\r\n\x00") {
		return "", errors.New("invalid VM login password hash")
	}
	parts := vmSHA512CryptPattern.FindStringSubmatch(logicalValue)
	if len(parts) != 4 {
		return "", errors.New("invalid VM login password hash")
	}
	rounds, err := strconv.Atoi(parts[1])
	if err != nil || rounds < 100000 || rounds > 1000000 {
		return "", errors.New("invalid VM login password hash")
	}
	return logicalValue, nil
}

func buildVMGuestSyncVolume(extensionSet map[string]string, guestFiles []volume.VMGuestFile,
	login vmLoginBootstrap) (*kubevirtv1.Volume, error) {
	if !looksLikeLinuxGuestHint(extensionSet[vmOSNameKey]) || len(guestFiles) == 0 && login.Mode == vmLoginModeNone {
		return nil, nil
	}
	syncScript := buildVMGuestSyncScript(guestFiles)
	userData := ""
	if login.Mode == vmLoginModeNone {
		userData = buildVMLegacyGuestSyncUserData(syncScript)
	} else {
		cloudConfig := map[string]any{
			"users": []any{"default"},
		}
		user := map[string]any{"name": login.Username}
		switch login.Mode {
		case vmLoginModeSSHKey:
			cloudConfig["ssh_pwauth"] = false
			user["ssh_authorized_keys"] = []string{login.AuthorizedKey}
		case vmLoginModePassword:
			cloudConfig["ssh_pwauth"] = true
			user["hashed_passwd"] = login.PasswordHash
			user["lock_passwd"] = false
			user["groups"] = []string{"sudo"}
			user["sudo"] = "ALL=(ALL) ALL"
			user["shell"] = "/bin/bash"
		default:
			return nil, errors.New("invalid VM login mode")
		}
		cloudConfig["users"] = append(cloudConfig["users"].([]any), user)
		if len(guestFiles) > 0 {
			cloudConfig["write_files"] = []map[string]string{{
				"path":        "/var/lib/cloud/scripts/per-boot/90-rainbond-sync.sh",
				"permissions": "0700",
				"content":     syncScript,
			}}
		}
		cloudConfigData, err := json.Marshal(cloudConfig)
		if err != nil {
			return nil, fmt.Errorf("marshal VM cloud-init configuration: %w", err)
		}
		userData = "#cloud-config\n" + string(cloudConfigData) + "\n"
	}
	return &kubevirtv1.Volume{
		Name: vmGuestSyncVolume,
		VolumeSource: kubevirtv1.VolumeSource{
			CloudInitNoCloud: &kubevirtv1.CloudInitNoCloudSource{
				UserData: userData,
			},
		},
	}, nil
}

func buildVMGuestSyncScript(guestFiles []volume.VMGuestFile) string {
	commands := make([]string, 0, len(guestFiles)+2)
	commands = append(commands, "#!/bin/bash", "set -eu")
	for _, file := range guestFiles {
		commands = append(commands,
			fmt.Sprintf("src_dir=$(blkid -L %s -o device | xargs -r dirname || true)", shellQuote(file.VolumeLabel)),
			"if [ -n \"$src_dir\" ]; then",
			fmt.Sprintf("  mkdir -p %s", shellQuote(parentDir(file.TargetPath))),
			fmt.Sprintf("  cp \"$src_dir/%s\" %s", escapeShellDoubleQuotes(file.SourceFile), shellQuote(file.TargetPath)),
			fmt.Sprintf("  chmod %s %s || true", shellQuote(file.Mode), shellQuote(file.TargetPath)),
			"fi",
		)
	}
	return strings.Join(commands, "\n") + "\n"
}

func buildVMLegacyGuestSyncUserData(syncScript string) string {
	commands := []string{
		"#!/bin/bash",
		"set -eu",
		"",
		"mkdir -p /var/lib/cloud/scripts/per-boot",
		"cat >/var/lib/cloud/scripts/per-boot/90-rainbond-sync.sh <<'EOF'",
		strings.TrimSuffix(syncScript, "\n"),
		"EOF",
		"chmod +x /var/lib/cloud/scripts/per-boot/90-rainbond-sync.sh",
		"/var/lib/cloud/scripts/per-boot/90-rainbond-sync.sh || true",
	}
	return strings.Join(commands, "\n") + "\n"
}

func parentDir(targetPath string) string {
	path := strings.TrimSpace(targetPath)
	if path == "" {
		return "/"
	}
	if idx := strings.LastIndex(path, "/"); idx > 0 {
		return path[:idx]
	}
	return "/"
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func escapeShellDoubleQuotes(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\$`, "`", "\\`")
	return replacer.Replace(value)
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

// BuildVMAccelerationDevices returns GPU and host devices requested by VM attributes.
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

func resolveVMInterfaceModel() string {
	return "virtio"
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

func looksLikeWindowsGuestHint(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return normalized != "" && strings.Contains(normalized, "windows")
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
