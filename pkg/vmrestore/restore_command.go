package vmrestore

import (
	"context"
	"strconv"
	"time"
)

// RestoreCommand is the bounded Console-to-Region contract. Physical names,
// namespaces, arbitrary VM data and caller success flags are absent.
type RestoreCommand struct {
	ExpectedOperationEpoch    string         `json:"expected_operation_epoch,omitempty"`
	SnapshotName              string         `json:"snapshot_name,omitempty"`
	PowerAfter                string         `json:"power_after,omitempty"`
	ExpectedControlRevision   uint64         `json:"expected_control_revision"`
	ExpectedBindingGeneration uint64         `json:"expected_binding_generation"`
	VolumeID                  uint           `json:"volume_id,omitempty"`
	Mode                      string         `json:"mode,omitempty"`
	ResourceKey               string         `json:"resource_key,omitempty"`
	Config                    *RestoreConfig `json:"config,omitempty"`
	CommitHash                string         `json:"commit_hash,omitempty"`
	DocumentHash              string         `json:"document_hash,omitempty"`
	Rollback                  bool           `json:"rollback,omitempty"`
	Cancelled                 bool           `json:"cancelled,omitempty"`
}

// RestoreSourceIdentity is a server-verified native source seal without media.
type RestoreSourceIdentity struct {
	SnapshotName string `json:"snapshot_name"`
	SnapshotUID  string `json:"snapshot_uid"`
	ContentUID   string `json:"content_uid"`
	VMUID        string `json:"vm_uid"`
	CoreHash     string `json:"core_hash"`
	DocumentHash string `json:"document_hash"`
}

// RestoreControlDTO supplies the initial prepare CAS without allocating a job.
type RestoreControlDTO struct {
	ControlRevision     uint64 `json:"control_revision"`
	BindingGeneration   uint64 `json:"binding_generation"`
	Fenced              bool   `json:"fenced"`
	AdmissionInProgress bool   `json:"admission_in_progress"`
	ActiveOperationID   string `json:"active_operation_id,omitempty"`
}

// Control is read-only and requires no native runtime connectivity.
func (e *RestoreEngine) Control(scope Scope) (RestoreControlDTO, error) {
	if e == nil || e.Store == nil {
		return RestoreControlDTO{}, ErrReferenceUnavailable
	}
	c, err := e.Store.ReadOnly(scope, "")
	if err != nil {
		return RestoreControlDTO{}, err
	}
	d := RestoreControlDTO{ControlRevision: c.State.ControlRevision, BindingGeneration: c.State.BindingGeneration, Fenced: c.State.Fence != nil, AdmissionInProgress: c.State.Admission != nil}
	if c.State.Fence != nil {
		d.ActiveOperationID = c.State.Fence.OperationID
	}
	return d, nil
}

func restoreOperationEpoch(receipt *Receipt) (string, error) {
	if receipt == nil || receipt.Restore == nil {
		return "", ErrNotFound
	}
	return snapshotDigest(struct {
		Scope                     Scope
		OperationID, ManifestHash string
		Intent                    RestoreIntent
	}{receipt.Scope, receipt.OperationID, receipt.ManifestHash, *receipt.Restore})
}

// RestoreOperationDTO is a safe Region receipt projection, never the total
// Console journal. Applied and complete remain separate.
type RestoreOperationDTO struct {
	ConfigAfter                *RestoreConfig        `json:"config_after,omitempty"`
	ConfigAfterHash            string                `json:"config_after_hash"`
	ConfigBefore               RestoreConfig         `json:"config_before"`
	ConfigBeforeHash           string                `json:"config_before_hash"`
	SourceBindingGeneration    uint64                `json:"source_binding_generation"`
	WasRunning                 bool                  `json:"was_running"`
	PowerAfter                 string                `json:"power_after"`
	ResumeRequired             bool                  `json:"resume_required"`
	SourceConfigReady          bool                  `json:"source_config_ready"`
	OperationEpoch             string                `json:"operation_epoch"`
	Source                     RestoreSourceIdentity `json:"source"`
	TargetVMUID                string                `json:"target_vm_uid"`
	ResourceFrameHash          string                `json:"resource_frame_hash,omitempty"`
	FenceOwned                 bool                  `json:"fence_owned"`
	FenceCleared               bool                  `json:"fence_cleared"`
	Outcome                    string                `json:"outcome,omitempty"`
	OperationBindingGeneration uint64                `json:"operation_binding_generation"`
	RollbackAcked              bool                  `json:"rollback_acked"`
	OperationID                string                `json:"operation_id"`
	ControlRevision            uint64                `json:"control_revision"`
	BindingGeneration          uint64                `json:"binding_generation"`
	Phase                      string                `json:"phase"`
	Applied                    bool                  `json:"applied"`
	Complete                   bool                  `json:"complete"`
	ConfigCommitted            bool                  `json:"config_committed"`
	ConsoleAcked               bool                  `json:"console_acked"`
	ConsoleAckHash             string                `json:"console_ack_hash,omitempty"`
	RollbackAckHash            string                `json:"rollback_ack_hash,omitempty"`
	ConfigCommitHash           string                `json:"config_commit_hash,omitempty"`
	RollbackConfigHash         string                `json:"rollback_config_hash,omitempty"`
	SnapshotUID                string                `json:"snapshot_uid"`
	ErrorCode                  string                `json:"error_code,omitempty"`
	Retryable                  bool                  `json:"retryable"`
}

