package vmrestore

import (
	"reflect"
	"sort"
	"strconv"
)

// RestoreVolumeConfig is only the existing Region disk configuration allowlist.
// It cannot change paths, ownership, platform IPs, ports or arbitrary attributes.
type RestoreVolumeConfig struct {
	VolumeID     uint   `json:"volume_id"`
	DiskKey      string `json:"disk_key"`
	CapacityGiB  int64  `json:"capacity_gib"`
	StorageClass string `json:"storage_class"`
	ReadOnly     bool   `json:"read_only"`
	AccessMode   string `json:"access_mode"`
}

// RestoreConfig is a safe selected configuration, not a raw VM or Console manifest.
type RestoreConfig struct {
	Hardware SnapshotConfig        `json:"hardware"`
	Volumes  []RestoreVolumeConfig `json:"volumes"`
}

// RestoreIntent pins the source and original Region-selected configuration only.
// ConfigAfter is supplied from Console's immutable manifest at each bounded call
// and is verified against ConfigAfterHash and the live native snapshot projection.
type RestoreIntent struct {
	CurrentVM          ObjectRef     `json:"current_vm"`
	SourceSnapshot     ObjectRef     `json:"source_snapshot"`
	SourceContent      ObjectRef     `json:"source_content"`
	SourceVMUID        string        `json:"source_vm_uid"`
	SourceCoreHash     string        `json:"source_core_hash"`
	SourceDocumentHash string        `json:"source_document_hash"`
	SourceGeneration   uint64        `json:"source_generation"`
	WasRunning         bool          `json:"was_running"`
	PowerAfter         string        `json:"power_after"`
	Before             RestoreConfig `json:"before"`
	ConfigAfterHash    string        `json:"config_after_hash"`
}

// RestoreCommitRequest is internal to Region execution after full copy evidence
// is sealed. It is not an admission/UID claim accepted from an ordinary UI body.
type RestoreCommitRequest struct {
	Config   RestoreConfig      `json:"config"`
	Bindings map[string]Binding `json:"bindings"`
}

func (c RestoreConfig) validate(scope Scope) error {
	h := c.Hardware
	if h.CPUMilli < 1000 || h.CPUMilli%1000 != 0 || h.CPUMilli > 4096000 || h.MemoryMiB < 1 || h.MemoryMiB > 1<<40 || (h.BootMode != "bios" && h.BootMode != "uefi") || len(h.DiskLayout) == 0 || len(h.DiskLayout) > 128 || len(c.Volumes) < len(h.DiskLayout) {
		return ErrCorrupt
	}
	volumes := map[uint]RestoreVolumeConfig{}
	for _, v := range c.Volumes {
		if v.VolumeID == 0 || v.DiskKey == "" || len(v.DiskKey) > 40 || v.CapacityGiB < 1 || v.CapacityGiB > 1<<40 || !validName(v.StorageClass) || (v.AccessMode != "ReadWriteOnce" && v.AccessMode != "ReadWriteMany" && v.AccessMode != "ReadOnlyMany") {
			return ErrCorrupt
		}
		if _, ok := volumes[v.VolumeID]; ok {
			return ErrCorrupt
		}
		volumes[v.VolumeID] = v
	}
	roots := 0
	seen := map[uint]bool{}
	orders := map[uint]bool{}
	for _, d := range h.DiskLayout {
		v, ok := volumes[d.VolumeID]
		if !ok || seen[d.VolumeID] || d.DiskKey != v.DiskKey || d.CanonicalName != canonicalVolumeName(d.VolumeID) || d.ReadOnly != v.ReadOnly || (d.Role != "root" && d.Role != "data") || (d.Device != "disk" && d.Device != "lun" && d.Device != "cdrom") || (d.Bus != "virtio" && d.Bus != "scsi" && d.Bus != "sata" && d.Bus != "usb") {
			return ErrCorrupt
		}
		if d.Role == "root" {
			roots++
			if d.ReadOnly {
				return ErrCorrupt
			}
		}
		if d.BootOrder > 0 {
			if orders[d.BootOrder] {
				return ErrCorrupt
			}
			orders[d.BootOrder] = true
		}
		seen[d.VolumeID] = true
	}
	if roots != 1 {
		return ErrCorrupt
	}
	return nil
}
func (r RestoreIntent) validate(scope Scope) error {
	if r.CurrentVM.Kind != "VirtualMachine" || r.CurrentVM.Namespace != scope.Namespace || !validName(r.CurrentVM.Name) || (r.CurrentVM.UID != "" && !validUID(r.CurrentVM.UID)) {
		return ErrCorrupt
	}
	if r.SourceSnapshot.Kind != "VirtualMachineSnapshot" || r.SourceContent.Kind != "VirtualMachineSnapshotContent" || r.SourceSnapshot.Namespace != scope.Namespace || r.SourceContent.Namespace != scope.Namespace || !validName(r.SourceSnapshot.Name) || !validName(r.SourceContent.Name) || !validUID(r.SourceSnapshot.UID) || !validUID(r.SourceContent.UID) || !validUID(r.SourceVMUID) || !hashPattern.MatchString(r.SourceCoreHash) || !hashPattern.MatchString(r.SourceDocumentHash) || !hashPattern.MatchString(r.ConfigAfterHash) || (r.PowerAfter != "previous" && r.PowerAfter != "stopped") {
		return ErrCorrupt
	}
	return r.Before.validate(scope)
}
func cloneBindings(input map[string]Binding) map[string]Binding {
	result := map[string]Binding{}
	for k, v := range input {
		if v.ActiveDV != nil {
			copy := *v.ActiveDV
			v.ActiveDV = &copy
		}
		if v.PreviousPVC != nil {
			copy := *v.PreviousPVC
			v.PreviousPVC = &copy
		}
		if v.PreviousDV != nil {
			copy := *v.PreviousDV
			v.PreviousDV = &copy
		}
		result[k] = v
	}
	return result
}

