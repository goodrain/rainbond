package vmrestore

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/pkg/vmrestore/copyio"
	"github.com/goodrain/rainbond/pkg/vmrestore/restoreplan"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type restoreFakeRuntime struct {
	observation RestoreRuntimeObservation
	applies     int
	applyErr    error
}

type testRestoreCapacity struct {
	releases   int
	releaseErr error
}

func (*testRestoreCapacity) Reserve(context.Context, Scope, string, RestoreResourcePlan, copyio.Report) (restoreplan.Capability, restoreplan.SpaceEvidence, error) {
	return restoreplan.Capability{}, restoreplan.SpaceEvidence{}, ErrRestoreCapability
}
func (*testRestoreCapacity) Check(context.Context, Scope, string, restoreplan.Budget) error {
	return nil
}
func (p *testRestoreCapacity) Release(context.Context, Scope, string) error {
	p.releases++
	return p.releaseErr
}

// capability_id: rainbond.vm-restore.bounded-execution
func TestCancellationFinalizeRequiresExplicitCancelAndObservedHalt(t *testing.T) {
	for _, cancelRecorded := range []bool{false, true} {
		f, e, r := engineFixture(t)
		_, err := f.store.executionUpdate(f.scope, f.expect.OperationID, "restore-writers-clear", true, func(c Current) (Mutation, error) {
			c.State.Fence.Phase = "stopped"
			return Mutation{State: c.State, Receipt: c.Receipt}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if cancelRecorded {
			_, err = f.store.executionUpdate(f.scope, f.expect.OperationID, "restore-cancel-requested", true, func(c Current) (Mutation, error) { return Mutation{State: c.State, Receipt: c.Receipt}, nil })
			if err != nil {
				t.Fatal(err)
			}
		}
		r.observation.Halted = false
		r.observation.WritersClear = false
		capacity := &testRestoreCapacity{}
		e.Capacity = capacity
		if _, err := e.Finalize(context.Background(), f.scope, f.expect.OperationID, false, true); err == nil || capacity.releases != 0 {
			t.Fatal("cancel finalized without halt and explicit intent", err)
		}
		c, _ := f.store.Read(f.scope, f.expect.OperationID)
		if c.State.Fence == nil {
			t.Fatal("cancel released active writer fence")
		}
		if cancelRecorded {
			r.observation.Halted = true
			r.observation.WritersClear = true
			capacity.releaseErr = errors.New("fixture release timeout")
			if _, err := e.Finalize(context.Background(), f.scope, f.expect.OperationID, false, true); !errors.Is(err, ErrRestoreUnknown) {
				t.Fatal(err)
			}
			c, _ = f.store.Read(f.scope, f.expect.OperationID)
			if c.State.Fence == nil || c.Receipt.Steps["capacity-released"].Applied {
				t.Fatal("release timeout became success")
			}
			capacity.releaseErr = nil
			if _, err := e.Finalize(context.Background(), f.scope, f.expect.OperationID, false, true); err != nil {
				t.Fatal(err)
			}
			c, _ = f.store.Read(f.scope, f.expect.OperationID)
			if c.State.Fence != nil || !c.Receipt.Steps["restore-finalize"].Applied {
				t.Fatal("cancel cleanup not finalized")
			}
		}
	}
}

func (r *restoreFakeRuntime) Observe(context.Context, RestoreRuntimeRequest) (RestoreRuntimeObservation, error) {
	return r.observation, nil
}
func (r *restoreFakeRuntime) Apply(context.Context, RestoreRuntimeRequest) (RestoreRuntimeObservation, error) {
	r.applies++
	return r.observation, r.applyErr
}
func engineFixture(t *testing.T) (restoreCommitFixture, *RestoreEngine, *restoreFakeRuntime) {
	f, _, resources := resourceFixture(t)
	startupHash, _ := snapshotDigest(testStartupPolicy("restored"))
	runtime := &restoreFakeRuntime{observation: RestoreRuntimeObservation{VM: f.intent.CurrentVM, BindingGeneration: 0, NetworkHash: strings.Repeat("e", 64), IdentityHash: startupHash}}
	engine := &RestoreEngine{Store: f.store, Resources: resources, Runtime: runtime}
	engine.WireRestoreResourceGuard()
	return f, engine, runtime
}

// capability_id: rainbond.vm-restore.bounded-execution
func TestRuntimeIntentSurvivesUnknownOutcomeAndCompletesOnlyObserved(t *testing.T) {
	f, e, r := engineFixture(t)
	r.applyErr = errors.New("SYNTHETIC_TIMEOUT")
	if _, err := e.RuntimeStep(context.Background(), f.scope, f.expect.OperationID, "halt"); !errors.Is(err, ErrRestoreUnknown) {
		t.Fatal(err)
	}
	c, _ := f.store.Read(f.scope, f.expect.OperationID)
	if !c.Receipt.Steps["runtime-intent-halt"].Applied || c.Receipt.Steps["restore-writers-clear"].Applied || c.State.Fence == nil {
		t.Fatal("unknown outcome released fence or claimed stopped")
	}
	r.observation.Halted = true
	r.observation.WritersClear = true
	c, err := e.RuntimeStep(context.Background(), f.scope, f.expect.OperationID, "halt")
	if err != nil || c.State.Fence.Phase != "stopped" || r.applies != 1 {
		t.Fatal(c.State.Fence, err, r.applies)
	}
	if _, err = e.RuntimeStep(context.Background(), f.scope, f.expect.OperationID, "halt"); err != nil || r.applies != 1 {
		t.Fatal("replay repeated mutation", err)
	}
}

// capability_id: rainbond.vm-restore.bounded-execution
func TestRuntimeRejectsReplacementVMStaleGenerationAndChangedNetworkBeforeApply(t *testing.T) {
	for _, tc := range []string{"uid", "generation", "network"} {
		t.Run(tc, func(t *testing.T) {
			f, e, r := engineFixture(t)
			switch tc {
			case "uid":
				r.observation.VM.UID = "replacement"
			case "generation":
				r.observation.BindingGeneration = 1
			case "network":
				r.observation.NetworkHash = strings.Repeat("a", 64)
			}
			if _, err := e.RuntimeStep(context.Background(), f.scope, f.expect.OperationID, "halt"); !errors.Is(err, ErrConflict) || r.applies != 0 {
				t.Fatal("unsafe runtime applied", err, r.applies)
			}
		})
	}
}

// capability_id: rainbond.vm-restore.bounded-execution
func TestUnsealedBooleanAndIncompleteStagesCannotCommit(t *testing.T) {
	f, e, _ := engineFixture(t)
	before, _ := f.store.Read(f.scope, f.expect.OperationID)
	if _, err := e.Commit(context.Background(), f.scope, f.expect.OperationID, f.after); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	after, _ := f.store.Read(f.scope, f.expect.OperationID)
	if after.State.BindingGeneration != before.State.BindingGeneration || after.Receipt.Steps[RestoreCommitStep].Applied {
		t.Fatal("partial evidence committed")
	}
	if _, err := e.Finalize(context.Background(), f.scope, f.expect.OperationID, false, false); !errors.Is(err, ErrConflict) {
		t.Fatal("finalized without ACK/sync", err)
	}
}

func TestRuntimeUnknownApplyIsObservedWithoutBlindReissue(t *testing.T) {
	f, e, r := engineFixture(t)
	r.applyErr = errors.New("fixture unknown mutation")
	for i := 0; i < 2; i++ {
		if _, err := e.RuntimeStep(context.Background(), f.scope, f.expect.OperationID, "halt"); !errors.Is(err, ErrRestoreUnknown) {
			t.Fatal(err)
		}
	}
	if r.applies != 1 {
		t.Fatal("unknown lifecycle mutation was repeated", r.applies)
	}
	r.observation.Halted = true
	r.observation.WritersClear = true
	if _, err := e.RuntimeStep(context.Background(), f.scope, f.expect.OperationID, "halt"); err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.vm-restore.bounded-execution
func TestRuntimeSyncCanApplyBeforeSelectedIdentityIsObserved(t *testing.T) {
	f, e, r := engineFixture(t)
	_, err := f.store.executionUpdate(f.scope, f.expect.OperationID, "fixture-console-acked", "fixture", func(c Current) (Mutation, error) {
		c.State.Fence.Phase = "console-acked"
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	r.observation.IdentityHash = ""
	r.observation.Halted = true
	r.observation.WritersClear = true
	if _, err := e.RuntimeStep(context.Background(), f.scope, f.expect.OperationID, "sync-halted"); !errors.Is(err, ErrRestorePending) || r.applies != 1 {
		t.Fatal("sync rejected current B before it could install selected A", err, r.applies)
	}
}

// capability_id: rainbond.vm-restore.bounded-execution
func TestRollbackCommandRechecksCurrentWritersBeforeAtomicBSelection(t *testing.T) {
	f, e, r := engineFixture(t)
	f.sealCopy(t)
	c, err := f.store.CommitRestoreDB(f.scope, f.expectStep(t, RestoreCommitStep), f.request(t))
	if err != nil {
		t.Fatal(err)
	}
	r.observation.BindingGeneration = c.State.BindingGeneration
	r.observation.Halted = true
	r.observation.WritersClear = false
	f.rollbackReady(t)
	c, _ = f.store.Read(f.scope, f.expect.OperationID)
	request := RestoreCommand{ExpectedControlRevision: c.State.ControlRevision, ExpectedBindingGeneration: c.State.BindingGeneration}
	status, _ := e.Status(f.scope, f.expect.OperationID)
	request.ExpectedOperationEpoch = status.OperationEpoch
	if _, err := e.Command(context.Background(), f.scope, f.expect.OperationID, "rollback", request); !errors.Is(err, ErrRestorePending) {
		t.Fatal("stale stopped proof switched disks while a new writer exists", err)
	}
	c, _ = f.store.Read(f.scope, f.expect.OperationID)
	if c.State.BindingGeneration != 1 || c.Receipt.Steps["restore-rollback"].Applied {
		t.Fatal("unsafe rollback changed database")
	}
	r.observation.WritersClear = true
	for _, b := range f.old {
		if _, err := e.Resources.Core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Create(context.Background(), &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: b.ActivePVC.Name, Namespace: f.scope.Namespace, UID: types.UID(b.ActivePVC.UID)}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	d, err := e.Command(context.Background(), f.scope, f.expect.OperationID, "rollback", request)
	if err != nil || d.RollbackConfigHash == "" {
		t.Fatal(d, err)
	}
	c, _ = f.store.Read(f.scope, f.expect.OperationID)
	s := engineScenario{f: f, e: e}
	s.command(t, "rollback-ack", RestoreCommand{CommitHash: c.Receipt.RollbackConfigHash, DocumentHash: strings.Repeat("a", 64)})
	r.observation.BindingGeneration = 2
	r.observation.DesiredMatches = true
	r.observation.IdentityHash, _ = snapshotDigest(testStartupPolicy("platform"))
	s.command(t, "rollback-sync-halted", RestoreCommand{})
	r.observation.Halted = false
	r.observation.Running = true
	s.command(t, "rollback-resume", RestoreCommand{})
	e.Capacity = &testRestoreCapacity{}
	d = s.command(t, "finalize", RestoreCommand{Rollback: true})
	if !d.Complete {
		t.Fatal("acknowledged rollback did not complete")
	}
}
