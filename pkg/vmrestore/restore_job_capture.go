package vmrestore

import (
	"context"
	"errors"
	"io"
	"reflect"
	"time"

	"github.com/goodrain/rainbond/pkg/vmrestore/copyio"
	"github.com/goodrain/rainbond/pkg/vmrestore/restoreplan"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// RestoreLogReader is only the bounded exact-Pod log channel. It cannot exec.
type RestoreLogReader interface {
	Open(context.Context, string, string, int64) (io.ReadCloser, error)
}
type kubeRestoreLogs struct{ core kubernetes.Interface }

func (k kubeRestoreLogs) Open(ctx context.Context, namespace, pod string, limit int64) (io.ReadCloser, error) {
	if k.core == nil || limit != copyio.MaxReportBytes+2 {
		return nil, ErrCorrupt
	}
	return k.core.CoreV1().Pods(namespace).GetLogs(pod, &corev1.PodLogOptions{Container: "copy", LimitBytes: &limit, Follow: false, Timestamps: false}).Stream(ctx)
}

// NewRestoreLogReader uses the established Kubernetes client, no new credentials.
func NewRestoreLogReader(core kubernetes.Interface) RestoreLogReader {
	return kubeRestoreLogs{core: core}
}

// RestoreCapture reads only the sealed Pod log channel and persists safe results.
type RestoreCapture struct {
	Core  kubernetes.Interface
	Logs  RestoreLogReader
	Store *Store
}

func copyPod(job *batchv1.Job, pod *corev1.Pod, wantImage, wantRuntime, frame string) bool {
	if pod.Namespace != job.Namespace || pod.DeletionTimestamp != nil || pod.Annotations[RestoreFrameAnnotation] != frame || len(pod.Spec.Containers) != 1 || len(pod.Spec.InitContainers) != 0 || len(pod.Spec.EphemeralContainers) != 0 || pod.Spec.Containers[0].Name != "copy" || pod.Spec.Containers[0].Image != wantImage || pod.Spec.HostNetwork || pod.Spec.HostPID || pod.Spec.HostIPC {
		return false
	}
	controller := metav1.GetControllerOf(pod)
	if controller == nil || controller.APIVersion != "batch/v1" || controller.Kind != "Job" || controller.UID != job.UID || controller.Name != job.Name {
		return false
	}
	for key, value := range job.Spec.Template.Labels {
		if pod.Labels[key] != value {
			return false
		}
	}
	for key, value := range job.Spec.Template.Annotations {
		if pod.Annotations[key] != value {
			return false
		}
	}
	if pod.Spec.RestartPolicy != corev1.RestartPolicyNever || pod.Spec.AutomountServiceAccountToken == nil || *pod.Spec.AutomountServiceAccountToken {
		return false
	}
	// Match actual Job template volume order/mounts/security; defaults and scheduler
	// node placement can differ but a sidecar, injected mount or changed command cannot.
	container := pod.Spec.Containers[0]
	want := job.Spec.Template.Spec.Containers[0]
	if !reflect.DeepEqual(container.Command, want.Command) || !reflect.DeepEqual(container.Args, want.Args) || !reflect.DeepEqual(container.VolumeMounts, want.VolumeMounts) || !sameCaptureSecurity(container.SecurityContext, want.SecurityContext) || !reflect.DeepEqual(pod.Spec.Volumes, job.Spec.Template.Spec.Volumes) || !reflect.DeepEqual(pod.Spec.SecurityContext, job.Spec.Template.Spec.SecurityContext) || len(container.Env) != 0 || len(container.EnvFrom) != 0 || container.TerminationMessagePolicy != want.TerminationMessagePolicy {
		return false
	}
	if len(pod.Status.ContainerStatuses) != 1 {
		return false
	}
	status := pod.Status.ContainerStatuses[0]
	return status.Name == "copy" && status.ImageID == wantRuntime && status.RestartCount == 0 && status.State.Terminated != nil && !status.State.Terminated.FinishedAt.IsZero() && status.State.Running == nil && status.State.Waiting == nil && (pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed)
}

func sameCaptureSecurity(a, b *corev1.SecurityContext) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	left, right := a.DeepCopy(), b.DeepCopy()
	// The API server materializes DefaultProcMount; Unmasked or any other
	// security-context change still fails the exact comparison.
	defaultMount := corev1.DefaultProcMount
	if left.ProcMount == nil {
		left.ProcMount = &defaultMount
	}
	if right.ProcMount == nil {
		right.ProcMount = &defaultMount
	}
	return reflect.DeepEqual(left, right)
}

