package vmrestore

import (
	"encoding/json"
	"strings"
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
)

func (f restoreCommitFixture) seedBeforeAttributes(t *testing.T) {
	t.Helper()
	layout := []restoredLayoutAttribute{}
	for i, d := range f.intent.Before.Hardware.DiskLayout {
		layout = append(layout, restoredLayoutAttribute{DiskKey: d.DiskKey, DiskName: d.CanonicalName, DiskRole: d.Role, DeviceType: d.Device, SourceKind: "volume", OrderIndex: i, Boot: d.BootOrder > 0, Bus: d.Bus, BootOrder: d.BootOrder})
	}
	data, err := json.Marshal(layout)
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"vm_boot_mode": f.intent.Before.Hardware.BootMode, "vm_disk_layout": string(data)} {
		saveType := "string"
		if name == "vm_disk_layout" {
			saveType = "json"
		}
		if err := f.db.Create(&dbmodel.ComponentK8sAttributes{TenantID: f.scope.TenantID, ComponentID: f.scope.ServiceID, Name: name, SaveType: saveType, AttributeValue: value}).Error; err != nil {
			t.Fatal(err)
		}
	}
}

// capability_id: rainbond.vm-restore.pinned-before-boot-layout
func TestPrepareRejectsStaleAndAmbiguousBootLayoutWithoutWrites(t *testing.T) {
	for _, kind := range []string{"boot", "bus", "device", "boot-order", "disk-name", "role", "source", "missing-disk", "extra-disk", "duplicate-disk", "duplicate-member", "case-alias", "invalid-json", "json-tail", "wrong-type", "unknown-field", "duplicate-attribute", "foreign-attribute", "save-type", "unknown-boot", "conflicting-boot-flag", "inconsistent-order", "mixed-boot-orders"} {
		t.Run(kind, func(t *testing.T) {
			f := newRestoreCommitFixture(t)
			f.seedBeforeAttributes(t)
			var row dbmodel.ComponentK8sAttributes
			if err := f.db.Where("component_id = ? AND name = ?", f.scope.ServiceID, "vm_disk_layout").Take(&row).Error; err != nil {
				t.Fatal(err)
			}
			value := row.AttributeValue
			switch kind {
			case "boot", "unknown-boot":
				mode := "uefi"
				if kind == "unknown-boot" {
					mode = "unrecognized"
				}
				if err := f.db.Model(&dbmodel.ComponentK8sAttributes{}).Where("component_id = ? AND name = ?", f.scope.ServiceID, "vm_boot_mode").Update("attribute_value", mode).Error; err != nil {
					t.Fatal(err)
				}
			case "bus":
				value = strings.Replace(value, `"bus":"virtio"`, `"bus":"sata"`, 1)
			case "device":
				value = strings.Replace(value, `"device_type":"disk"`, `"device_type":"lun"`, 1)
			case "boot-order":
				value = strings.Replace(value, `"boot_order":1`, `"boot_order":4`, 1)
			case "disk-name":
				value = strings.Replace(value, `"disk_name":"manual7"`, `"disk_name":"manual8"`, 1)
			case "role":
				value = strings.Replace(value, `"disk_role":"root"`, `"disk_role":"data"`, 1)
			case "source":
				value = strings.Replace(value, `"source_kind":"volume"`, `"source_kind":"container_disk"`, 1)
			case "missing-disk":
				var a []json.RawMessage
				json.Unmarshal([]byte(value), &a)
				a = a[:2]
				data, _ := json.Marshal(a)
				value = string(data)
			case "extra-disk":
				value = strings.TrimSuffix(value, "]") + `,{"disk_key":"unknown"}]`
			case "duplicate-disk":
				var a []json.RawMessage
				json.Unmarshal([]byte(value), &a)
				a[1] = a[0]
				data, _ := json.Marshal(a)
				value = string(data)
			case "duplicate-member":
				value = strings.Replace(value, `"bus":"virtio"`, `"bus":"sata","bus":"virtio"`, 1)
			case "case-alias":
				value = strings.Replace(value, `"bus":"virtio"`, `"BUS":"sata","bus":"virtio"`, 1)
			case "invalid-json":
				value = value[:len(value)-1]
			case "json-tail":
				value += `[]`
			case "wrong-type":
				value = `{}`
			case "unknown-field":
				value = strings.Replace(value, `"disk_key":`, `"arbitrary":true,"disk_key":`, 1)
			case "duplicate-attribute":
				row.ID = 0
				if err := f.db.Create(&row).Error; err != nil {
					t.Fatal(err)
				}
			case "foreign-attribute":
				if err := f.db.Model(&row).Update("tenant_id", "foreign").Error; err != nil {
					t.Fatal(err)
				}
			case "save-type":
				if err := f.db.Model(&row).Update("save_type", "unsupported").Error; err != nil {
					t.Fatal(err)
				}
			case "conflicting-boot-flag":
				value = strings.Replace(value, `"boot":true`, `"boot":false`, 1)
			case "inconsistent-order":
				value = strings.Replace(value, `"order_index":0`, `"order_index":9`, 1)
			case "mixed-boot-orders":
				value = strings.Replace(value, `,"boot_order":1`, "", 1)
			}
			if value != row.AttributeValue {
				if err := f.db.Model(&dbmodel.ComponentK8sAttributes{}).Where("ID = ?", row.ID).Update("attribute_value", value).Error; err != nil {
					t.Fatal(err)
				}
			}
			before := f.databaseImage(t)
			if _, err := f.store.PrepareRestoreDB(f.scope, f.expect, f.intent, f.old); err == nil || f.databaseImage(t) != before {
				t.Fatal("stale or ambiguous B boot/layout was sealed or changed SQL")
			}
		})
	}
}