// RestoreConfigHash sorts copies, so a caller cannot mutate persisted intent by reordering a request.
func RestoreConfigHash(config RestoreConfig) (string, error) {
	c := config
	c.Hardware.DiskLayout = append([]SnapshotLayout(nil), config.Hardware.DiskLayout...)
	c.Volumes = append([]RestoreVolumeConfig(nil), config.Volumes...)
	sort.Slice(c.Hardware.DiskLayout, func(i, j int) bool { return c.Hardware.DiskLayout[i].VolumeID < c.Hardware.DiskLayout[j].VolumeID })
	sort.Slice(c.Volumes, func(i, j int) bool { return c.Volumes[i].VolumeID < c.Volumes[j].VolumeID })
	return snapshotDigest(c)
}

// BuildRestoreBindings atomically replaces exactly A's writable disks while
// retaining later B-only disks physically and marking them explicitly detached.
func BuildRestoreBindings(scope Scope, operationID, snapshotUID string, old map[string]Binding, config RestoreConfig, targets map[uint]ObjectRef) (map[string]Binding, error) {
	if !idPattern.MatchString(operationID) || !validUID(snapshotUID) || config.validate(scope) != nil || validateBindings(old) != nil {
		return nil, ErrCorrupt
	}
	next := cloneBindings(old)
	protected := map[string]bool{}
	seenTargets := map[string]bool{}
	identities := map[[3]string]string{}
	consistentIdentity := func(ref ObjectRef) bool {
		key := [3]string{ref.Kind, ref.Namespace, ref.Name}
		if uid, exists := identities[key]; exists && uid != ref.UID {
			return false
		}
		identities[key] = ref.UID
		return true
	}
	for _, b := range old {
		for _, ref := range []*ObjectRef{&b.ActivePVC, b.ActiveDV, b.PreviousPVC, b.PreviousDV} {
			if ref == nil {
				continue
			}
			if !consistentIdentity(*ref) {
				return nil, ErrConflict
			}
			protected[ref.UID] = true
		}
	}
	for _, target := range targets {
		if protected[target.UID] || seenTargets[target.UID] || !consistentIdentity(target) {
			return nil, ErrConflict
		}
		seenTargets[target.UID] = true
	}
	selected := map[uint]bool{}
	used := map[uint]bool{}
	for _, disk := range config.Hardware.DiskLayout {
		key := strconv.FormatUint(uint64(disk.VolumeID), 10)
		before, ok := next[key]
		if !ok || before.DiskKey != disk.DiskKey || before.Role != disk.Role || before.ActivePVC.Namespace != scope.Namespace {
			return nil, ErrConflict
		}
		selected[disk.VolumeID] = true
		if disk.ReadOnly {
			if _, exists := targets[disk.VolumeID]; exists {
				return nil, ErrConflict
			}
			before.Attached = true
			next[key] = before
			continue
		}
		target, ok := targets[disk.VolumeID]
		if !ok || !validRef(target) || target.Namespace != scope.Namespace || target.UID == before.ActivePVC.UID {
			return nil, ErrConflict
		}
		previous := before.ActivePVC
		before.PreviousPVC = &previous
		before.PreviousDV = before.ActiveDV
		before.ActivePVC = target
		before.ActiveDV = nil
		before.Attached = true
		before.RestoreID = operationID
		before.SnapshotUID = snapshotUID
		next[key] = before
		used[disk.VolumeID] = true
	}
	if len(used) != len(targets) {
		return nil, ErrConflict
	}
	for key, b := range next {
		if !selected[b.VolumeID] {
			if b.Role == "root" {
				return nil, ErrConflict
			}
			b.Attached = false
			next[key] = b
		}
	}
	state := NewState(scope)
	state.Bindings = next
	if err := state.Validate(scope); err != nil {
		return nil, err
	}
	return next, nil
}
func sameRestoreIntent(a, b *RestoreIntent) bool { return reflect.DeepEqual(a, b) }