func captureJobTerminal(job *batchv1.Job) (bool, bool) {
	complete, failed := false, false
	for _, condition := range job.Status.Conditions {
		if condition.Status != corev1.ConditionTrue {
			continue
		}
		complete = complete || condition.Type == batchv1.JobComplete
		failed = failed || condition.Type == batchv1.JobFailed
	}
	return complete, failed
}
func (c *RestoreCapture) jobPod(ctx context.Context, scope Scope, jobRef, podRef ObjectRef, image, runtime, frame string, desired *batchv1.Job) (*batchv1.Job, *corev1.Pod, error) {
	job, err := c.Core.BatchV1().Jobs(scope.Namespace).Get(ctx, jobRef.Name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, ErrReferenceUnavailable
	}
	if jobRef.Kind != "Job" || jobRef.Namespace != scope.Namespace || string(job.UID) != jobRef.UID || job.DeletionTimestamp != nil || job.Annotations[RestoreFrameAnnotation] != frame || len(job.Spec.Template.Spec.Containers) != 1 || job.Spec.Template.Spec.Containers[0].Image != image {
		return nil, nil, restoreplan.ErrResultIdentity
	}
	complete, failed := captureJobTerminal(job)
	if job.Status.Active != 0 || (!complete && !failed) {
		return nil, nil, ErrRestorePending
	}
	want, _ := desiredObject(desired, "Job")
	actual, _ := desiredObject(job, "Job")
	if !sameDesired(want, actual) {
		return nil, nil, restoreplan.ErrResultIdentity
	}
	// Labels can be missing or changed. Scan the complete namespace and count
	// actual owner references as well; never hide a second writer by a selector.
	options := metav1.ListOptions{Limit: 200}
	var pod *corev1.Pod
	seenTokens := map[string]bool{}
	for {
		if ctx.Err() != nil {
			return nil, nil, ErrReferenceUnavailable
		}
		if len(seenTokens) >= 256 {
			return nil, nil, ErrReferenceUnavailable
		}
		list, err := c.Core.CoreV1().Pods(scope.Namespace).List(ctx, options)
		if err != nil {
			return nil, nil, ErrReferenceUnavailable
		}
		for i := range list.Items {
			candidate := &list.Items[i]
			owned := candidate.Name == podRef.Name || candidate.Labels["batch.kubernetes.io/controller-uid"] == string(job.UID) || candidate.Labels["controller-uid"] == string(job.UID)
			for _, owner := range candidate.OwnerReferences {
				owned = owned || (owner.Kind == "Job" && owner.UID == job.UID)
			}
			if owned {
				if pod != nil {
					return nil, nil, ErrRestorePending
				}
				pod = candidate.DeepCopy()
			}
		}
		if list.Continue == "" {
			break
		}
		if seenTokens[list.Continue] {
			return nil, nil, ErrReferenceUnavailable
		}
		seenTokens[list.Continue] = true
		options.Continue = list.Continue
	}
	if pod == nil {
		return nil, nil, ErrRestorePending
	}
	if podRef.Kind != "Pod" || podRef.Namespace != scope.Namespace || pod.Name != podRef.Name || string(pod.UID) != podRef.UID || !copyPod(desired, pod, image, runtime, frame) {
		return nil, nil, restoreplan.ErrResultIdentity
	}
	return job, pod, nil
}