func TestPrepareAcceptsCanonicalSingleDiskDefaultsAndRejectsForgedBus(t *testing.T) {
	for _, kind := range []string{"absent", "blank", "empty-array", "null", "legacy-alias", "forged-bus", "ambiguous-default", "bootstrap-dependent"} {
		t.Run(kind, func(t *testing.T) {
			f := newRestoreCommitFixture(t)
			if err := f.db.Where("ID IN (?)", []uint{8, 9}).Delete(&dbmodel.TenantServiceVolume{}).Error; err != nil {
				t.Fatal(err)
			}
			delete(f.old, "8")
			delete(f.old, "9")
			f.intent.Before.Hardware.DiskLayout = f.intent.Before.Hardware.DiskLayout[:1]
			f.intent.Before.Hardware.DiskLayout[0].Bus = "sata"
			f.intent.Before.Volumes = f.intent.Before.Volumes[:1]
			if err := f.db.Model(&dbmodel.TenantServiceVolume{}).Where("ID = ?", 7).Update("volume_path", "/disk").Error; err != nil {
				t.Fatal(err)
			}
			if kind == "forged-bus" {
				f.intent.Before.Hardware.DiskLayout[0].Bus = "virtio"
			}
			if kind == "ambiguous-default" {
				row := dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 8}, ServiceID: f.scope.ServiceID, VolumeName: "extra", VolumePath: "/disk-extra", VolumeType: "cephfs-vm-snapshot", VolumeCapacity: 10, AccessMode: "ReadWriteMany"}
				if err := f.db.Create(&row).Error; err != nil {
					t.Fatal(err)
				}
				// Keep every disk row, selected setting and UID consistent. The
				// sole ambiguity is the absent multi-disk ordering contract.
				f.intent.Before.Volumes = append(f.intent.Before.Volumes, RestoreVolumeConfig{VolumeID: 8, DiskKey: "extra", CapacityGiB: 10, StorageClass: row.VolumeType, AccessMode: row.AccessMode})
				f.intent.Before.Hardware.DiskLayout = append(f.intent.Before.Hardware.DiskLayout, SnapshotLayout{VolumeID: 8, DiskKey: "extra", CanonicalName: "manual8", Role: "data", Device: "disk", Bus: "scsi", BootOrder: 2})
				f.old["8"] = Binding{VolumeID: 8, DiskKey: "extra", CanonicalName: "manual8", Role: "data", Attached: true, ActivePVC: ObjectRef{Kind: "PersistentVolumeClaim", Namespace: f.scope.Namespace, Name: "manual8", UID: "original-b-extra"}}
			}
			if kind == "bootstrap-dependent" {
				if err := f.db.Create(&dbmodel.ComponentK8sAttributes{TenantID: f.scope.TenantID, ComponentID: f.scope.ServiceID, Name: "vm_ssh_username", SaveType: "string", AttributeValue: "NON_SECRET_METADATA_SENTINEL"}).Error; err != nil {
					t.Fatal(err)
				}
			}
			if kind == "blank" || kind == "empty-array" || kind == "null" || kind == "legacy-alias" {
				layout, boot := "", ""
				if kind == "empty-array" {
					layout = "[]"
				}
				if kind == "null" {
					layout = "null"
				}
				if kind == "legacy-alias" {
					boot = " LEGACY "
				}
				for name, value := range map[string]string{"vm_boot_mode": boot, "vm_disk_layout": layout} {
					if err := f.db.Create(&dbmodel.ComponentK8sAttributes{TenantID: f.scope.TenantID, ComponentID: f.scope.ServiceID, Name: name, AttributeValue: value}).Error; err != nil {
						t.Fatal(err)
					}
				}
			}
			before := f.databaseImage(t)
			result, err := f.store.PrepareRestoreDB(f.scope, f.expect, f.intent, f.old)
			refuse := kind == "forged-bus" || kind == "ambiguous-default" || kind == "bootstrap-dependent"
			if refuse {
				if err == nil || f.databaseImage(t) != before {
					t.Fatal("unproved default B layout was accepted")
				}
			} else if err != nil || result.Receipt.Restore.Before.Hardware.BootMode != "bios" || result.Receipt.Restore.Before.Hardware.DiskLayout[0].Bus != "sata" {
				t.Fatalf("canonical B defaults were rejected: %v", err)
			}
		})
	}
}

