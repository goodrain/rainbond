package vmrestore

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

func fixtureStore(t *testing.T) (*Store, *gorm.DB, Scope) {
	t.Helper()
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "restore.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	database.LogMode(false)
	if err := database.AutoMigrate(&dbmodel.Tenants{}, &dbmodel.TenantServices{}, &dbmodel.ComponentK8sAttributes{}).Error; err != nil {
		t.Fatal(err)
	}
	scope := Scope{TenantID: "tenant", ServiceID: "service", Namespace: "team"}
	if err := database.Create(&dbmodel.Tenants{UUID: scope.TenantID, Name: "team", Namespace: scope.Namespace}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&dbmodel.TenantServices{TenantID: scope.TenantID, ServiceID: scope.ServiceID, ExtendMethod: string(dbmodel.ServiceTypeVM)}).Error; err != nil {
		t.Fatal(err)
	}
	return NewStore(database), database, scope
}
func expectation() Expectation {
	return Expectation{OperationID: "operation-a", ManifestHash: strings.Repeat("a", 64), Step: "prepare", RequestHash: strings.Repeat("b", 64)}
}
func prepareState(scope Scope, e Expectation) func(Current) (Mutation, error) {
	return func(current Current) (Mutation, error) {
		current.State.Fence = &Fence{Kind: "restore", Phase: "prepared", OperationID: e.OperationID, ManifestHash: e.ManifestHash, TargetVMUID: "vm-uid"}
		r := NewReceipt(scope, e.OperationID, e.ManifestHash)
		return Mutation{State: current.State, Receipt: &r}, nil
	}
}

// capability_id: rainbond.vm-restore.atomic-state-store
func TestStoreCASReplayAndRequestIdentity(t *testing.T) {
	store, _, scope := fixtureStore(t)
	e := expectation()
	result, err := store.Mutate(scope, e, prepareState(scope, e))
	if err != nil {
		t.Fatal(err)
	}
	if result.State.ControlRevision != 1 || result.State.BindingGeneration != 0 {
		t.Fatal("incorrect initial revisions")
	}
	replay, err := store.Mutate(scope, e, func(Current) (Mutation, error) { t.Fatal("replay repeated side effect"); return Mutation{}, nil })
	if err != nil || !replay.Replayed || replay.State.ControlRevision != 1 {
		t.Fatal("idempotent step changed state")
	}
	e.RequestHash = strings.Repeat("c", 64)
	if _, err := store.Mutate(scope, e, prepareState(scope, e)); !errors.Is(err, ErrConflict) {
		t.Fatal("same step accepted different fingerprint")
	}
	e = expectation()
	e.Step = "allocate"
	if _, err := store.Mutate(scope, e, prepareState(scope, e)); !errors.Is(err, ErrConflict) {
		t.Fatal("stale CAS accepted")
	}
}

func TestStoreRejectsDuplicateAndCrossTenantRows(t *testing.T) {
	for _, mode := range []string{"duplicate", "wrong-tenant", "bad-json"} {
		t.Run(mode, func(t *testing.T) {
			store, database, scope := fixtureStore(t)
			state := NewState(scope)
			state.ControlRevision = 1
			value, _ := EncodeRecord(state)
			row := dbmodel.ComponentK8sAttributes{TenantID: scope.TenantID, ComponentID: scope.ServiceID, Name: StateKey, SaveType: "json", AttributeValue: value}
			if mode == "wrong-tenant" {
				row.TenantID = "other"
			}
			if mode == "bad-json" {
				row.AttributeValue = "invalid"
			}
			if err := database.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			if mode == "duplicate" {
				if _, err := store.Read(scope, ""); err != nil {
					t.Fatal("unique valid baseline was unreadable")
				}
				row.ID = 0
				if err := database.Create(&row).Error; err != nil {
					t.Fatal(err)
				}
			}
			if _, err := store.Read(scope, ""); !errors.Is(err, ErrCorrupt) {
				t.Fatal("ambiguous persisted state was accepted")
			}
		})
	}
}

