package vmrestore

import (
	"errors"
	"reflect"
)

const NativeRestoreCommitStep = "native-region-commit"

// ErrNativeRestoreConfigurationChanged keeps native data restore within the
// current Console/Region configuration; cross-configuration rollback needs ACK.
var ErrNativeRestoreConfigurationChanged = errors.New("native_restore_configuration_changed")

// NativeRestoreControl seals the platform selection independently of the copy
// provider. A CSI/KubeVirt clone is never represented as inspected/copied data.
type NativeRestoreControl struct {
	Config              RestoreConfig        `json:"config"`
	BeforeStartup       RestoreStartupPolicy `json:"before_startup"`
	SourceStartup       RestoreStartupPolicy `json:"source_startup"`
	SnapshotOperationID string               `json:"snapshot_operation_id"`
	NativeProofHash     string               `json:"native_proof_hash"`
	RestoreName         string               `json:"restore_name"`
	RequestID           string               `json:"request_id"`
	Nonce               string               `json:"nonce"`
}

func (n NativeRestoreControl) validate(scope Scope, intent *RestoreIntent) error {
	if intent == nil || n.Config.validate(scope) != nil || n.BeforeStartup.validate(scope) != nil || n.SourceStartup.validate(scope) != nil || n.SourceStartup.BootstrapMode != "restored" || !idPattern.MatchString(n.SnapshotOperationID) || !hashPattern.MatchString(n.NativeProofHash) || !validName(n.RestoreName) || !ValidNativeRestoreRequestID(n.RequestID) || !validUID(n.Nonce) {
		return ErrCorrupt
	}
	hash, err := RestoreConfigHash(n.Config)
	beforeHash, beforeErr := RestoreConfigHash(intent.Before)
	if err != nil || beforeErr != nil || hash != intent.ConfigAfterHash || hash != beforeHash {
		return ErrConflict
	}
	return nil
}

func nativeOwner(c Current, scope Scope, e Expectation) error {
	if restoreOwner(c, scope, e) != nil || c.Receipt.NativeRestore == nil || c.Receipt.Execution != nil {
		return ErrConflict
	}
	return nil
}

// PrepareNativeRestoreDB reserves the same durable component fence as all
// ordinary mutations, and pins the original physical disk set before any RPC.
func (s *Store) PrepareNativeRestoreDB(scope Scope, e Expectation, intent RestoreIntent, control NativeRestoreControl, old map[string]Binding) (Current, error) {
	if intent.validate(scope) != nil || control.validate(scope, &intent) != nil || validateBindings(old) != nil || len(old) != len(intent.Before.Volumes) {
		return Current{}, ErrCorrupt
	}
	e.Step = "native-prepare"
	e.RequestHash, _ = snapshotDigest(struct {
		Intent  RestoreIntent
		Control NativeRestoreControl
		Old     map[string]Binding
	}{intent, control, old})
	tx := s.db.Begin()
	if tx.Error != nil {
		return Current{}, tx.Error
	}
	defer tx.Rollback()
	result, err := s.MutateTx(tx, scope, e, func(c Current) (Mutation, error) {
		if c.State.Fence != nil || c.State.Admission != nil || c.Receipt != nil || c.State.BindingGeneration != intent.SourceGeneration {
			return Mutation{}, ErrBusy
		}
		if err := requireBeforeNativeConfig(tx, scope, intent.Before); err != nil {
			return Mutation{}, err
		}
		for key, b := range c.State.Bindings {
			if !reflect.DeepEqual(old[key], b) {
				return Mutation{}, ErrConflict
			}
		}
		r := NewReceipt(scope, e.OperationID, e.ManifestHash)
		r.OldBindings = cloneBindings(old)
		r.RetainOldPVCs = true
		r.Restore = &intent
		r.NativeRestore = &control
		r.Resources = []ResourceRef{{ObjectRef: intent.CurrentVM, Purpose: "native-source-vm"}, {ObjectRef: intent.SourceSnapshot, Purpose: "native-source-snapshot"}, {ObjectRef: intent.SourceContent, Purpose: "native-source-content"}}
		c.State.Fence = &Fence{Kind: "restore", Phase: "native-prepared", OperationID: e.OperationID, ManifestHash: e.ManifestHash, TargetVMUID: intent.CurrentVM.UID}
		return Mutation{State: c.State, Receipt: &r}, nil
	})
	if err != nil {
		return Current{}, err
	}
	if err = tx.Commit().Error; err != nil {
		return Current{}, err
	}
	return result, nil
}