func TestPrepareAcceptsLegacyLayoutWithProvenDeviceDefaults(t *testing.T) {
	f := newRestoreCommitFixture(t)
	for i, id := range []uint{7, 8, 9} {
		path, bus := "/disk-data", "scsi"
		if i == 0 {
			path, bus = "/disk", "sata"
		}
		f.intent.Before.Hardware.DiskLayout[i].Bus = bus
		if err := f.db.Model(&dbmodel.TenantServiceVolume{}).Where("ID = ?", id).Update("volume_path", path).Error; err != nil {
			t.Fatal(err)
		}
	}
	layout := `[{"disk_key":"disk","disk_role":"root","order_index":0,"boot":true},{"disk_key":"data-1","order_index":1,"boot":false},{"disk_key":"data-2","order_index":2,"boot":false}]`
	if err := f.db.Create(&dbmodel.ComponentK8sAttributes{TenantID: f.scope.TenantID, ComponentID: f.scope.ServiceID, Name: "vm_disk_layout", SaveType: "json", AttributeValue: layout}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.PrepareRestoreDB(f.scope, f.expect, f.intent, f.old); err != nil {
		t.Fatalf("effective legacy B layout rejected: %v", err)
	}
}

func TestCommitRejectsBootLayoutDriftAfterPrepare(t *testing.T) {
	for _, name := range []string{"vm_boot_mode", "vm_disk_layout"} {
		t.Run(name, func(t *testing.T) {
			f := newRestoreCommitFixture(t)
			f.prepare(t)
			f.sealCopy(t)
			var row dbmodel.ComponentK8sAttributes
			if err := f.db.Where("component_id = ? AND name = ?", f.scope.ServiceID, name).Take(&row).Error; err != nil {
				t.Fatal(err)
			}
			value := "uefi"
			if name == "vm_disk_layout" {
				value = strings.Replace(row.AttributeValue, `"bus":"virtio"`, `"bus":"sata"`, 1)
			}
			if err := f.db.Model(&row).Update("attribute_value", value).Error; err != nil {
				t.Fatal(err)
			}
			before := f.databaseImage(t)
			if _, err := f.store.CommitRestoreDB(f.scope, f.expectStep(t, RestoreCommitStep), f.request(t)); err == nil || f.databaseImage(t) != before {
				t.Fatal("commit overwrote B configuration changed after preparation")
			}
		})
	}
}

func TestPreparePinsMatchingUEFIAndStorageBusesForRollback(t *testing.T) {
	f := newRestoreCommitFixture(t)
	f.intent.Before.Hardware.BootMode = "uefi"
	f.intent.Before.Hardware.DiskLayout[0].Bus = "sata"
	f.intent.Before.Hardware.DiskLayout[1].Bus = "scsi"
	f.seedBeforeAttributes(t)
	var original dbmodel.ComponentK8sAttributes
	if err := f.db.Where("component_id = ? AND name = ?", f.scope.ServiceID, "vm_disk_layout").Take(&original).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.PrepareRestoreDB(f.scope, f.expect, f.intent, f.old); err != nil {
		t.Fatal(err)
	}
	f.sealCopy(t)
	if _, err := f.store.CommitRestoreDB(f.scope, f.expectStep(t, RestoreCommitStep), f.request(t)); err != nil {
		t.Fatal(err)
	}
	f.rollbackReady(t)
	if _, err := f.store.RollbackRestoreDB(f.scope, f.expectStep(t, "restore-rollback")); err != nil {
		t.Fatal(err)
	}
	var boot, layout dbmodel.ComponentK8sAttributes
	if err := f.db.Where("component_id = ? AND name = ?", f.scope.ServiceID, "vm_boot_mode").Take(&boot).Error; err != nil || boot.AttributeValue != "uefi" {
		t.Fatal("B UEFI was not preserved")
	}
	if err := f.db.Where("component_id = ? AND name = ?", f.scope.ServiceID, "vm_disk_layout").Take(&layout).Error; err != nil || layout.AttributeValue != original.AttributeValue {
		t.Fatal("B SATA/SCSI layout was not preserved")
	}
}
