package vmrestore

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/jinzhu/gorm"
)

func snapshotReadbackMap(t *testing.T, dto SnapshotOperationDTO) map[string]interface{} {
	t.Helper()
	raw, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

type snapshotReadbackHook struct {
	SnapshotBackend
	afterObserve func()
}

func (h *snapshotReadbackHook) Observe(ctx context.Context, scope Scope, intent SnapshotIntent, resources []ResourceRef) (SnapshotObservation, error) {
	observed, err := h.SnapshotBackend.Observe(ctx, scope, intent, resources)
	if err == nil && h.afterObserve != nil {
		callback := h.afterObserve
		h.afterObserve = nil
		callback()
	}
	return observed, err
}

// capability_id: rainbond.vm-snapshot.authoritative-readback
func TestSnapshotReadbackRefreshesFenceAfterNativeObservation(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	ready := engineReady(t, engine, scope, backend)
	if _, err := engine.Resume(context.Background(), scope, readyACK("readback-resume", ready)); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Finalize(context.Background(), scope, readyACK("readback-finalize", ready)); err != nil {
		t.Fatal(err)
	}
	engine.backend = &snapshotReadbackHook{SnapshotBackend: backend, afterObserve: func() {
		current, err := engine.store.Read(scope, ready.OperationID)
		if err != nil {
			t.Fatal(err)
		}
		_, err = engine.store.Mutate(scope, Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration, OperationID: "new-op", ManifestHash: strings.Repeat("a", 64), RequestHash: strings.Repeat("b", 64), Step: "new-prepare"}, func(c Current) (Mutation, error) {
			r := NewReceipt(scope, "new-op", strings.Repeat("a", 64))
			c.State.Fence = &Fence{Kind: "restore", Phase: "prepared", OperationID: "new-op", ManifestHash: r.ManifestHash, TargetVMUID: "new-vm"}
			return Mutation{State: c.State, Receipt: &r}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}}
	dto, err := engine.Get(context.Background(), scope, ready.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	value := snapshotReadbackMap(t, dto)
	if value["fence_cleared"] != false || value["fence_owned"] != false || value["finalized_control_revision"] == nil {
		t.Fatal("old finalize advertised a newly acquired operation fence as cleared", value)
	}
}

// capability_id: rainbond.vm-snapshot.authoritative-readback
func TestSnapshotReadbackDoesNotInferReleaseFromACKOrAbsentFence(t *testing.T) {
	for _, mode := range []string{"missing-finalize-step", "pending-resume", "same-operation-fence-after-finalize"} {
		t.Run(mode, func(t *testing.T) {
			engine, backend, scope := snapshotEngineFixture(t)
			ready := engineReady(t, engine, scope, backend)
			if mode == "pending-resume" {
				backend.resumeError = errors.New("READBACK_RESPONSE_FAULT")
			}
			_, resumeErr := engine.Resume(context.Background(), scope, readyACK("readback-resume", ready))
			if mode == "pending-resume" && resumeErr == nil || mode != "pending-resume" && resumeErr != nil {
				t.Fatal("resume fixture did not establish intended state", resumeErr)
			}
			if mode == "same-operation-fence-after-finalize" {
				if _, err := engine.Finalize(context.Background(), scope, readyACK("readback-finalize", ready)); err != nil {
					t.Fatal(err)
				}
			}
			if mode != "pending-resume" {
				before, err := engine.store.Read(scope, ready.OperationID)
				if err != nil {
					t.Fatal(err)
				}
				_, err = engine.store.Mutate(scope, Expectation{ControlRevision: before.State.ControlRevision, BindingGeneration: before.State.BindingGeneration, OperationID: ready.OperationID, ManifestHash: ready.CoreHash, RequestHash: strings.Repeat("a", 64), Step: "readback-fixture"}, func(c Current) (Mutation, error) {
					c.State.Fence = nil
					if mode == "same-operation-fence-after-finalize" {
						c.State.Fence = &Fence{Kind: "delete", Phase: "deleting-snapshot", OperationID: ready.OperationID, ManifestHash: ready.CoreHash, TargetVMAbsent: true}
					}
					return Mutation{State: c.State, Receipt: c.Receipt}, nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			before, err := engine.store.Read(scope, ready.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			dto, err := engine.Get(context.Background(), scope, ready.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			value := snapshotReadbackMap(t, dto)
			if value["fence_cleared"] != false || value["fence_owned"] != (mode != "missing-finalize-step") {
				t.Fatal("ACK/absence/pending effect was promoted into release proof", value)
			}
			if mode != "same-operation-fence-after-finalize" && value["finalized_control_revision"] != nil {
				t.Fatal("missing finalize step acquired a release revision")
			}
			after, _ := engine.store.Read(scope, ready.OperationID)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("metadata GET added business writes")
			}
		})
	}
}

// capability_id: rainbond.vm-snapshot.authoritative-readback
func TestSnapshotReadbackUsesPersistentSourceACKAndFinalizeEvidence(t *testing.T) {
	ctx := context.Background()
	engine, backend, scope := snapshotEngineFixture(t)
	ready := engineReady(t, engine, scope, backend)
	before, err := engine.store.Read(scope, ready.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	check := func(dto SnapshotOperationDTO, acked, finalized, owned, cleared bool, revision uint64) {
		t.Helper()
		value := snapshotReadbackMap(t, dto)
		for key, want := range map[string]interface{}{"namespace": before.Receipt.Snapshot.SourceVM.Namespace, "source_vm_name": before.Receipt.Snapshot.SourceVM.Name, "source_vm_uid": before.Receipt.Snapshot.SourceVM.UID, "manifest_id": before.Receipt.ManifestID, "fence_owned": owned, "fence_cleared": cleared} {
			if value[key] != want {
				t.Errorf("%s lacks authoritative readback: got=%v want=%v", key, value[key], want)
			}
		}
		if value["manifest_acked"] != acked || value["finalized"] != finalized {
			t.Fatal("existing lifecycle flags changed", value)
		}
		if acked && value["manifest_document_hash"] != strings.Repeat("d", 64) {
			t.Error("accepted manifest document is not readable", value["manifest_document_hash"])
		}
		if !acked && value["manifest_document_hash"] != nil {
			t.Fatal("unaccepted document was projected")
		}
		if revision > 0 && value["finalized_control_revision"] != float64(revision) {
			t.Error("successful finalize revision missing", value["finalized_control_revision"])
		}
		if revision == 0 && value["finalized_control_revision"] != nil {
			t.Fatal("unfinalized operation acquired release revision")
		}
	}
	check(ready, false, false, true, false, 0)
	if _, err := engine.Resume(ctx, scope, readyACK("readback-resume", ready)); err != nil {
		t.Fatal(err)
	}
	acked, err := engine.Get(ctx, scope, ready.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	check(acked, true, false, true, false, 0)
	final, err := engine.Finalize(ctx, scope, readyACK("readback-finalize", ready))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := engine.store.Read(scope, ready.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	step := sealed.Receipt.Steps["snapshot-finalize"]
	if !step.Applied || sealed.State.Fence != nil || sealed.Receipt.ConsoleAckHash != strings.Repeat("d", 64) {
		t.Fatal("actual finalize did not atomically persist ACK/release evidence")
	}
	check(final, true, true, false, true, step.ControlRevision)
	var seq int
	var name, filename string
	if err := engine.store.db.DB().QueryRow("PRAGMA database_list").Scan(&seq, &name, &filename); err != nil {
		t.Fatal(err)
	}
	if err := engine.store.db.Close(); err != nil {
		t.Fatal(err)
	}
	database, err := gorm.Open("sqlite3", filename)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.LogMode(false)
	engine = NewSnapshotEngine(NewStore(database), backend)
	backend.fixture.content.Spec.Source.VirtualMachine.Annotations = map[string]string{"never-expose": "PRIVATE_VM_ANNOTATION_SENTINEL"}
	reopened, err := engine.Get(ctx, scope, ready.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	check(reopened, true, true, false, true, step.ControlRevision)
	raw, _ := json.MarshalIndent(reopened, "", "  ")
	if strings.Contains(string(raw), "PRIVATE_VM_ANNOTATION_SENTINEL") || strings.Contains(string(raw), "cloudInit") {
		t.Fatal("raw VM data escaped safe metadata projection")
	}
	if output := os.Getenv("SNAPSHOT_READBACK_FIXTURE_OUTPUT"); output != "" {
		if err := os.WriteFile(output, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	after, _ := engine.store.Read(scope, ready.OperationID)
	if !reflect.DeepEqual(sealed, after) {
		t.Fatal("GET/reopen changed persistent operation state")
	}
	_, err = engine.store.Mutate(scope, Expectation{ControlRevision: after.State.ControlRevision, BindingGeneration: after.State.BindingGeneration, OperationID: "other-op", ManifestHash: strings.Repeat("a", 64), RequestHash: strings.Repeat("b", 64), Step: "other-prepare"}, func(c Current) (Mutation, error) {
		r := NewReceipt(scope, "other-op", strings.Repeat("a", 64))
		c.State.Fence = &Fence{Kind: "restore", Phase: "prepared", OperationID: "other-op", ManifestHash: r.ManifestHash, TargetVMUID: "other-vm"}
		return Mutation{State: c.State, Receipt: &r}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	busy, err := engine.Get(ctx, scope, ready.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	check(busy, true, true, false, false, step.ControlRevision)
	foreign := scope
	foreign.Namespace = "foreign"
	if _, err := engine.Get(ctx, foreign, ready.OperationID); err == nil {
		t.Fatal("foreign namespace read accepted private metadata")
	}
}
