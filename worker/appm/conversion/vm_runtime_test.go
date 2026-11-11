package conversion

// capability_id: rainbond.worker.appm.vm-container-disk-cdrom

import (
	"bytes"
	"crypto/ed25519"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/worker/appm/volume"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

// capability_id: rainbond.worker.appm.vm-default-interface-virtio
func TestBuildVMRuntimeConfigRandomNetwork(t *testing.T) {
	cfg, err := buildVMRuntimeConfig(nil, nil, nil, nil)
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
	if cfg.Interfaces[0].Model != "virtio" {
		t.Fatalf("expected unknown guest network to default to virtio, got %q", cfg.Interfaces[0].Model)
	}
	if cfg.Interfaces[0].Bridge == nil {
		t.Fatalf("expected default bridge interface so VM guest uses pod IP")
	}
	if cfg.Interfaces[0].Masquerade != nil {
		t.Fatalf("did not expect default masquerade interface when VM guest should use pod IP")
	}
	if len(cfg.Volumes) != 0 {
		t.Fatalf("expected no extra volumes for random network, got %d", len(cfg.Volumes))
	}
	if len(cfg.Disks) != 0 {
		t.Fatalf("expected no extra disks for random network, got %d", len(cfg.Disks))
	}
}

func TestBuildVMRuntimeConfigRandomWindowsNetworkUsesVirtio(t *testing.T) {
	cfg, err := buildVMRuntimeConfig(map[string]string{
		"vm_os_name": "Windows Server 2022",
	}, nil, nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.Interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(cfg.Interfaces))
	}
	if cfg.Interfaces[0].Model != "virtio" {
		t.Fatalf("expected windows random network to use virtio, got %q", cfg.Interfaces[0].Model)
	}
	if cfg.Interfaces[0].Bridge == nil {
		t.Fatalf("expected windows random network to use bridge binding")
	}
}

func TestBuildVMRuntimeConfigRecognizedLinuxNameUsesVirtio(t *testing.T) {
	cfg, err := buildVMRuntimeConfig(map[string]string{
		"vm_os_name": "Ubuntu 22.04.5 LTS",
	}, nil, nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.Interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(cfg.Interfaces))
	}
	if cfg.Interfaces[0].Model != "virtio" {
		t.Fatalf("expected recognized linux guest to use virtio, got %q", cfg.Interfaces[0].Model)
	}
	if cfg.Interfaces[0].Bridge == nil {
		t.Fatalf("expected recognized linux guest to use bridge binding")
	}
}

func TestBuildVMRuntimeConfigIgnoresRemovedNetworkFields(t *testing.T) {
	cfg, err := buildVMRuntimeConfig(map[string]string{
		"vm_network_mode": "fixed",
		"vm_network_name": "default/bridge-test",
		"vm_fixed_ip":     "10.250.250.10/24",
		"vm_gateway":      "10.250.250.1",
		"vm_dns_servers":  "223.5.5.5,8.8.8.8",
	}, nil, nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.Networks) != 1 {
		t.Fatalf("expected 1 network, got %d", len(cfg.Networks))
	}
	if cfg.Networks[0].Pod == nil {
		t.Fatalf("expected removed network fields to keep default pod network")
	}
	if len(cfg.Interfaces) != 1 || cfg.Interfaces[0].Bridge == nil {
		t.Fatalf("expected removed network fields to keep bridge interface")
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
	}, nil, nil, nil)
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
	}, nil, nil, nil)
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
	}, nil, nil)
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
		{
			Name: "POD_IP",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"},
			},
		},
	}, nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	content := cfg.ConfigMaps[0].Data["rainbond.env"]
	if content != "DEMO_HOST=overwritten\nDEMO_PORT=8080\n" {
		t.Fatalf("unexpected deduplicated env content: %q", content)
	}
}

