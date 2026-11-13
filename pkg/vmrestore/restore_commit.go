package vmrestore

import (
	"reflect"
	"sort"
	"strconv"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

func restoreOwner(c Current, scope Scope, expected Expectation) error {
	if c.Receipt == nil || c.Receipt.Restore == nil || c.Receipt.Scope != scope || c.Receipt.ManifestHash != expected.ManifestHash || c.Receipt.OperationID != expected.OperationID || c.State.Fence == nil || c.State.Fence.Kind != "restore" || c.State.Fence.OperationID != expected.OperationID || c.State.Fence.ManifestHash != expected.ManifestHash {
		return ErrConflict
	}
	return nil
}
func requireDiskRows(tx *gorm.DB, scope Scope, config RestoreConfig, exactConfig bool) error {
	for _, v := range config.Volumes {
		var rows []dbmodel.TenantServiceVolume
		if err := lockedQuery(tx).Where("ID = ?", v.VolumeID).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) != 1 || rows[0].ServiceID != scope.ServiceID || rows[0].VolumeName != v.DiskKey || !pvcDisk(rows[0]) {
			return ErrConflict
		}
		if exactConfig && (rows[0].VolumeCapacity != v.CapacityGiB || rows[0].VolumeType != v.StorageClass || rows[0].IsReadOnly != v.ReadOnly || rows[0].AccessMode != v.AccessMode) {
			return ErrConflict
		}
	}
	return nil
}
func requireBeforeConfig(tx *gorm.DB, scope Scope, config RestoreConfig) error {
	var service dbmodel.TenantServices
	if err := tx.Where("service_id = ? AND tenant_id = ?", scope.ServiceID, scope.TenantID).Take(&service).Error; err != nil {
		return err
	}
	if service.ContainerCPU != config.Hardware.CPUMilli || int64(service.ContainerMemory) != config.Hardware.MemoryMiB {
		return ErrConflict
	}
	if err := requireDiskRows(tx, scope, config, true); err != nil {
		return err
	}
	return requireBeforeBootLayout(tx, scope, config)
}

// PrepareRestoreDB only persists verified live B identities and Region intent.
// The execution layer must obtain old from the unique live Resolver immediately
// before this CAS, and verify the source native manifest independently.
func (s *Store) PrepareRestoreDB(scope Scope, expected Expectation, intent RestoreIntent, old map[string]Binding) (Current, error) {
	if expected.Step != "restore-prepare" || intent.validate(scope) != nil || validateBindings(old) != nil || len(old) != len(intent.Before.Volumes) {
		return Current{}, ErrCorrupt
	}
	expected.RequestHash, _ = snapshotDigest(struct {
		Intent RestoreIntent
		Old    map[string]Binding
	}{intent, old})
	tx := s.db.Begin()
	if tx.Error != nil {
		return Current{}, tx.Error
	}
	defer tx.Rollback()
	result, err := s.MutateTx(tx, scope, expected, func(c Current) (Mutation, error) {
		if c.State.Fence != nil || c.State.Admission != nil || c.Receipt != nil || c.State.BindingGeneration != intent.SourceGeneration {
			return Mutation{}, ErrBusy
		}
		if err := requireBeforeConfig(tx, scope, intent.Before); err != nil {
			return Mutation{}, err
		}
		var rows []dbmodel.TenantServiceVolume
		if err := lockedQuery(tx).Where("service_id = ?", scope.ServiceID).Find(&rows).Error; err != nil {
			return Mutation{}, err
		}
		count := 0
		for _, row := range rows {
			if !pvcDisk(row) {
				continue
			}
			count++
			binding, ok := old[strconv.FormatUint(uint64(row.ID), 10)]
			if !ok || binding.DiskKey != row.VolumeName || binding.Role != platformDiskRole(row) || binding.ActivePVC.Namespace != scope.Namespace {
				return Mutation{}, ErrConflict
			}
		}
		if count != len(old) {
			return Mutation{}, ErrConflict
		}
		for k, b := range c.State.Bindings {
			if !reflect.DeepEqual(old[k], b) {
				return Mutation{}, ErrConflict
			}
		}
		receipt := NewReceipt(scope, expected.OperationID, expected.ManifestHash)
		receipt.OldBindings = cloneBindings(old)
		receipt.RetainOldPVCs = true
		receipt.Restore = &intent
		c.State.Fence = &Fence{Kind: "restore", Phase: "prepared", OperationID: expected.OperationID, ManifestHash: expected.ManifestHash, TargetVMUID: intent.CurrentVM.UID, TargetVMAbsent: intent.CurrentVM.UID == ""}
		return Mutation{State: c.State, Receipt: &receipt}, nil
	})
	if err != nil {
		return Current{}, err
	}
	if err := tx.Commit().Error; err != nil {
		return Current{}, err
	}
	return result, nil
}

