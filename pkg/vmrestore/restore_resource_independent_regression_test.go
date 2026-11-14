package vmrestore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type independentJobAPI struct {
	mu               sync.Mutex
	job              *unstructured.Unstructured
	creates, deletes int
	deleteUIDs       []string
}

func independentResourceHTTP(t *testing.T, e *RestoreResourceExecutor, namespace string) *independentJobAPI {
	t.Helper()
	a := &independentJobAPI{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		defer a.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		status := func(code int, reason metav1.StatusReason) {
			w.WriteHeader(code)
			_ = json.NewEncoder(w).Encode(metav1.Status{TypeMeta: metav1.TypeMeta{Kind: "Status", APIVersion: "v1"}, Status: metav1.StatusFailure, Reason: reason, Code: int32(code)})
		}
		if !strings.HasPrefix(r.URL.Path, "/apis/batch/v1/namespaces/"+namespace+"/jobs") && !strings.HasPrefix(r.URL.Path, "/api/v1/namespaces/"+namespace+"/configmaps") {
			t.Errorf("unexpected fake API path %s", r.URL.Path)
			status(404, metav1.StatusReasonNotFound)
			return
		}
		switch r.Method {
		case http.MethodGet:
			if a.job == nil {
				status(404, metav1.StatusReasonNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(a.job.Object)
		case http.MethodPost:
			if a.job != nil {
				status(409, metav1.StatusReasonAlreadyExists)
				return
			}
			obj := &unstructured.Unstructured{}
			if err := json.NewDecoder(r.Body).Decode(&obj.Object); err != nil {
				t.Error(err)
				status(400, metav1.StatusReasonBadRequest)
				return
			}
			a.creates++
			obj.SetUID(types.UID(fmt.Sprintf("independent-job-uid-%d", a.creates)))
			obj.SetResourceVersion(fmt.Sprint(a.creates))
			a.job = obj.DeepCopy()
			w.WriteHeader(201)
			_ = json.NewEncoder(w).Encode(obj.Object)
		case http.MethodDelete:
			var options metav1.DeleteOptions
			if err := json.NewDecoder(r.Body).Decode(&options); err != nil {
				t.Error(err)
				status(400, metav1.StatusReasonBadRequest)
				return
			}
			if options.Preconditions == nil || options.Preconditions.UID == nil {
				t.Error("DELETE omitted actual UID precondition")
				status(409, metav1.StatusReasonConflict)
				return
			}
			a.deleteUIDs = append(a.deleteUIDs, string(*options.Preconditions.UID))
			if a.job == nil {
				status(404, metav1.StatusReasonNotFound)
				return
			}
			if *options.Preconditions.UID != a.job.GetUID() {
				status(409, metav1.StatusReasonConflict)
				return
			}
			a.deletes++
			a.job = nil
			w.WriteHeader(200)
			_ = json.NewEncoder(w).Encode(metav1.Status{Status: metav1.StatusSuccess})
		default:
			status(405, metav1.StatusReasonMethodNotAllowed)
		}
	}))
	t.Cleanup(server.Close)
	client, err := dynamic.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	e.Dynamic = client
	return a
}

func independentInspectObject(t *testing.T, frame RestoreResourcePlan, kind string) interface{} {
	t.Helper()
	plan := inspectPlan(frame)
	mounts := []RestoreMounts{}
	for i, disk := range frame.Disks {
		mounts = append(mounts, RestoreMounts{VolumeID: disk.VolumeID, ROX: ObjectRef{Kind: "PersistentVolumeClaim", Namespace: frame.Scope.Namespace, Name: frame.name(disk.VolumeID, "rox"), UID: plan.Disks[i].SourcePVCUID}, Previous: disk.PreviousPVC})
	}
	job, cm, err := RestoreCopyJob(frame, plan, mounts)
	if err != nil {
		t.Fatal(err)
	}
	if kind == "ConfigMap" {
		return cm
	}
	return job
}

