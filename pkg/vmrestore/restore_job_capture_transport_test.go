package vmrestore

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/goodrain/rainbond/pkg/vmrestore/copyio"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	kt "k8s.io/client-go/testing"
)

// capability_id: rainbond.vm-restore.result-capture
func TestCaptureUsesConcreteKubernetesLogEndpointAndReplays(t *testing.T) {
	f, c, p, job, pod, fixture := captureFixture(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != "GET" || r.URL.Path != "/api/v1/namespaces/"+f.scope.Namespace+"/pods/"+pod.Name+"/log" || r.URL.Query().Get("container") != "copy" || r.URL.Query().Get("limitBytes") != strconv.Itoa(copyio.MaxReportBytes+2) || r.URL.Query().Get("follow") == "true" || r.URL.Query().Get("timestamps") == "true" {
			t.Error("log reader changed the bounded readonly API request")
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, fixture.body)
	}))
	defer server.Close()
	core, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	c.Logs = NewRestoreLogReader(core)
	if report, err := c.Capture(context.Background(), f.scope, f.expect.OperationID, p, job, pod); err != nil || !report.OK {
		t.Fatal("real HTTP log request failed", err)
	}
	before, err := f.store.Read(f.scope, f.expect.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Capture(context.Background(), f.scope, f.expect.OperationID, p, job, pod); err != nil {
		t.Fatal("same exact source did not replay", err)
	}
	after, err := f.store.Read(f.scope, f.expect.OperationID)
	if err != nil || requests.Load() != 2 || before.State.ControlRevision != after.State.ControlRevision || after.State.Fence == nil || after.Receipt.Steps["restore-copy-verified"].Applied || after.Receipt.ConfigCommitHash != "" {
		t.Fatal("capture replay rewrote state, unlocked or committed restore")
	}
}

func TestCaptureScansCompletePodPages(t *testing.T) {
	for _, kind := range []string{"valid", "hidden-owned", "repeated-token"} {
		t.Run(kind, func(t *testing.T) {
			f, c, p, j, ref, logs := captureFixture(t)
			pod, err := c.Core.CoreV1().Pods(f.scope.Namespace).Get(context.Background(), ref.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			pages := 0
			c.Core.(*kubefake.Clientset).PrependReactor("list", "pods", func(kt.Action) (bool, runtime.Object, error) {
				pages++
				list := &corev1.PodList{}
				if kind == "repeated-token" {
					list.Continue = "same"
					return true, list, nil
				}
				if pages%2 == 1 {
					list.Continue = "second"
					if kind == "hidden-owned" {
						list.Items = []corev1.Pod{*pod.DeepCopy()}
					}
				} else {
					copy := pod.DeepCopy()
					if kind == "hidden-owned" {
						copy.Name = "hidden-owned"
						copy.UID = "hidden-uid"
						copy.Labels = nil
						copy.Status.Phase = corev1.PodRunning
					}
					list.Items = []corev1.Pod{*copy}
				}
				return true, list, nil
			})
			_, err = c.Capture(context.Background(), f.scope, f.expect.OperationID, p, j, ref)
			if kind == "valid" {
				if err != nil || pages != 4 {
					t.Fatal("complete paginated source rejected", err, pages)
				}
			} else if err == nil || logs.opens != 0 {
				t.Fatal("incomplete/ambiguous paginated source became success")
			}
		})
	}
}

type missingCaptureLogs struct{}

func (missingCaptureLogs) Open(context.Context, string, string, int64) (io.ReadCloser, error) {
	return nil, nil
}

func TestCaptureMissingAndUnavailableLogChannelsStaySafe(t *testing.T) {
	for _, kind := range []string{"nil-core", "nil-store", "nil-logs", "missing-stream", "http-error", "cancelled", "invalid-limit"} {
		t.Run(kind, func(t *testing.T) {
			f, c, p, j, pod, _ := captureFixture(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "PRIVATE_LOG_PROVIDER_DETAIL", http.StatusInternalServerError)
			}))
			defer server.Close()
			core, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			switch kind {
			case "nil-core":
				c.Core = nil
			case "nil-store":
				c.Store = nil
			case "nil-logs":
				c.Logs = nil
			case "missing-stream":
				c.Logs = missingCaptureLogs{}
			case "http-error":
				c.Logs = NewRestoreLogReader(core)
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "invalid-limit":
				if _, err := NewRestoreLogReader(core).Open(ctx, f.scope.Namespace, pod.Name, 1); err == nil {
					t.Fatal("unbounded caller limit was accepted")
				}
				if _, err := NewRestoreLogReader(nil).Open(ctx, f.scope.Namespace, pod.Name, copyio.MaxReportBytes+2); err == nil {
					t.Fatal("missing concrete client was accepted")
				}
				return
			}
			if report, err := c.Capture(ctx, f.scope, f.expect.OperationID, p, j, pod); err == nil || report.OK || strings.Contains(err.Error(), "PRIVATE_LOG_PROVIDER_DETAIL") {
				t.Fatal("missing/unavailable log channel became success or leaked provider text", err)
			}
		})
	}
}

