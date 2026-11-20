package vmrestore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/goodrain/rainbond/pkg/vmrestore/copyio"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

type restoreFakeLogs struct {
	body  string
	opens int
	after func()
}

func (l *restoreFakeLogs) Open(_ context.Context, namespace, pod string, limit int64) (io.ReadCloser, error) {
	l.opens++
	if limit != copyio.MaxReportBytes+2 {
		return nil, errors.New("unbounded logs")
	}
	if l.after != nil {
		l.after()
	}
	return io.NopCloser(strings.NewReader(l.body)), nil
}
func captureFixture(t *testing.T) (restoreCommitFixture, *RestoreCapture, copyio.Plan, ObjectRef, ObjectRef, *restoreFakeLogs) {
	t.Helper()
	f, frame, _ := resourceFixture(t)
	plan := inspectPlan(frame)
	mounts := []RestoreMounts{}
	for i, d := range frame.Disks {
		mounts = append(mounts, RestoreMounts{VolumeID: d.VolumeID, ROX: ObjectRef{Kind: "PersistentVolumeClaim", Namespace: f.scope.Namespace, Name: frame.name(d.VolumeID, "rox"), UID: plan.Disks[i].SourcePVCUID}, Previous: d.PreviousPVC})
	}
	job, _, err := RestoreCopyJob(frame, plan, mounts)
	if err != nil {
		t.Fatal(err)
	}
	job.UID = "job-uid"
	job.Status.Succeeded = 1
	job.ResourceVersion = "1"
	job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
	owner := true
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "copy-pod", Namespace: f.scope.Namespace, UID: "pod-uid", ResourceVersion: "1", Annotations: job.Spec.Template.Annotations, Labels: map[string]string{"batch.kubernetes.io/controller-uid": string(job.UID)}, OwnerReferences: []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "Job", Name: job.Name, UID: job.UID, Controller: &owner}}}, Spec: *job.Spec.Template.Spec.DeepCopy(), Status: corev1.PodStatus{Phase: corev1.PodSucceeded, ContainerStatuses: []corev1.ContainerStatus{{Name: "copy", ImageID: frame.RuntimeImageID, State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}}}}}
	for key, value := range job.Spec.Template.Labels {
		pod.Labels[key] = value
	}
	pod.Status.ContainerStatuses[0].State.Terminated.StartedAt = metav1.NewTime(time.Unix(1700000000, 0))
	pod.Status.ContainerStatuses[0].State.Terminated.FinishedAt = metav1.NewTime(time.Unix(1700000010, 0))
	report := copyio.Report{Protocol: copyio.Protocol, OperationID: plan.OperationID, PlanHash: plan.PlanHash, Mode: "inspect", OK: true, Phase: "complete", Disks: []copyio.DiskResult{}}
	for _, d := range plan.Disks {
		file := copyio.FileResult{LogicalBytes: 8, ReadBytes: 8, SHA256: strings.Repeat("1", 64), NonzeroChunkBytes: 8}
		report.Disks = append(report.Disks, copyio.DiskResult{VolumeID: d.VolumeID, SourcePVCUID: d.SourcePVCUID, PreviousPVCUID: d.PreviousPVCUID, Source: file, PreviousBefore: file})
	}
	raw, _ := json.Marshal(report)
	sum := sha256.Sum256(raw)
	summary, _ := json.Marshal(copyio.Summary{Protocol: copyio.Protocol, OperationID: plan.OperationID, PlanHash: plan.PlanHash, OK: true, ReportSHA256: hex.EncodeToString(sum[:]), ReportBytes: len(raw), DiskCount: len(report.Disks)})
	pod.Status.ContainerStatuses[0].State.Terminated.Message = string(summary)
	jobRef := refOf("Job", job)
	podRef := refOf("Pod", pod)
	_, err = f.store.executionUpdate(f.scope, f.expect.OperationID, "capture-test-seed", true, func(c Current) (Mutation, error) {
		c.Receipt.Resources = append(c.Receipt.Resources, ResourceRef{ObjectRef: jobRef, Purpose: "restore-inspect"}, ResourceRef{ObjectRef: podRef, Purpose: "restore-inspect-pod"})
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	objects := []runtime.Object{job, pod}
	for _, mount := range mounts {
		for _, ref := range []ObjectRef{mount.ROX, mount.Previous} {
			objects = append(objects, &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: ref.Name, Namespace: ref.Namespace, UID: types.UID(ref.UID)}})
		}
	}
	logs := &restoreFakeLogs{body: string(raw) + "\n"}
	return f, &RestoreCapture{Core: kubefake.NewSimpleClientset(objects...), Logs: logs, Store: f.store}, plan, jobRef, podRef, logs
}

