package vmrestore

import (
	"context"
	"reflect"
	"strconv"
	"time"

	"github.com/goodrain/rainbond/pkg/vmrestore/copyio"
	"github.com/goodrain/rainbond/pkg/vmrestore/restoreplan"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RestorePreparation derives from the sealed native A manifest and current
// platform projection. No raw VM, bootstrap payload, or secret is persisted.
type RestorePreparation struct {
	Intent                       RestoreIntent
	Config                       RestoreConfig
	Frame                        RestoreResourcePlan
	NetworkHash, IdentityHash    string
	BeforeStartup, SourceStartup RestoreStartupPolicy
}

// Rollback rechecks stopped writers and every retained B PVC immediately before
// the existing atomic configuration/binding rollback transaction.
func (e *RestoreEngine) Rollback(ctx context.Context, scope Scope, op string) (Current, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c, err := e.read(ctx, scope, op)
	if err != nil {
		return c, err
	}
	if c.Receipt.Steps["restore-rollback"].Applied {
		return c, nil
	}
	if c.State.Fence.Phase != "rollback-stopped" || !c.Receipt.Steps["restore-rollback-writers-clear"].Applied {
		return c, ErrConflict
	}
	o, err := e.Runtime.Observe(ctx, e.runtimeRequest(c, "rollback-halt"))
	if err != nil {
		return c, err
	}
	if !validRuntimeObservation(c, "rollback-halt", o) {
		return c, ErrConflict
	}
	if !o.Halted || !o.WritersClear {
		return c, ErrRestorePending
	}
	for _, b := range c.Receipt.OldBindings {
		claim, err := e.Resources.Core.CoreV1().PersistentVolumeClaims(scope.Namespace).Get(ctx, b.ActivePVC.Name, metav1.GetOptions{})
		if err != nil {
			return c, ErrReferenceUnavailable
		}
		if CheckLivePVC(b.ActivePVC, claim.Namespace, claim.Name, string(claim.UID)) != nil || claim.DeletionTimestamp != nil || claim.Status.Phase != corev1.ClaimBound {
			return c, ErrConflict
		}
	}
	expect := Expectation{ControlRevision: c.State.ControlRevision, BindingGeneration: c.State.BindingGeneration, OperationID: op, ManifestHash: c.Receipt.ManifestHash, Step: "restore-rollback"}
	return e.Store.RollbackRestoreDB(scope, expect)
}

func (p RestorePreparation) validate(scope Scope, op, snapshot, power string) error {
	hash, err := RestoreConfigHash(p.Config)
	if err != nil || p.Config.validate(scope) != nil || p.Intent.validate(scope) != nil || hash != p.Intent.ConfigAfterHash || p.Frame.validate() != nil || p.Frame.Scope != scope || p.Frame.OperationID != op || p.Intent.SourceSnapshot.Name != snapshot || p.Intent.PowerAfter != power || p.Frame.SourceGeneration != p.Intent.SourceGeneration || !hashPattern.MatchString(p.NetworkHash) || !hashPattern.MatchString(p.IdentityHash) {
		return ErrCorrupt
	}
	writable := map[uint]bool{}
	for _, disk := range p.Config.Hardware.DiskLayout {
		if !disk.ReadOnly {
			writable[disk.VolumeID] = true
		}
	}
	if len(writable) != len(p.Frame.Disks) {
		return ErrConflict
	}
	for _, disk := range p.Frame.Disks {
		if !writable[disk.VolumeID] {
			return ErrConflict
		}
	}
	x := p.execution()
	if x.validate(scope, op, p.Frame.CoreHash) != nil {
		return ErrCorrupt
	}
	return nil
}

func (p RestorePreparation) execution() RestoreExecution {
	return RestoreExecution{ConfigAfter: &p.Config, Frame: p.Frame, Intents: map[string]RestoreObjectIntent{}, Evidence: map[string]restoreplan.CopyEvidence{}, NetworkHash: p.NetworkHash, IdentityHash: p.IdentityHash, BeforeStartup: p.BeforeStartup, SourceStartup: p.SourceStartup}
}

// RestorePreparationProvider is supplied by snapshot/lifecycle integration. It
// verifies native Content/CSI chain and A stable identity, not a UI's asserted hash.
type RestorePreparationProvider interface {
	Read(context.Context, Scope, string, string, string) (RestorePreparation, error)
}

// RestoreCapacityProvider takes a trusted inspect report and holds a provider
// reservation until cleanup/finalize. Zero/default capability rejects execution.
type RestoreCapacityProvider interface {
	Reserve(context.Context, Scope, string, RestoreResourcePlan, copyio.Report) (restoreplan.Capability, restoreplan.SpaceEvidence, error)
	Check(context.Context, Scope, string, restoreplan.Budget) error
	Release(context.Context, Scope, string) error
}

// RestoreEngine only serves explicit bounded Console commands. It never scans
// tasks or advances background work independently of the one Console orchestrator.
type RestoreEngine struct {
	Store     *Store
	Resolver  *Resolver
	Runtime   RestoreRuntime
	Resources *RestoreResourceExecutor
	Capture   *RestoreCapture
	Source    RestorePreparationProvider
	Capacity  RestoreCapacityProvider
}

// Prepare materializes B through the unique live Resolver, including detached
// disks. Initial canonical identity is never invented from names on rollback.
func (e *RestoreEngine) Prepare(ctx context.Context, scope Scope, op, snapshot, power string, guarded ...Expectation) (Current, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if len(guarded) > 1 {
		return Current{}, ErrCorrupt
	}
	if e == nil || e.Store == nil {
		return Current{}, ErrCorrupt
	}
	if e.Resolver == nil || e.Source == nil || e.Resources == nil {
		return Current{}, ErrReferenceUnavailable
	}
	c, err := e.Store.Read(scope, op)
	if err != nil {
		return c, err
	}
	if c.Receipt != nil {
		if c.Receipt.Restore == nil || c.Receipt.Restore.SourceSnapshot.Name != snapshot || c.Receipt.Restore.PowerAfter != power {
			return c, ErrConflict
		}
		if c.Receipt.Execution != nil {
			return c, nil
		}
		// Recover the DB-prepare/frame gap without selecting different B identities.
		p, sourceErr := e.Source.Read(ctx, scope, op, snapshot, power)
		if sourceErr != nil {
			return c, sourceErr
		}
		if err := p.validate(scope, op, snapshot, power); err != nil {
			return c, err
		}
		if !reflect.DeepEqual(c.Receipt.Restore, &p.Intent) || p.Frame.CoreHash != c.Receipt.ManifestHash {
			return c, ErrConflict
		}
		return e.Store.InitializeRestoreExecution(scope, op, p.execution())
	}
	p, err := e.Source.Read(ctx, scope, op, snapshot, power)
	if err != nil {
		return c, err
	}
	if err := p.validate(scope, op, snapshot, power); err != nil {
		return c, err
	}
	if err = e.Resources.Certification.validate(p.Frame); err != nil {
		return c, err
	}
	view, err := e.Resolver.ReadBindingView(ctx, scope)
	if err != nil {
		return c, err
	}
	if view.BindingGeneration != p.Intent.SourceGeneration || view.Fence != nil || view.MutationInProgress {
		return c, ErrBusy
	}
	old := map[string]Binding{}
	for _, volume := range view.Volumes {
		b, bindErr := volume.MaterializedBinding()
		if bindErr != nil {
			return c, bindErr
		}
		old[strconv.FormatUint(uint64(volume.VolumeID), 10)] = b
	}
	for _, d := range p.Frame.Disks {
		b, ok := old[strconv.FormatUint(uint64(d.VolumeID), 10)]
		if !ok || !sameObject(b.ActivePVC, d.PreviousPVC) {
			return c, ErrConflict
		}
	}
	exp := Expectation{ControlRevision: view.ControlRevision, BindingGeneration: view.BindingGeneration, OperationID: op, ManifestHash: p.Frame.CoreHash, Step: "restore-prepare"}
	if len(guarded) == 1 {
		exp.ControlRevision = guarded[0].ControlRevision
		exp.BindingGeneration = guarded[0].BindingGeneration
	}
	c, err = e.Store.PrepareRestoreDB(scope, exp, p.Intent, old)
	if err != nil {
		return c, err
	}
	return e.Store.InitializeRestoreExecution(scope, op, p.execution())
}
func (e *RestoreEngine) read(ctx context.Context, scope Scope, op string) (Current, error) {
	if e == nil || e.Store == nil {
		return Current{}, ErrCorrupt
	}
	if e.Runtime == nil || e.Resources == nil {
		return Current{}, ErrReferenceUnavailable
	}
	return e.Resources.current(ctx, scope, op)
}
func (e *RestoreEngine) runtimeRequest(c Current, action string) RestoreRuntimeRequest {
	intent := c.Receipt.Restore
	hash := intent.ConfigAfterHash
	if action == "halt" || action == "rollback-halt" {
		hash, _ = RestoreConfigHash(intent.Before)
	}
	if action == "rollback-sync-halted" || action == "rollback-resume" {
		hash, _ = RestoreConfigHash(intent.Before)
	}
	startup := c.Receipt.Execution.SourceStartup
	if action == "halt" || action == "rollback-halt" || action == "rollback-sync-halted" || action == "rollback-resume" {
		startup = c.Receipt.Execution.BeforeStartup
	}
	return RestoreRuntimeRequest{Startup: startup, Scope: c.Receipt.Scope, OperationID: c.Receipt.OperationID, CoreHash: c.Receipt.ManifestHash, Action: action, IntentID: c.Receipt.OperationID + "-" + action, BindingGeneration: c.State.BindingGeneration, VM: intent.CurrentVM, ConfigHash: hash}
}
func runtimeReady(action string, o RestoreRuntimeObservation) bool {
	switch action {
	case "halt", "rollback-halt":
		return o.Halted && o.WritersClear
	case "sync-halted", "rollback-sync-halted":
		return o.Halted && o.WritersClear && o.DesiredMatches && o.ObservedGeneration >= o.VMGeneration
	case "resume", "rollback-resume":
		return o.Running && o.DesiredMatches && o.ObservedGeneration >= o.VMGeneration
	}
	return false
}
func validRuntimeObservation(c Current, action string, o RestoreRuntimeObservation) bool {
	if o.BindingGeneration != c.State.BindingGeneration || !sameObject(o.VM, c.Receipt.Restore.CurrentVM) || o.NetworkHash != c.Receipt.Execution.NetworkHash {
		return false
	}
	if (action == "sync-halted" || action == "resume") && o.DesiredMatches {
		return o.IdentityHash == c.Receipt.Execution.IdentityHash
	}
	if (action == "rollback-sync-halted" || action == "rollback-resume") && o.DesiredMatches {
		hash, _ := snapshotDigest(c.Receipt.Execution.BeforeStartup)
		return o.IdentityHash == hash
	}
	return true
}

// RuntimeStep seals intent before any mutation and reconciles observations after
// a crash. Unknown outcomes do not release either the fence or action intent.
func (e *RestoreEngine) RuntimeStep(ctx context.Context, scope Scope, op, action string) (Current, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c, err := e.read(ctx, scope, op)
	if err != nil {
		return c, err
	}
	step := "restore-" + action
	if action == "halt" {
		step = "restore-writers-clear"
	}
	if action == "rollback-halt" {
		step = "restore-rollback-writers-clear"
	}
	if c.Receipt.Steps[step].Applied {
		return c, nil
	}
	phase := c.State.Fence.Phase
	allowed := map[string]bool{"halt": phase == "prepared", "sync-halted": phase == "console-acked", "resume": phase == "synced", "rollback-halt": phase == "region-committed" || phase == "console-acked" || phase == "synced" || phase == "resumed", "rollback-sync-halted": phase == "rollback-console-acked", "rollback-resume": phase == "rollback-synced"}
	if !allowed[action] {
		return c, ErrConflict
	}
	if action == "resume" || action == "rollback-resume" {
		if !c.Receipt.Restore.WasRunning || c.Receipt.Restore.PowerAfter != "previous" {
			return c, ErrConflict
		}
		for _, v := range c.Receipt.Execution.Intents {
			if v.Purpose != "restore-target" && !v.Deleted {
				return c, ErrRestorePending
			}
		}
	}
	request := e.runtimeRequest(c, action)
	if _, err = e.Store.executionUpdate(scope, op, "runtime-intent-"+action, request, func(c Current) (Mutation, error) { return Mutation{State: c.State, Receipt: c.Receipt}, nil }); err != nil {
		return c, err
	}
	c, err = e.read(ctx, scope, op)
	if err != nil {
		return c, err
	}
	o, err := e.Runtime.Observe(ctx, request)
	if err != nil {
		return c, err
	}
	if !validRuntimeObservation(c, action, o) {
		return c, ErrConflict
	}
	if !runtimeReady(action, o) {
		if (action == "sync-halted" || action == "rollback-sync-halted") && (!o.Halted || !o.WritersClear) {
			return c, ErrRestorePending
		}
		if (action == "resume" || action == "rollback-resume") && !o.DesiredMatches {
			return c, ErrConflict
		}
		// Recheck exact owner after observation and before a side effect.
		if _, err = e.read(ctx, scope, op); err != nil {
			return c, err
		}
		issued, issueErr := e.Store.executionUpdate(scope, op, "runtime-issued-"+action, request, func(c Current) (Mutation, error) { return Mutation{State: c.State, Receipt: c.Receipt}, nil })
		if issueErr != nil {
			return c, issueErr
		}
		if issued.Replayed {
			return c, ErrRestoreUnknown
		}
		if _, err = e.Runtime.Apply(ctx, request); err != nil {
			return c, ErrRestoreUnknown
		}
		return c, ErrRestorePending
	}
	step = "restore-" + action
	next := map[string]string{"halt": "stopped", "sync-halted": "synced", "resume": "resumed", "rollback-halt": "rollback-stopped", "rollback-sync-halted": "rollback-synced", "rollback-resume": "rollback-resumed"}[action]
	if action == "halt" {
		step = "restore-writers-clear"
	}
	if action == "rollback-halt" {
		step = "restore-rollback-writers-clear"
	}
	return e.Store.executionUpdate(scope, op, step, o, func(c Current) (Mutation, error) {
		c.State.Fence.Phase = next
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
}

// AssessCopy uses persisted inspect evidence; callers cannot assert a write budget.
func (e *RestoreEngine) AssessCopy(ctx context.Context, scope Scope, op string) (restoreplan.Budget, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c, err := e.read(ctx, scope, op)
	if err != nil {
		return restoreplan.Budget{}, err
	}
	if e.Capacity == nil {
		return restoreplan.Budget{}, ErrRestoreCapability
	}
	evidence, ok := c.Receipt.Execution.Evidence["inspect"]
	if !ok || evidence.Report == nil || !evidence.Report.OK || evidence.ParseCode != "" || !c.Receipt.Steps["restore-stage-inspect-verified"].Applied {
		return restoreplan.Budget{}, ErrConflict
	}
	cap, space, err := e.Capacity.Reserve(ctx, scope, op, c.Receipt.Execution.Frame, *evidence.Report)
	if err != nil {
		return restoreplan.Budget{}, err
	}
	inputs := []restoreplan.DiskInput{}
	for index, d := range c.Receipt.Execution.Frame.Disks {
		if index >= len(evidence.Report.Disks) {
			return restoreplan.Budget{}, ErrCorrupt
		}
		report := evidence.Report.Disks[index]
		if report.VolumeID != d.VolumeID {
			return restoreplan.Budget{}, ErrConflict
		}
		inputs = append(inputs, restoreplan.DiskInput{VolumeID: d.VolumeID, BackupRequestBytes: uint64(d.ROXRequestBytes), SnapshotRestoreBytes: uint64(d.ROXRequestBytes), Source: report.Source, Previous: report.PreviousBefore})
	}
	budget, err := restoreplan.Assess(time.Now(), op, cap, space, inputs)
	if err != nil {
		return budget, err
	}
	for i, d := range budget.Disks {
		if d.PVCRequestBytes != uint64(c.Receipt.Execution.Frame.Disks[i].TargetRequestBytes) {
			return restoreplan.Budget{}, ErrConflict
		}
	}
	return budget, nil
}

// SealStage only runs after CaptureAndValidate has persisted and validated the
// complete exact-UID payload. A public bool or Job Succeeded cannot seal a stage.
func (e *RestoreEngine) SealStage(ctx context.Context, scope Scope, op string, plan copyio.Plan, job, pod ObjectRef) (Current, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c, err := e.read(ctx, scope, op)
	if err != nil {
		return c, err
	}
	if e.Capture == nil {
		return c, ErrCorrupt
	}
	report, err := e.Capture.Capture(ctx, scope, op, plan, job, pod)
	if err != nil {
		return c, err
	}
	if plan.Mode == "verify" {
		c, err = e.read(ctx, scope, op)
		if err != nil {
			return c, err
		}
		for _, mode := range []string{"inspect", "copy"} {
			v, ok := c.Receipt.Execution.Evidence[mode]
			if !ok || v.Report == nil || !c.Receipt.Steps["restore-stage-"+mode+"-verified"].Applied {
				return c, ErrConflict
			}
			if len(v.Report.Disks) != len(report.Disks) {
				return c, ErrConflict
			}
			for i, d := range report.Disks {
				before := v.Report.Disks[i]
				if d.VolumeID != before.VolumeID || d.Source.SHA256 != before.Source.SHA256 || d.Source.LogicalBytes != before.Source.LogicalBytes || d.PreviousBefore.SHA256 != before.PreviousBefore.SHA256 || d.PreviousBefore.LogicalBytes != before.PreviousBefore.LogicalBytes {
					return c, copyio.ErrDigest
				}
			}
		}
	}
	return e.Store.executionUpdate(scope, op, "restore-stage-"+plan.Mode+"-verified", struct {
		Plan   string
		Report copyio.Report
	}{plan.PlanHash, report}, func(c Current) (Mutation, error) { return Mutation{State: c.State, Receipt: c.Receipt}, nil })
}

// Commit requires fresh stopped/writer-free identities immediately before the
// existing 5b atomic transaction; it never writes selected fields one disk at a time.
func (e *RestoreEngine) Commit(ctx context.Context, scope Scope, op string, config RestoreConfig) (Current, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c, err := e.read(ctx, scope, op)
	if err != nil {
		return c, err
	}
	if !c.Receipt.Steps["restore-stage-verify-verified"].Applied {
		return c, ErrConflict
	}
	if e.Source == nil {
		return c, ErrRestoreCapability
	}
	source, err := e.Source.Read(ctx, scope, op, c.Receipt.Restore.SourceSnapshot.Name, c.Receipt.Restore.PowerAfter)
	if err != nil {
		return c, err
	}
	if source.validate(scope, op, c.Receipt.Restore.SourceSnapshot.Name, c.Receipt.Restore.PowerAfter) != nil || !reflect.DeepEqual(&source.Intent, c.Receipt.Restore) || source.Frame.FrameHash != c.Receipt.Execution.Frame.FrameHash {
		return c, ErrConflict
	}
	// Revalidate every native source and actual ROX/target/B UID near commit.
	for _, d := range c.Receipt.Execution.Frame.Disks {
		if err = e.Resources.VerifySource(ctx, scope, op, d.VolumeID); err != nil {
			return c, err
		}
		before, getErr := e.Resources.Core.CoreV1().PersistentVolumeClaims(scope.Namespace).Get(ctx, d.PreviousPVC.Name, metav1.GetOptions{})
		if getErr != nil {
			return c, ErrReferenceUnavailable
		}
		if CheckLivePVC(d.PreviousPVC, before.Namespace, before.Name, string(before.UID)) != nil || before.DeletionTimestamp != nil {
			return c, ErrConflict
		}
		for _, prefix := range []string{"rox", "target"} {
			if _, err = e.Resources.PinBoundClaim(ctx, scope, op, prefix+strconv.FormatUint(uint64(d.VolumeID), 10)); err != nil {
				return c, err
			}
		}
	}
	o, err := e.Runtime.Observe(ctx, e.runtimeRequest(c, "halt"))
	if err != nil {
		return c, err
	}
	if !sameObject(o.VM, c.Receipt.Restore.CurrentVM) || !o.Halted || !o.WritersClear || o.BindingGeneration != c.State.BindingGeneration || o.NetworkHash != c.Receipt.Execution.NetworkHash {
		return c, ErrConflict
	}
	targets := map[uint]ObjectRef{}
	for _, d := range c.Receipt.Execution.Frame.Disks {
		v, ok := c.Receipt.Execution.Intents["target"+strconv.FormatUint(uint64(d.VolumeID), 10)]
		if !ok || v.Object.UID == "" || v.Deleted || v.DeleteRequested {
			return c, ErrConflict
		}
		targets[d.VolumeID] = v.Object
	}
	bindings, err := BuildRestoreBindings(scope, op, c.Receipt.Restore.SourceSnapshot.UID, c.Receipt.OldBindings, config, targets)
	if err != nil {
		return c, err
	}
	// This final seal is produced by the engine only after all three persisted stages.
	if !c.Receipt.Steps["restore-copy-verified"].Applied {
		c, err = e.Store.executionUpdate(scope, op, "restore-copy-verified", struct {
			Observation RestoreRuntimeObservation
			Bindings    map[string]Binding
		}{o, bindings}, func(c Current) (Mutation, error) {
			if !c.Receipt.Steps["restore-stage-inspect-verified"].Applied || !c.Receipt.Steps["restore-stage-copy-verified"].Applied || !c.Receipt.Steps["restore-stage-verify-verified"].Applied {
				return Mutation{}, ErrConflict
			}
			c.State.Fence.Phase = "copy-verified"
			return Mutation{State: c.State, Receipt: c.Receipt}, nil
		})
		if err != nil {
			return c, err
		}
	}
	exp := Expectation{ControlRevision: c.State.ControlRevision, BindingGeneration: c.State.BindingGeneration, OperationID: op, ManifestHash: c.Receipt.ManifestHash, Step: RestoreCommitStep}
	return e.Store.CommitRestoreDB(scope, exp, RestoreCommitRequest{Config: config, Bindings: bindings})
}

// Finalize releases only after ACK, observed desired/power and every temporary
// object/PV deletion. Old B rollback bindings remain protected in the receipt.
func (e *RestoreEngine) Finalize(ctx context.Context, scope Scope, op string, rollback, cancelled bool) (Current, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c, err := e.read(ctx, scope, op)
	if err != nil {
		return c, err
	}
	phase := c.State.Fence.Phase
	if cancelled {
		if c.Receipt.Steps[RestoreCommitStep].Applied || phase != "stopped" || !c.Receipt.Steps["restore-cancel-requested"].Applied {
			return c, ErrConflict
		}
	} else if rollback {
		if c.Receipt.RollbackAckHash == "" || (phase != "rollback-synced" && phase != "rollback-resumed") {
			return c, ErrConflict
		}
	} else {
		if c.Receipt.ConsoleAckHash == "" || (phase != "synced" && phase != "resumed") {
			return c, ErrConflict
		}
	}
	if rollback && cancelled {
		return c, ErrConflict
	}
	if !cancelled {
		wantRunning := c.Receipt.Restore.WasRunning && c.Receipt.Restore.PowerAfter == "previous"
		resumed := phase == "resumed" || phase == "rollback-resumed"
		if wantRunning != resumed {
			return c, ErrConflict
		}
	}
	for _, v := range c.Receipt.Execution.Intents {
		if v.Purpose == "restore-target" && !cancelled && !rollback {
			continue
		}
		if !v.Deleted {
			return c, ErrRestorePending
		}
	}
	action := "sync-halted"
	if cancelled {
		action = "halt"
	}
	if phase == "resumed" {
		action = "resume"
	}
	if rollback {
		action = "rollback-sync-halted"
		if phase == "rollback-resumed" {
			action = "rollback-resume"
		}
	}
	o, err := e.Runtime.Observe(ctx, e.runtimeRequest(c, action))
	if err != nil {
		return c, err
	}
	if !validRuntimeObservation(c, action, o) || !runtimeReady(action, o) {
		return c, ErrConflict
	}
	if e.Capacity == nil {
		return c, ErrRestoreCapability
	}
	if _, err = e.Store.executionUpdate(scope, op, "capacity-release-intent", op, func(c Current) (Mutation, error) { return Mutation{State: c.State, Receipt: c.Receipt}, nil }); err != nil {
		return c, err
	}
	if err = e.Capacity.Release(ctx, scope, op); err != nil {
		return c, ErrRestoreUnknown
	}
	if _, err = e.Store.executionUpdate(scope, op, "capacity-released", op, func(c Current) (Mutation, error) { return Mutation{State: c.State, Receipt: c.Receipt}, nil }); err != nil {
		return c, err
	}
	return e.Store.executionUpdate(scope, op, "restore-finalize", struct {
		Rollback, Cancelled bool
		Observation         RestoreRuntimeObservation
	}{rollback, cancelled, o}, func(c Current) (Mutation, error) {
		c.Receipt.SnapshotReferenceReleased = true
		c.State.Fence = nil
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
}
