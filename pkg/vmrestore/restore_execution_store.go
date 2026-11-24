package vmrestore

import (
	"errors"
	"reflect"
	"strconv"

	"github.com/goodrain/rainbond/pkg/vmrestore/restoreplan"
)

// executionUpdate changes DB only. It retries optimistic conflicts, never I/O.
func (s *Store) executionUpdate(scope Scope, op, step string, request interface{}, fn func(Current) (Mutation, error)) (Current, error) {
	hash, err := snapshotDigest(request)
	if err != nil {
		return Current{}, err
	}
	for attempt := 0; attempt < 3; attempt++ {
		c, err := s.Read(scope, op)
		if err != nil {
			return c, err
		}
		if c.Receipt == nil {
			return c, ErrNotFound
		}
		exp := Expectation{ControlRevision: c.State.ControlRevision, BindingGeneration: c.State.BindingGeneration, OperationID: op, ManifestHash: c.Receipt.ManifestHash, Step: step, RequestHash: hash}
		c, err = s.Mutate(scope, exp, func(now Current) (Mutation, error) {
			if err := restoreOwner(now, scope, exp); err != nil {
				return Mutation{}, err
			}
			return fn(now)
		})
		if !errors.Is(err, ErrConflict) {
			return c, err
		}
	}
	return Current{}, ErrConflict
}

