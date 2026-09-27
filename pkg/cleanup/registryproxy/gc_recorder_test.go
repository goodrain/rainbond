//go:build linux || darwin

package registryproxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
)

func TestGCRecorderRejectsForeignMeasurementsBeforeAdmission(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(http.StatusForbidden) }))
	defer server.Close()
	client, err := coordination.NewCoordinationClient(server.URL, "isolated-fixture", true)
	if err != nil {
		t.Fatal(err)
	}
	binding := coordination.StorageRegistration{StorageID: "owned", Generation: "one", VolumeUID: "volume", RootPath: "/registry"}
	request := coordination.CoordinationRequest{StorageID: "owned", Generation: "one", OperationID: "gc", Owner: "executor", Kind: "gc", Scope: "*", Fingerprint: "confirmation"}
	if _, err := NewGCRecorder(nil, binding, request); err == nil {
		t.Fatal("nil control client accepted")
	}
	wrong := request
	wrong.StorageID = "foreign"
	if _, err := NewGCRecorder(client, binding, wrong); err == nil {
		t.Fatal("foreign task accepted")
	}
	recorder, err := NewGCRecorder(client, binding, request)
	if err != nil {
		t.Fatal(err)
	}
	fp, _ := binding.Fingerprint()
	measured := StorageMeasurement{Protocol: 1, StorageID: "owned", Generation: "one", BindingFingerprint: fp, FilesystemID: "owned-fs", ObservedAt: time.Now(), TotalBytes: 1024, FreeBytes: 512, AvailableBytes: 256}
	foreign := measured
	foreign.Generation = "another"
	ctx := context.Background()
	if err := recorder.BeginGC(ctx, foreign); !errors.Is(err, ErrStorageIdentity) {
		t.Fatal(err)
	}
	if err := recorder.ObserveGC(ctx, foreign); !errors.Is(err, ErrStorageIdentity) {
		t.Fatal(err)
	}
	if err := recorder.CompleteGC(ctx, "invented"); !errors.Is(err, coordination.ErrCoordinationChanged) {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("invalid evidence reached control plane")
	}
	if err := recorder.BeginGC(ctx, measured); !errors.Is(err, coordination.ErrCoordinationDenied) {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("denied admission continued recording", calls)
	}
}

func TestGCJobRecorderUsesVerifiedGateWithoutFallback(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v2/cleanup/stores/owned/operations/gc/maintenance/enter-job" {
			t.Error("used generic admission or continued after rejection", r.URL.Path)
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	client, err := coordination.NewCoordinationClient(server.URL, "isolated-fixture", true)
	if err != nil {
		t.Fatal(err)
	}
	binding := coordination.StorageRegistration{StorageID: "owned", Generation: "one", VolumeUID: "volume", RootPath: "/registry"}
	request := coordination.CoordinationRequest{StorageID: "owned", Generation: "one", OperationID: "gc", Owner: "executor", Kind: "gc", Scope: "*", Fingerprint: "confirmation"}
	if _, err := NewGCJobRecorder(client, binding, request, coordination.GCExecutorLocator{}); err == nil {
		t.Fatal("missing executor accepted")
	}
	recorder, err := NewGCJobRecorder(client, binding, request, coordination.GCExecutorLocator{Pod: "job-pod", PodUID: "pod-uid"})
	if err != nil {
		t.Fatal(err)
	}
	fp, _ := binding.Fingerprint()
	measured := StorageMeasurement{Protocol: 1, StorageID: "owned", Generation: "one", BindingFingerprint: fp, FilesystemID: "owned-fs", ObservedAt: time.Now(), TotalBytes: 1024, FreeBytes: 512, AvailableBytes: 256}
	if err := recorder.BeginGC(context.Background(), measured); !errors.Is(err, coordination.ErrCoordinationDenied) {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("failed admission replayed", calls)
	}
}
