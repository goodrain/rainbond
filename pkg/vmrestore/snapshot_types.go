package vmrestore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/goodrain/rainbond/pkg/vmstorage"

	"k8s.io/apimachinery/pkg/util/validation"

	kubevirtv1 "kubevirt.io/api/core/v1"
)

// Snapshot metadata keys keep full fingerprints in annotations, never label values.
const (
	SnapshotCoreAnnotation      = "rainbond.io/snapshot-core-hash"
	SnapshotOperationLabel      = "rainbond.io/snapshot-operation"
	SnapshotSourceUIDAnnotation = "rainbond.io/snapshot-source-uid"
)

// Snapshot errors deliberately contain no native/guest error text.
var (
	ErrSnapshotInvalid    = errors.New("VM snapshot request is invalid")
	ErrSnapshotConfig     = errors.New("VM snapshot configuration is unsupported")
	ErrSnapshotIncomplete = errors.New("VM snapshot does not cover every writable disk")
	ErrSnapshotStorage    = errors.New("VM snapshot storage is unsupported")
	ErrSnapshotPending    = errors.New("VM snapshot dependency is not ready")
)

// SnapshotDisk contains logical identity and server-observed physical ownership.
type SnapshotDisk struct {
	VolumeID       uint       `json:"volume_id"`
	DiskKey        string     `json:"disk_key"`
	CanonicalName  string     `json:"canonical_name"`
	Role           string     `json:"role"`
	SourcePVC      ObjectRef  `json:"source_pvc"`
	SourceDV       *ObjectRef `json:"source_dv,omitempty"`
	ReadOnly       bool       `json:"read_only"`
	StorageClass   string     `json:"storage_class"`
	RequestedBytes int64      `json:"requested_bytes"`
}