func (c *RestoreCapture) current(scope Scope, op string, plan copyio.Plan) (Current, error) {
	state, err := c.Store.Read(scope, op)
	if err != nil {
		return state, captureSafeError(err)
	}
	if state.Receipt == nil || state.Receipt.Execution == nil || restoreOwner(state, scope, Expectation{OperationID: op, ManifestHash: plan.ManifestHash}) != nil {
		return state, ErrConflict
	}
	frame := state.Receipt.Execution.Frame
	if frame.FrameHash != plan.FrameHash || frame.CoreHash != plan.ManifestHash || frame.OperationID != op || frame.SourceGeneration != state.State.BindingGeneration || state.Receipt.Restore.SourceGeneration != frame.SourceGeneration || state.Receipt.Steps[RestoreCommitStep].Applied || len(frame.Disks) != len(plan.Disks) {
		return state, ErrConflict
	}
	return state, nil
}

func captureSafeError(err error) error {
	if err == nil {
		return nil
	}
	for _, known := range []error{ErrConflict, ErrBusy, ErrCorrupt, ErrNotFound} {
		if errors.Is(err, known) {
			return known
		}
	}
	return ErrReferenceUnavailable
}

func (c *RestoreCapture) persist(scope Scope, op string, plan copyio.Plan, evidence restoreplan.CopyEvidence) error {
	if _, err := c.current(scope, op, plan); err != nil {
		return err
	}
	if err := c.Store.saveCopyEvidence(scope, op, plan.Mode, evidence); err != nil {
		return captureSafeError(err)
	}
	state, err := c.current(scope, op, plan)
	if err != nil {
		return err
	}
	// A historical step replay must not claim that it replaced a newer failure
	// observation. The fixed parent writer is under review; fail closed if its
	// persisted value differs, rather than signing a successful capture.
	if !reflect.DeepEqual(state.Receipt.Execution.Evidence[plan.Mode], evidence) {
		return ErrConflict
	}
	return nil
}

func (c *RestoreCapture) mountIdentities(ctx context.Context, namespace string, mounts []RestoreMounts) bool {
	for _, mount := range mounts {
		refs := []ObjectRef{mount.ROX, mount.Previous}
		if mount.Target != nil {
			refs = append(refs, *mount.Target)
		}
		for _, ref := range refs {
			pvc, err := c.Core.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, ref.Name, metav1.GetOptions{})
			if err != nil || CheckLivePVC(ref, pvc.Namespace, pvc.Name, string(pvc.UID)) != nil || pvc.DeletionTimestamp != nil {
				return false
			}
		}
	}
	return true
}

