package vmrestore

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// capability_id: rainbond.vm-restore.owned-state-mutation
func TestStateOnlyMutationRequiresExplicitExistingAdmissionOwner(t *testing.T) {
	for _, owner := range []string{"", "foreign-owner", "held-owner"} {
		t.Run("owner="+owner, func(t *testing.T) {
			store, _, scope := fixtureStore(t)
			before, err := store.Mutate(scope, Expectation{}, func(c Current) (Mutation, error) {
				if err := c.State.AdmitMutation("held-owner", "config-change"); err != nil {
					return Mutation{}, err
				}
				return Mutation{State: c.State}, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			called := false
			result, err := store.Mutate(scope, Expectation{ControlRevision: before.State.ControlRevision, BindingGeneration: before.State.BindingGeneration, AdmissionOwner: owner}, func(c Current) (Mutation, error) {
				called = true
				if err := c.State.FinishMutation("held-owner", true); err != nil {
					return Mutation{}, err
				}
				return Mutation{State: c.State}, nil
			})
			if owner != "held-owner" {
				if !errors.Is(err, ErrBusy) || called {
					t.Fatal("missing/foreign owner reached state callback", err)
				}
				after, readErr := store.Read(scope, "")
				if readErr != nil || !reflect.DeepEqual(before, after) {
					t.Fatal("rejected actor changed state", readErr)
				}
				return
			}
			if err != nil || !called || result.State.Admission != nil || result.State.ControlRevision != before.State.ControlRevision+1 || result.Receipt != nil {
				t.Fatal("existing owner could not finish without creating a new receipt", err)
			}
			raw, _ := json.Marshal(result.State)
			if strings.Contains(string(raw), "admission_owner") || strings.Contains(string(raw), "AdmissionOwner") {
				t.Fatal("internal expectation became persisted authority")
			}
		})
	}
}

// capability_id: rainbond.vm-restore.owned-state-mutation
func TestAdmissionOwnerCannotAuthorizeAnotherReceiptOrScope(t *testing.T) {
	store, _, scope := fixtureStore(t)
	before, err := store.Mutate(scope, Expectation{}, func(c Current) (Mutation, error) {
		if err := c.State.AdmitMutation("held-owner", "config-change"); err != nil {
			return Mutation{}, err
		}
		return Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, otherScope := range []bool{false, true} {
		expect := Expectation{ControlRevision: before.State.ControlRevision, BindingGeneration: before.State.BindingGeneration, AdmissionOwner: "held-owner"}
		requested := scope
		if otherScope {
			requested.Namespace = "other"
		} else {
			expect.OperationID, expect.ManifestHash, expect.RequestHash, expect.Step = "other-operation", strings.Repeat("a", 64), strings.Repeat("b", 64), "prepare"
		}
		if _, err := store.Mutate(requested, expect, func(Current) (Mutation, error) {
			t.Fatal("state owner escaped to another scope or receipt")
			return Mutation{}, nil
		}); !errors.Is(err, ErrCorrupt) {
			t.Fatal("invalid authority combination accepted", err)
		}
	}
}

// capability_id: rainbond.vm-restore.owned-state-mutation
func TestOwnedStateMutationCannotReuseStaleControlRevision(t *testing.T) {
	store, _, scope := fixtureStore(t)
	before, err := store.Mutate(scope, Expectation{}, func(c Current) (Mutation, error) {
		if err := c.State.AdmitMutation("held-owner", "config-change"); err != nil {
			return Mutation{}, err
		}
		return Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	expected := Expectation{ControlRevision: before.State.ControlRevision, BindingGeneration: before.State.BindingGeneration, AdmissionOwner: "held-owner"}
	advanced, err := store.Mutate(scope, expected, func(c Current) (Mutation, error) {
		if err := c.State.FinishMutation("held-owner", false); err != nil {
			return Mutation{}, err
		}
		return Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Mutate(scope, expected, func(Current) (Mutation, error) {
		t.Fatal("stale execution reached callback despite same owner")
		return Mutation{}, nil
	}); !errors.Is(err, ErrConflict) {
		t.Fatal("same owner bypassed stale CAS", err)
	}
	after, err := store.Read(scope, "")
	if err != nil || !reflect.DeepEqual(advanced, after) {
		t.Fatal("stale execution changed its newer admission", err)
	}
}