// SnapshotIntent is the bounded Region resource intent, not a manifest copy.
// The authoritative complete configuration and manifest live in Console.
type SnapshotIntent struct {
	OperationID string         `json:"operation_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	SourceVM    ObjectRef      `json:"source_vm"`
	WasRunning  bool           `json:"was_running"`
	PowerAfter  string         `json:"power_after"`
	ConfigHash  string         `json:"config_hash"`
	Disks       []SnapshotDisk `json:"disks"`
}

// SnapshotLayout excludes initialization and arbitrary VM/Secret fields.
type SnapshotLayout struct {
	VolumeID      uint   `json:"volume_id"`
	DiskKey       string `json:"disk_key"`
	CanonicalName string `json:"canonical_name"`
	Role          string `json:"role"`
	Device        string `json:"device"`
	Bus           string `json:"bus"`
	BootOrder     uint   `json:"boot_order"`
	ReadOnly      bool   `json:"read_only"`
}

// SnapshotConfig contains only approved VM configuration with explicit CPU and memory units.
type SnapshotConfig struct {
	CPUMilli   int              `json:"cpu_milli"`
	MemoryMiB  int64            `json:"memory_mib"`
	BootMode   string           `json:"boot_mode"`
	DiskLayout []SnapshotLayout `json:"disk_layout"`
}

// SnapshotCoverage makes omitted writable disks visible instead of trusting Native Ready alone.
type SnapshotCoverage struct {
	Complete              bool     `json:"complete"`
	ExpectedWritableDisks int      `json:"expected_writable_disks"`
	IncludedDisks         int      `json:"included_disks"`
	MissingDiskKeys       []string `json:"missing_disk_keys"`
	ExcludedReadOnly      []string `json:"excluded_read_only"`
}

// SnapshotBackup pins source PVC and actual CSI snapshot identities.
type SnapshotBackup struct {
	SnapshotDisk
	VolumeSnapshot ObjectRef `json:"volume_snapshot"`
}

// SnapshotDTO is the safe Region projection; Console additionally requires its durable manifest.
type SnapshotDTO struct {
	Namespace       string           `json:"namespace,omitempty"`
	SourceVMName    string           `json:"source_vm_name,omitempty"`
	SnapshotName    string           `json:"snapshot_name"`
	SnapshotUID     string           `json:"snapshot_uid,omitempty"`
	SourceVMUID     string           `json:"source_vm_uid"`
	ContentName     string           `json:"content_name,omitempty"`
	ContentUID      string           `json:"content_uid,omitempty"`
	OperationID     string           `json:"operation_id"`
	CoreHash        string           `json:"core_hash"`
	Description     string           `json:"description"`
	Phase           string           `json:"phase"`
	ReadyToUse      bool             `json:"ready_to_use"`
	SourceLockClear bool             `json:"source_lock_clear"`
	CreationTime    string           `json:"creation_time,omitempty"`
	Consistency     string           `json:"consistency"`
	Coverage        SnapshotCoverage `json:"coverage"`
	Config          *SnapshotConfig  `json:"config,omitempty"`
	Disks           []SnapshotBackup `json:"disks"`
	ErrorCode       string           `json:"error_code,omitempty"`
}

// SnapshotCreateRequest preserves the legacy name field while adding bounded-operation options.
type SnapshotCreateRequest struct {
	Operator    string `json:"operator,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description"`
	RequestID   string `json:"request_id,omitempty"`
	ManifestID  string `json:"manifest_id,omitempty"`
	PowerAfter  string `json:"power_after,omitempty"`
}

// SnapshotActionRequest carries only internal worker commands and durable-record acknowledgement.
type SnapshotActionRequest struct {
	OperationID     string `json:"operation_id"`
	CoreHash        string `json:"core_hash"`
	RequestID       string `json:"request_id"`
	SnapshotUID     string `json:"snapshot_uid,omitempty"`
	ContentUID      string `json:"content_uid,omitempty"`
	NativeProofHash string `json:"native_proof_hash,omitempty"`
	ManifestID      string `json:"manifest_id,omitempty"`
	DocumentHash    string `json:"manifest_document_hash,omitempty"`
	AckKind         string `json:"ack_kind,omitempty"`
}

func snapshotDigest(value interface{}) (string, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// SnapshotCoreHash uses Go-produced canonical material; Console persists this
// opaque hash and separately hashes its completed manifest document.
func SnapshotCoreHash(scope Scope, intent SnapshotIntent) (string, error) {
	clone := intent
	clone.Disks = append([]SnapshotDisk(nil), intent.Disks...)
	sort.Slice(clone.Disks, func(i, j int) bool { return clone.Disks[i].VolumeID < clone.Disks[j].VolumeID })
	return snapshotDigest(struct {
		Version int            `json:"schema_version"`
		Scope   Scope          `json:"scope"`
		Intent  SnapshotIntent `json:"intent"`
	}{1, scope, clone})
}
func validateSnapshotIntent(scope Scope, intent SnapshotIntent) error {
	if !idPattern.MatchString(intent.OperationID) || !validScope(scope) || len(validation.IsDNS1123Subdomain(intent.Name)) > 0 || len(intent.Name) > 253 || len(intent.Description) > 2048 || !hashPattern.MatchString(intent.ConfigHash) {
		return ErrSnapshotInvalid
	}
	if intent.SourceVM.Kind != "VirtualMachine" || intent.SourceVM.Namespace != scope.Namespace || !validUID(intent.SourceVM.UID) || len(validation.IsDNS1123Subdomain(intent.SourceVM.Name)) > 0 {
		return ErrConflict
	}
	if intent.PowerAfter != "previous" && intent.PowerAfter != "stopped" {
		return ErrSnapshotInvalid
	}
	if len(intent.Disks) == 0 {
		return ErrSnapshotIncomplete
	}
	ids := map[uint]bool{}
	names := map[string]bool{}
	rootWritable := false
	for _, disk := range intent.Disks {
		if disk.VolumeID == 0 || disk.CanonicalName != canonicalVolumeName(disk.VolumeID) || disk.DiskKey == "" || ids[disk.VolumeID] || names[disk.CanonicalName] || !validRef(disk.SourcePVC) || disk.SourcePVC.Namespace != scope.Namespace || disk.RequestedBytes <= 0 {
			return ErrConflict
		}
		ids[disk.VolumeID] = true
		names[disk.CanonicalName] = true
		if disk.Role != "root" && disk.Role != "data" {
			return ErrSnapshotConfig
		}
		if disk.Role == "root" && !disk.ReadOnly {
			rootWritable = true
		}
		if disk.SourceDV != nil && (disk.SourceDV.Kind != "DataVolume" || disk.SourceDV.Namespace != scope.Namespace || !validUID(disk.SourceDV.UID)) {
			return ErrConflict
		}
	}
	if !rootWritable {
		return ErrSnapshotIncomplete
	}
	return nil
}
func canonicalVolumeName(id uint) string { return "manual" + strconv.FormatUint(uint64(id), 10) }

// snapshotInitializationVolume recognizes only generated configuration media.
// Unknown source types fail closed instead of disappearing from disk coverage.
func snapshotInitializationVolume(source kubevirtv1.VolumeSource) bool {
	switch {
	case source.CloudInitNoCloud != nil:
		return source == (kubevirtv1.VolumeSource{CloudInitNoCloud: source.CloudInitNoCloud})
	case source.CloudInitConfigDrive != nil:
		return source == (kubevirtv1.VolumeSource{CloudInitConfigDrive: source.CloudInitConfigDrive})
	case source.Sysprep != nil:
		return source == (kubevirtv1.VolumeSource{Sysprep: source.Sysprep})
	case source.ConfigMap != nil:
		return source == (kubevirtv1.VolumeSource{ConfigMap: source.ConfigMap})
	case source.Secret != nil:
		return source == (kubevirtv1.VolumeSource{Secret: source.Secret})
	case source.DownwardAPI != nil:
		return source == (kubevirtv1.VolumeSource{DownwardAPI: source.DownwardAPI})
	case source.ServiceAccount != nil:
		return source == (kubevirtv1.VolumeSource{ServiceAccount: source.ServiceAccount})
	case source.DownwardMetrics != nil:
		return source == (kubevirtv1.VolumeSource{DownwardMetrics: source.DownwardMetrics})
	default:
		return false
	}
}

// SnapshotSafeConfig explicitly projects only the approved configuration.
func SnapshotSafeConfig(vm *kubevirtv1.VirtualMachine, disks []SnapshotDisk) (SnapshotConfig, error) {
	config := SnapshotConfig{BootMode: "bios", DiskLayout: []SnapshotLayout{}}
	if vm == nil || vm.Spec.Template == nil {
		return config, ErrSnapshotConfig
	}
	domain := vm.Spec.Template.Spec.Domain
	if domain.CPU == nil {
		return config, ErrSnapshotConfig
	}
	sockets, cores, threads := uint64(domain.CPU.Sockets), uint64(domain.CPU.Cores), uint64(domain.CPU.Threads)
	if sockets == 0 {
		sockets = 1
	}
	if cores == 0 {
		cores = 1
	}
	if threads == 0 {
		threads = 1
	}
	if sockets > 4096 || cores > 4096 || threads > 4096 {
		return config, ErrSnapshotConfig
	}
	count := sockets * cores * threads
	if count == 0 || count > 4096 {
		return config, ErrSnapshotConfig
	}
	config.CPUMilli = int(count * 1000)
	if domain.Memory == nil || domain.Memory.Guest == nil {
		return config, ErrSnapshotConfig
	}
	amount := domain.Memory.Guest.Value()
	if amount <= 0 || amount%(1<<20) != 0 {
		return config, ErrSnapshotConfig
	}
	config.MemoryMiB = amount / (1 << 20)
	if domain.Firmware != nil && domain.Firmware.Bootloader != nil && domain.Firmware.Bootloader.EFI != nil {
		efi := domain.Firmware.Bootloader.EFI
		// KubeVirt defaults omitted SecureBoot to true; boot_mode cannot preserve it.
		if efi.SecureBoot == nil || *efi.SecureBoot {
			return config, ErrSnapshotConfig
		}
		// EFI NVRAM is writable backend state outside the approved disk backups.
		if efi.Persistent != nil && *efi.Persistent {
			return config, ErrSnapshotConfig
		}
		config.BootMode = "uefi"
	}
	if tpm := domain.Devices.TPM; tpm != nil && tpm.Persistent != nil && *tpm.Persistent {
		// Persistent TPM state also requires backend storage coverage.
		return config, ErrSnapshotConfig
	}
	logical := map[string]SnapshotDisk{}
	for _, d := range disks {
		if _, exists := logical[d.CanonicalName]; exists {
			return config, ErrSnapshotConfig
		}
		logical[d.CanonicalName] = d
	}
	volumes := map[string]kubevirtv1.Volume{}
	managed := 0
	for _, volume := range vm.Spec.Template.Spec.Volumes {
		if _, exists := volumes[volume.Name]; exists {
			return config, ErrSnapshotConfig
		}
		volumes[volume.Name] = volume
		identity, ok := logical[volume.Name]
		if !ok {
			if !snapshotInitializationVolume(volume.VolumeSource) && !snapshotReadOnlyInstallationMedia(vm, volume) {
				return config, ErrSnapshotIncomplete
			}
			continue
		}
		switch {
		case volume.PersistentVolumeClaim != nil && volume.VolumeSource == (kubevirtv1.VolumeSource{PersistentVolumeClaim: volume.PersistentVolumeClaim}):
			if volume.PersistentVolumeClaim.ClaimName != identity.SourcePVC.Name {
				return config, ErrConflict
			}
		case volume.DataVolume != nil && volume.VolumeSource == (kubevirtv1.VolumeSource{DataVolume: volume.DataVolume}):
			if identity.SourceDV == nil || volume.DataVolume.Name != identity.SourceDV.Name {
				return config, ErrConflict
			}
		default:
			return config, ErrSnapshotConfig
		}
		managed++
	}
	if managed != len(disks) {
		return config, ErrSnapshotConfig
	}
	seen := map[string]bool{}
	for _, disk := range domain.Devices.Disks {
		identity, ok := logical[disk.Name]
		if !ok {
			if !snapshotInitializationVolume(volumes[disk.Name].VolumeSource) && !snapshotReadOnlyInstallationMedia(vm, volumes[disk.Name]) {
				return config, ErrSnapshotIncomplete
			}
			continue
		} // Initialization devices are not VM disk-layout input.
		if seen[disk.Name] {
			return config, ErrSnapshotConfig
		}
		seen[disk.Name] = true
		actualReadOnly := false
		switch {
		case disk.Disk != nil:
			actualReadOnly = disk.Disk.ReadOnly
		case disk.LUN != nil:
			actualReadOnly = disk.LUN.ReadOnly
		case disk.CDRom != nil:
			actualReadOnly = disk.CDRom.ReadOnly == nil || *disk.CDRom.ReadOnly
		}
		for _, volume := range vm.Spec.Template.Spec.Volumes {
			if volume.Name == disk.Name && volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ReadOnly {
				actualReadOnly = true
			}
		}
		if actualReadOnly != identity.ReadOnly {
			return config, ErrSnapshotConfig
		}
		item := SnapshotLayout{VolumeID: identity.VolumeID, DiskKey: identity.DiskKey, CanonicalName: disk.Name, Role: identity.Role, ReadOnly: identity.ReadOnly}
		if disk.BootOrder != nil {
			item.BootOrder = *disk.BootOrder
		}
		switch {
		case disk.Disk != nil:
			item.Device = "disk"
			item.Bus = string(disk.Disk.Bus)
		case disk.LUN != nil:
			item.Device = "lun"
			item.Bus = string(disk.LUN.Bus)
		case disk.CDRom != nil:
			item.Device = "cdrom"
			item.Bus = string(disk.CDRom.Bus)
		default:
			return config, ErrSnapshotConfig
		}
		if item.Bus == "" {
			return config, ErrSnapshotConfig
		}
		if !strings.Contains("|virtio|scsi|sata|usb|", "|"+item.Bus+"|") {
			return config, ErrSnapshotConfig
		}
		config.DiskLayout = append(config.DiskLayout, item)
	}
	if len(seen) != len(disks) {
		return config, ErrSnapshotConfig
	}
	sort.Slice(config.DiskLayout, func(i, j int) bool { return config.DiskLayout[i].VolumeID < config.DiskLayout[j].VolumeID })
	return config, nil
}

// Only an immutable container-backed CD-ROM may be excluded with installation media.
// Keep this aligned with the current-VM capability inspection; mixed sources and
// unmanaged writable container disks still require snapshot coverage.
func snapshotReadOnlyInstallationMedia(vm *kubevirtv1.VirtualMachine, volume kubevirtv1.Volume) bool {
	if volume.ContainerDisk == nil || volume.ContainerDisk.Image == "" || volume.VolumeSource != (kubevirtv1.VolumeSource{ContainerDisk: volume.ContainerDisk}) || !vmstorage.ReadOnlyInstallationMedia(vm, volume) {
		return false
	}
	count := 0
	for _, device := range vm.Spec.Template.Spec.Domain.Devices.Disks {
		if device.Name == volume.Name {
			count++
			if device.CDRom == nil || device.Disk != nil || device.LUN != nil || device.CDRom.ReadOnly != nil && !*device.CDRom.ReadOnly {
				return false
			}
		}
	}
	return count == 1
}
