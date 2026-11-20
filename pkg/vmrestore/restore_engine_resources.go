package vmrestore

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/goodrain/rainbond/pkg/vmrestore/copyio"
	"github.com/goodrain/rainbond/pkg/vmrestore/restoreplan"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WireRestoreResourceGuard is called once by the integration factory before
// exposing commands. Its writer check immediately precedes every create/delete.
func (e *RestoreEngine) WireRestoreResourceGuard() {
	e.Resources.BeforeMutation = func(ctx context.Context, scope Scope, op, key string) error {
		c, err := e.read(ctx, scope, op)
		if err != nil {
			return err
		}
		o, err := e.Runtime.Observe(ctx, e.runtimeRequest(c, "halt"))
		if err != nil {
			return err
		}
		// Cancelling may stop only our sealed Job while it is still a writer.
		if strings.HasPrefix(key, "delete:job-") && c.Receipt.Steps["restore-cancel-requested"].Applied {
			return nil
		}
		if !sameObject(o.VM, c.Receipt.Restore.CurrentVM) || o.NetworkHash != c.Receipt.Execution.NetworkHash || o.BindingGeneration != c.State.BindingGeneration || !o.Halted || !o.WritersClear {
			return ErrRestorePending
		}
		return nil
	}
}

// EnsureClaim issues exactly one ROX or blank-target create per request. Before
// targets, a fresh held reservation must cover persisted real inspect bytes.
func (e *RestoreEngine) EnsureClaim(ctx context.Context, scope Scope, op string, volumeID uint, target bool) (ObjectRef, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c, err := e.read(ctx, scope, op)
	if err != nil {
		return ObjectRef{}, err
	}
	if c.State.Fence.Phase != "stopped" {
		return ObjectRef{}, ErrConflict
	}
	if err = e.Resources.VerifySource(ctx, scope, op, volumeID); err != nil {
		return ObjectRef{}, err
	}
	rox, blank, err := RestoreClaims(c.Receipt.Execution.Frame, volumeID)
	if err != nil {
		return ObjectRef{}, err
	}
	key := "rox" + strconv.FormatUint(uint64(volumeID), 10)
	purpose := "restore-rox"
	claim := rox
	if target {
		budget, budgetErr := e.AssessCopy(ctx, scope, op)
		if budgetErr != nil {
			return ObjectRef{}, budgetErr
		}
		if err = e.Capacity.Check(ctx, scope, op, budget); err != nil {
			return ObjectRef{}, err
		}
		key = "target" + strconv.FormatUint(uint64(volumeID), 10)
		purpose = "restore-target"
		claim = blank
	}
	return e.Resources.Ensure(ctx, scope, op, key, purpose, "PersistentVolumeClaim", claim)
}
func (e *RestoreEngine) stagePlan(ctx context.Context, c Current, mode string) (copyio.Plan, []RestoreMounts, error) {
	frame := c.Receipt.Execution.Frame
	p := copyio.Plan{Protocol: copyio.Protocol, OperationID: frame.OperationID, ManifestHash: frame.CoreHash, FrameHash: frame.FrameHash, Mode: mode, DeadlineSeconds: frame.DeadlineSeconds, TargetUID: frame.TargetUID, TargetGID: frame.TargetGID}
	mounts := []RestoreMounts{}
	var budget restoreplan.Budget
	if mode != "inspect" {
		var err error
		budget, err = e.AssessCopy(ctx, c.Receipt.Scope, c.Receipt.OperationID)
		if err != nil {
			return p, nil, err
		}
		if err = e.Capacity.Check(ctx, c.Receipt.Scope, c.Receipt.OperationID, budget); err != nil {
			return p, nil, err
		}
	}
	for index, d := range frame.Disks {
		rox, ok := c.Receipt.Execution.Intents["rox"+strconv.FormatUint(uint64(d.VolumeID), 10)]
		if !ok || rox.Object.UID == "" || rox.BoundPV == nil || rox.DeleteRequested {
			return p, nil, ErrRestorePending
		}
		disk := copyio.DiskPlan{VolumeID: d.VolumeID, SourcePVCUID: rox.Object.UID, PreviousPVCUID: d.PreviousPVC.UID, SourceMaxBytes: d.ROXRequestBytes, PreviousMaxBytes: d.PreviousMaxBytes}
		mount := RestoreMounts{VolumeID: d.VolumeID, ROX: rox.Object, Previous: d.PreviousPVC}
		if mode != "inspect" {
			if index >= len(budget.Disks) || budget.Disks[index].VolumeID != d.VolumeID {
				return p, nil, ErrConflict
			}
			target, ok := c.Receipt.Execution.Intents["target"+strconv.FormatUint(uint64(d.VolumeID), 10)]
			if !ok || target.Object.UID == "" || target.BoundPV == nil || target.DeleteRequested {
				return p, nil, ErrRestorePending
			}
			b := budget.Disks[index]
			disk.TargetPVCUID = target.Object.UID
			disk.ExpectedSourceBytes = int64(b.SourceBytes)
			disk.ExpectedSourceSHA256 = b.SourceSHA256
			disk.ExpectedPreviousBytes = int64(b.PreviousBytes)
			disk.ExpectedPreviousSHA256 = b.PreviousSHA256
			disk.WriteLimitBytes = int64(b.WriteLimitBytes)
			targetRef := target.Object
			mount.Target = &targetRef
		}
		p.Disks = append(p.Disks, disk)
		mounts = append(mounts, mount)
	}
	p, err := copyio.SealPlan(p)
	return p, mounts, err
}