type restoredLayoutAttribute struct {
	DiskKey    string `json:"disk_key"`
	DiskName   string `json:"disk_name"`
	DiskRole   string `json:"disk_role"`
	DeviceType string `json:"device_type"`
	SourceKind string `json:"source_kind"`
	OrderIndex int    `json:"order_index"`
	Boot       bool   `json:"boot"`
	Bus        string `json:"bus"`
	BootOrder  uint   `json:"boot_order"`
}

func writeRestoreConfig(tx *gorm.DB, scope Scope, config RestoreConfig) error {
	if err := config.validate(scope); err != nil {
		return err
	}
	if err := requireDiskRows(tx, scope, config, false); err != nil {
		return err
	}
	q := tx.Model(&dbmodel.TenantServices{}).Where("service_id = ? AND tenant_id = ?", scope.ServiceID, scope.TenantID).Updates(map[string]interface{}{"container_cpu": config.Hardware.CPUMilli, "container_memory": config.Hardware.MemoryMiB})
	if q.Error != nil {
		return q.Error
	}
	for _, v := range config.Volumes {
		q := tx.Model(&dbmodel.TenantServiceVolume{}).Where("ID = ? AND service_id = ?", v.VolumeID, scope.ServiceID).Updates(map[string]interface{}{"volume_capacity": v.CapacityGiB, "volume_type": v.StorageClass, "is_read_only": v.ReadOnly, "access_mode": v.AccessMode})
		if q.Error != nil {
			return q.Error
		}
	}
	// RestoreConfigHash treats request array order as immaterial. Derive the
	// persisted display/order projection from those sealed settings as well.
	disks := append([]SnapshotLayout(nil), config.Hardware.DiskLayout...)
	sort.Slice(disks, func(i, j int) bool {
		a, b := disks[i], disks[j]
		if a.BootOrder == b.BootOrder {
			return a.VolumeID < b.VolumeID
		}
		if a.BootOrder == 0 {
			return false
		}
		return b.BootOrder == 0 || a.BootOrder < b.BootOrder
	})
	layout := []restoredLayoutAttribute{}
	for index, d := range disks {
		layout = append(layout, restoredLayoutAttribute{DiskKey: d.DiskKey, DiskName: d.CanonicalName, DiskRole: d.Role, DeviceType: d.Device, SourceKind: "volume", OrderIndex: index, Boot: d.BootOrder > 0, Bus: d.Bus, BootOrder: d.BootOrder})
	}
	value, err := EncodeRecord(layout)
	if err != nil {
		return err
	}
	for key, value := range map[string]string{"vm_boot_mode": config.Hardware.BootMode, "vm_disk_layout": value} {
		var rows []dbmodel.ComponentK8sAttributes
		if err := lockedQuery(tx).Where("component_id = ? AND name = ?", scope.ServiceID, key).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) > 1 || (len(rows) == 1 && (rows[0].TenantID != scope.TenantID || rows[0].Name != key)) {
			return ErrCorrupt
		}
		row := dbmodel.ComponentK8sAttributes{TenantID: scope.TenantID, ComponentID: scope.ServiceID, Name: key, SaveType: "string", AttributeValue: value}
		if key == "vm_disk_layout" {
			row.SaveType = "json"
		}
		if len(rows) == 0 {
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Model(&dbmodel.ComponentK8sAttributes{}).Where("ID = ? AND component_id = ? AND tenant_id = ?", rows[0].ID, scope.ServiceID, scope.TenantID).Updates(map[string]interface{}{"attribute_value": value, "save_type": row.SaveType}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// CommitRestoreDB atomically changes all bindings plus selected Region configuration.
// restore-copy-verified is sealed only by the internal execution engine after full
// persisted inspect/copy/remount-verify evidence. No public endpoint accepts it.
func (s *Store) CommitRestoreDB(scope Scope, expected Expectation, request RestoreCommitRequest) (Current, error) {
	expected.RequestHash, _ = snapshotDigest(request)
	if expected.Step != RestoreCommitStep || request.Config.validate(scope) != nil || len(request.Config.Volumes) != len(request.Config.Hardware.DiskLayout) {
		return Current{}, ErrCorrupt
	}
	tx := s.db.Begin()
	if tx.Error != nil {
		return Current{}, tx.Error
	}
	defer tx.Rollback()
	result, err := s.MutateTx(tx, scope, expected, func(c Current) (Mutation, error) {
		if err := restoreOwner(c, scope, expected); err != nil {
			return Mutation{}, err
		}
		intent := c.Receipt.Restore
		hash, err := RestoreConfigHash(request.Config)
		if err != nil || hash != intent.ConfigAfterHash {
			return Mutation{}, ErrConflict
		}
		if c.State.Fence.Phase != "copy-verified" || c.State.BindingGeneration != intent.SourceGeneration || !c.Receipt.Steps["restore-copy-verified"].Applied || !c.Receipt.Steps["restore-writers-clear"].Applied {
			return Mutation{}, ErrConflict
		}
		if err := requireBeforeConfig(tx, scope, intent.Before); err != nil {
			return Mutation{}, err
		}
		targets := map[uint]ObjectRef{}
		for _, d := range request.Config.Hardware.DiskLayout {
			if d.ReadOnly {
				continue
			}
			binding, ok := request.Bindings[strconv.FormatUint(uint64(d.VolumeID), 10)]
			if !ok {
				return Mutation{}, ErrConflict
			}
			targets[d.VolumeID] = binding.ActivePVC
			found := false
			for _, resource := range c.Receipt.Resources {
				if resource.Purpose == "restore-target" && sameObject(resource.ObjectRef, binding.ActivePVC) {
					found = true
				}
			}
			if !found {
				return Mutation{}, ErrConflict
			}
		}
		want, err := BuildRestoreBindings(scope, expected.OperationID, intent.SourceSnapshot.UID, c.Receipt.OldBindings, request.Config, targets)
		if err != nil || !reflect.DeepEqual(want, request.Bindings) {
			return Mutation{}, ErrConflict
		}
		if err := writeRestoreConfig(tx, scope, request.Config); err != nil {
			return Mutation{}, err
		}
		c.State.Bindings = cloneBindings(request.Bindings)
		c.State.BindingGeneration++
		c.State.Fence.Phase = "region-committed"
		c.Receipt.NewBindings = cloneBindings(request.Bindings)
		c.Receipt.ConfigCommitHash = expected.RequestHash
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		return Current{}, err
	}
	if err := tx.Commit().Error; err != nil {
		return Current{}, err
	}
	return result, nil
}

// AcknowledgeRestoreDB records the trusted Console worker's durable commit ACK.
// The Console worker sends only after reloading its own committed manifest/job.
func (s *Store) AcknowledgeRestoreDB(scope Scope, expected Expectation, configCommitHash, consoleDocumentHash string) (Current, error) {
	expected.RequestHash, _ = snapshotDigest(struct{ Commit, Document string }{configCommitHash, consoleDocumentHash})
	if expected.Step != "restore-console-ack" || !hashPattern.MatchString(consoleDocumentHash) || !hashPattern.MatchString(configCommitHash) {
		return Current{}, ErrCorrupt
	}
	return s.Mutate(scope, expected, func(c Current) (Mutation, error) {
		if err := restoreOwner(c, scope, expected); err != nil {
			return Mutation{}, err
		}
		if c.State.Fence.Phase != "region-committed" || c.Receipt.ConfigCommitHash != configCommitHash || !c.Receipt.Steps[RestoreCommitStep].Applied {
			return Mutation{}, ErrConflict
		}
		c.Receipt.ConsoleAckHash = consoleDocumentHash
		c.State.Fence.Phase = "console-acked"
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
}

// RollbackRestoreDB retains pinned materialized B bindings and advances generation.
// It never clears the mapping to make canonical manual<ID> fallback legal again.
// Engine requires a fresh stopped/no-writer observation before calling this.
func (s *Store) RollbackRestoreDB(scope Scope, expected Expectation) (Current, error) {
	expected.RequestHash, _ = snapshotDigest(struct{ OperationID, CoreHash, Action string }{expected.OperationID, expected.ManifestHash, "rollback"})
	if expected.Step != "restore-rollback" {
		return Current{}, ErrCorrupt
	}
	tx := s.db.Begin()
	if tx.Error != nil {
		return Current{}, tx.Error
	}
	defer tx.Rollback()
	result, err := s.MutateTx(tx, scope, expected, func(c Current) (Mutation, error) {
		if err := restoreOwner(c, scope, expected); err != nil {
			return Mutation{}, err
		}
		commit := c.Receipt.Steps[RestoreCommitStep]
		if c.State.Fence.Phase != "rollback-stopped" || !c.Receipt.Steps["restore-rollback-writers-clear"].Applied || !commit.Applied || c.Receipt.Steps["restore-finalize"].Applied || c.Receipt.Steps["restore-rollback"].Applied {
			return Mutation{}, ErrConflict
		}
		// A newer binding generation belongs to a different physical selection.
		// Even this operation's current CAS cannot authorize overwriting it with B.
		if commit.BindingGeneration != c.State.BindingGeneration || commit.RequestHash != c.Receipt.ConfigCommitHash || !reflect.DeepEqual(c.State.Bindings, c.Receipt.NewBindings) {
			return Mutation{}, ErrConflict
		}
		if err := writeRestoreConfig(tx, scope, c.Receipt.Restore.Before); err != nil {
			return Mutation{}, err
		}
		c.State.Bindings = cloneBindings(c.Receipt.OldBindings)
		c.State.BindingGeneration++
		c.State.Fence.Phase = "region-rolled-back"
		c.Receipt.RollbackConfigHash, _ = snapshotDigest(struct {
			Config     RestoreConfig
			Bindings   map[string]Binding
			Generation uint64
		}{c.Receipt.Restore.Before, c.Receipt.OldBindings, c.State.BindingGeneration})
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		return Current{}, err
	}
	if err := tx.Commit().Error; err != nil {
		return Current{}, err
	}
	return result, nil
}

// AcknowledgeRollbackDB records Console's durable B rollback commit separately
// from the original A commit ACK. Neither ACK releases the operation fence.
func (s *Store) AcknowledgeRollbackDB(scope Scope, expected Expectation, rollbackConfigHash, consoleDocumentHash string) (Current, error) {
	if expected.Step != "restore-rollback-ack" || !hashPattern.MatchString(rollbackConfigHash) || !hashPattern.MatchString(consoleDocumentHash) {
		return Current{}, ErrCorrupt
	}
	expected.RequestHash, _ = snapshotDigest(struct{ Config, Document string }{rollbackConfigHash, consoleDocumentHash})
	return s.Mutate(scope, expected, func(c Current) (Mutation, error) {
		if err := restoreOwner(c, scope, expected); err != nil {
			return Mutation{}, err
		}
		if c.State.Fence.Phase != "region-rolled-back" || !c.Receipt.Steps["restore-rollback"].Applied || c.Receipt.RollbackConfigHash != rollbackConfigHash {
			return Mutation{}, ErrConflict
		}
		c.Receipt.RollbackAckHash = consoleDocumentHash
		c.State.Fence.Phase = "rollback-console-acked"
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
}