// Capture rereads exact Job/Pod identities after complete bounded logs. The typed
// safe payload is saved before the 5a verifier compares any full disk digests.
func (c *RestoreCapture) Capture(ctx context.Context, scope Scope, op string, plan copyio.Plan, jobRef, podRef ObjectRef) (copyio.Report, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if c == nil || c.Core == nil || c.Logs == nil || c.Store == nil || plan.Validate() != nil {
		return copyio.Report{}, ErrCorrupt
	}
	state, err := c.current(scope, op, plan)
	if err != nil {
		return copyio.Report{}, err
	}
	frame := state.Receipt.Execution.Frame
	if frame.FrameHash != plan.FrameHash || frame.CoreHash != plan.ManifestHash || frame.OperationID != op {
		return copyio.Report{}, ErrConflict
	}
	// Both server-observed UIDs must already be recorded before any logs are read.
	foundJob, foundPod := false, false
	for _, v := range state.Receipt.Resources {
		foundJob = foundJob || (sameObject(v.ObjectRef, jobRef) && v.Purpose == "restore-"+plan.Mode)
		foundPod = foundPod || (sameObject(v.ObjectRef, podRef) && v.Purpose == "restore-"+plan.Mode+"-pod")
	}
	if !foundJob || !foundPod {
		return copyio.Report{}, ErrConflict
	}
	mounts := []RestoreMounts{}
	for index, d := range frame.Disks {
		if plan.Disks[index].SourceMaxBytes > d.ROXRequestBytes || plan.Disks[index].PreviousMaxBytes > d.PreviousMaxBytes {
			return copyio.Report{}, ErrConflict
		}
		mount := RestoreMounts{VolumeID: d.VolumeID, Previous: d.PreviousPVC, ROX: ObjectRef{Kind: "PersistentVolumeClaim", Namespace: scope.Namespace, Name: frame.name(d.VolumeID, "rox"), UID: plan.Disks[index].SourcePVCUID}}
		if plan.Mode != "inspect" {
			target := ObjectRef{Kind: "PersistentVolumeClaim", Namespace: scope.Namespace, Name: frame.name(d.VolumeID, "target"), UID: plan.Disks[index].TargetPVCUID}
			mount.Target = &target
		}
		mounts = append(mounts, mount)
	}
	desired, _, err := RestoreCopyJob(frame, plan, mounts)
	if err != nil {
		return copyio.Report{}, err
	}
	desired.UID = types.UID(jobRef.UID)
	if !c.mountIdentities(ctx, scope.Namespace, mounts) {
		return copyio.Report{}, restoreplan.ErrResultIdentity
	}
	job, pod, err := c.jobPod(ctx, scope, jobRef, podRef, frame.WorkerImage, frame.RuntimeImageID, frame.FrameHash, desired)
	if err != nil {
		return copyio.Report{}, err
	}
	expected := restoreplan.ResultIdentity{Namespace: scope.Namespace, JobName: jobRef.Name, JobUID: jobRef.UID, PodName: podRef.Name, PodUID: podRef.UID, OwnerJobUID: jobRef.UID, ImageID: frame.RuntimeImageID, FrameHash: frame.FrameHash}
	observed := expected
	observed.ExitCode = int(pod.Status.ContainerStatuses[0].State.Terminated.ExitCode)
	stream, err := c.Logs.Open(ctx, scope.Namespace, podRef.Name, copyio.MaxReportBytes+2)
	if err != nil || stream == nil {
		return copyio.Report{}, ErrReferenceUnavailable
	}
	raw, readErr := io.ReadAll(io.LimitReader(stream, copyio.MaxReportBytes+2))
	closeErr := stream.Close()
	// A maximal response cannot establish EOF; a valid protocol report is smaller
	// and its exact bytes must also match the termination summary hash/count.
	observed.LogsComplete = readErr == nil && closeErr == nil && len(raw) < copyio.MaxReportBytes+2
	afterJob, after, identityErr := c.jobPod(ctx, scope, jobRef, podRef, frame.WorkerImage, frame.RuntimeImageID, frame.FrameHash, desired)
	decision := ""
	if identityErr != nil || after.UID != pod.UID || after.ResourceVersion != pod.ResourceVersion || !reflect.DeepEqual(after.Status, pod.Status) || afterJob.ResourceVersion != job.ResourceVersion || !reflect.DeepEqual(afterJob.Status, job.Status) || !c.mountIdentities(ctx, scope.Namespace, mounts) {
		observed.LogsComplete = false
		decision = "source_identity_unverified"
	}
	complete, failed := captureJobTerminal(job)
	if decision == "" && (!complete || failed || job.Status.Succeeded != 1 || job.Status.Failed != 0 || pod.Status.Phase != corev1.PodSucceeded || observed.ExitCode != 0) {
		decision = "job_or_pod_failed"
	}
	term := []byte(pod.Status.ContainerStatuses[0].State.Terminated.Message)
	var saved restoreplan.CopyEvidence
	persisted := false
	report, resultErr := restoreplan.CaptureAndValidate(raw, term, observed, restoreplan.ResultExpectation{Identity: expected, Plan: plan}, func(e restoreplan.CopyEvidence) error {
		if decision != "" {
			e.ParseCode = decision
		}
		if err := c.persist(scope, op, plan, e); err != nil {
			return err
		}
		saved, persisted = e, true
		return nil
	})
	if resultErr != nil && persisted && saved.ParseCode == "" {
		saved.ParseCode = "result_validation_failed"
		if err := c.persist(scope, op, plan, saved); err != nil {
			return copyio.Report{}, err
		}
	}
	if resultErr == nil && decision != "" {
		resultErr = restoreplan.ErrResultIncomplete
	}
	return report, resultErr
}
