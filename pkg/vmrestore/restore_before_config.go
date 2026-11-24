package vmrestore

import (
	"encoding/json"
	"io"
	"sort"
	"strings"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// Optional legacy fields are normalized only where Region has an unambiguous
// default. An unsupported layout must not be replaced by a caller's Before.
type beforeLayoutItem struct {
	DiskKey    string `json:"disk_key"`
	DiskName   string `json:"disk_name"`
	DiskRole   string `json:"disk_role"`
	DeviceType string `json:"device_type"`
	SourceKind string `json:"source_kind"`
	OrderIndex int    `json:"order_index"`
	Boot       *bool  `json:"boot"`
	Bus        string `json:"bus"`
	BootOrder  *uint  `json:"boot_order"`
	Image      string `json:"image,omitempty"`
}

func decodeBeforeLayout(raw string) ([]beforeLayoutItem, error) {
	if len(raw) > MaxRecordBytes {
		return nil, ErrCorrupt
	}
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	start, err := decoder.Token()
	if err != nil || (start != nil && start != json.Delim('[')) {
		return nil, ErrCorrupt
	}
	items := []beforeLayoutItem{}
	if start != nil {
		for decoder.More() {
			if len(items) >= 128 {
				return nil, ErrCorrupt
			}
			if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
				return nil, ErrCorrupt
			}
			fields := map[string]json.RawMessage{}
			for decoder.More() {
				token, err := decoder.Token()
				name, ok := token.(string)
				if err != nil || !ok {
					return nil, ErrCorrupt
				}
				switch name {
				case "disk_key", "disk_name", "disk_role", "device_type", "source_kind", "order_index", "boot", "bus", "boot_order", "image":
				default:
					return nil, ErrCorrupt
				}
				if _, duplicate := fields[name]; duplicate {
					return nil, ErrCorrupt
				}
				var value json.RawMessage
				if decoder.Decode(&value) != nil {
					return nil, ErrCorrupt
				}
				fields[name] = value
			}
			if token, err := decoder.Token(); err != nil || token != json.Delim('}') {
				return nil, ErrCorrupt
			}
			data, err := json.Marshal(fields)
			if err != nil {
				return nil, ErrCorrupt
			}
			var item beforeLayoutItem
			if json.Unmarshal(data, &item) != nil {
				return nil, ErrCorrupt
			}
			if _, present := fields["image"]; present && item.SourceKind != "container_disk" && item.SourceKind != "installer" {
				return nil, ErrCorrupt
			}
			items = append(items, item)
		}
		if token, err := decoder.Token(); err != nil || token != json.Delim(']') {
			return nil, ErrCorrupt
		}
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, ErrCorrupt
	}
	return items, nil
}

func defaultBeforeDevice(path string) (string, string) {
	switch {
	case path == "/disk":
		return "disk", "sata"
	case strings.HasPrefix(path, "/disk-"):
		return "disk", "scsi"
	case path == "/lun" || strings.HasPrefix(path, "/lun-"):
		return "lun", "scsi"
	case path == "/cdrom" || strings.HasPrefix(path, "/cdrom-"):
		return "cdrom", "sata"
	default:
		return "", ""
	}
}

