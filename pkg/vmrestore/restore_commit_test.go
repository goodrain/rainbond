package vmrestore

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

type restoreCommitFixture struct {
	store   *Store
	db      *gorm.DB
	scope   Scope
	intent  RestoreIntent
	old     map[string]Binding
	after   RestoreConfig
	targets map[uint]ObjectRef
	expect  Expectation
}

func newRestoreCommitFixture(t *testing.T) restoreCommitFixture {
	t.Helper()
	store, database, scope := fixtureStore(t)
	if err := database.AutoMigrate(&dbmodel.TenantServiceVolume{}, &dbmodel.TenantServicesPort{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&dbmodel.TenantServices{}).Where("service_id = ?", scope.ServiceID).Updates(map[string]interface{}{"container_cpu": 4000, "container_memory": 8192}).Error; err != nil {
		t.Fatal(err)
	}
	before := RestoreConfig{Hardware: SnapshotConfig{CPUMilli: 4000, MemoryMiB: 8192, BootMode: "bios", DiskLayout: []SnapshotLayout{}}, Volumes: []RestoreVolumeConfig{}}
	old := map[string]Binding{}
	for index, id := range []uint{7, 8, 9} {
		key := "data-" + strconv.Itoa(index)
		role := "data"
		if index == 0 {
			key = "disk"
			role = "root"
		}
		row := dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: id}, ServiceID: scope.ServiceID, VolumeName: key, VolumeType: "cephfs-vm-snapshot", VolumeCapacity: int64(40 - index*10), AccessMode: "ReadWriteMany"}
		if err := database.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
		before.Hardware.DiskLayout = append(before.Hardware.DiskLayout, SnapshotLayout{VolumeID: id, DiskKey: key, CanonicalName: canonicalVolumeName(id), Role: role, Device: "disk", Bus: "virtio", BootOrder: uint(index + 1)})
		before.Volumes = append(before.Volumes, RestoreVolumeConfig{VolumeID: id, DiskKey: key, CapacityGiB: row.VolumeCapacity, StorageClass: row.VolumeType, AccessMode: row.AccessMode})
		old[strconv.FormatUint(uint64(id), 10)] = Binding{VolumeID: id, DiskKey: key, CanonicalName: canonicalVolumeName(id), Role: role, Attached: true, ActivePVC: ObjectRef{Kind: "PersistentVolumeClaim", Namespace: scope.Namespace, Name: canonicalVolumeName(id), UID: "original-b-" + strconv.Itoa(index)}}
	}
	after := before
	after.Hardware = before.Hardware
	after.Hardware.CPUMilli = 2000
	after.Hardware.MemoryMiB = 4096
	after.Hardware.BootMode = "uefi"
	after.Hardware.DiskLayout = append([]SnapshotLayout(nil), before.Hardware.DiskLayout[:2]...)
	after.Volumes = append([]RestoreVolumeConfig(nil), before.Volumes[:2]...)
	after.Volumes[0].CapacityGiB = 20
	after.Volumes[1].CapacityGiB = 10
	hash, err := RestoreConfigHash(after)
	if err != nil {
		t.Fatal(err)
	}
	intent := RestoreIntent{CurrentVM: ObjectRef{Kind: "VirtualMachine", Namespace: scope.Namespace, Name: "vm-a", UID: "vm-b"}, SourceSnapshot: ObjectRef{Kind: "VirtualMachineSnapshot", Namespace: scope.Namespace, Name: "snapshot-a", UID: "snapshot-a-uid"}, SourceContent: ObjectRef{Kind: "VirtualMachineSnapshotContent", Namespace: scope.Namespace, Name: "content-a", UID: "content-a-uid"}, SourceVMUID: "historical-vm-uid", SourceCoreHash: strings.Repeat("c", 64), SourceDocumentHash: strings.Repeat("d", 64), Before: before, ConfigAfterHash: hash, WasRunning: true, PowerAfter: "previous"}
	targets := map[uint]ObjectRef{7: {Kind: "PersistentVolumeClaim", Namespace: scope.Namespace, Name: "restore-root-a", UID: "target-root-a"}, 8: {Kind: "PersistentVolumeClaim", Namespace: scope.Namespace, Name: "restore-data-a", UID: "target-data-a"}}
	e := expectation()
	e.Step = "restore-prepare"
	return restoreCommitFixture{store, database, scope, intent, old, after, targets, e}
}
func (f restoreCommitFixture) prepare(t *testing.T) Current {
	t.Helper()
	// Positive fixtures model an explicitly configured B. Default normalization
	// has separate tests which call PrepareRestoreDB directly without this seed.
	var count int
	if err := f.db.Model(&dbmodel.ComponentK8sAttributes{}).Where("component_id = ? AND name IN (?)", f.scope.ServiceID, []string{"vm_boot_mode", "vm_disk_layout"}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		f.seedBeforeAttributes(t)
	}
	c, err := f.store.PrepareRestoreDB(f.scope, f.expect, f.intent, f.old)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func (f restoreCommitFixture) expectStep(t *testing.T, step string) Expectation {
	t.Helper()
	c, err := f.store.Read(f.scope, f.expect.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	e := f.expect
	e.ControlRevision = c.State.ControlRevision
	e.BindingGeneration = c.State.BindingGeneration
	e.Step = step
	e.RequestHash, _ = snapshotDigest(step)
	return e
}
func (f restoreCommitFixture) sealCopy(t *testing.T) {
	t.Helper()
	for _, step := range []string{"restore-writers-clear", "restore-copy-verified"} {
		e := f.expectStep(t, step)
		_, err := f.store.Mutate(f.scope, e, func(c Current) (Mutation, error) {
			c.State.Fence.Phase = "copy-verified"
			for _, pvc := range f.targets {
				c.Receipt.Resources = resourceOnce(c.Receipt.Resources, ResourceRef{ObjectRef: pvc, Purpose: "restore-target"})
			}
			return Mutation{State: c.State, Receipt: c.Receipt}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
func (f restoreCommitFixture) request(t *testing.T) RestoreCommitRequest {
	t.Helper()
	bindings, err := BuildRestoreBindings(f.scope, f.expect.OperationID, f.intent.SourceSnapshot.UID, f.old, f.after, f.targets)
	if err != nil {
		t.Fatal(err)
	}
	return RestoreCommitRequest{Config: f.after, Bindings: bindings}
}

// capability_id: rainbond.vm-restore.atomic-config-bindings
func TestCommitRestoresAllDisksAndHardwareAtomicallyRetainingBAndPorts(t *testing.T) {
	f := newRestoreCommitFixture(t)
	f.prepare(t)
	f.sealCopy(t)
	request := f.request(t)
	unrelated := map[string]string{"vm_login_user": "NON_SECRET_METADATA_SENTINEL", "vm_fixed_ip_enabled": "true", "vm_fixed_ip": "192.0.2.45"}
	for key, value := range unrelated {
		if err := f.db.Create(&dbmodel.ComponentK8sAttributes{TenantID: f.scope.TenantID, ComponentID: f.scope.ServiceID, Name: key, AttributeValue: value}).Error; err != nil {
			t.Fatal(err)
		}
	}
	port := dbmodel.TenantServicesPort{TenantID: f.scope.TenantID, ServiceID: f.scope.ServiceID, ContainerPort: 22, MappingPort: 30222, Protocol: "tcp", K8sServiceName: "stable-service"}
	if err := f.db.Create(&port).Error; err != nil {
		t.Fatal(err)
	}
	e := f.expectStep(t, "restore-commit")
	result, err := f.store.CommitRestoreDB(f.scope, e, request)
	if err != nil {
		t.Fatal(err)
	}
	if result.State.BindingGeneration != 1 || result.State.Bindings["7"].ActivePVC.UID != "target-root-a" || result.State.Bindings["9"].Attached || !reflect.DeepEqual(result.Receipt.OldBindings, f.old) {
		t.Fatal("all disk switch or retained original B identity incorrect")
	}
	if result.State.Bindings["7"].PreviousPVC.UID != f.old["7"].ActivePVC.UID || result.State.Bindings["7"].ActiveDV != nil {
		t.Fatal("new blank PVC fabricated DV or lost B")
	}
	step := result.Receipt.Steps[RestoreCommitStep]
	if step.RequestHash != result.Receipt.ConfigCommitHash || step.BindingGeneration != result.State.BindingGeneration || !step.Applied {
		t.Fatal("commit did not seal the shared generation/hash witness")
	}
	if sealed, err := committedBindingFootprint(*result.Receipt, 7); err != nil || !sealed {
		t.Fatalf("shared Resolver cannot recognize the producer's sealed commit: %v", err)
	}
	var service dbmodel.TenantServices
	if err := f.db.Where("service_id = ?", f.scope.ServiceID).Take(&service).Error; err != nil {
		t.Fatal(err)
	}
	if service.ContainerCPU != 2000 || service.ContainerMemory != 4096 {
		t.Fatal("hardware was not in binding commit")
	}
	var boot, layout dbmodel.ComponentK8sAttributes
	if err := f.db.Where("component_id = ? AND name = ?", f.scope.ServiceID, "vm_boot_mode").Take(&boot).Error; err != nil || boot.AttributeValue != "uefi" {
		t.Fatal("boot mode was not restored with disks")
	}
	if err := f.db.Where("component_id = ? AND name = ?", f.scope.ServiceID, "vm_disk_layout").Take(&layout).Error; err != nil {
		t.Fatal(err)
	}
	var diskLayout []restoredLayoutAttribute
	if err := json.Unmarshal([]byte(layout.AttributeValue), &diskLayout); err != nil || len(diskLayout) != 2 || diskLayout[0].Bus != "virtio" || diskLayout[0].BootOrder != 1 || diskLayout[1].DiskKey != f.after.Volumes[1].DiskKey {
		t.Fatal("selected disk layout or bus/boot settings were lost")
	}
	var currentPort dbmodel.TenantServicesPort
	if err := f.db.Where("ID = ?", port.ID).Take(&currentPort).Error; err != nil || currentPort.MappingPort != port.MappingPort || currentPort.ContainerPort != port.ContainerPort || currentPort.K8sServiceName != port.K8sServiceName || currentPort.TenantID != port.TenantID || currentPort.ServiceID != port.ServiceID {
		t.Fatal("restore rewrote current platform ports")
	}
	for key, value := range unrelated {
		var retained dbmodel.ComponentK8sAttributes
		if err := f.db.Where("component_id = ? AND name = ?", f.scope.ServiceID, key).Take(&retained).Error; err != nil || retained.AttributeValue != value {
			t.Fatalf("restore modified current platform IP or initialization metadata: %s", key)
		}
	}
	replay, err := f.store.CommitRestoreDB(f.scope, e, request)
	if err != nil || !replay.Replayed || replay.State.BindingGeneration != 1 {
		t.Fatal("commit replay advanced generation")
	}
	request.Bindings["7"] = f.old["7"]
	if _, err := f.store.CommitRestoreDB(f.scope, e, request); !errors.Is(err, ErrConflict) {
		t.Fatal("same step accepted changed physical targets")
	}
}

func TestFailedConfigWriteRollsBackAllBindingAndHardwareChanges(t *testing.T) {
	f := newRestoreCommitFixture(t)
	f.prepare(t)
	f.sealCopy(t)
	for i := 0; i < 2; i++ {
		if err := f.db.Create(&dbmodel.ComponentK8sAttributes{TenantID: f.scope.TenantID, ComponentID: f.scope.ServiceID, Name: "vm_boot_mode", AttributeValue: "bios"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	_, err := f.store.CommitRestoreDB(f.scope, f.expectStep(t, "restore-commit"), f.request(t))
	if !errors.Is(err, ErrCorrupt) {
		t.Fatal("duplicate selected attribute did not reject commit")
	}
	c, err := f.store.Read(f.scope, f.expect.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if c.State.BindingGeneration != 0 || len(c.State.Bindings) != 0 || c.Receipt.ConfigCommitHash != "" {
		t.Fatal("failed transaction partially switched bindings")
	}
	var service dbmodel.TenantServices
	f.db.Where("service_id = ?", f.scope.ServiceID).Take(&service)
	if service.ContainerCPU != 4000 || service.ContainerMemory != 8192 {
		t.Fatal("failed transaction partially changed hardware")
	}
	var row dbmodel.TenantServiceVolume
	f.db.Where("ID = ?", 7).Take(&row)
	if row.VolumeCapacity != 40 {
		t.Fatal("failed transaction partially changed disk quota")
	}
}
func TestCommitRejectsMissingCopyProofUIDSubstitutionAndForeignVolume(t *testing.T) {
	for _, kind := range []string{"missing-proof", "wrong-pvc", "foreign-row"} {
		t.Run(kind, func(t *testing.T) {
			f := newRestoreCommitFixture(t)
			f.prepare(t)
			if kind != "missing-proof" {
				f.sealCopy(t)
			}
			request := f.request(t)
			if kind == "wrong-pvc" {
				b := request.Bindings["7"]
				b.ActivePVC.UID = "replacement"
				request.Bindings["7"] = b
			}
			if kind == "foreign-row" {
				f.db.Model(&dbmodel.TenantServiceVolume{}).Where("ID = ?", 8).Update("service_id", "different")
			}
			if _, err := f.store.CommitRestoreDB(f.scope, f.expectStep(t, "restore-commit"), request); err == nil {
				t.Fatal("unverified commit was accepted")
			}
		})
	}
}

// capability_id: rainbond.vm-restore.pinned-rollback
func TestRollbackKeepsMaterializedCanonicalIdentitiesAndAdvancesGeneration(t *testing.T) {
	f := newRestoreCommitFixture(t)
	f.prepare(t)
	f.sealCopy(t)
	committed, err := f.store.CommitRestoreDB(f.scope, f.expectStep(t, "restore-commit"), f.request(t))
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.store.AcknowledgeRestoreDB(f.scope, f.expectStep(t, "restore-console-ack"), committed.Receipt.ConfigCommitHash, strings.Repeat("e", 64))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.RollbackRestoreDB(f.scope, f.expectStep(t, "restore-rollback")); err == nil {
		t.Fatal("rollback accepted no fresh writer-clear step")
	}
	_, err = f.store.Mutate(f.scope, f.expectStep(t, "restore-rollback-writers-clear"), func(c Current) (Mutation, error) {
		c.State.Fence.Phase = "rollback-stopped"
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	e := f.expectStep(t, "restore-rollback")
	rolled, err := f.store.RollbackRestoreDB(f.scope, e)
	if err != nil {
		t.Fatal(err)
	}
	if rolled.State.BindingGeneration != 2 || !reflect.DeepEqual(rolled.State.Bindings, f.old) || rolled.State.Fence == nil {
		t.Fatal("rollback cleared binding map or released fence before Console ACK/runtime sync")
	}
	var service dbmodel.TenantServices
	if err := f.db.Where("service_id = ?", f.scope.ServiceID).Take(&service).Error; err != nil || service.ContainerCPU != f.intent.Before.Hardware.CPUMilli || int64(service.ContainerMemory) != f.intent.Before.Hardware.MemoryMiB {
		t.Fatal("rollback left A hardware with B disks")
	}
	var boot dbmodel.ComponentK8sAttributes
	if err := f.db.Where("component_id = ? AND name = ?", f.scope.ServiceID, "vm_boot_mode").Take(&boot).Error; err != nil || boot.AttributeValue != f.intent.Before.Hardware.BootMode {
		t.Fatal("rollback did not restore B boot mode")
	}
	for _, volume := range f.intent.Before.Volumes {
		var row dbmodel.TenantServiceVolume
		if err := f.db.Where("ID = ?", volume.VolumeID).Take(&row).Error; err != nil || row.VolumeCapacity != volume.CapacityGiB || row.VolumeType != volume.StorageClass || row.AccessMode != volume.AccessMode || row.IsReadOnly != volume.ReadOnly {
			t.Fatal("rollback did not restore every original B disk setting")
		}
	}
	resolution, err := rolled.State.ResolveVolume(7)
	if err != nil || !resolution.Mapped || resolution.PVC.UID != f.old["7"].ActivePVC.UID {
		t.Fatal("rollback returned to unprotected canonical fallback")
	}
	replay, err := f.store.RollbackRestoreDB(f.scope, e)
	if err != nil || !replay.Replayed || replay.State.BindingGeneration != 2 {
		t.Fatal("rollback replay changed generation")
	}
}
func TestPreparePinsCompleteBAndRejectsDriftOrIntentRewrites(t *testing.T) {
	f := newRestoreCommitFixture(t)
	f.prepare(t)
	c, err := f.store.Read(f.scope, f.expect.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Receipt.OldBindings) != 3 || c.Receipt.OldBindings["7"].RestoreID != "" {
		t.Fatal("canonical B identity missing or fabricated snapshot provenance")
	}
	replayed, err := f.store.PrepareRestoreDB(f.scope, f.expect, f.intent, f.old)
	if err != nil || !replayed.Replayed || replayed.State.ControlRevision != c.State.ControlRevision {
		t.Fatal("prepare replay repeated work or changed pinned B")
	}
	_, err = f.store.Mutate(f.scope, f.expectStep(t, "rewrite-intent"), func(c Current) (Mutation, error) {
		c.Receipt.Restore.SourceSnapshot.UID = "other"
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatal("immutable restore intent was rewritten")
	}
	f2 := newRestoreCommitFixture(t)
	delete(f2.old, "9")
	if _, err := f2.store.PrepareRestoreDB(f2.scope, f2.expect, f2.intent, f2.old); err == nil {
		t.Fatal("prepare lost B-only disk identity")
	}
}

func TestSealedCommitAndRollbackAcknowledgementsCannotBeRewritten(t *testing.T) {
	f := newRestoreCommitFixture(t)
	f.prepare(t)
	f.sealCopy(t)
	committed, err := f.store.CommitRestoreDB(f.scope, f.expectStep(t, "restore-commit"), f.request(t))
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.store.Mutate(f.scope, f.expectStep(t, "rewrite-config"), func(c Current) (Mutation, error) {
		c.Receipt.ConfigCommitHash = ""
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatal("sealed config commit hash was cleared")
	}
	_, err = f.store.AcknowledgeRestoreDB(f.scope, f.expectStep(t, "restore-console-ack"), committed.Receipt.ConfigCommitHash, strings.Repeat("e", 64))
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.store.Mutate(f.scope, f.expectStep(t, "restore-rollback-writers-clear"), func(c Current) (Mutation, error) {
		c.State.Fence.Phase = "rollback-stopped"
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	rolled, err := f.store.RollbackRestoreDB(f.scope, f.expectStep(t, "restore-rollback"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.AcknowledgeRollbackDB(f.scope, f.expectStep(t, "restore-rollback-ack"), committed.Receipt.ConfigCommitHash, strings.Repeat("f", 64)); err == nil {
		t.Fatal("original A hash was accepted as B rollback ACK")
	}
	acked, err := f.store.AcknowledgeRollbackDB(f.scope, f.expectStep(t, "restore-rollback-ack"), rolled.Receipt.RollbackConfigHash, strings.Repeat("f", 64))
	if err != nil {
		t.Fatal(err)
	}
	if acked.State.Fence == nil || acked.State.Fence.Phase != "rollback-console-acked" || acked.Receipt.ConsoleAckHash == acked.Receipt.RollbackAckHash {
		t.Fatal("rollback ACK released fence or replaced original commit evidence")
	}
}

func (f restoreCommitFixture) databaseImage(t *testing.T) string {
	t.Helper()
	var service dbmodel.TenantServices
	var volumes []dbmodel.TenantServiceVolume
	var attributes []dbmodel.ComponentK8sAttributes
	if err := f.db.Where("service_id = ?", f.scope.ServiceID).Take(&service).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Where("service_id = ?", f.scope.ServiceID).Order("ID").Find(&volumes).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Where("component_id = ?", f.scope.ServiceID).Order("ID").Find(&attributes).Error; err != nil {
		t.Fatal(err)
	}
	hash, err := snapshotDigest(struct {
		Service    dbmodel.TenantServices
		Volumes    []dbmodel.TenantServiceVolume
		Attributes []dbmodel.ComponentK8sAttributes
	}{service, volumes, attributes})
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

func (f restoreCommitFixture) rollbackReady(t *testing.T) {
	t.Helper()
	_, err := f.store.Mutate(f.scope, f.expectStep(t, "restore-rollback-writers-clear"), func(c Current) (Mutation, error) {
		c.State.Fence.Phase = "rollback-stopped"
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRestoreLateSQLFailureRollsBackConfigAndBothLedgerRows(t *testing.T) {
	for _, rollback := range []bool{false, true} {
		t.Run(strconv.FormatBool(rollback), func(t *testing.T) {
			f := newRestoreCommitFixture(t)
			f.prepare(t)
			f.sealCopy(t)
			if rollback {
				if _, err := f.store.CommitRestoreDB(f.scope, f.expectStep(t, RestoreCommitStep), f.request(t)); err != nil {
					t.Fatal(err)
				}
				f.rollbackReady(t)
			}
			before := f.databaseImage(t)
			// Fail the receipt write after hardware, volumes, attributes and state
			// have already been written inside the real SQLite transaction.
			table := f.db.NewScope(&dbmodel.ComponentK8sAttributes{}).QuotedTableName()
			key, _ := ReceiptKey(f.expect.OperationID)
			trigger := fmt.Sprintf("CREATE TRIGGER reject_restore_receipt BEFORE UPDATE ON %s WHEN OLD.name = '%s' BEGIN SELECT RAISE(ABORT, 'forced receipt failure'); END", table, key)
			if err := f.db.Exec(trigger).Error; err != nil {
				t.Fatal(err)
			}
			var err error
			if rollback {
				_, err = f.store.RollbackRestoreDB(f.scope, f.expectStep(t, "restore-rollback"))
			} else {
				_, err = f.store.CommitRestoreDB(f.scope, f.expectStep(t, RestoreCommitStep), f.request(t))
			}
			if err == nil || f.databaseImage(t) != before {
				t.Fatal("late SQL failure left partial selected config, binding state or receipt")
			}
			if err := f.db.Exec("DROP TRIGGER reject_restore_receipt").Error; err != nil {
				t.Fatal(err)
			}
			// A retry after a rejected transaction is fresh, not a false replay.
			var result Current
			if rollback {
				result, err = f.store.RollbackRestoreDB(f.scope, f.expectStep(t, "restore-rollback"))
			} else {
				result, err = f.store.CommitRestoreDB(f.scope, f.expectStep(t, RestoreCommitStep), f.request(t))
			}
			if err != nil || result.Replayed {
				t.Fatalf("failed transaction consumed the step: %v", err)
			}
		})
	}
}

func TestRestorePrepareRejectsStaleScopeGenerationAndUnknownAdmission(t *testing.T) {
	for _, kind := range []string{"generation", "cpu", "capacity", "tenant", "namespace", "role", "unknown-admission", "source-proof"} {
		t.Run(kind, func(t *testing.T) {
			f := newRestoreCommitFixture(t)
			f.seedBeforeAttributes(t)
			scope := f.scope
			switch kind {
			case "generation":
				f.intent.SourceGeneration++
			case "cpu":
				f.intent.Before.Hardware.CPUMilli += 1000
			case "capacity":
				f.intent.Before.Volumes[1].CapacityGiB++
			case "tenant":
				scope.TenantID = "foreign-tenant"
			case "namespace":
				scope.Namespace = "foreign-team"
			case "role":
				binding := f.old["8"]
				binding.Role = "root"
				f.old["8"] = binding
			case "unknown-admission":
				result, err := f.store.Mutate(scope, Expectation{}, func(c Current) (Mutation, error) {
					c.State.Admission = &Admission{Token: "pending-resize", Action: "resize", OutcomeUnknown: true}
					return Mutation{State: c.State}, nil
				})
				if err != nil {
					t.Fatal(err)
				}
				f.expect.ControlRevision = result.State.ControlRevision
			case "source-proof":
				f.intent.SourceContent.UID = ""
			}
			before := f.databaseImage(t)
			if _, err := f.store.PrepareRestoreDB(scope, f.expect, f.intent, f.old); err == nil || f.databaseImage(t) != before {
				t.Fatal("stale or unauthorized preparation mutated database")
			}
		})
	}
}

func TestRestoreCommitCASAndSourceConfigCannotBeSubstituted(t *testing.T) {
	for _, kind := range []string{"revision", "generation", "config", "unclaimed-target", "wrong-step", "disk-drift"} {
		t.Run(kind, func(t *testing.T) {
			f := newRestoreCommitFixture(t)
			f.prepare(t)
			f.sealCopy(t)
			e := f.expectStep(t, RestoreCommitStep)
			r := f.request(t)
			switch kind {
			case "revision":
				e.ControlRevision--
			case "generation":
				e.BindingGeneration++
			case "config":
				r.Config.Hardware.MemoryMiB++
			case "unclaimed-target":
				b := r.Bindings["8"]
				b.ActivePVC.Name, b.ActivePVC.UID = "unclaimed", "unclaimed-uid"
				r.Bindings["8"] = b
			case "wrong-step":
				e.Step = "other-step"
			case "disk-drift":
				if err := f.db.Model(&dbmodel.TenantServiceVolume{}).Where("ID = ?", 8).Update("volume_capacity", 60).Error; err != nil {
					t.Fatal(err)
				}
			}
			before := f.databaseImage(t)
			if _, err := f.store.CommitRestoreDB(f.scope, e, r); err == nil || f.databaseImage(t) != before {
				t.Fatal("stale or substituted commit changed configuration or identities")
			}
		})
	}
}

func TestRollbackRejectsBindingChangesAfterItsSealedCommit(t *testing.T) {
	f := newRestoreCommitFixture(t)
	f.prepare(t)
	f.sealCopy(t)
	if _, err := f.store.CommitRestoreDB(f.scope, f.expectStep(t, RestoreCommitStep), f.request(t)); err != nil {
		t.Fatal(err)
	}
	_, err := f.store.Mutate(f.scope, f.expectStep(t, "unexpected-binding-change"), func(c Current) (Mutation, error) {
		b := c.State.Bindings["8"]
		b.ActivePVC = ObjectRef{Kind: "PersistentVolumeClaim", Namespace: f.scope.Namespace, Name: "other-active", UID: "other-active-uid"}
		c.State.Bindings["8"] = b
		c.State.BindingGeneration++
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	f.rollbackReady(t)
	before := f.databaseImage(t)
	if _, err := f.store.RollbackRestoreDB(f.scope, f.expectStep(t, "restore-rollback")); err == nil || f.databaseImage(t) != before {
		t.Fatal("rollback overwrote identities changed after its sealed commit")
	}
}

func TestRestoreLayoutProjectionIsStableForEquivalentSealedConfig(t *testing.T) {
	var previousValue, previousHash string
	for _, reverse := range []bool{false, true} {
		f := newRestoreCommitFixture(t)
		f.after.Hardware.DiskLayout[0].BootOrder = 4
		f.after.Hardware.DiskLayout[1].BootOrder = 2
		nonboot := f.intent.Before.Hardware.DiskLayout[2]
		nonboot.BootOrder = 0
		f.after.Hardware.DiskLayout = append(f.after.Hardware.DiskLayout, nonboot)
		f.after.Volumes = append(f.after.Volumes, f.intent.Before.Volumes[2])
		f.targets[9] = ObjectRef{Kind: "PersistentVolumeClaim", Namespace: f.scope.Namespace, Name: "restore-nonboot-a", UID: "target-nonboot-a"}
		if reverse {
			f.after.Hardware.DiskLayout[0], f.after.Hardware.DiskLayout[2] = f.after.Hardware.DiskLayout[2], f.after.Hardware.DiskLayout[0]
			f.after.Volumes[0], f.after.Volumes[2] = f.after.Volumes[2], f.after.Volumes[0]
		}
		var err error
		f.intent.ConfigAfterHash, err = RestoreConfigHash(f.after)
		if err != nil {
			t.Fatal(err)
		}
		f.prepare(t)
		f.sealCopy(t)
		if _, err := f.store.CommitRestoreDB(f.scope, f.expectStep(t, RestoreCommitStep), f.request(t)); err != nil {
			t.Fatal(err)
		}
		var row dbmodel.ComponentK8sAttributes
		if err := f.db.Where("component_id = ? AND name = ?", f.scope.ServiceID, "vm_disk_layout").Take(&row).Error; err != nil {
			t.Fatal(err)
		}
		var layout []restoredLayoutAttribute
		if err := json.Unmarshal([]byte(row.AttributeValue), &layout); err != nil || len(layout) != 3 || layout[0].BootOrder != 2 || layout[1].BootOrder != 4 || layout[2].BootOrder != 0 || layout[0].OrderIndex != 0 || layout[1].OrderIndex != 1 {
			t.Fatal("persisted layout order did not derive from the sealed boot settings")
		}
		if reverse && (previousHash != f.intent.ConfigAfterHash || previousValue != row.AttributeValue) {
			t.Fatal("equivalent sealed configurations produced different persisted layouts")
		}
		previousHash, previousValue = f.intent.ConfigAfterHash, row.AttributeValue
	}
}