// Status is strictly read-only and remains available when a dependency is down.
func (e *RestoreEngine) Status(scope Scope, op string) (RestoreOperationDTO, error) {
	if e == nil || e.Store == nil || !idPattern.MatchString(op) {
		return RestoreOperationDTO{}, ErrCorrupt
	}
	c, err := e.Store.ReadOnly(scope, op)
	if err != nil {
		return RestoreOperationDTO{}, err
	}
	if c.Receipt == nil || c.Receipt.Restore == nil {
		return RestoreOperationDTO{}, ErrNotFound
	}
	d := RestoreOperationDTO{OperationID: op, ControlRevision: c.State.ControlRevision, BindingGeneration: c.State.BindingGeneration, Phase: "pending", Complete: c.Receipt.Steps["restore-finalize"].Applied, ConfigCommitted: c.Receipt.Steps[RestoreCommitStep].Applied, ConsoleAcked: c.Receipt.ConsoleAckHash != "", ConfigCommitHash: c.Receipt.ConfigCommitHash, RollbackConfigHash: c.Receipt.RollbackConfigHash, SnapshotUID: c.Receipt.Restore.SourceSnapshot.UID}
	d.ConsoleAckHash = c.Receipt.ConsoleAckHash
	d.RollbackAckHash = c.Receipt.RollbackAckHash
	d.OperationEpoch, err = restoreOperationEpoch(c.Receipt)
	if err != nil {
		return d, err
	}
	intent := c.Receipt.Restore
	d.ConfigAfterHash = intent.ConfigAfterHash
	d.ConfigBefore = intent.Before
	d.ConfigBeforeHash, err = RestoreConfigHash(intent.Before)
	if err != nil {
		return d, err
	}
	d.SourceBindingGeneration = intent.SourceGeneration
	d.WasRunning = intent.WasRunning
	d.PowerAfter = intent.PowerAfter
	d.ResumeRequired = intent.WasRunning && intent.PowerAfter == "previous"
	d.Source = RestoreSourceIdentity{SnapshotName: intent.SourceSnapshot.Name, SnapshotUID: intent.SourceSnapshot.UID, ContentUID: intent.SourceContent.UID, VMUID: intent.SourceVMUID, CoreHash: intent.SourceCoreHash, DocumentHash: intent.SourceDocumentHash}
	d.TargetVMUID = intent.CurrentVM.UID
	d.FenceOwned = c.State.Fence != nil && c.State.Fence.OperationID == op
	d.FenceCleared = d.Complete && !d.FenceOwned
	d.RollbackAcked = c.Receipt.RollbackAckHash != ""
	d.OperationBindingGeneration = intent.SourceGeneration
	for _, step := range []string{RestoreCommitStep, "restore-rollback", "restore-finalize"} {
		if v := c.Receipt.Steps[step]; v.Applied {
			d.OperationBindingGeneration = v.BindingGeneration
		}
	}
	if c.Receipt.Execution != nil {
		d.ResourceFrameHash = c.Receipt.Execution.Frame.FrameHash
		if config := c.Receipt.Execution.ConfigAfter; config != nil {
			hash, err := RestoreConfigHash(*config)
			if err != nil || hash != intent.ConfigAfterHash || config.validate(scope) != nil {
				return d, ErrCorrupt
			}
			d.ConfigAfter = config
			d.SourceConfigReady = true
		}
	}
	if d.Complete {
		if !d.FenceCleared {
			return d, ErrCorrupt
		}
		d.Phase = "complete"
		switch {
		case c.Receipt.Steps["restore-rollback"].Applied:
			d.Outcome = "rolled-back"
		case d.ConfigCommitted:
			d.Outcome = "restored"
		case c.Receipt.Steps["restore-cancel-requested"].Applied:
			d.Outcome = "cancelled"
		default:
			return d, ErrCorrupt
		}
	}
	if c.State.Fence != nil && c.State.Fence.OperationID == op {
		d.Phase = c.State.Fence.Phase
	}
	return d, nil
}