// requireBeforeBootLayout runs under MutateTx's service/state lock and locks the
// selected attributes/volume rows too. No Kubernetes or guest state is read here.
func requireBeforeBootLayout(tx *gorm.DB, scope Scope, config RestoreConfig) error {
	var attributes []dbmodel.ComponentK8sAttributes
	if err := lockedQuery(tx).Where("component_id = ? AND name IN (?)", scope.ServiceID, []string{"vm_boot_mode", "vm_disk_layout"}).Find(&attributes).Error; err != nil {
		return err
	}
	values := map[string]string{}
	for _, row := range attributes {
		if _, duplicate := values[row.Name]; duplicate {
			return ErrCorrupt
		}
		if row.TenantID != scope.TenantID || (row.SaveType != "" && row.SaveType != "string" && (row.Name != "vm_disk_layout" || row.SaveType != "json")) {
			return ErrCorrupt
		}
		values[row.Name] = row.AttributeValue
	}
	boot := strings.ToLower(strings.TrimSpace(values["vm_boot_mode"]))
	switch boot {
	case "", "bios", "legacy":
		boot = "bios"
	case "uefi", "efi":
		boot = "uefi"
	default:
		return ErrCorrupt
	}
	if boot != config.Hardware.BootMode {
		return ErrConflict
	}
	items, err := decodeBeforeLayout(values["vm_disk_layout"])
	if err != nil {
		return err
	}
	var rows []dbmodel.TenantServiceVolume
	if err := lockedQuery(tx).Where("service_id = ?", scope.ServiceID).Order("ID").Find(&rows).Error; err != nil {
		return err
	}
	volumes := map[string]dbmodel.TenantServiceVolume{}
	for _, row := range rows {
		if !pvcDisk(row) {
			continue
		}
		if _, duplicate := volumes[row.VolumeName]; duplicate {
			return ErrCorrupt
		}
		volumes[row.VolumeName] = row
	}
	if len(items) == 0 {
		// Legacy conversion has no SQL ordering contract for multiple bare disks.
		// A single root has a well-defined default; larger layouts must be explicit.
		if len(volumes) != 1 {
			return ErrConflict
		}
		for _, row := range volumes {
			if platformDiskRole(row) != "root" {
				return ErrConflict
			}
			items = append(items, beforeLayoutItem{DiskKey: row.VolumeName})
		}
	}
	if len(items) != len(config.Hardware.DiskLayout) {
		return ErrConflict
	}
	wanted := map[uint]SnapshotLayout{}
	for _, disk := range config.Hardware.DiskLayout {
		wanted[disk.VolumeID] = disk
	}
	seen := map[string]bool{}
	explicitBootOrders := 0
	for i := range items {
		item := &items[i]
		row, exists := volumes[item.DiskKey]
		if !exists || seen[item.DiskKey] || item.OrderIndex < 0 || (item.SourceKind != "" && item.SourceKind != "volume") {
			return ErrConflict
		}
		seen[item.DiskKey] = true
		if item.DiskName != "" && item.DiskName != canonicalVolumeName(row.ID) {
			return ErrConflict
		}
		if item.DiskRole == "" {
			item.DiskRole = platformDiskRole(row)
		}
		if item.DiskRole != platformDiskRole(row) {
			return ErrConflict
		}
		device, bus := defaultBeforeDevice(row.VolumePath)
		if item.DeviceType == "" {
			item.DeviceType = device
		}
		if item.Bus == "" {
			if item.DiskRole == "root" {
				// Bootstrap can change the implicit root bus. Inspect only presence,
				// never credential values; require an explicit bus in that case.
				var count int
				if err := tx.Model(&dbmodel.ComponentK8sAttributes{}).Where("component_id = ? AND name IN (?) AND TRIM(attribute_value) <> ''", scope.ServiceID, []string{"vm_ssh_username", "vm_ssh_authorized_key", "vm_login_password_hash", "vm_login_password"}).Count(&count).Error; err != nil {
					return err
				}
				if count != 0 {
					return ErrConflict
				}
			}
			item.Bus = bus
		}
		if item.BootOrder != nil {
			explicitBootOrders++
		}
	}
	if explicitBootOrders != 0 && explicitBootOrders != len(items) {
		return ErrConflict
	}
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.OrderIndex != b.OrderIndex {
			return a.OrderIndex < b.OrderIndex
		}
		if a.DiskRole != b.DiskRole {
			return a.DiskRole == "root"
		}
		return a.DiskKey < b.DiskKey
	})
	previousOrder, reachedNonboot := uint(0), false
	for i, item := range items {
		row := volumes[item.DiskKey]
		order := uint(i + 1)
		if item.BootOrder != nil {
			order = *item.BootOrder
			if item.Boot != nil && *item.Boot != (order > 0) {
				return ErrConflict
			}
			if order == 0 {
				reachedNonboot = true
			} else {
				if reachedNonboot || order <= previousOrder {
					return ErrConflict
				}
				previousOrder = order
			}
		}
		disk, exists := wanted[row.ID]
		if !exists || disk.DiskKey != item.DiskKey || disk.Role != item.DiskRole || disk.Device != item.DeviceType || disk.Bus != item.Bus || disk.BootOrder != order {
			return ErrConflict
		}
	}
	return nil
}
