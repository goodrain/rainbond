package vmrestore

import (
	"context"
	"errors"
	"strconv"
	"testing"
)

type casPrepareSource struct {
	value   RestorePreparation
	entered chan struct{}
	release chan struct{}
}

func (p *casPrepareSource) Read(ctx context.Context, _ Scope, _, _, _ string) (RestorePreparation, error) {
	close(p.entered)
	select {
	case <-p.release:
		return p.value, nil
	case <-ctx.Done():
		return RestorePreparation{}, ctx.Err()
	}
}

// capability_id: rainbond.vm-restore.rpc-command-cas
func TestPrepareRPCUsesOriginalCASAtItsActualDatabaseCommit(t *testing.T) {
	f, e, source := enginePrepareFixture(t)
	barrier := &casPrepareSource{value: source.value, entered: make(chan struct{}), release: make(chan struct{})}
	e.Source = barrier
	result := make(chan error, 1)
	go func() {
		_, err := e.Command(context.Background(), f.scope, f.expect.OperationID, "prepare", RestoreCommand{SnapshotName: f.intent.SourceSnapshot.Name, PowerAfter: f.intent.PowerAfter})
		result <- err
	}()
	<-barrier.entered
	_, err := f.store.Mutate(f.scope, Expectation{}, func(c Current) (Mutation, error) { return Mutation{State: c.State}, nil })
	if err != nil {
		close(barrier.release)
		t.Fatal(err)
	}
	close(barrier.release)
	if err := <-result; !errors.Is(err, ErrConflict) {
		t.Fatal("prepare adopted a newer revision after source verification", err)
	}
	c, _ := f.store.Read(f.scope, f.expect.OperationID)
	if c.State.Fence != nil || c.Receipt != nil || c.State.ControlRevision != 1 {
		t.Fatal("stale prepare acquired state or released another writer")
	}
}

// capability_id: rainbond.vm-restore.rpc-command-cas
func TestActualCommandsWithSameCASHaveOneDurableWinner(t *testing.T) {
	for round := 0; round < 20; round++ {
		t.Run(strconv.Itoa(round), func(t *testing.T) {
			f, e, r := engineFixture(t)
			r.observation.Halted = true
			r.observation.WritersClear = true
			d, _ := e.Status(f.scope, f.expect.OperationID)
			request := RestoreCommand{ExpectedOperationEpoch: d.OperationEpoch, ExpectedControlRevision: d.ControlRevision, ExpectedBindingGeneration: d.BindingGeneration}
			start := make(chan struct{})
			type outcome struct {
				action string
				dto    RestoreOperationDTO
				err    error
			}
			results := make(chan outcome, 2)
			for _, action := range []string{"cancel", "halt"} {
				go func(action string) {
					<-start
					dto, err := e.Command(context.Background(), f.scope, f.expect.OperationID, action, request)
					results <- outcome{action, dto, err}
				}(action)
			}
			close(start)
			wins := 0
			winner := ""
			for i := 0; i < 2; i++ {
				result := <-results
				if result.err == nil && result.dto.Applied {
					wins++
					winner = result.action
				} else if !errors.Is(result.err, ErrConflict) {
					t.Fatal("losing command did not report conflict", result.err)
				}
			}
			if wins != 1 {
				t.Fatal("mutually exclusive old-CAS commands had multiple winners", wins)
			}
			c, _ := f.store.Read(f.scope, f.expect.OperationID)
			if c.State.Fence == nil || r.applies != 0 {
				t.Fatal("loser changed external runtime or cleared fence")
			}
			if winner == "cancel" && c.Receipt.Steps["restore-writers-clear"].Applied {
				t.Fatal("losing halt committed")
			}
			if winner == "halt" && c.Receipt.Steps["restore-cancel-requested"].Applied {
				t.Fatal("losing cancel committed")
			}
		})
	}
}

// capability_id: rainbond.vm-restore.rpc-command-cas
func TestFreshCASReentryKeepsUnknownRuntimeIssuedOnce(t *testing.T) {
	f, e, r := engineFixture(t)
	r.applyErr = errors.New("fixture unknown write")
	for i := 0; i < 2; i++ {
		d, _ := e.Status(f.scope, f.expect.OperationID)
		_, err := e.Command(context.Background(), f.scope, f.expect.OperationID, "halt", RestoreCommand{ExpectedOperationEpoch: d.OperationEpoch, ExpectedControlRevision: d.ControlRevision, ExpectedBindingGeneration: d.BindingGeneration})
		if !errors.Is(err, ErrRestoreUnknown) {
			t.Fatal(err)
		}
	}
	if r.applies != 1 {
		t.Fatal("legal reentry blindly repeated runtime mutation", r.applies)
	}
	r.observation.Halted = true
	r.observation.WritersClear = true
	d, _ := e.Status(f.scope, f.expect.OperationID)
	result, err := e.Command(context.Background(), f.scope, f.expect.OperationID, "halt", RestoreCommand{ExpectedOperationEpoch: d.OperationEpoch, ExpectedControlRevision: d.ControlRevision, ExpectedBindingGeneration: d.BindingGeneration})
	if err != nil || !result.Applied || result.Complete {
		t.Fatal("same operation could not reconcile actual completion", err)
	}
	d, _ = e.Status(f.scope, f.expect.OperationID)
	if _, err := e.Command(context.Background(), f.scope, f.expect.OperationID, "halt", RestoreCommand{ExpectedOperationEpoch: d.OperationEpoch, ExpectedControlRevision: d.ControlRevision, ExpectedBindingGeneration: d.BindingGeneration}); err != nil || r.applies != 1 {
		t.Fatal("successful reentry repeated mutation", err)
	}
}