// Command executes at most one bounded command; Console decides the next call.
func (e *RestoreEngine) Command(ctx context.Context, scope Scope, op, action string, request RestoreCommand) (RestoreOperationDTO, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if e == nil || e.Store == nil || !idPattern.MatchString(op) {
		return RestoreOperationDTO{}, ErrCorrupt
	}
	before, err := e.Store.Read(scope, op)
	if err != nil {
		return RestoreOperationDTO{}, err
	}
	fingerprint, hashErr := snapshotDigest(struct {
		Action  string
		Request RestoreCommand
	}{action, request})
	if hashErr != nil {
		return RestoreOperationDTO{}, ErrCorrupt
	}
	claimStep := "rpc-claim-" + strconv.FormatUint(request.ExpectedControlRevision, 36)
	sameClaim := before.Receipt != nil && before.Receipt.Steps[claimStep].Applied && before.Receipt.Steps[claimStep].RequestHash == fingerprint
	if before.State.ControlRevision != request.ExpectedControlRevision || before.State.BindingGeneration != request.ExpectedBindingGeneration {
		if !sameClaim {
			return RestoreOperationDTO{}, ErrConflict
		}
	}
	if action != "prepare" {
		epoch, err := restoreOperationEpoch(before.Receipt)
		if err != nil {
			return RestoreOperationDTO{}, err
		}
		if request.ExpectedOperationEpoch != epoch {
			return RestoreOperationDTO{}, ErrConflict
		}
	} else if request.ExpectedOperationEpoch != "" {
		epoch, err := restoreOperationEpoch(before.Receipt)
		if err != nil || epoch != request.ExpectedOperationEpoch {
			return RestoreOperationDTO{}, ErrConflict
		}
	}
	known := map[string]bool{"prepare": true, "halt": true, "sync-halted": true, "resume": true, "rollback-halt": true, "rollback-sync-halted": true, "rollback-resume": true, "ensure-rox": true, "ensure-target": true, "pin-rox": true, "pin-target": true, "stage-plan": true, "stage-job": true, "stage-pod": true, "stage-capture": true, "commit": true, "config-ack": true, "rollback": true, "rollback-ack": true, "cleanup": true, "cancel": true, "finalize": true}
	if !known[action] {
		return RestoreOperationDTO{}, ErrCorrupt
	}
	if action == "prepare" && (e.Resolver == nil || e.Source == nil || e.Resources == nil) {
		return RestoreOperationDTO{}, ErrReferenceUnavailable
	}
	storeOnly := action == "cancel" || action == "config-ack" || action == "rollback-ack" || action == "prepare"
	if !storeOnly && (e.Runtime == nil || e.Resources == nil) {
		return RestoreOperationDTO{}, ErrReferenceUnavailable
	}
	if (action == "ensure-rox" || action == "ensure-target" || action == "pin-rox" || action == "pin-target") && request.VolumeID == 0 {
		return RestoreOperationDTO{}, ErrCorrupt
	}
	if (action == "stage-plan" || action == "stage-job" || action == "stage-pod" || action == "stage-capture") && request.Mode != "inspect" && request.Mode != "copy" && request.Mode != "verify" {
		return RestoreOperationDTO{}, ErrCorrupt
	}
	if action == "commit" && request.Config == nil {
		return RestoreOperationDTO{}, ErrCorrupt
	}
	if action == "cleanup" && !idPattern.MatchString(request.ResourceKey) {
		return RestoreOperationDTO{}, ErrCorrupt
	}
	if action != "prepare" || before.Receipt != nil {
		if before.Receipt == nil || before.Receipt.Restore == nil {
			return RestoreOperationDTO{}, ErrNotFound
		}
		// The successful claim is the request's linearization point. Distinct
		// actions using one client revision share a step key but cannot share its
		// request fingerprint. Store checks the caller CAS under its SQL lock.
		expect := Expectation{ControlRevision: request.ExpectedControlRevision, BindingGeneration: request.ExpectedBindingGeneration, OperationID: op, ManifestHash: before.Receipt.ManifestHash, RequestHash: fingerprint, Step: claimStep}
		before, err = e.Store.Mutate(scope, expect, func(now Current) (Mutation, error) {
			epoch, err := restoreOperationEpoch(now.Receipt)
			if err != nil {
				return Mutation{}, err
			}
			if request.ExpectedOperationEpoch != "" && request.ExpectedOperationEpoch != epoch {
				return Mutation{}, ErrConflict
			}
			if err := restoreOwner(now, scope, expect); err != nil {
				return Mutation{}, err
			}
			return Mutation{State: now.State, Receipt: now.Receipt}, nil
		})
		if err != nil {
			return RestoreOperationDTO{}, err
		}
	}
	if action == "prepare" {
		if !validName(request.SnapshotName) || (request.PowerAfter != "previous" && request.PowerAfter != "stopped") {
			return RestoreOperationDTO{}, ErrCorrupt
		}
		_, err = e.Prepare(ctx, scope, op, request.SnapshotName, request.PowerAfter, Expectation{ControlRevision: before.State.ControlRevision, BindingGeneration: before.State.BindingGeneration})
	} else {
		if before.Receipt == nil || before.Receipt.Restore == nil {
			return RestoreOperationDTO{}, ErrNotFound
		}
		expect := Expectation{ControlRevision: before.State.ControlRevision, BindingGeneration: before.State.BindingGeneration, OperationID: op, ManifestHash: before.Receipt.ManifestHash}
		switch action {
		case "halt", "sync-halted", "resume", "rollback-halt", "rollback-sync-halted", "rollback-resume":
			_, err = e.RuntimeStep(ctx, scope, op, action)
		case "ensure-rox", "ensure-target":
			if request.VolumeID == 0 {
				return RestoreOperationDTO{}, ErrCorrupt
			}
			_, err = e.EnsureClaim(ctx, scope, op, request.VolumeID, action == "ensure-target")
		case "pin-rox", "pin-target":
			if e.Resources == nil {
				return RestoreOperationDTO{}, ErrReferenceUnavailable
			}
			prefix := "rox"
			if action == "pin-target" {
				prefix = "target"
			}
			if request.VolumeID == 0 {
				return RestoreOperationDTO{}, ErrCorrupt
			}
			_, err = e.Resources.PinBoundClaim(ctx, scope, op, prefix+strconv.FormatUint(uint64(request.VolumeID), 10))
		case "stage-plan", "stage-job":
			_, err = e.EnsureStage(ctx, scope, op, request.Mode, action == "stage-job")
		case "stage-pod":
			_, err = e.ObserveStagePod(ctx, scope, op, request.Mode)
		case "stage-capture":
			_, err = e.CaptureStage(ctx, scope, op, request.Mode)
		case "commit":
			if request.Config == nil {
				return RestoreOperationDTO{}, ErrCorrupt
			}
			_, err = e.Commit(ctx, scope, op, *request.Config)
		case "config-ack":
			expect.Step = "restore-console-ack"
			_, err = e.Store.AcknowledgeRestoreDB(scope, expect, request.CommitHash, request.DocumentHash)
		case "rollback":
			_, err = e.Rollback(ctx, scope, op)
		case "rollback-ack":
			expect.Step = "restore-rollback-ack"
			_, err = e.Store.AcknowledgeRollbackDB(scope, expect, request.CommitHash, request.DocumentHash)
		case "cleanup":
			if !idPattern.MatchString(request.ResourceKey) {
				return RestoreOperationDTO{}, ErrCorrupt
			}
			err = e.CleanupObject(ctx, scope, op, request.ResourceKey, request.Cancelled)
		case "cancel":
			if before.Receipt.Steps[RestoreCommitStep].Applied {
				return RestoreOperationDTO{}, ErrConflict
			}
			_, err = e.Store.executionUpdate(scope, op, "restore-cancel-requested", true, func(c Current) (Mutation, error) { return Mutation{State: c.State, Receipt: c.Receipt}, nil })
		case "finalize":
			_, err = e.Finalize(ctx, scope, op, request.Rollback, request.Cancelled)
		default:
			return RestoreOperationDTO{}, ErrCorrupt
		}
	}
	d, statusErr := e.Status(scope, op)
	if err != nil {
		return d, err
	}
	if statusErr != nil {
		return d, statusErr
	}
	d.Applied = true
	return d, nil
}
