package vmrestore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/pkg/vmrestore/copyio"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubefake "k8s.io/client-go/kubernetes/fake"
	kt "k8s.io/client-go/testing"
)

// capability_id: rainbond.vm-restore.result-capture
func TestCaptureIncompleteDiskCoverageIsRejectedWithoutPanic(t *testing.T) {
	f, c, p, job, pod, logs := captureFixture(t)
	p.Disks = p.Disks[:1]
	p, _ = copyio.SealPlan(p)
	defer func() {
		if value := recover(); value != nil {
			t.Fatalf("short disk plan panicked: %v", value)
		}
	}()
	if _, err := c.Capture(context.Background(), f.scope, f.expect.OperationID, p, job, pod); err == nil || logs.opens != 0 {
		t.Fatal("incomplete plan reached logs or became success")
	}
}

func TestCaptureFailedJobCannotTurnSuccessfulMessageIntoSuccess(t *testing.T) {
	f, c, p, job, pod, _ := captureFixture(t)
	actual, err := c.Core.BatchV1().Jobs(f.scope.Namespace).Get(context.Background(), job.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	actual.Status.Succeeded, actual.Status.Failed = 0, 1
	actual.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue}}
	if _, err := c.Core.BatchV1().Jobs(f.scope.Namespace).UpdateStatus(context.Background(), actual, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Capture(context.Background(), f.scope, f.expect.OperationID, p, job, pod); err == nil {
		t.Fatal("failed Job plus success payload became verified success")
	}
	state, err := f.store.Read(f.scope, f.expect.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	e := state.Receipt.Execution.Evidence["inspect"]
	if e.Report == nil || e.Summary == nil || e.ParseCode == "" {
		t.Fatal("failed Job's full safe payload or terminal decision was not recorded")
	}
}

func TestCaptureUnlabelledOwnedPodPreventsClosedSource(t *testing.T) {
	f, c, p, job, pod, logs := captureFixture(t)
	extra, err := c.Core.CoreV1().Pods(f.scope.Namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	extra.Name, extra.UID, extra.ResourceVersion = "extra-copy-pod", "extra-pod-uid", ""
	extra.Labels = nil
	extra.Status.Phase = corev1.PodRunning
	extra.Status.ContainerStatuses[0].State = corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}
	if _, err := c.Core.CoreV1().Pods(f.scope.Namespace).Create(context.Background(), extra, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Capture(context.Background(), f.scope, f.expect.OperationID, p, job, pod); err == nil || logs.opens != 0 {
		t.Fatal("label filtering hid another owned writer")
	}
}

func TestCaptureTransientRecheckFailureCanRetrySameActualPod(t *testing.T) {
	f, c, p, job, pod, _ := captureFixture(t)
	failed, lists := false, 0
	c.Core.(*kubefake.Clientset).PrependReactor("list", "pods", func(kt.Action) (bool, runtime.Object, error) {
		lists++
		if !failed && lists == 2 {
			failed = true
			return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, pod.Name, fmt.Errorf("fixture observation unavailable"))
		}
		return false, nil, nil
	})
	if _, err := c.Capture(context.Background(), f.scope, f.expect.OperationID, p, job, pod); err == nil || !failed {
		t.Fatal("unverified post-read source was accepted")
	}
	if _, err := c.Capture(context.Background(), f.scope, f.expect.OperationID, p, job, pod); err != nil {
		t.Fatalf("transient read failure poisoned the same Pod's evidence identity: %v", err)
	}
	state, err := f.store.Read(f.scope, f.expect.OperationID)
	if err != nil || state.Receipt.Execution.Evidence["inspect"].Identity.PodUID != pod.UID || state.Receipt.Execution.Evidence["inspect"].Report == nil {
		t.Fatal("same-UID retry did not persist its complete safe result")
	}
}

func TestCaptureGenerationDriftAcrossLogReadCannotPersistSuccess(t *testing.T) {
	f, c, p, job, pod, logs := captureFixture(t)
	logs.after = func() { resourceGenerationDrift(t, f) }
	if _, err := c.Capture(context.Background(), f.scope, f.expect.OperationID, p, job, pod); err == nil {
		t.Fatal("stale source generation produced capture success")
	}
	state, err := f.store.Read(f.scope, f.expect.OperationID)
	if err != nil || state.State.Fence == nil || state.Receipt.Steps["restore-copy-verified"].Applied || len(state.Receipt.Execution.Evidence) != 0 {
		t.Fatal("stale capture changed the receipt or fence")
	}
}

type captureStreamLogs struct {
	body                  string
	readError, closeError error
	opens                 int
}
type captureErrorStream struct {
	io.Reader
	readError, closeError error
	exhausted             bool
}

func (l *captureStreamLogs) Open(context.Context, string, string, int64) (io.ReadCloser, error) {
	l.opens++
	return &captureErrorStream{Reader: strings.NewReader(l.body), readError: l.readError, closeError: l.closeError}, nil
}
func (r *captureErrorStream) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err == io.EOF && r.readError != nil {
		return n, r.readError
	}
	return n, err
}
func (r *captureErrorStream) Close() error { return r.closeError }

func TestCaptureRequiresStreamEOFAndSuccessfulClose(t *testing.T) {
	for _, when := range []string{"read", "close"} {
		t.Run(when, func(t *testing.T) {
			f, c, p, job, pod, fixture := captureFixture(t)
			logs := &captureStreamLogs{body: fixture.body}
			if when == "read" {
				logs.readError = errors.New("fixture stream failed")
			} else {
				logs.closeError = errors.New("fixture close failed")
			}
			c.Logs = logs
			if _, err := c.Capture(context.Background(), f.scope, f.expect.OperationID, p, job, pod); err == nil {
				t.Fatal("unclosed/incomplete stream became success")
			}
			state, err := f.store.Read(f.scope, f.expect.OperationID)
			if err != nil || state.Receipt.Execution.Evidence["inspect"].Identity.LogsComplete || state.Receipt.Execution.Evidence["inspect"].ParseCode == "" {
				t.Fatal("incomplete source verdict was not saved")
			}
		})
	}
}