func TestCaptureRejectsUntrustedIdentityAndOpenSourceMatrix(t *testing.T) {
	for _, kind := range []string{"job-uid", "job-deleting", "job-active", "job-not-terminal", "job-image", "job-frame", "pod-scope", "pod-core", "pod-owner", "pod-restart", "pod-status", "pod-unfinished", "pod-running", "pod-privilege", "pod-default-proc", "pod-unmasked", "pod-env", "pod-sidecar", "pod-init", "pod-ephemeral", "pod-hostnetwork", "pod-token", "pod-command", "pod-mount", "pod-volume", "pod-policy", "pvc-uid", "pvc-deleting", "get-error", "list-error", "no-pod", "receipt-purpose", "plan-budget", "plan-frame", "plan-previous"} {
		t.Run(kind, func(t *testing.T) {
			f, c, p, j, pr, logs := captureFixture(t)
			ctx := context.Background()
			job, err := c.Core.BatchV1().Jobs(f.scope.Namespace).Get(ctx, j.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			pod, err := c.Core.CoreV1().Pods(f.scope.Namespace).Get(ctx, pr.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "job-uid":
				job.UID = "replacement"
			case "job-deleting":
				now := metav1.Now()
				job.DeletionTimestamp = &now
			case "job-active":
				job.Status.Active = 1
			case "job-not-terminal":
				job.Status.Conditions = nil
			case "job-image":
				job.Spec.Template.Spec.Containers[0].Image = "other@sha256:" + strings.Repeat("a", 64)
			case "job-frame":
				job.Annotations[RestoreFrameAnnotation] = strings.Repeat("b", 64)
			case "pod-scope":
				pod.Labels["service_id"] = "other-service"
			case "pod-core":
				pod.Annotations[RestoreCoreAnnotation] = strings.Repeat("b", 64)
			case "pod-owner":
				pod.OwnerReferences[0].UID = "other-job-uid"
			case "pod-restart":
				pod.Status.ContainerStatuses[0].RestartCount = 1
			case "pod-status":
				pod.Status.ContainerStatuses = nil
			case "pod-unfinished":
				pod.Status.ContainerStatuses[0].State.Terminated.FinishedAt = metav1.Time{}
			case "pod-running":
				pod.Status.Phase = corev1.PodRunning
			case "pod-privilege":
				yes := true
				pod.Spec.Containers[0].SecurityContext.AllowPrivilegeEscalation = &yes
			case "pod-default-proc":
				value := corev1.DefaultProcMount
				pod.Spec.Containers[0].SecurityContext.ProcMount = &value
			case "pod-unmasked":
				value := corev1.UnmaskedProcMount
				pod.Spec.Containers[0].SecurityContext.ProcMount = &value
			case "pod-env":
				pod.Spec.Containers[0].Env = []corev1.EnvVar{{Name: "UNTRUSTED", Value: "fixture"}}
			case "pod-sidecar":
				pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{Name: "sidecar", Image: "other"})
			case "pod-init":
				pod.Spec.InitContainers = []corev1.Container{{Name: "init", Image: "other"}}
			case "pod-ephemeral":
				pod.Spec.EphemeralContainers = []corev1.EphemeralContainer{{EphemeralContainerCommon: corev1.EphemeralContainerCommon{Name: "debug"}}}
			case "pod-hostnetwork":
				pod.Spec.HostNetwork = true
			case "pod-token":
				yes := true
				pod.Spec.AutomountServiceAccountToken = &yes
			case "pod-command":
				pod.Spec.Containers[0].Command = []string{"other-command"}
			case "pod-mount":
				pod.Spec.Containers[0].VolumeMounts[0].ReadOnly = false
			case "pod-volume":
				pod.Spec.Volumes[2].PersistentVolumeClaim.ClaimName = "foreign-pvc"
			case "pod-policy":
				pod.Spec.Containers[0].TerminationMessagePolicy = corev1.TerminationMessageFallbackToLogsOnError
			case "pvc-uid", "pvc-deleting":
				claim, err := c.Core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(ctx, pod.Spec.Volumes[2].PersistentVolumeClaim.ClaimName, metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}
				if kind == "pvc-uid" {
					claim.UID = "replacement"
				} else {
					now := metav1.Now()
					claim.DeletionTimestamp = &now
				}
				if _, err := c.Core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Update(ctx, claim, metav1.UpdateOptions{}); err != nil {
					t.Fatal(err)
				}
			case "get-error":
				c.Core.(*kubefake.Clientset).PrependReactor("get", "jobs", func(kt.Action) (bool, runtime.Object, error) { return true, nil, errors.New("PRIVATE_PROVIDER_DETAIL") })
			case "list-error":
				c.Core.(*kubefake.Clientset).PrependReactor("list", "pods", func(kt.Action) (bool, runtime.Object, error) { return true, nil, errors.New("PRIVATE_PROVIDER_DETAIL") })
			case "no-pod":
				if err := c.Core.CoreV1().Pods(f.scope.Namespace).Delete(ctx, pr.Name, metav1.DeleteOptions{}); err != nil {
					t.Fatal(err)
				}
			case "receipt-purpose":
				_, err = f.store.executionUpdate(f.scope, f.expect.OperationID, "capture-purpose-change", true, func(s Current) (Mutation, error) {
					s.Receipt.Resources[0].Purpose = "other-purpose"
					return Mutation{State: s.State, Receipt: s.Receipt}, nil
				})
				if err != nil {
					t.Fatal(err)
				}
			case "plan-budget":
				p.Disks[0].SourceMaxBytes++
				p, _ = copyio.SealPlan(p)
			case "plan-frame":
				p.FrameHash = strings.Repeat("b", 64)
				p, _ = copyio.SealPlan(p)
			case "plan-previous":
				p.Disks[0].PreviousPVCUID = "other-previous"
				p, _ = copyio.SealPlan(p)
			}
			if strings.HasPrefix(kind, "job-") {
				if _, err := c.Core.BatchV1().Jobs(f.scope.Namespace).Update(ctx, job, metav1.UpdateOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			if strings.HasPrefix(kind, "pod-") {
				if _, err := c.Core.CoreV1().Pods(f.scope.Namespace).Update(ctx, pod, metav1.UpdateOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			_, err = c.Capture(ctx, f.scope, f.expect.OperationID, p, j, pr)
			if kind == "pod-default-proc" {
				if err != nil {
					t.Fatal("API default ProcMount was rejected", err)
				}
				return
			}
			if err == nil || logs.opens != 0 || strings.Contains(err.Error(), "PRIVATE_PROVIDER_DETAIL") {
				t.Fatalf("invalid source reached logs/success or leaked provider text: %v opens=%d", err, logs.opens)
			}
		})
	}
}

func TestCaptureSaveFailureKeepsReceiptAndFence(t *testing.T) {
	f, c, p, j, pod, _ := captureFixture(t)
	before, err := f.store.Read(f.scope, f.expect.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	trigger := `CREATE TRIGGER capture_save_failure BEFORE UPDATE ON component_k8s_attributes WHEN NEW.name LIKE 'vm_restore_receipt_%' AND NEW.attribute_value LIKE '%evidence-inspect-%' BEGIN SELECT RAISE(ABORT,'PRIVATE_SQL_DETAIL'); END`
	if err := f.db.Exec(trigger).Error; err != nil {
		t.Fatal(err)
	}
	if report, err := c.Capture(context.Background(), f.scope, f.expect.OperationID, p, j, pod); err == nil || report.OK || strings.Contains(err.Error(), "PRIVATE_SQL_DETAIL") {
		t.Fatal("save failure became success or exposed SQL detail", err)
	}
	after, err := f.store.Read(f.scope, f.expect.OperationID)
	if err != nil || after.State.ControlRevision != before.State.ControlRevision || after.State.Fence == nil || len(after.Receipt.Execution.Evidence) != 0 {
		t.Fatal("failed save partially changed receipt/fence")
	}
}