// SealNativeRestoreObservation is called only after live native/PVC/PV and
// stopped-writer checks; the following commit compares its full sealed result.
func (s *Store) SealNativeRestoreObservation(scope Scope, e Expectation, ref ObjectRef, bindings map[string]Binding) (Current, error) {
	if ref.Kind != "VirtualMachineRestore" || ref.Namespace != scope.Namespace || !validResourceRef(ResourceRef{ObjectRef: ref, Purpose: "native-restore"}, scope) || validateBindings(bindings) != nil {
		return Current{}, ErrCorrupt
	}
	e.Step = "native-observed"
	e.RequestHash, _ = snapshotDigest(struct {
		Restore  ObjectRef
		Bindings map[string]Binding
	}{ref, bindings})
	return s.Mutate(scope, e, func(c Current) (Mutation, error) {
		if nativeOwner(c, scope, e) != nil || c.State.Fence.Phase != "native-prepared" || ref.Name != c.Receipt.NativeRestore.RestoreName {
			return Mutation{}, ErrConflict
		}
		c.Receipt.Resources = resourceOnce(c.Receipt.Resources, ResourceRef{ObjectRef: ref, Purpose: "native-restore"})
		for _, b := range bindings {
			if b.Attached {
				c.Receipt.Resources = resourceOnce(c.Receipt.Resources, ResourceRef{ObjectRef: b.ActivePVC, Purpose: "restore-target"})
				if b.ActiveDV != nil {
					c.Receipt.Resources = resourceOnce(c.Receipt.Resources, ResourceRef{ObjectRef: *b.ActiveDV, Purpose: "native-restored-dv"})
				}
			}
		}
		c.Receipt.NewBindings = cloneBindings(bindings)
		c.State.Fence.Phase = "native-observed"
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
}

// CommitNativeRestoreDB commits exact observed CSI clones and selected platform
// configuration atomically. It does not weaken CommitRestoreDB's copy checks.
func (s *Store) CommitNativeRestoreDB(scope Scope, e Expectation, request RestoreCommitRequest) (Current, error) {
	if e.Step != NativeRestoreCommitStep || request.Config.validate(scope) != nil || validateBindings(request.Bindings) != nil {
		return Current{}, ErrCorrupt
	}
	e.RequestHash, _ = snapshotDigest(request)
	tx := s.db.Begin()
	if tx.Error != nil {
		return Current{}, tx.Error
	}
	defer tx.Rollback()
	result, err := s.MutateTx(tx, scope, e, func(c Current) (Mutation, error) {
		if nativeOwner(c, scope, e) != nil || c.State.Fence.Phase != "native-observed" || !c.Receipt.Steps["native-observed"].Applied || !reflect.DeepEqual(c.Receipt.NewBindings, request.Bindings) || !reflect.DeepEqual(c.Receipt.NativeRestore.Config, request.Config) {
			return Mutation{}, ErrConflict
		}
		var observed *ObjectRef
		for _, r := range c.Receipt.Resources {
			if r.Purpose == "native-restore" {
				copy := r.ObjectRef
				observed = &copy
			}
		}
		if observed == nil {
			return Mutation{}, ErrConflict
		}
		hash, _ := snapshotDigest(struct {
			Restore  ObjectRef
			Bindings map[string]Binding
		}{*observed, request.Bindings})
		if hash != c.Receipt.Steps["native-observed"].RequestHash {
			return Mutation{}, ErrConflict
		}
		if err := requireBeforeNativeConfig(tx, scope, c.Receipt.Restore.Before); err != nil {
			return Mutation{}, err
		}
		// Native restore is restricted to the same logical configuration. Only
		// replace physical bindings; retain Console's display/media declarations.
		beforeHash, _ := RestoreConfigHash(c.Receipt.Restore.Before)
		afterHash, _ := RestoreConfigHash(request.Config)
		if beforeHash != afterHash {
			return Mutation{}, ErrNativeRestoreConfigurationChanged
		}
		c.State.Bindings = cloneBindings(request.Bindings)
		c.State.BindingGeneration++
		c.State.Fence.Phase = "native-committed"
		c.Receipt.ConfigCommitHash = e.RequestHash
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		return Current{}, err
	}
	if err = tx.Commit().Error; err != nil {
		return Current{}, err
	}
	return result, nil
}

// FinalizeNativeRestoreDB is a command-only release after the caller verifies
// the committed Resolver view matches the halted VM's actual device references.
func (s *Store) FinalizeNativeRestoreDB(scope Scope, e Expectation) (Current, error) {
	e.Step = "native-finalize"
	e.RequestHash, _ = snapshotDigest(struct{ Operation string }{e.OperationID})
	return s.Mutate(scope, e, func(c Current) (Mutation, error) {
		if nativeOwner(c, scope, e) != nil || c.State.Fence.Phase != "native-committed" || !c.Receipt.Steps[NativeRestoreCommitStep].Applied || c.Receipt.ConfigCommitHash == "" {
			return Mutation{}, ErrConflict
		}
		c.State.Fence = nil
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
}