func TestStoreRejectsUnpairedFenceBeforeCommit(t *testing.T) {
	for _, mode := range []string{"different-operation", "different-hash", "ordinary-write-without-receipt"} {
		t.Run(mode, func(t *testing.T) {
			store, _, scope := fixtureStore(t)
			e := expectation()
			if mode == "ordinary-write-without-receipt" {
				e = Expectation{}
			}
			_, err := store.Mutate(scope, e, func(current Current) (Mutation, error) {
				operationID, manifestHash := "operation-a", strings.Repeat("a", 64)
				if mode == "different-operation" {
					operationID = "operation-b"
				}
				if mode == "different-hash" {
					manifestHash = strings.Repeat("c", 64)
				}
				current.State.Fence = &Fence{Kind: "restore", Phase: "prepared", OperationID: operationID, ManifestHash: manifestHash, TargetVMUID: "vm-uid"}
				if mode == "ordinary-write-without-receipt" {
					return Mutation{State: current.State}, nil
				}
				receipt := NewReceipt(scope, e.OperationID, e.ManifestHash)
				return Mutation{State: current.State, Receipt: &receipt}, nil
			})
			if !errors.Is(err, ErrConflict) {
				t.Fatal("unpaired fence was not rejected before commit")
			}
			read, err := store.Read(scope, "")
			if err != nil || read.State.ControlRevision != 0 || read.State.Fence != nil {
				t.Fatal("rejected mutation left an unreadable or partial ledger")
			}
		})
	}
}

func TestStoreRejectsOrdinaryMutationOfAnotherOperationsFence(t *testing.T) {
	for _, mode := range []string{"clear", "replace", "preserve"} {
		t.Run(mode, func(t *testing.T) {
			store, _, scope := fixtureStore(t)
			e := expectation()
			prepared, err := store.Mutate(scope, e, prepareState(scope, e))
			if err != nil {
				t.Fatal(err)
			}
			called := false
			_, err = store.Mutate(scope, Expectation{ControlRevision: prepared.State.ControlRevision}, func(current Current) (Mutation, error) {
				called = true
				if mode == "clear" {
					current.State.Fence = nil
				} else if mode == "replace" {
					current.State.Fence.OperationID = "other-operation"
				}
				return Mutation{State: current.State}, nil
			})
			if !errors.Is(err, ErrBusy) || called {
				t.Fatal("ordinary mutation reached a callback while another operation owned the fence")
			}
			read, err := store.Read(scope, e.OperationID)
			if err != nil || read.State.ControlRevision != 1 || read.State.Fence.OperationID != e.OperationID {
				t.Fatal("ordinary mutation changed another operation's fence")
			}
		})
	}
}

