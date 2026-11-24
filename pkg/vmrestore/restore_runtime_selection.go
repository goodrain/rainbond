package vmrestore

import (
	"sort"
	"strings"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// readSelectedRuntimeConfig interprets the existing Console declaration without
// rewriting it. Display names and read-only installation media are not PVC
// identities; the native admission checks those media against the live render.
func readSelectedRuntimeConfig(db *gorm.DB, scope Scope, forUpdate bool) (RestoreConfig, error) {
	query := func() *gorm.DB {
		if forUpdate {
			return lockedQuery(db)
		}
		return db
	}
	var service dbmodel.TenantServices
	if err := query().Where("service_id = ? AND tenant_id = ?", scope.ServiceID, scope.TenantID).Take(&service).Error; err != nil {
		return RestoreConfig{}, err
	}
	var rows []dbmodel.TenantServiceVolume
	if err := query().Where("service_id = ?", scope.ServiceID).Order("ID").Find(&rows).Error; err != nil {
		return RestoreConfig{}, err
	}
	var attrs []dbmodel.ComponentK8sAttributes
	if err := query().Where("component_id = ? AND name IN (?)", scope.ServiceID, []string{"vm_boot_mode", "vm_disk_layout"}).Find(&attrs).Error; err != nil {
		return RestoreConfig{}, err
	}
	values := map[string]string{}
	for _, attr := range attrs {
		if _, exists := values[attr.Name]; exists || attr.TenantID != scope.TenantID || (attr.SaveType != "" && attr.SaveType != "string" && (attr.Name != "vm_disk_layout" || attr.SaveType != "json")) {
			return RestoreConfig{}, ErrCorrupt
		}
		values[attr.Name] = attr.AttributeValue
	}
	boot := strings.ToLower(strings.TrimSpace(values["vm_boot_mode"]))
	switch boot {
	case "", "legacy":
		boot = "bios"
	case "efi":
		boot = "uefi"
	}
	items, err := decodeBeforeLayout(values["vm_disk_layout"])
	if err != nil {
		return RestoreConfig{}, err
	}
	byKey := map[string]dbmodel.TenantServiceVolume{}
	for _, row := range rows {
		if !pvcDisk(row) {
			continue
		}
		if _, duplicate := byKey[row.VolumeName]; duplicate {
			return RestoreConfig{}, ErrCorrupt
		}
		byKey[row.VolumeName] = row
	}
	if len(items) == 0 && len(byKey) == 1 {
		for _, row := range byKey {
			if platformDiskRole(row) != "root" {
				return RestoreConfig{}, ErrCorrupt
			}
			items = append(items, beforeLayoutItem{DiskKey: row.VolumeName})
		}
	}
	seen := map[string]bool{}
	for i := range items {
		item := &items[i]
		if item.DiskKey == "" || seen[item.DiskKey] || item.OrderIndex < 0 {
			return RestoreConfig{}, ErrCorrupt
		}
		seen[item.DiskKey] = true
		row, exists := byKey[item.DiskKey]
		switch item.SourceKind {
		case "container_disk", "installer":
			if exists || item.DeviceType != "cdrom" || item.DiskRole == "root" || (item.SourceKind == "container_disk" && strings.TrimSpace(item.Image) == "") {
				return RestoreConfig{}, ErrCorrupt
			}
			continue
		case "", "volume":
			if !exists {
				return RestoreConfig{}, ErrCorrupt
			}
		default:
			return RestoreConfig{}, ErrCorrupt
		}
		if item.DiskRole == "" {
			item.DiskRole = platformDiskRole(row)
		}
		if item.DiskRole != platformDiskRole(row) {
			return RestoreConfig{}, ErrCorrupt
		}
		device, bus := defaultBeforeDevice(row.VolumePath)
		if item.DeviceType == "" {
			item.DeviceType = device
		}
		if item.Bus == "" {
			if item.DiskRole == "root" {
				// Bootstrap can choose a different root bus. Inspect presence only;
				// an ambiguous bootstrap layout must keep an explicit bus.
				var count int
				if err := db.Model(&dbmodel.ComponentK8sAttributes{}).Where("component_id = ? AND name IN (?) AND TRIM(attribute_value) <> ''", scope.ServiceID, []string{"vm_ssh_username", "vm_ssh_authorized_key", "vm_login_password_hash", "vm_login_password"}).Count(&count).Error; err != nil {
					return RestoreConfig{}, err
				}
				if count != 0 {
					return RestoreConfig{}, ErrCorrupt
				}
			}
			item.Bus = bus
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].OrderIndex != items[j].OrderIndex {
			return items[i].OrderIndex < items[j].OrderIndex
		}
		if items[i].DiskRole != items[j].DiskRole {
			return items[i].DiskRole == "root"
		}
		return items[i].DiskKey < items[j].DiskKey
	})
	config := RestoreConfig{Hardware: SnapshotConfig{CPUMilli: service.ContainerCPU, MemoryMiB: int64(service.ContainerMemory), BootMode: boot, DiskLayout: []SnapshotLayout{}}, Volumes: []RestoreVolumeConfig{}}
	for index, item := range items {
		if item.SourceKind == "container_disk" || item.SourceKind == "installer" {
			continue
		}
		row := byKey[item.DiskKey]
		order := uint(index + 1)
		if item.BootOrder != nil {
			order = *item.BootOrder
		}
		mode := row.AccessMode
		switch strings.ToUpper(strings.TrimSpace(mode)) {
		case "RWO", "READWRITEONCE":
			mode = "ReadWriteOnce"
		case "RWX", "READWRITEMANY":
			mode = "ReadWriteMany"
		case "ROX", "READONLYMANY":
			mode = "ReadOnlyMany"
		}
		config.Hardware.DiskLayout = append(config.Hardware.DiskLayout, SnapshotLayout{VolumeID: row.ID, DiskKey: row.VolumeName, CanonicalName: canonicalVolumeName(row.ID), Role: item.DiskRole, Device: item.DeviceType, Bus: item.Bus, BootOrder: order, ReadOnly: row.IsReadOnly})
		config.Volumes = append(config.Volumes, RestoreVolumeConfig{VolumeID: row.ID, DiskKey: row.VolumeName, CapacityGiB: row.VolumeCapacity, StorageClass: row.VolumeType, ReadOnly: row.IsReadOnly, AccessMode: mode})
	}
	return config, config.validate(scope)
}

func requireBeforeNativeConfig(tx *gorm.DB, scope Scope, expected RestoreConfig) error {
	actual, err := readSelectedRuntimeConfig(tx, scope, true)
	if err != nil {
		return err
	}
	want, err := RestoreConfigHash(expected)
	if err != nil {
		return err
	}
	got, err := RestoreConfigHash(actual)
	if err != nil || want != got {
		return ErrConflict
	}
	return nil
}