func TestBuildVMRuntimeConfigUsesStableEnvConfigMapName(t *testing.T) {
	cfgA, err := buildVMRuntimeConfig(map[string]string{
		"service_id": "service-vm-1",
	}, []corev1.EnvVar{{Name: "DEMO_HOST", Value: "demo-service"}}, nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	cfgB, err := buildVMRuntimeConfig(map[string]string{
		"service_id": "service-vm-1",
	}, []corev1.EnvVar{{Name: "DEMO_HOST", Value: "demo-service"}}, nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfgA.ConfigMaps[0].Name != cfgB.ConfigMaps[0].Name {
		t.Fatalf("expected stable configmap name, got %q and %q", cfgA.ConfigMaps[0].Name, cfgB.ConfigMaps[0].Name)
	}
	if cfgA.ConfigMaps[0].Name != "vm-env-service-vm-1" {
		t.Fatalf("expected stable vm env configmap name, got %q", cfgA.ConfigMaps[0].Name)
	}
}

func TestBuildVMRuntimeConfigIncludesEnvSecretsInEnvConfigMap(t *testing.T) {
	cfg, err := buildVMRuntimeConfig(map[string]string{
		"service_id": "service-vm-1",
	}, []corev1.EnvVar{
		{Name: "DEMO_HOST", Value: "demo-service"},
	}, []*corev1.Secret{
		{
			Data: map[string][]byte{
				"DEMO_TOKEN": []byte("secret-token"),
				"DEMO_HOST":  []byte("secret-host"),
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	content := cfg.ConfigMaps[0].Data["rainbond.env"]
	if content != "DEMO_HOST=demo-service\nDEMO_TOKEN=secret-token\n" {
		t.Fatalf("expected env file to merge secret-backed vars, got %q", content)
	}
}

func TestBuildVMRuntimeConfigAddsCloudInitSyncDiskForLinuxGuests(t *testing.T) {
	cfg, err := buildVMRuntimeConfig(map[string]string{
		"vm_os_name": "Ubuntu 22.04.5 LTS",
		"service_id": "service-vm-1",
	}, []corev1.EnvVar{
		{Name: "DEMO_HOST", Value: "demo-service"},
	}, nil, []volume.VMGuestFile{
		{
			VolumeLabel: "RBDCFG5D41402A",
			SourceFile:  "app.conf",
			TargetPath:  "/etc/demo/app.conf",
			Mode:        "0640",
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.Volumes) != 2 {
		t.Fatalf("expected env disk and cloud-init disk, got %d", len(cfg.Volumes))
	}
	if cfg.Volumes[1].CloudInitNoCloud == nil {
		t.Fatalf("expected second volume to be cloud-init, got %#v", cfg.Volumes[1].VolumeSource)
	}
	userData := cfg.Volumes[1].CloudInitNoCloud.UserData
	if userData == "" {
		t.Fatal("expected cloud-init userData to be rendered")
	}
	if !strings.Contains(userData, "/var/lib/cloud/scripts/per-boot/90-rainbond-sync.sh") {
		t.Fatalf("expected cloud-init userData to install per-boot sync script, got %q", userData)
	}
	if !strings.Contains(userData, "RBDCFG5D41402A") {
		t.Fatalf("expected cloud-init userData to reference config volume label, got %q", userData)
	}
	if !strings.Contains(userData, "/etc/demo/app.conf") {
		t.Fatalf("expected cloud-init userData to reference guest target path, got %q", userData)
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

// capability_id: rainbond.vm-runtime.ssh-authorized-keys-cloud-init
func TestValidateVMSSHBootstrap(t *testing.T) {
	signer, err := ssh.NewSignerFromKey(ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize)))
	if err != nil {
		t.Fatal(err)
	}
	authorizedKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	tests := []struct {
		name     string
		username string
		key      string
		wantErr  bool
	}{
		{name: "valid public key", username: "ubuntu", key: authorizedKey},
		{name: "missing pair"},
		{name: "invalid username", username: "ubuntu;id", key: authorizedKey, wantErr: true},
		{name: "partial username", username: "ubuntu", wantErr: true},
		{name: "partial key", key: authorizedKey, wantErr: true},
		{name: "private key", username: "ubuntu", key: "-----BEGIN OPENSSH PRIVATE KEY-----", wantErr: true},
		{name: "multiple lines", username: "ubuntu", key: authorizedKey + "\n" + authorizedKey, wantErr: true},
		{name: "invalid key syntax", username: "ubuntu", key: "ssh-ed25519 not-base64", wantErr: true},
		{name: "oversized key", username: "ubuntu", key: "ssh-ed25519 " + strings.Repeat("A", vmSSHAuthorizedKeyLimit), wantErr: true},
		{name: "key options", username: "ubuntu", key: "command=\"id\" " + authorizedKey, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateVMLoginBootstrap(map[string]string{
				vmSSHUsernameKey: tt.username, vmSSHPublicKeyKey: tt.key,
			})
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.key != "" && strings.Contains(err.Error(), tt.key) {
				t.Fatal("validation error contains submitted key text")
			}
			if !tt.wantErr && tt.username != "" && (got.Username != tt.username || got.AuthorizedKey != authorizedKey || got.Mode != vmLoginModeSSHKey) {
				t.Fatal("validated SSH login did not match the submitted username and key")
			}
		})
	}
}

// capability_id: rainbond.vm-runtime.ssh-authorized-keys-cloud-init
func TestBuildVMRuntimeConfigMergesSSHKeyAndGuestFileSync(t *testing.T) {
	signer, err := ssh.NewSignerFromKey(ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize)))
	if err != nil {
		t.Fatal(err)
	}
	authorizedKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	cfg, err := buildVMRuntimeConfig(map[string]string{
		vmOSNameKey: "Ubuntu 24.04", vmBootSourceFormatKey: "qcow2",
		vmSSHUsernameKey: "ubuntu", vmSSHPublicKeyKey: authorizedKey,
	}, nil, nil, []volume.VMGuestFile{{VolumeLabel: "RBDTEST", SourceFile: "settings.conf", TargetPath: "/etc/rainbond/settings.conf", Mode: "0644"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Volumes) != 1 || cfg.Volumes[0].CloudInitNoCloud == nil {
		t.Fatalf("expected one cloud-init volume, got %#v", cfg.Volumes)
	}
	userData := cfg.Volumes[0].CloudInitNoCloud.UserData
	if !strings.HasPrefix(userData, "#cloud-config\n") {
		t.Fatal("SSH bootstrap must be rendered as cloud-config so password SSH can be explicitly disabled")
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(strings.TrimPrefix(userData, "#cloud-config\n")), &config); err != nil {
		t.Fatalf("expected JSON-compatible cloud-config, got %v", err)
	}
	if passwordSSH, ok := config["ssh_pwauth"].(bool); !ok || passwordSSH {
		t.Fatalf("expected cloud-init to disable password SSH, got %#v", config["ssh_pwauth"])
	}
	users, ok := config["users"].([]any)
	if !ok || len(users) != 2 {
		t.Fatalf("expected default and bootstrap user definitions, got %#v", config["users"])
	}
	user, ok := users[1].(map[string]any)
	if !ok || user["name"] != "ubuntu" {
		t.Fatalf("expected Ubuntu bootstrap user, got %#v", users[1])
	}
	keys, ok := user["ssh_authorized_keys"].([]any)
	if !ok || len(keys) != 1 || keys[0] != authorizedKey {
		t.Fatalf("expected exactly one authorized key, got %#v", user["ssh_authorized_keys"])
	}
	writeFiles, ok := config["write_files"].([]any)
	if !ok || len(writeFiles) != 1 {
		t.Fatalf("expected guest-file sync script to be written, got %#v", config["write_files"])
	}
	syncFile, ok := writeFiles[0].(map[string]any)
	if !ok {
		t.Fatalf("expected guest-file sync file definition, got %#v", writeFiles[0])
	}
	content, _ := syncFile["content"].(string)
	path, _ := syncFile["path"].(string)
	for _, want := range []string{"settings.conf", "RBDTEST", "/etc/rainbond/settings.conf", "90-rainbond-sync.sh"} {
		if !strings.Contains(content, want) && !strings.Contains(path, want) {
			t.Errorf("guest-file sync cloud-init is missing %q", want)
		}
	}
}

// capability_id: rainbond.vm-runtime.ssh-authorized-keys-cloud-init
func TestBuildVMRuntimeConfigRejectsSSHBootstrapForISOOrNonLinuxGuest(t *testing.T) {
	signer, err := ssh.NewSignerFromKey(ed25519.NewKeyFromSeed(bytes.Repeat([]byte{11}, ed25519.SeedSize)))
	if err != nil {
		t.Fatal(err)
	}
	authorizedKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	for _, tc := range []struct {
		name string
		os   string
		fmt  string
	}{
		{name: "ISO", os: "Ubuntu 24.04", fmt: "iso"},
		{name: "Windows disk", os: "Windows Server 2025", fmt: "qcow2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildVMRuntimeConfig(map[string]string{
				vmOSNameKey: tc.os, vmBootSourceFormatKey: tc.fmt,
				vmSSHUsernameKey: "ubuntu", vmSSHPublicKeyKey: authorizedKey,
			}, nil, nil, nil)
			if err == nil {
				t.Fatal("expected Linux disk-boot validation error")
			}
			if strings.Contains(err.Error(), authorizedKey) {
				t.Fatal("validation error contains submitted key text")
			}
		})
	}
}

// capability_id: rainbond.vm-runtime.ssh-authorized-keys-cloud-init
func TestVMRuntimeAttributeNamesIncludesSSHBootstrap(t *testing.T) {
	names := vmRuntimeAttributeNames()
	found := map[string]bool{"vm_ssh_username": false, "vm_ssh_authorized_key": false}
	for _, name := range names {
		if _, ok := found[name]; ok {
			found[name] = true
		}
	}
	for name, ok := range found {
		if !ok {
			t.Errorf("worker does not hydrate %s", name)
		}
	}
}

// capability_id: rainbond.vm-runtime.password-hash-bootstrap
func TestValidateVMLoginBootstrapModes(t *testing.T) {
	key := generatedVMTestAuthorizedKey(t)
	hash := generatedVMTestSHA512CryptHash(t)
	tests := []struct {
		name    string
		attrs   map[string]string
		mode    vmLoginMode
		wantErr bool
	}{
		{name: "no login settings", attrs: nil, mode: vmLoginModeNone},
		{name: "key login", attrs: map[string]string{vmSSHUsernameKey: "ubuntu", vmSSHPublicKeyKey: key}, mode: vmLoginModeSSHKey},
		{name: "password hash login", attrs: map[string]string{vmOSNameKey: "Ubuntu 24.04", vmBootSourceFormatKey: "qcow2", vmSSHUsernameKey: "ubuntu", vmLoginPasswordHashKey: hash}, mode: vmLoginModePassword},
		{name: "key and hash mixed", attrs: map[string]string{vmSSHUsernameKey: "ubuntu", vmSSHPublicKeyKey: key, vmLoginPasswordHashKey: hash}, wantErr: true},
		{name: "plain password rejected", attrs: map[string]string{vmSSHUsernameKey: "ubuntu", "vm_login_password": "not-a-hash"}, wantErr: true},
		{name: "password username required", attrs: map[string]string{vmLoginPasswordHashKey: hash}, wantErr: true},
		{name: "password root rejected", attrs: map[string]string{vmOSNameKey: "Ubuntu 24.04", vmBootSourceFormatKey: "qcow2", vmSSHUsernameKey: "root", vmLoginPasswordHashKey: hash}, wantErr: true},
		{name: "password ISO rejected", attrs: map[string]string{vmOSNameKey: "Ubuntu 24.04", vmBootSourceFormatKey: "iso", vmSSHUsernameKey: "ubuntu", vmLoginPasswordHashKey: hash}, wantErr: true},
		{name: "password Windows rejected", attrs: map[string]string{vmOSNameKey: "Windows Server 2025", vmBootSourceFormatKey: "qcow2", vmSSHUsernameKey: "ubuntu", vmLoginPasswordHashKey: hash}, wantErr: true},
		{name: "rounds below minimum rejected", attrs: map[string]string{vmOSNameKey: "Ubuntu 24.04", vmBootSourceFormatKey: "qcow2", vmSSHUsernameKey: "ubuntu", vmLoginPasswordHashKey: strings.Replace(hash, "rounds=500000", "rounds=099999", 1)}, wantErr: true},
		{name: "rounds above maximum rejected", attrs: map[string]string{vmOSNameKey: "Ubuntu 24.04", vmBootSourceFormatKey: "qcow2", vmSSHUsernameKey: "ubuntu", vmLoginPasswordHashKey: strings.Replace(hash, "rounds=500000", "rounds=1000001", 1)}, wantErr: true},
		{name: "bad salt rejected", attrs: map[string]string{vmOSNameKey: "Ubuntu 24.04", vmBootSourceFormatKey: "qcow2", vmSSHUsernameKey: "ubuntu", vmLoginPasswordHashKey: "$6$rounds=500000$short$" + strings.Repeat("A", 86)}, wantErr: true},
		{name: "bad digest length rejected", attrs: map[string]string{vmOSNameKey: "Ubuntu 24.04", vmBootSourceFormatKey: "qcow2", vmSSHUsernameKey: "ubuntu", vmLoginPasswordHashKey: "$6$rounds=500000$0123456789abcdef$" + strings.Repeat("A", 85)}, wantErr: true},
		{name: "wrong crypt algorithm rejected", attrs: map[string]string{vmOSNameKey: "Ubuntu 24.04", vmBootSourceFormatKey: "qcow2", vmSSHUsernameKey: "ubuntu", vmLoginPasswordHashKey: strings.Replace(hash, "$6$", "$5$", 1)}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateVMLoginBootstrap(tt.attrs)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validation error state mismatch: %v", err)
			}
			if err == nil && got.Mode != tt.mode {
				t.Fatalf("login mode mismatch: got %q", got.Mode)
			}
			if err != nil && strings.Contains(err.Error(), hash) {
				t.Fatal("validation error echoed a generated credential")
			}
		})
	}
}

// capability_id: rainbond.vm-runtime.password-hash-bootstrap
func TestBuildVMRuntimeConfigPasswordLoginCloudInit(t *testing.T) {
	hash := generatedVMTestSHA512CryptHash(t)
	cfg, err := buildVMRuntimeConfig(map[string]string{
		vmOSNameKey: "Ubuntu 24.04", vmBootSourceFormatKey: "qcow2",
		vmSSHUsernameKey: "ubuntu", vmLoginPasswordHashKey: hash,
	}, nil, nil, []volume.VMGuestFile{{
		VolumeLabel: "RBDTEST", SourceFile: "settings.conf",
		TargetPath: "/etc/rainbond/settings.conf", Mode: "0644",
	}})
	if err != nil {
		t.Fatal("password cloud-init generation failed")
	}
	if len(cfg.Volumes) != 1 || cfg.Volumes[0].CloudInitNoCloud == nil {
		t.Fatal("password login did not create the cloud-init volume")
	}
	userData := cfg.Volumes[0].CloudInitNoCloud.UserData
	if !strings.HasPrefix(userData, "#cloud-config\n") {
		t.Fatal("password login did not generate cloud-config")
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(strings.TrimPrefix(userData, "#cloud-config\n")), &config); err != nil {
		t.Fatal("cloud-init config was not JSON-compatible")
	}
	if config["ssh_pwauth"] != true {
		t.Fatal("password mode did not enable SSH password authentication")
	}
	users, ok := config["users"].([]any)
	if !ok || len(users) != 2 || users[0] != "default" {
		t.Fatal("password mode did not preserve the default cloud-image user")
	}
	user, ok := users[1].(map[string]any)
	if !ok || user["name"] != "ubuntu" || user["hashed_passwd"] != hash || user["lock_passwd"] != false {
		t.Fatal("password mode did not configure an unlocked hash-backed user")
	}
	if user["sudo"] != "ALL=(ALL) ALL" || user["shell"] != "/bin/bash" {
		t.Fatal("password user did not receive the approved Ubuntu admin shell configuration")
	}
	if _, ok := user["ssh_authorized_keys"]; ok {
		t.Fatal("password mode unexpectedly emitted an SSH key")
	}
	groups, ok := user["groups"].([]any)
	if !ok || len(groups) != 1 || groups[0] != "sudo" {
		t.Fatal("password user was not added to the sudo group")
	}
	files, ok := config["write_files"].([]any)
	if !ok || len(files) != 1 {
		t.Fatal("password mode did not preserve guest file sync")
	}
	file, ok := files[0].(map[string]any)
	if !ok {
		t.Fatal("guest file sync definition is invalid")
	}
	path, _ := file["path"].(string)
	content, _ := file["content"].(string)
	if !strings.Contains(path, "90-rainbond-sync.sh") || !strings.Contains(content, "settings.conf") || !strings.Contains(content, "RBDTEST") {
		t.Fatal("password mode changed the guest file sync payload")
	}
}

// capability_id: rainbond.vm-runtime.password-hash-bootstrap
func TestVMRuntimeAttributeNamesIncludesLoginPasswordHash(t *testing.T) {
	for _, name := range vmRuntimeAttributeNames() {
		if name == vmLoginPasswordHashKey {
			return
		}
	}
	t.Fatal("worker does not hydrate the VM login hash attribute")
}

func generatedVMTestSHA512CryptHash(t *testing.T) string {
	t.Helper()
	saltBytes := make([]byte, 8)
	if _, err := cryptorand.Read(saltBytes); err != nil {
		t.Fatal("could not generate test hash material")
	}
	alphabet := "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	digest := make([]byte, 86)
	for i := range digest {
		index, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			t.Fatal("could not generate test hash material")
		}
		digest[i] = alphabet[index.Int64()]
	}
	return "$6$rounds=500000$" + hex.EncodeToString(saltBytes) + "$" + string(digest)
}

func generatedVMTestAuthorizedKey(t *testing.T) string {
	t.Helper()
	signer, err := ssh.NewSignerFromKey(ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize)))
	if err != nil {
		t.Fatal("could not generate test public key")
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
}