// InitializeRestoreExecution is internal, using only a server-built certified frame.
func (s *Store) InitializeRestoreExecution(scope Scope, op string, x RestoreExecution) (Current, error) {
	if x.validate(scope, op, x.Frame.CoreHash) != nil {
		return Current{}, ErrCorrupt
	}
	return s.executionUpdate(scope, op, "restore-resource-frame", x, func(c Current) (Mutation, error) {
		if x.ConfigAfter != nil {
			hash, err := RestoreConfigHash(*x.ConfigAfter)
			if err != nil || hash != c.Receipt.Restore.ConfigAfterHash {
				return Mutation{}, ErrConflict
			}
		}
		if c.Receipt.Execution != nil || c.Receipt.Restore.SourceGeneration != x.Frame.SourceGeneration || c.State.Fence.Phase != "prepared" {
			return Mutation{}, ErrConflict
		}
		for _, disk := range x.Frame.Disks {
			binding, exists := c.Receipt.OldBindings[strconv.FormatUint(uint64(disk.VolumeID), 10)]
			if !exists || !sameObject(binding.ActivePVC, disk.PreviousPVC) {
				return Mutation{}, ErrConflict
			}
		}
		c.Receipt.Execution = &x
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
}
func (s *Store) resourceIntent(scope Scope, op string, v RestoreObjectIntent) (Current, error) {
	return s.executionUpdate(scope, op, "intent-"+v.Key, v, func(c Current) (Mutation, error) {
		if c.Receipt.Execution == nil {
			return Mutation{}, ErrConflict
		}
		if prior, ok := c.Receipt.Execution.Intents[v.Key]; ok && !reflect.DeepEqual(prior, v) {
			return Mutation{}, ErrConflict
		}
		c.Receipt.Execution.Intents[v.Key] = v
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
}
func (s *Store) resourceObserved(scope Scope, op, key string, ref ObjectRef, pv *ObjectRef) (Current, error) {
	req := struct {
		Ref ObjectRef
		PV  *ObjectRef
	}{ref, pv}
	return s.executionUpdate(scope, op, "observed-"+key, req, func(c Current) (Mutation, error) {
		if c.Receipt.Execution == nil {
			return Mutation{}, ErrConflict
		}
		v, ok := c.Receipt.Execution.Intents[key]
		if !ok || v.Object.Kind != ref.Kind || v.Object.Namespace != ref.Namespace || v.Object.Name != ref.Name || !validUID(ref.UID) || (v.Object.UID != "" && v.Object.UID != ref.UID) {
			return Mutation{}, ErrConflict
		}
		v.Object = ref
		v.BoundPV = pv
		c.Receipt.Execution.Intents[key] = v
		c.Receipt.Resources = resourceOnce(c.Receipt.Resources, ResourceRef{ObjectRef: ref, Purpose: v.Purpose})
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
}

// resourceCreateIssued is the single durable authority to issue this intent's
// Create. A replay may observe its outcome, but must never repeat the request.
func (s *Store) resourceCreateIssued(scope Scope, op string, intent RestoreObjectIntent) (Current, error) {
	return s.executionUpdate(scope, op, "create-"+intent.Key, intent, func(c Current) (Mutation, error) {
		if c.Receipt.Execution == nil || !resourceCreateAllowed(c) {
			return Mutation{}, ErrConflict
		}
		current, exists := c.Receipt.Execution.Intents[intent.Key]
		if !exists || !reflect.DeepEqual(current, intent) || current.Object.UID != "" || current.DeleteRequested || current.Deleted {
			return Mutation{}, ErrConflict
		}
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
}
func (s *Store) resourceDeleting(scope Scope, op, key string, done bool) (Current, error) {
	suffix := "delete-"
	if done {
		suffix = "deleted-"
	}
	return s.executionUpdate(scope, op, suffix+key, done, func(c Current) (Mutation, error) {
		if c.Receipt.Execution == nil {
			return Mutation{}, ErrConflict
		}
		v, ok := c.Receipt.Execution.Intents[key]
		if !ok || v.Object.UID == "" || (done && !v.DeleteRequested) {
			return Mutation{}, ErrConflict
		}
		v.DeleteRequested = true
		v.Deleted = done
		c.Receipt.Execution.Intents[key] = v
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
}
func (s *Store) saveCopyEvidence(scope Scope, op, mode string, e restoreplan.CopyEvidence) error {
	// Current authority is distinct from an immutable historical observation.
	// A->B->A must install A again, while A->A performs no SQL write.
	hash, err := snapshotDigest(e)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 3; attempt++ {
		current, err := s.Read(scope, op)
		if err != nil {
			return err
		}
		if current.Receipt == nil || current.Receipt.Execution == nil || restoreOwner(current, scope, Expectation{OperationID: op, ManifestHash: current.Receipt.ManifestHash}) != nil {
			return ErrConflict
		}
		if old, exists := current.Receipt.Execution.Evidence[mode]; exists {
			if old.Identity.PodUID != e.Identity.PodUID {
				return ErrConflict
			}
			if reflect.DeepEqual(old, e) {
				return nil
			}
		}
		step := "evidence-" + mode + "-" + strconv.FormatUint(current.State.ControlRevision, 36) + "-" + hash[:12]
		updated, err := s.executionUpdate(scope, op, step, e, func(c Current) (Mutation, error) {
			if c.Receipt.Execution == nil {
				return Mutation{}, ErrConflict
			}
			if old, exists := c.Receipt.Execution.Evidence[mode]; exists && old.Identity.PodUID != e.Identity.PodUID {
				return Mutation{}, ErrConflict
			}
			c.Receipt.Execution.Evidence[mode] = e
			return Mutation{State: c.State, Receipt: c.Receipt}, nil
		})
		if err != nil && !errors.Is(err, ErrConflict) {
			return err
		}
		if err == nil && reflect.DeepEqual(updated.Receipt.Execution.Evidence[mode], e) {
			return nil
		}
	}
	return ErrConflict
}

// validExecutionTransition protects immutable intent identity even from a faulty
// internal callback; known UID/delete/evidence state never moves backwards.
func validExecutionTransition(old, next *RestoreExecution) bool {
	if old == nil {
		return true
	}
	if next == nil {
		return false
	}
	if !reflect.DeepEqual(old.Frame, next.Frame) || !reflect.DeepEqual(old.ConfigAfter, next.ConfigAfter) || old.NetworkHash != next.NetworkHash || old.IdentityHash != next.IdentityHash || !reflect.DeepEqual(old.BeforeStartup, next.BeforeStartup) || !reflect.DeepEqual(old.SourceStartup, next.SourceStartup) {
		return false
	}
	for key, a := range old.Intents {
		b, ok := next.Intents[key]
		if !ok || a.Key != b.Key || a.Nonce != b.Nonce || a.DesiredHash != b.DesiredHash || a.Purpose != b.Purpose || a.Object.Kind != b.Object.Kind || a.Object.Namespace != b.Object.Namespace || a.Object.Name != b.Object.Name || (a.Object.UID != "" && a.Object.UID != b.Object.UID) || (a.BoundPV != nil && !reflect.DeepEqual(a.BoundPV, b.BoundPV)) || (a.DeleteRequested && !b.DeleteRequested) || (a.Deleted && !b.Deleted) {
			return false
		}
	}
	for mode, a := range old.Evidence {
		b, ok := next.Evidence[mode]
		if !ok || a.Identity.PodUID != b.Identity.PodUID {
			return false
		}
	}
	return true
}

// Resource control steps retain ownership in the main receipt; duplicating its
// full resource list into every intent/observe/delete step grows quadratically.
func executionStepResources(receipt *Receipt, step string) []ResourceRef {
	if receipt.Execution != nil && step != RestoreCommitStep {
		return nil
	}
	return append([]ResourceRef(nil), receipt.Resources...)
}