// EnsureStage creates either the immutable plan ConfigMap or the one serial Job.
// Previous stage Job/Pod deletion must be observed before a new stage mounts disks.
func (e *RestoreEngine) EnsureStage(ctx context.Context, scope Scope, op, mode string, job bool) (ObjectRef, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c, err := e.read(ctx, scope, op)
	if err != nil {
		return ObjectRef{}, err
	}
	if c.State.Fence.Phase != "stopped" {
		return ObjectRef{}, ErrConflict
	}
	if mode != "inspect" && mode != "copy" && mode != "verify" {
		return ObjectRef{}, ErrCorrupt
	}
	previous := ""
	if mode == "copy" {
		previous = "inspect"
	}
	if mode == "verify" {
		previous = "copy"
	}
	if previous != "" {
		if !c.Receipt.Steps["restore-stage-"+previous+"-verified"].Applied {
			return ObjectRef{}, ErrConflict
		}
		for _, key := range []string{"job-" + previous, "pod-" + previous} {
			v, ok := c.Receipt.Execution.Intents[key]
			if !ok || !v.Deleted {
				return ObjectRef{}, ErrRestorePending
			}
		}
	}
	for _, d := range c.Receipt.Execution.Frame.Disks {
		if err = e.Resources.VerifySource(ctx, scope, op, d.VolumeID); err != nil {
			return ObjectRef{}, err
		}
	}
	plan, mounts, err := e.stagePlan(ctx, c, mode)
	if err != nil {
		return ObjectRef{}, err
	}
	desired, cm, err := RestoreCopyJob(c.Receipt.Execution.Frame, plan, mounts)
	if err != nil {
		return ObjectRef{}, err
	}
	if !job {
		return e.Resources.Ensure(ctx, scope, op, "plan-"+mode, "restore-plan", "ConfigMap", cm)
	}
	cmIntent, ok := c.Receipt.Execution.Intents["plan-"+mode]
	if !ok || cmIntent.Object.UID == "" || cmIntent.DeleteRequested {
		return ObjectRef{}, ErrConflict
	}
	return e.Resources.Ensure(ctx, scope, op, "job-"+mode, "restore-"+mode, "Job", desired)
}

// ObserveStagePod seals the single actual controlled Pod UID before reading logs.
// A replacement Pod cannot reuse an earlier result or cleanup permission.
func (e *RestoreEngine) ObserveStagePod(ctx context.Context, scope Scope, op, mode string) (ObjectRef, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c, err := e.read(ctx, scope, op)
	if err != nil {
		return ObjectRef{}, err
	}
	v, ok := c.Receipt.Execution.Intents["job-"+mode]
	if (mode != "inspect" && mode != "copy" && mode != "verify") || c.State.Fence.Phase != "stopped" || !ok || v.Object.UID == "" || v.DeleteRequested {
		return ObjectRef{}, ErrConflict
	}
	job, err := e.Resources.Core.BatchV1().Jobs(scope.Namespace).Get(ctx, v.Object.Name, metav1.GetOptions{})
	if err != nil {
		return ObjectRef{}, ErrReferenceUnavailable
	}
	if string(job.UID) != v.Object.UID || job.DeletionTimestamp != nil {
		return ObjectRef{}, ErrConflict
	}
	var pod *corev1.Pod
	options := metav1.ListOptions{Limit: 200}
	seen := map[string]bool{}
	for {
		list, err := e.Resources.Core.CoreV1().Pods(scope.Namespace).List(ctx, options)
		if err != nil {
			return ObjectRef{}, ErrReferenceUnavailable
		}
		for i := range list.Items {
			candidate := &list.Items[i]
			owned := candidate.Labels["batch.kubernetes.io/controller-uid"] == v.Object.UID || candidate.Labels["controller-uid"] == v.Object.UID
			for _, owner := range candidate.OwnerReferences {
				owned = owned || (owner.Kind == "Job" && string(owner.UID) == v.Object.UID)
			}
			if owned {
				if pod != nil {
					return ObjectRef{}, ErrRestorePending
				}
				pod = candidate.DeepCopy()
			}
		}
		if list.Continue == "" {
			break
		}
		if seen[list.Continue] || len(seen) >= 256 {
			return ObjectRef{}, ErrReferenceUnavailable
		}
		seen[list.Continue] = true
		options.Continue = list.Continue
	}
	if pod == nil || pod.DeletionTimestamp != nil {
		return ObjectRef{}, ErrRestorePending
	}
	owner := metav1.GetControllerOf(pod)
	if owner == nil || owner.UID != job.UID || owner.Kind != "Job" || owner.Name != job.Name || pod.Annotations[RestoreFrameAnnotation] != c.Receipt.Execution.Frame.FrameHash {
		return ObjectRef{}, ErrConflict
	}
	ref := refOf("Pod", pod)
	key := "pod-" + mode
	if prior, ok := c.Receipt.Execution.Intents[key]; ok {
		if !sameObject(prior.Object, ref) {
			return ObjectRef{}, ErrConflict
		}
		return ref, nil
	}
	hash, _ := snapshotDigest(struct{ Job, Pod ObjectRef }{v.Object, ref})
	intent := RestoreObjectIntent{Key: key, Nonce: v.Nonce, DesiredHash: hash, Object: ref, Purpose: "restore-" + mode + "-pod"}
	if _, err = e.Store.resourceIntent(scope, op, intent); err != nil {
		return ObjectRef{}, err
	}
	_, err = e.Store.resourceObserved(scope, op, key, ref, nil)
	return ref, err
}

