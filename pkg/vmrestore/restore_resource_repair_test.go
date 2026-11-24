package vmrestore

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/goodrain/rainbond/pkg/vmrestore/restoreplan"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kt "k8s.io/client-go/testing"
)

// capability_id: rainbond.vm-restore.resource-repair-safety
func TestResourceEvidenceReentryRestoresCurrentAuthority(t *testing.T) {
	f, frame, _ := resourceFixture(t)
	good := restoreplan.CopyEvidence{Identity: restoreplan.ResultIdentity{Namespace: f.scope.Namespace, JobUID: "job-uid", PodUID: "pod-uid", FrameHash: frame.FrameHash, LogsComplete: true}, LogBytes: 12, LogSHA256: strings.Repeat("a", 64)}
	bad := good
	bad.ParseCode = "json_incomplete"
	bad.LogBytes = 6
	bad.LogSHA256 = strings.Repeat("b", 64)
	bad.Identity.LogsComplete = false
	if err := f.store.saveCopyEvidence(f.scope, f.expect.OperationID, "inspect", good); err != nil {
		t.Fatal(err)
	}
	if err := f.store.saveCopyEvidence(f.scope, f.expect.OperationID, "inspect", bad); err != nil {
		t.Fatal(err)
	}
	if err := f.store.saveCopyEvidence(f.scope, f.expect.OperationID, "inspect", good); err != nil {
		t.Fatal(err)
	}
	c, err := f.store.Read(f.scope, f.expect.OperationID)
	if err != nil || !reflect.DeepEqual(c.Receipt.Execution.Evidence["inspect"], good) {
		t.Fatal("historical step replay did not restore authoritative complete evidence")
	}
	revision := c.State.ControlRevision
	if err := f.store.saveCopyEvidence(f.scope, f.expect.OperationID, "inspect", good); err != nil {
		t.Fatal(err)
	}
	c, err = f.store.Read(f.scope, f.expect.OperationID)
	if err != nil || c.State.ControlRevision != revision {
		t.Fatal("identical current evidence replay rewrote SQL")
	}
	if c.State.Fence == nil || c.Receipt.Steps["restore-copy-verified"].Applied {
		t.Fatal("saving observation advanced restore or released its fence")
	}
}

func TestResourceLateEnsureCannotCreateAfterOrdinaryStageCleanup(t *testing.T) {
	f, frame, e := resourceFixture(t)
	object := &corev1.ConfigMap{ObjectMeta: frame.metadata(frame.name(0, "inspect-plan"), "restore-plan"), Data: map[string]string{"plan.json": "fixture"}}
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	var hookMu sync.Mutex
	paused := false
	e.BeforeMutation = func(ctx context.Context, _ Scope, _ string, key string) error {
		hookMu.Lock()
		wait := key == "inspect-plan" && !paused
		if wait {
			paused = true
		}
		hookMu.Unlock()
		if wait {
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}
	done := make(chan error, 1)
	go func() {
		_, err := e.Ensure(context.Background(), f.scope, f.expect.OperationID, "inspect-plan", "restore-plan", "ConfigMap", object)
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("late caller did not pause")
	}
	if _, err := e.Ensure(context.Background(), f.scope, f.expect.OperationID, "inspect-plan", "restore-plan", "ConfigMap", object); err != nil {
		t.Fatal(err)
	}
	if err := e.Delete(context.Background(), f.scope, f.expect.OperationID, "inspect-plan"); !errors.Is(err, ErrRestorePending) {
		t.Fatal(err)
	}
	if err := e.Delete(context.Background(), f.scope, f.expect.OperationID, "inspect-plan"); err != nil {
		t.Fatal(err)
	}
	once.Do(func() { close(release) })
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cleaned resource was reported as current")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("late caller did not finish")
	}
	creates := 0
	for _, a := range e.Dynamic.(*dynamicfake.FakeDynamicClient).Actions() {
		if a.GetVerb() == "create" {
			creates++
		}
	}
	if creates != 1 {
		t.Fatal("ordinary stage cleanup allowed obsolete creation", creates)
	}
	c, err := f.store.Read(f.scope, f.expect.OperationID)
	if err != nil || !c.Receipt.Execution.Intents["inspect-plan"].Deleted || c.State.Fence == nil {
		t.Fatal("cleanup history or fence lost")
	}
}

func TestResourceCreateIssuedUnknownCannotBeReissued(t *testing.T) {
	f, frame, e := resourceFixture(t)
	object := &corev1.ConfigMap{ObjectMeta: frame.metadata(frame.name(0, "inspect-plan"), "restore-plan")}
	creates := 0
	e.Dynamic.(*dynamicfake.FakeDynamicClient).PrependReactor("create", "configmaps", func(kt.Action) (bool, runtime.Object, error) {
		creates++
		return true, nil, errors.New("fixture unknown create without observation")
	})
	for i := 0; i < 2; i++ {
		if _, err := e.Ensure(context.Background(), f.scope, f.expect.OperationID, "inspect-plan", "restore-plan", "ConfigMap", object); !errors.Is(err, ErrRestoreUnknown) {
			t.Fatal(err)
		}
	}
	if creates != 1 {
		t.Fatal("unknown issued create was blindly repeated", creates)
	}
	if err := e.Delete(context.Background(), f.scope, f.expect.OperationID, "inspect-plan"); err == nil {
		t.Fatal("cleanup of unobserved UID was marked complete")
	}
}

func TestResourceObservedSpecChangesStillConflict(t *testing.T) {
	f, frame, e := resourceFixture(t)
	object := &corev1.ConfigMap{ObjectMeta: frame.metadata(frame.name(0, "inspect-plan"), "restore-plan"), Data: map[string]string{"plan.json": "original"}}
	ref, err := e.Ensure(context.Background(), f.scope, f.expect.OperationID, "inspect-plan", "restore-plan", "ConfigMap", object)
	if err != nil {
		t.Fatal(err)
	}
	u, err := e.Dynamic.Resource(restoreGVR["ConfigMap"]).Namespace(f.scope.Namespace).Get(context.Background(), ref.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := unstructured.SetNestedField(u.Object, "changed", "data", "plan.json"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Dynamic.Resource(restoreGVR["ConfigMap"]).Namespace(f.scope.Namespace).Update(context.Background(), u, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Ensure(context.Background(), f.scope, f.expect.OperationID, "inspect-plan", "restore-plan", "ConfigMap", object); !errors.Is(err, ErrConflict) {
		t.Fatal("real spec mismatch was normalized away", err)
	}
}