// capability_id: rainbond.vm-restore.result-capture
func TestCapturePersistsFullTypedEvidenceBeforeCrossStageFailure(t *testing.T) {
	f, c, p, j, pod, logs := captureFixture(t)
	// Preserve a full valid shape but mismatch its summary; safe typed report must
	// remain available before the assertion rejects it.
	logs.body = strings.Replace(logs.body, strings.Repeat("1", 64), strings.Repeat("2", 64), 1)
	if _, err := c.Capture(context.Background(), f.scope, f.expect.OperationID, p, j, pod); err == nil {
		t.Fatal("accepted changed complete payload")
	}
	saved, err := f.store.Read(f.scope, f.expect.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	e := saved.Receipt.Execution.Evidence["inspect"]
	if e.Report == nil || e.Summary == nil || e.Report.Disks[0].Source.SHA256 != strings.Repeat("2", 64) || e.LogBytes != len(logs.body) {
		t.Fatal("lost safe result before assertion")
	}
}

// capability_id: rainbond.vm-restore.result-capture
func TestCaptureExactUIDImageAndFullBoundedLogs(t *testing.T) {
	for _, tc := range []string{"success", "pod-uid", "runtime-image", "oversize", "truncated", "changed-after-read", "failed-terminal"} {
		t.Run(tc, func(t *testing.T) {
			f, c, p, j, pod, logs := captureFixture(t)
			current, _ := c.Core.CoreV1().Pods(f.scope.Namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
			switch tc {
			case "pod-uid":
				current.UID = "replacement"
			case "runtime-image":
				current.Status.ContainerStatuses[0].ImageID = "containerd://sha256:" + strings.Repeat("b", 64)
			case "oversize":
				logs.body = strings.Repeat("x", copyio.MaxReportBytes+3)
			case "truncated":
				logs.body = logs.body[:len(logs.body)/2]
			case "changed-after-read":
				logs.after = func() {
					changed, _ := c.Core.CoreV1().Pods(f.scope.Namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
					changed.UID = "replacement"
					_, _ = c.Core.CoreV1().Pods(f.scope.Namespace).Update(context.Background(), changed, metav1.UpdateOptions{})
				}
			case "failed-terminal":
				current.Status.Phase = corev1.PodFailed
				current.Status.ContainerStatuses[0].State.Terminated.ExitCode = 2
			}
			if tc != "changed-after-read" {
				_, _ = c.Core.CoreV1().Pods(f.scope.Namespace).Update(context.Background(), current, metav1.UpdateOptions{})
			}
			_, err := c.Capture(context.Background(), f.scope, f.expect.OperationID, p, j, pod)
			if tc == "success" {
				if err != nil {
					t.Fatal(err)
				}
				saved, _ := f.store.Read(f.scope, f.expect.OperationID)
				if saved.Receipt.Execution.Evidence["inspect"].Report == nil {
					t.Fatal("no persisted report")
				}
				return
			}
			if err == nil {
				t.Fatal("accepted invalid capture")
			}
			if (tc == "pod-uid" || tc == "runtime-image") && logs.opens != 0 {
				t.Fatal("read logs before identity check")
			}
			if tc == "oversize" || tc == "truncated" || tc == "changed-after-read" || tc == "failed-terminal" {
				saved, _ := f.store.Read(f.scope, f.expect.OperationID)
				if _, ok := saved.Receipt.Execution.Evidence["inspect"]; !ok {
					t.Fatal("discarded failure evidence")
				}
			}
		})
	}
}