// capability_id: rainbond.vm-restore.resource-repair-safety
func TestIndependentResourceLateEnsureCannotRecreateCleanedResource(t *testing.T) {
	f, frame, e := resourceFixture(t)
	api := independentResourceHTTP(t, e, f.scope.Namespace)
	job := independentInspectObject(t, frame, "ConfigMap")
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	var paused atomic.Bool
	e.BeforeMutation = func(ctx context.Context, _ Scope, _ string, key string) error {
		if key == "inspect-plan" && paused.CompareAndSwap(false, true) {
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}
	type result struct {
		ref ObjectRef
		err error
	}
	done := make(chan result, 1)
	go func() {
		ref, err := e.Ensure(context.Background(), f.scope, f.expect.OperationID, "inspect-plan", "restore-plan", "ConfigMap", job)
		done <- result{ref, err}
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("first Ensure never reached pre-mutation hook")
	}
	first, err := e.Ensure(context.Background(), f.scope, f.expect.OperationID, "inspect-plan", "restore-plan", "ConfigMap", job)
	if err != nil {
		t.Fatal(err)
	}
	// Cancellation/cleanup is still fenced, but a concurrent retry has already
	// observed and then fully deleted this exact, durable resource intent.
	if _, err = e.Store.executionUpdate(f.scope, f.expect.OperationID, "independent-cancelling", true, func(c Current) (Mutation, error) {
		c.State.Fence.Phase = "cancelling"
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = e.Delete(context.Background(), f.scope, f.expect.OperationID, "inspect-plan"); err != ErrRestorePending {
		t.Fatalf("initial exact delete=%v", err)
	}
	if err = e.Delete(context.Background(), f.scope, f.expect.OperationID, "inspect-plan"); err != nil {
		t.Fatal(err)
	}
	once.Do(func() { close(release) })
	var late result
	select {
	case late = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("late Ensure did not return")
	}
	c, err := f.store.Read(f.scope, f.expect.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	api.mu.Lock()
	creates := api.creates
	liveUID := ""
	if api.job != nil {
		liveUID = string(api.job.GetUID())
	}
	api.mu.Unlock()
	cleanupAgain := e.Delete(context.Background(), f.scope, f.expect.OperationID, "inspect-plan")
	t.Logf("first_uid=%s late_error=%v successful_creates=%d live_uid=%s receipt_uid=%s receipt_deleted=%t cleanup_again=%v fence_retained=%t", first.UID, late.err, creates, liveUID, c.Receipt.Execution.Intents["inspect-plan"].Object.UID, c.Receipt.Execution.Intents["inspect-plan"].Deleted, cleanupAgain, c.State.Fence != nil)
	if creates != 1 || liveUID != "" {
		t.Error("obsolete Ensure created a new resource after exact cleanup was already sealed complete")
	}
	if c.State.Fence == nil {
		t.Error("resource call released restore fence")
	}
}

func TestIndependentResourceHTTPUIDPreconditionAndFailedDeleteRemainPending(t *testing.T) {
	f, frame, e := resourceFixture(t)
	api := independentResourceHTTP(t, e, f.scope.Namespace)
	job := independentInspectObject(t, frame, "ConfigMap")
	first, err := e.Ensure(context.Background(), f.scope, f.expect.OperationID, "inspect-plan", "restore-plan", "ConfigMap", job)
	if err != nil {
		t.Fatal(err)
	}
	again, err := e.Ensure(context.Background(), f.scope, f.expect.OperationID, "inspect-plan", "restore-plan", "ConfigMap", job)
	if err != nil || !sameObject(first, again) {
		t.Fatalf("ordinary replay changed UID: %v", err)
	}
	e.BeforeMutation = func(_ context.Context, _ Scope, _ string, key string) error {
		if key == "delete:inspect-plan" {
			api.mu.Lock()
			api.job.SetUID("independent-replacement")
			api.mu.Unlock()
		}
		return nil
	}
	err = e.Delete(context.Background(), f.scope, f.expect.OperationID, "inspect-plan")
	if err != ErrRestoreUnknown {
		t.Fatalf("actual HTTP409 delete became success: %v", err)
	}
	c, readErr := f.store.Read(f.scope, f.expect.OperationID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	api.mu.Lock()
	deletes, creates := api.deletes, api.creates
	uids := append([]string(nil), api.deleteUIDs...)
	api.mu.Unlock()
	if creates != 1 || deletes != 0 || len(uids) != 1 || uids[0] != first.UID || c.Receipt.Execution.Intents["inspect-plan"].Deleted || c.State.Fence == nil {
		t.Fatal("UID replacement was deleted or failed request was marked done")
	}
	if err = e.Delete(context.Background(), f.scope, f.expect.OperationID, "inspect-plan"); err == nil {
		t.Fatal("replacement UID was accepted on retry")
	}
}

func TestIndependentResourceJobRealDynamicRoundTripMustSealUID(t *testing.T) {
	f, frame, e := resourceFixture(t)
	api := independentResourceHTTP(t, e, f.scope.Namespace)
	job := independentInspectObject(t, frame, "Job")
	ref, err := e.Ensure(context.Background(), f.scope, f.expect.OperationID, "inspect-job", "restore-inspect", "Job", job)
	want, _ := desiredObject(job, "Job")
	got, getErr := e.Dynamic.Resource(restoreGVR["Job"]).Namespace(f.scope.Namespace).Get(context.Background(), want.GetName(), metav1.GetOptions{})
	if getErr != nil {
		t.Fatal(getErr)
	}
	wantNumber, _, _ := unstructured.NestedFieldNoCopy(want.Object, "spec", "backoffLimit")
	gotNumber, _, _ := unstructured.NestedFieldNoCopy(got.Object, "spec", "backoffLimit")
	c, readErr := f.store.Read(f.scope, f.expect.OperationID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	api.mu.Lock()
	created := api.creates
	api.mu.Unlock()
	t.Logf("ensure_error=%v successful_creates=%d server_uid=%s sealed_uid=%s want_backoff_type=%T want_backoff=%v got_backoff_type=%T got_backoff=%v", err, created, got.GetUID(), c.Receipt.Execution.Intents["inspect-job"].Object.UID, wantNumber, wantNumber, gotNumber, gotNumber)
	if err != nil || ref.UID == "" || c.Receipt.Execution.Intents["inspect-job"].Object.UID != string(got.GetUID()) {
		t.Error("valid real dynamic-client Job response did not seal its created UID")
	}
}