func TestStoreFenceOwnerCanFinishAndOldStepsReplayDuringNextOperation(t *testing.T) {
	store, _, scope := fixtureStore(t)
	first := expectation()
	prepared, err := store.Mutate(scope, first, prepareState(scope, first))
	if err != nil {
		t.Fatal(err)
	}
	finish := first
	finish.ControlRevision = prepared.State.ControlRevision
	finish.Step, finish.RequestHash = "finalize", strings.Repeat("c", 64)
	finished, err := store.Mutate(scope, finish, func(current Current) (Mutation, error) {
		current.State.Fence = nil
		return Mutation{State: current.State, Receipt: current.Receipt}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	second := Expectation{ControlRevision: finished.State.ControlRevision, OperationID: "operation-b", ManifestHash: strings.Repeat("d", 64), Step: "prepare", RequestHash: strings.Repeat("e", 64)}
	started, err := store.Mutate(scope, second, prepareState(scope, second))
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := store.Mutate(scope, first, func(Current) (Mutation, error) {
		t.Fatal("historical successful step repeated a callback")
		return Mutation{}, ErrBusy
	})
	if err != nil || !replayed.Replayed || replayed.State.Fence.OperationID != second.OperationID {
		t.Fatal("historical replay was blocked or changed the current fence")
	}
	late := first
	late.ControlRevision = started.State.ControlRevision
	late.Step, late.RequestHash = "late-step", strings.Repeat("f", 64)
	called := false
	if _, err := store.Mutate(scope, late, func(Current) (Mutation, error) {
		called = true
		return Mutation{}, ErrCorrupt
	}); !errors.Is(err, ErrBusy) || called {
		t.Fatal("old operation performed a new write during the next operation")
	}
}

func TestStoreDuplicateRowsWithValidCommittedBaseline(t *testing.T) {
	for _, kind := range []string{"state", "receipt"} {
		t.Run(kind, func(t *testing.T) {
			store, database, scope := fixtureStore(t)
			e := expectation()
			baseline, err := store.Mutate(scope, e, prepareState(scope, e))
			if err != nil || baseline.State.ControlRevision != 1 {
				t.Fatal("could not create a valid committed baseline")
			}
			if _, err := store.Read(scope, e.OperationID); err != nil {
				t.Fatal(err)
			}
			name := StateKey
			if kind == "receipt" {
				name, _ = ReceiptKey(e.OperationID)
			}
			var row dbmodel.ComponentK8sAttributes
			if err := database.Where("component_id = ? AND name = ?", scope.ServiceID, name).First(&row).Error; err != nil {
				t.Fatal(err)
			}
			row.ID = 0
			if err := database.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := store.Read(scope, e.OperationID); !errors.Is(err, ErrCorrupt) {
				t.Fatal("duplicate valid ledger rows were accepted")
			}
			if err := database.Where("ID = ?", row.ID).Delete(&dbmodel.ComponentK8sAttributes{}).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := store.Read(scope, e.OperationID); err != nil {
				t.Fatal("unique valid ledger was not readable after removing the duplicate")
			}
		})
	}
}

func TestMutateTxRollsBackStateAndReceiptTogether(t *testing.T) {
	store, database, scope := fixtureStore(t)
	e := expectation()
	tx := database.Begin()
	if _, err := store.MutateTx(tx, scope, e, prepareState(scope, e)); err != nil {
		t.Fatal(err)
	}
	tx.Rollback()
	read, err := store.Read(scope, e.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if read.State.ControlRevision != 0 || read.Receipt != nil {
		t.Fatal("rollback left partial control or receipt")
	}
}

func TestMissingServiceNeverRecreatesLedger(t *testing.T) {
	store, database, scope := fixtureStore(t)
	if err := database.Where("service_id = ?", scope.ServiceID).Delete(&dbmodel.TenantServices{}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := store.Mutate(scope, expectation(), prepareState(scope, expectation())); !errors.Is(err, ErrNotFound) {
		t.Fatal("missing component created state")
	}
	var count int
	database.Model(&dbmodel.ComponentK8sAttributes{}).Count(&count)
	if count != 0 {
		t.Fatal("missing component created ledger rows")
	}
}

// Store exclusively seals a successful step after its callback returns.
func TestStoreRejectsManufacturedFutureStep(t *testing.T) {
	for _, preexisting := range []bool{false, true} {
		t.Run(map[bool]string{false: "new-operation", true: "existing-operation"}[preexisting], func(t *testing.T) {
			store, _, scope := fixtureStore(t)
			e := expectation()
			if preexisting {
				if _, err := store.Mutate(scope, e, prepareState(scope, e)); err != nil {
					t.Fatal(err)
				}
				e.ControlRevision = 1
				e.Step = "allocate"
			}
			futureHash := strings.Repeat("d", 64)
			result, err := store.Mutate(scope, e, func(c Current) (Mutation, error) {
				var r Receipt
				if c.Receipt == nil {
					r = NewReceipt(scope, e.OperationID, e.ManifestHash)
				} else {
					r = *c.Receipt
				}
				c.State.Fence = &Fence{Kind: "restore", Phase: "prepared", OperationID: e.OperationID, ManifestHash: e.ManifestHash, TargetVMUID: "vm-uid"}
				r.Steps["finalize"] = StepResult{RequestHash: futureHash, ControlRevision: 999, BindingGeneration: 999, Applied: false}
				return Mutation{State: c.State, Receipt: &r}, nil
			})
			if err == nil {
				t.Error("callback manufactured an unexecuted future step")
				e.Step = "finalize"
				e.RequestHash = futureHash
				e.ControlRevision = result.State.ControlRevision
				called := false
				replay, err := store.Mutate(scope, e, func(c Current) (Mutation, error) { called = true; return Mutation{}, ErrBusy })
				if err == nil && replay.Replayed && !called {
					t.Error("unapplied fabricated step replayed without its effect callback")
				}
			}
		})
	}
}

func TestStoreRejectsUnappliedPersistedStepReplay(t *testing.T) {
	store, db, scope := fixtureStore(t)
	e := expectation()
	initial, err := store.Mutate(scope, e, prepareState(scope, e))
	if err != nil {
		t.Fatal(err)
	}
	receipt := *initial.Receipt
	step := receipt.Steps[e.Step]
	step.Applied = false
	step.ErrorCode = "side-effect-not-applied"
	receipt.Steps[e.Step] = step
	encoded, err := EncodeRecord(receipt)
	if err != nil {
		t.Fatal(err)
	}
	key, _ := ReceiptKey(e.OperationID)
	if err := db.Model(&dbmodel.ComponentK8sAttributes{}).Where("component_id = ? AND name = ?", scope.ServiceID, key).Update("attribute_value", encoded).Error; err != nil {
		t.Fatal(err)
	}
	called := false
	result, err := store.Mutate(scope, e, func(Current) (Mutation, error) { called = true; return Mutation{}, ErrBusy })
	if err == nil && result.Replayed && !called {
		t.Fatal("persisted Applied=false/ErrorCode step was treated as successful replay")
	}
}

func TestStoreRejectsIncompletePersistedState(t *testing.T) {
	for _, value := range []string{`{}`, `{"schema_version":1}`, `{"schema_version":1,"scope":{"tenant_id":"tenant","service_id":"service","namespace":"team"}}`} {
		t.Run(value, func(t *testing.T) {
			store, db, scope := fixtureStore(t)
			if err := db.Create(&dbmodel.ComponentK8sAttributes{TenantID: scope.TenantID, ComponentID: scope.ServiceID, Name: StateKey, SaveType: "json", AttributeValue: value}).Error; err != nil {
				t.Fatal(err)
			}
			current, err := store.Read(scope, "")
			if err == nil {
				_, found, resolveErr := current.State.ResolveClaim(123)
				if !found && resolveErr == nil {
					t.Error("incomplete persisted ledger accepted as pristine canonical fallback")
				}
			}
		})
	}
}

func TestStoreRejectsMissingCurrentWithRemainingReceipt(t *testing.T) {
	store, db, scope := fixtureStore(t)
	e := expectation()
	_, err := store.Mutate(scope, e, func(c Current) (Mutation, error) {
		c.State.Bindings["123"] = Binding{VolumeID: 123, DiskKey: "disk", CanonicalName: "manual123", Role: "root", Attached: true, ActivePVC: ObjectRef{Kind: "PersistentVolumeClaim", Namespace: scope.Namespace, Name: "restored-a", UID: "restored-uid"}, RestoreID: e.OperationID, SnapshotUID: "snapshot-uid"}
		c.State.BindingGeneration = 1
		c.State.Fence = &Fence{Kind: "restore", Phase: "prepared", OperationID: e.OperationID, ManifestHash: e.ManifestHash, TargetVMUID: "vm-uid"}
		r := NewReceipt(scope, e.OperationID, e.ManifestHash)
		r.NewBindings = c.State.Bindings
		return Mutation{State: c.State, Receipt: &r}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Where("component_id = ? AND name = ?", scope.ServiceID, StateKey).Delete(&dbmodel.ComponentK8sAttributes{}).Error; err != nil {
		t.Fatal(err)
	}
	current, err := store.Read(scope, "")
	if err == nil {
		_, found, resolveErr := current.State.ResolveClaim(123)
		if !found && resolveErr == nil {
			t.Fatal("missing current ledger with durable receipt accepted; restored A becomes canonical fallback")
		}
	}
}

func TestStoreRejectsMissingReceiptForActiveFence(t *testing.T) {
	store, db, scope := fixtureStore(t)
	e := expectation()
	initial, err := store.Mutate(scope, e, prepareState(scope, e))
	if err != nil {
		t.Fatal(err)
	}
	key, _ := ReceiptKey(e.OperationID)
	if err := db.Where("component_id = ? AND name = ?", scope.ServiceID, key).Delete(&dbmodel.ComponentK8sAttributes{}).Error; err != nil {
		t.Fatal(err)
	}
	e.ControlRevision = initial.State.ControlRevision
	e.Step = "allocate"
	e.RequestHash = strings.Repeat("f", 64)
	called := false
	result, err := store.Mutate(scope, e, func(c Current) (Mutation, error) {
		called = true
		r := NewReceipt(scope, e.OperationID, e.ManifestHash)
		return Mutation{State: c.State, Receipt: &r}, nil
	})
	if err == nil && called && result.Receipt != nil {
		t.Fatal("active fence's missing receipt recreated; prior immutable history is lost")
	}
}

func TestStorePersistedStateRequiresStructuralFields(t *testing.T) {
	for _, field := range []string{"schema_version", "scope", "control_revision", "binding_generation", "bindings"} {
		t.Run(field, func(t *testing.T) {
			store, database, scope := fixtureStore(t)
			state := NewState(scope)
			state.ControlRevision = 1
			state.BindingGeneration = 1
			state.Bindings["123"] = stateValidationBinding()
			encoded, err := EncodeRecord(state)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal([]byte(encoded), &fields); err != nil {
				t.Fatal(err)
			}
			delete(fields, field)
			value, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			if err := database.Create(&dbmodel.ComponentK8sAttributes{TenantID: scope.TenantID, ComponentID: scope.ServiceID, Name: StateKey, SaveType: "json", AttributeValue: string(value)}).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := store.Read(scope, ""); !errors.Is(err, ErrCorrupt) {
				t.Fatal("missing persisted structural field was defaulted")
			}
		})
	}
}

func TestStoreNeverRestoredIgnoresUnrelatedAndLiteralLookalikeRows(t *testing.T) {
	store, database, scope := fixtureStore(t)
	rows := []dbmodel.ComponentK8sAttributes{
		{TenantID: "other", ComponentID: "other-service", Name: ReceiptPrefix + "operation-a", AttributeValue: "not-this-components-ledger"},
		{TenantID: scope.TenantID, ComponentID: scope.ServiceID, Name: "vmXrestoreXreceiptXoperation-a", AttributeValue: "ordinary-attribute"},
		{TenantID: scope.TenantID, ComponentID: scope.ServiceID, Name: "vm_disk_layout", AttributeValue: "ordinary-attribute"},
	}
	for i := range rows {
		if err := database.Create(&rows[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	read, err := store.Read(scope, "")
	if err != nil || read.State.ControlRevision != 0 || read.Receipt != nil {
		t.Fatal("unrelated rows prevented a genuinely new component read")
	}
	if _, mapped, err := read.State.ResolveClaim(123); err != nil || mapped {
		t.Fatal("unrelated rows were adopted as this component's binding")
	}
}

func TestStoreValidatesActiveFenceReceiptOnUnqualifiedReads(t *testing.T) {
	for _, mode := range []string{"missing", "different-hash"} {
		t.Run(mode, func(t *testing.T) {
			store, database, scope := fixtureStore(t)
			e := expectation()
			prepared, err := store.Mutate(scope, e, prepareState(scope, e))
			if err != nil {
				t.Fatal(err)
			}
			key, _ := ReceiptKey(e.OperationID)
			if mode == "missing" {
				if err := database.Where("component_id = ? AND name = ?", scope.ServiceID, key).Delete(&dbmodel.ComponentK8sAttributes{}).Error; err != nil {
					t.Fatal(err)
				}
			} else {
				receipt := *prepared.Receipt
				receipt.ManifestHash = strings.Repeat("f", 64)
				value, err := EncodeRecord(receipt)
				if err != nil {
					t.Fatal(err)
				}
				if err := database.Model(&dbmodel.ComponentK8sAttributes{}).Where("component_id = ? AND name = ?", scope.ServiceID, key).Update("attribute_value", value).Error; err != nil {
					t.Fatal(err)
				}
			}
			for _, requestedID := range []string{"", "another-operation"} {
				if _, err := store.Read(scope, requestedID); !errors.Is(err, ErrCorrupt) {
					t.Fatal("unqualified read accepted a broken active fence ledger")
				}
			}
		})
	}
}

func TestStoreRejectsImpossiblePersistedStepResults(t *testing.T) {
	for _, mode := range []string{"unapplied", "error-code", "zero-revision", "future-revision", "future-generation", "same-revision", "duplicate-revision"} {
		t.Run(mode, func(t *testing.T) {
			store, database, scope := fixtureStore(t)
			e := expectation()
			prepared, err := store.Mutate(scope, e, prepareState(scope, e))
			if err != nil {
				t.Fatal(err)
			}
			receipt := *prepared.Receipt
			step := receipt.Steps[e.Step]
			switch mode {
			case "unapplied":
				step.Applied = false
			case "error-code":
				step.ErrorCode = "not-applied"
			case "zero-revision":
				step.ControlRevision = 0
			case "future-revision":
				step.ControlRevision = 2
			case "future-generation":
				step.BindingGeneration = 2
			case "same-revision":
				step.BindingGeneration = 1
			case "duplicate-revision":
				receipt.Steps["finalize"] = step
			}
			receipt.Steps[e.Step] = step
			value, err := EncodeRecord(receipt)
			if err != nil {
				t.Fatal(err)
			}
			key, _ := ReceiptKey(e.OperationID)
			if err := database.Model(&dbmodel.ComponentK8sAttributes{}).Where("component_id = ? AND name = ?", scope.ServiceID, key).Update("attribute_value", value).Error; err != nil {
				t.Fatal(err)
			}
			called := false
			if _, err := store.Mutate(scope, e, func(Current) (Mutation, error) {
				called = true
				return Mutation{}, ErrBusy
			}); !errors.Is(err, ErrCorrupt) || called {
				t.Fatal("invalid persisted step reached replay or effect callback")
			}
		})
	}
}

func TestStoreReplaysSuccessfulEarlierStepAfterLaterProgress(t *testing.T) {
	store, _, scope := fixtureStore(t)
	e := expectation()
	prepared, err := store.Mutate(scope, e, prepareState(scope, e))
	if err != nil {
		t.Fatal(err)
	}
	later := e
	later.ControlRevision = prepared.State.ControlRevision
	later.Step = "allocate"
	later.RequestHash = strings.Repeat("c", 64)
	if _, err := store.Mutate(scope, later, func(current Current) (Mutation, error) {
		return Mutation{State: current.State, Receipt: current.Receipt}, nil
	}); err != nil {
		t.Fatal(err)
	}
	replayed, err := store.Mutate(scope, e, func(Current) (Mutation, error) {
		t.Fatal("successful earlier step repeated its callback")
		return Mutation{}, ErrBusy
	})
	if err != nil || !replayed.Replayed || replayed.State.ControlRevision != 2 {
		t.Fatal("valid earlier receipt could not be replayed after later progress")
	}
}

func TestStoreRejectsIncompletePersistedReceipt(t *testing.T) {
	for _, field := range []string{"retain_old_pvcs", "snapshot_reference_released", "step.applied", "step.control_revision", "step.binding_generation", "step.request_hash"} {
		t.Run(field, func(t *testing.T) {
			store, database, scope := fixtureStore(t)
			e := expectation()
			prepared, err := store.Mutate(scope, e, prepareState(scope, e))
			if err != nil {
				t.Fatal(err)
			}
			value, err := EncodeRecord(prepared.Receipt)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal([]byte(value), &fields); err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(field, "step.") {
				var steps map[string]map[string]json.RawMessage
				if err := json.Unmarshal(fields["steps"], &steps); err != nil {
					t.Fatal(err)
				}
				delete(steps[e.Step], strings.TrimPrefix(field, "step."))
				fields["steps"], err = json.Marshal(steps)
			} else {
				delete(fields, field)
			}
			if err != nil {
				t.Fatal(err)
			}
			altered, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			key, _ := ReceiptKey(e.OperationID)
			if err := database.Model(&dbmodel.ComponentK8sAttributes{}).Where("component_id = ? AND name = ?", scope.ServiceID, key).Update("attribute_value", string(altered)).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := store.Read(scope, e.OperationID); !errors.Is(err, ErrCorrupt) {
				t.Fatal("missing receipt field was silently defaulted")
			}
		})
	}
}