// CaptureStage derives the plan and identities from receipts, never a request body.
func (e *RestoreEngine) CaptureStage(ctx context.Context, scope Scope, op, mode string) (Current, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c, err := e.read(ctx, scope, op)
	if err != nil {
		return c, err
	}
	plan, _, err := e.stagePlan(ctx, c, mode)
	if err != nil {
		return c, err
	}
	job, ok := c.Receipt.Execution.Intents["job-"+mode]
	if !ok {
		return c, ErrConflict
	}
	pod, ok := c.Receipt.Execution.Intents["pod-"+mode]
	if !ok {
		return c, ErrConflict
	}
	return e.SealStage(ctx, scope, op, plan, job.Object, pod.Object)
}

// CleanupObject requires result persistence before Job/Pod deletion; failed jobs
// need an explicit cancel path that preserves their typed failure evidence first.
func (e *RestoreEngine) CleanupObject(ctx context.Context, scope Scope, op, key string, cancelled bool) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c, err := e.read(ctx, scope, op)
	if err != nil {
		return err
	}
	if cancelled {
		if c.Receipt.Steps[RestoreCommitStep].Applied {
			return ErrConflict
		}
		c, err = e.Store.executionUpdate(scope, op, "restore-cancel-requested", true, func(c Current) (Mutation, error) { return Mutation{State: c.State, Receipt: c.Receipt}, nil })
		if err != nil {
			return err
		}
	}
	v, ok := c.Receipt.Execution.Intents[key]
	if !ok {
		return ErrConflict
	}
	if v.Object.Kind == "Job" || v.Object.Kind == "Pod" {
		mode := ""
		for _, candidate := range []string{"inspect", "copy", "verify"} {
			if v.Purpose == "restore-"+candidate || v.Purpose == "restore-"+candidate+"-pod" {
				mode = candidate
			}
		}
		if mode == "" {
			return ErrConflict
		}
		if _, ok := c.Receipt.Execution.Evidence[mode]; !ok {
			if !cancelled {
				return ErrConflict
			}
			// Persist typed cancellation outcome before deleting a running Job/Pod.
			jobIdentity := c.Receipt.Execution.Intents["job-"+mode].Object
			podIdentity := c.Receipt.Execution.Intents["pod-"+mode].Object
			ev := restoreplan.CopyEvidence{Identity: restoreplan.ResultIdentity{Namespace: scope.Namespace, JobName: jobIdentity.Name, JobUID: jobIdentity.UID, PodName: podIdentity.Name, PodUID: podIdentity.UID, OwnerJobUID: jobIdentity.UID, ImageID: c.Receipt.Execution.Frame.RuntimeImageID, FrameHash: c.Receipt.Execution.Frame.FrameHash, LogsComplete: false}, LogBytes: 0, LogSHA256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", ParseCode: "cancel_requested_result_unavailable"}
			if err = e.Store.saveCopyEvidence(scope, op, mode, ev); err != nil {
				return err
			}
		}
	}
	if v.Purpose == "restore-target" && c.Receipt.Steps[RestoreCommitStep].Applied && !c.Receipt.Steps["restore-rollback"].Applied {
		return ErrConflict
	}
	if v.Purpose == "restore-target" && !cancelled && !c.Receipt.Steps["restore-rollback"].Applied {
		return ErrConflict
	}
	// Pod GVR is deletion-only and always requires its sealed UID.
	return e.Resources.Delete(ctx, scope, op, key)
}
