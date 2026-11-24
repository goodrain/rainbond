package vmrestore

import (
	"context"
	"encoding/json"
	"errors"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestIndependentV11SealedReadIgnoresLiveBAndCallerChanges(t *testing.T) {
	f, e, source := enginePrepareFixture(t)
	source.value.Config.Volumes[0].AccessMode = "ReadWriteOnce"
	source.value.Intent.ConfigAfterHash, _ = RestoreConfigHash(source.value.Config)
	source.value.Frame.Disks[0].TargetAccessMode = "ReadWriteOnce"
	source.value.Frame.FrameHash, _ = RestoreResourceFrameHash(source.value.Frame)
	if _, err := e.Prepare(context.Background(), f.scope, f.expect.OperationID, f.intent.SourceSnapshot.Name, f.intent.PowerAfter); err != nil {
		t.Fatal(err)
	}
	first, err := e.Status(f.scope, f.expect.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if first.ConfigAfter == nil || !first.SourceConfigReady || first.ConfigAfter.Volumes[0].AccessMode != "ReadWriteOnce" || first.ConfigBefore.Volumes[0].AccessMode != "ReadWriteMany" {
		t.Fatal("sealed A/B mixed")
	}
	beforeCPU := first.ConfigBefore.Hardware.CPUMilli
	if err := f.db.Model(&dbmodel.TenantServices{}).Where("service_id = ?", f.scope.ServiceID).Update("container_cpu", 12345).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&dbmodel.ComponentK8sAttributes{TenantID: f.scope.TenantID, ComponentID: f.scope.ServiceID, Name: "vm_ssh_username", AttributeValue: "PUBLIC_REDACTION_SENTINEL"}).Error; err != nil {
		t.Fatal(err)
	}
	first.ConfigAfter.Volumes[0].AccessMode = "ReadOnlyMany"
	second, err := e.Status(f.scope, f.expect.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if second.ConfigBefore.Hardware.CPUMilli != beforeCPU || second.ConfigAfter.Volumes[0].AccessMode != "ReadWriteOnce" {
		t.Fatal("GET used live B or mutable caller DTO")
	}
	a, _ := RestoreConfigHash(*second.ConfigAfter)
	b, _ := RestoreConfigHash(second.ConfigBefore)
	if a != second.ConfigAfterHash || b != second.ConfigBeforeHash || second.Source.VMUID == second.TargetVMUID {
		t.Fatal("authoritative hash/source/target identity lost")
	}
	raw, _ := json.Marshal(second)
	if strings.Contains(string(raw), "PUBLIC_REDACTION_SENTINEL") || strings.Contains(string(raw), "vm_ssh_username") {
		t.Fatal("initialization field leaked")
	}
	c, _ := f.store.Read(f.scope, f.expect.OperationID)
	rev := c.State.ControlRevision
	foreign := f.scope
	foreign.TenantID = "different-tenant"
	denied, err := e.Status(foreign, f.expect.OperationID)
	if err == nil || denied.OperationID != "" || denied.ConfigAfter != nil {
		t.Fatal("cross-scope receipt leaked")
	}
	c, _ = f.store.Read(f.scope, f.expect.OperationID)
	if c.State.ControlRevision != rev {
		t.Fatal("read progressed state")
	}
}

func TestIndependentV11OriginalPowerSealMatrix(t *testing.T) {
	for _, running := range []bool{false, true} {
		for _, power := range []string{"previous", "stopped"} {
			t.Run(power+string(rune('0'+map[bool]int{false: 0, true: 1}[running])), func(t *testing.T) {
				f, e, s := enginePrepareFixture(t)
				s.value.Intent.WasRunning = running
				s.value.Intent.PowerAfter = power
				if _, err := e.Prepare(context.Background(), f.scope, f.expect.OperationID, f.intent.SourceSnapshot.Name, power); err != nil {
					t.Fatal(err)
				}
				d, err := e.Status(f.scope, f.expect.OperationID)
				if err != nil {
					t.Fatal(err)
				}
				if d.WasRunning != running || d.PowerAfter != power || d.ResumeRequired != (running && power == "previous") {
					t.Fatal("power derived from current state instead of immutable intent")
				}
			})
		}
	}
}

func TestIndependentV11HistoricalAAndDefaultQualificationFailClosed(t *testing.T) {
	f, e, _ := engineFixture(t)
	d, err := e.Status(f.scope, f.expect.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if d.SourceConfigReady || d.ConfigAfter != nil || len(d.ConfigBefore.Volumes) != 3 {
		t.Fatal("missing A silently replaced by B/default")
	}
	f2, e2, _ := enginePrepareFixture(t)
	e2.Source = UnqualifiedRestorePreparation{}
	if _, err := e2.Prepare(context.Background(), f2.scope, f2.expect.OperationID, f2.intent.SourceSnapshot.Name, f2.intent.PowerAfter); !errors.Is(err, ErrRestoreCapability) {
		t.Fatal(err)
	}
	c, _ := f2.store.Read(f2.scope, f2.expect.OperationID)
	if c.State.Fence != nil || c.Receipt != nil {
		t.Fatal("unqualified source acquired authority")
	}
}

type reviewCASPause struct {
	context.Context
	once             sync.Once
	reached, release chan struct{}
}

func (c *reviewCASPause) Deadline() (time.Time, bool) {
	c.once.Do(func() { close(c.reached); <-c.release })
	return time.Time{}, false
}
func TestIndependentV11CallerCASRemainsBindingAcrossConcurrentProgress(t *testing.T) {
	f, e, r := engineFixture(t)
	r.observation.Halted = true
	r.observation.WritersClear = true
	d, _ := e.Status(f.scope, f.expect.OperationID)
	pause := &reviewCASPause{Context: context.Background(), reached: make(chan struct{}), release: make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		<-pause.reached
		_, err := f.store.executionUpdate(f.scope, f.expect.OperationID, "restore-cancel-requested", true, func(c Current) (Mutation, error) { return Mutation{State: c.State, Receipt: c.Receipt}, nil })
		done <- err
		close(pause.release)
	}()
	result, err := e.Command(pause, f.scope, f.expect.OperationID, "halt", RestoreCommand{ExpectedOperationEpoch: d.OperationEpoch, ExpectedControlRevision: d.ControlRevision, ExpectedBindingGeneration: d.BindingGeneration})
	if concurrentErr := <-done; concurrentErr != nil {
		t.Fatal("control setup failed", concurrentErr)
	}
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("caller stale CAS accepted after actual concurrent Store commit: err=%v applied=%v before_revision=%d after_revision=%d", err, result.Applied, d.ControlRevision, result.ControlRevision)
	}
}

func TestIndependentV11UnknownDoesNotReapplyOrClearFence(t *testing.T) {
	f, e, r := engineFixture(t)
	r.applyErr = errors.New("PUBLIC_TIMEOUT_SENTINEL")
	issue := func() (RestoreOperationDTO, error) {
		d, _ := e.Status(f.scope, f.expect.OperationID)
		return e.Command(context.Background(), f.scope, f.expect.OperationID, "halt", RestoreCommand{ExpectedOperationEpoch: d.OperationEpoch, ExpectedControlRevision: d.ControlRevision, ExpectedBindingGeneration: d.BindingGeneration})
	}
	if _, err := issue(); !errors.Is(err, ErrRestoreUnknown) {
		t.Fatal(err)
	}
	if _, err := issue(); !errors.Is(err, ErrRestoreUnknown) {
		t.Fatal(err)
	}
	d, _ := e.Status(f.scope, f.expect.OperationID)
	if r.applies != 1 || d.Complete || !d.FenceOwned || d.FenceCleared {
		t.Fatal("unknown retried physical IO or claimed terminal")
	}
	r.applyErr = nil
	r.observation.Halted = true
	r.observation.WritersClear = true
	if _, err := issue(); err != nil {
		t.Fatal(err)
	}
	d, _ = e.Status(f.scope, f.expect.OperationID)
	if d.Complete || !d.FenceOwned || r.applies != 1 {
		t.Fatal("observed halt became entire restore success")
	}
}

func TestIndependentV11AtomicCommitFailureAndACKOrder(t *testing.T) {
	f := newRestoreCommitFixture(t)
	f.prepare(t)
	if _, err := f.store.AcknowledgeRestoreDB(f.scope, f.expectStep(t, "restore-console-ack"), strings.Repeat("a", 64), strings.Repeat("b", 64)); err == nil {
		t.Fatal("ACK before commit accepted")
	}
	f.sealCopy(t)
	before, _ := f.store.Read(f.scope, f.expect.OperationID)
	var volumesBefore []dbmodel.TenantServiceVolume
	f.db.Order("ID").Find(&volumesBefore)
	f.db.Callback().Update().Before("gorm:update").Register("independent_abort_hardware", func(scope *gorm.Scope) {
		if scope.TableName() == "tenant_services" {
			scope.Err(errors.New("PUBLIC_INJECTED_DB_FAILURE"))
		}
	})
	_, err := f.store.CommitRestoreDB(f.scope, f.expectStep(t, "restore-commit"), f.request(t))
	f.db.Callback().Update().Remove("independent_abort_hardware")
	if err == nil {
		t.Fatal("failure injection did not fire")
	}
	after, _ := f.store.Read(f.scope, f.expect.OperationID)
	var volumesAfter []dbmodel.TenantServiceVolume
	f.db.Order("ID").Find(&volumesAfter)
	if !reflect.DeepEqual(before.State, after.State) || !reflect.DeepEqual(before.Receipt, after.Receipt) || !reflect.DeepEqual(volumesBefore, volumesAfter) {
		t.Fatal("failed hardware/binding transaction left partial A")
	}
	committed, err := f.store.CommitRestoreDB(f.scope, f.expectStep(t, "restore-commit"), f.request(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.AcknowledgeRestoreDB(f.scope, f.expectStep(t, "restore-console-ack"), strings.Repeat("f", 64), strings.Repeat("b", 64)); err == nil {
		t.Fatal("foreign commit hash ACK accepted")
	}
	acked, err := f.store.AcknowledgeRestoreDB(f.scope, f.expectStep(t, "restore-console-ack"), committed.Receipt.ConfigCommitHash, strings.Repeat("b", 64))
	if err != nil {
		t.Fatal(err)
	}
	if acked.State.Fence == nil || acked.State.Fence.Phase != "console-acked" || acked.Receipt.Steps["restore-finalize"].Applied {
		t.Fatal("ACK released fence or completed operation")
	}
}

func TestIndependentV11OldTerminalCannotClearNewFence(t *testing.T) {
	f, e, r := engineFixture(t)
	r.observation.Halted = true
	r.observation.WritersClear = true
	e.Capacity = &testRestoreCapacity{}
	if _, err := e.RuntimeStep(context.Background(), f.scope, f.expect.OperationID, "halt"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.executionUpdate(f.scope, f.expect.OperationID, "restore-cancel-requested", true, func(c Current) (Mutation, error) { return Mutation{State: c.State, Receipt: c.Receipt}, nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Finalize(context.Background(), f.scope, f.expect.OperationID, false, true); err != nil {
		t.Fatal(err)
	}
	old, _ := e.Status(f.scope, f.expect.OperationID)
	if !old.Complete || !old.FenceCleared || old.FenceOwned || old.Outcome != "cancelled" {
		t.Fatal("cancel terminal algebra invalid")
	}
	current, _ := f.store.Read(f.scope, f.expect.OperationID)
	next := f
	next.expect.OperationID = "new-restore-op"
	next.expect.ControlRevision = current.State.ControlRevision
	next.expect.BindingGeneration = current.State.BindingGeneration
	next.intent.SourceGeneration = current.State.BindingGeneration
	next.prepare(t)
	next.sealCopy(t)
	if _, err := next.store.CommitRestoreDB(next.scope, next.expectStep(t, "restore-commit"), next.request(t)); err != nil {
		t.Fatal(err)
	}
	historical, err := e.Status(f.scope, f.expect.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	now, _ := e.Control(f.scope)
	if !historical.Complete || historical.FenceOwned || !historical.FenceCleared || historical.OperationBindingGeneration >= historical.BindingGeneration || now.ActiveOperationID != "new-restore-op" {
		t.Fatal("historical terminal overwritten with new authority")
	}
	if _, err := e.Command(context.Background(), f.scope, f.expect.OperationID, "finalize", RestoreCommand{ExpectedOperationEpoch: historical.OperationEpoch, ExpectedControlRevision: now.ControlRevision, ExpectedBindingGeneration: now.BindingGeneration, Cancelled: true}); err == nil {
		t.Fatal("old terminal command was authorized against new fence")
	}
	now, _ = e.Control(f.scope)
	if now.ActiveOperationID != "new-restore-op" {
		t.Fatal("old task cleared new owner")
	}
}

func TestIndependentV11SequentialEpochAndBothCASControls(t *testing.T) {
	f, e, r := engineFixture(t)
	d, _ := e.Status(f.scope, f.expect.OperationID)
	r.observation.Halted = true
	r.observation.WritersClear = true
	valid := RestoreCommand{ExpectedOperationEpoch: d.OperationEpoch, ExpectedControlRevision: d.ControlRevision, ExpectedBindingGeneration: d.BindingGeneration}
	cases := []RestoreCommand{valid, valid, valid, valid}
	cases[0].ExpectedOperationEpoch = ""
	cases[1].ExpectedOperationEpoch = strings.Repeat("f", 64)
	cases[2].ExpectedControlRevision++
	cases[3].ExpectedBindingGeneration++
	for i, c := range cases {
		if _, err := e.Command(context.Background(), f.scope, f.expect.OperationID, "halt", c); !errors.Is(err, ErrConflict) {
			t.Fatal("invalid guard accepted", i, err)
		}
	}
	after, _ := e.Status(f.scope, f.expect.OperationID)
	if after.ControlRevision != d.ControlRevision || r.applies != 0 {
		t.Fatal("rejected guard changed state")
	}
	accepted, err := e.Command(context.Background(), f.scope, f.expect.OperationID, "halt", valid)
	if err != nil || !accepted.Applied || accepted.Complete {
		t.Fatal("normal control failed", err)
	}
}

func TestIndependentV11AtomicRollbackFailureKeepsFullAThenRestoresB(t *testing.T) {
	f := newRestoreCommitFixture(t)
	f.prepare(t)
	f.sealCopy(t)
	if _, err := f.store.CommitRestoreDB(f.scope, f.expectStep(t, "restore-commit"), f.request(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Mutate(f.scope, f.expectStep(t, "restore-rollback-writers-clear"), func(c Current) (Mutation, error) {
		c.State.Fence.Phase = "rollback-stopped"
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	}); err != nil {
		t.Fatal(err)
	}
	before, _ := f.store.Read(f.scope, f.expect.OperationID)
	var beforeVolumes []dbmodel.TenantServiceVolume
	f.db.Order("ID").Find(&beforeVolumes)
	f.db.Callback().Update().Before("gorm:update").Register("independent_rollback_abort", func(scope *gorm.Scope) {
		if scope.TableName() == "tenant_services" {
			scope.Err(errors.New("PUBLIC_ROLLBACK_DB_FAILURE"))
		}
	})
	_, err := f.store.RollbackRestoreDB(f.scope, f.expectStep(t, "restore-rollback"))
	f.db.Callback().Update().Remove("independent_rollback_abort")
	if err == nil {
		t.Fatal("rollback failure injection did not fire")
	}
	after, _ := f.store.Read(f.scope, f.expect.OperationID)
	var afterVolumes []dbmodel.TenantServiceVolume
	f.db.Order("ID").Find(&afterVolumes)
	if !reflect.DeepEqual(before.State, after.State) || !reflect.DeepEqual(before.Receipt, after.Receipt) || !reflect.DeepEqual(beforeVolumes, afterVolumes) {
		t.Fatal("failed rollback left partial B")
	}
	rolled, err := f.store.RollbackRestoreDB(f.scope, f.expectStep(t, "restore-rollback"))
	if err != nil {
		t.Fatal(err)
	}
	for key, b := range f.old {
		if !sameObject(rolled.State.Bindings[key].ActivePVC, b.ActivePVC) {
			t.Fatal("not all retained B restored", key)
		}
	}
	var service dbmodel.TenantServices
	f.db.Where("service_id = ?", f.scope.ServiceID).Take(&service)
	if service.ContainerCPU != f.intent.Before.Hardware.CPUMilli || int64(service.ContainerMemory) != f.intent.Before.Hardware.MemoryMiB {
		t.Fatal("B hardware not restored atomically")
	}
	if _, err := f.store.AcknowledgeRollbackDB(f.scope, f.expectStep(t, "restore-rollback-ack"), rolled.Receipt.RollbackConfigHash, strings.Repeat("b", 64)); err != nil {
		t.Fatal(err)
	}
	c, _ := f.store.Read(f.scope, f.expect.OperationID)
	if c.State.Fence == nil || c.State.Fence.Phase != "rollback-console-acked" {
		t.Fatal("rollback ACK cleared fence")
	}
}

func TestIndependentV11MissingAccessModeCannotBecomeSourceReady(t *testing.T){
 f,e,source:=enginePrepareFixture(t)
 source.value.Config.Volumes[0].AccessMode=""
 source.value.Intent.ConfigAfterHash,_=RestoreConfigHash(source.value.Config)
 if _,err:=e.Prepare(context.Background(),f.scope,f.expect.OperationID,f.intent.SourceSnapshot.Name,f.intent.PowerAfter);err==nil{t.Fatal("missing authoritative access mode silently defaulted")}
 c,err:=f.store.Read(f.scope,f.expect.OperationID);if err!=nil{t.Fatal(err)}
 if c.State.Fence!=nil||c.Receipt!=nil{t.Fatal("incomplete source acquired fence")}
}
