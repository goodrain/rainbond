package controller

// capability_id: rainbond.vm-snapshot.http-bounded-contract

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goodrain/rainbond/pkg/vmstorage"

	"github.com/goodrain/rainbond/api/handler"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
)

func TestVMSnapshotControllerCreateVMSnapshot(t *testing.T) {
	controller := &VMSnapshotController{
		createSnapshot: func(serviceID string, req *handler.VMSnapshotRequest) (*handler.VMSnapshotStatus, error) {
			if serviceID != "service-1" {
				t.Fatalf("unexpected service id %s", serviceID)
			}
			if req.Name != "snap-1" {
				t.Fatalf("unexpected request %#v", req)
			}
			return &handler.VMSnapshotStatus{SnapshotName: "snap-1"}, nil
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/v2/tenants/demo/services/demo/vm-snapshots", bytes.NewBufferString(`{"name":"snap-1","description":"demo"}`))
	req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("service_id"), "service-1"))
	req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("tenant_id"), "tenant"))
	recorder := httptest.NewRecorder()

	controller.CreateVMSnapshot(recorder, req)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d", recorder.Code)
	}
}

func TestVMSnapshotControllerCreateVMSnapshotRequiresName(t *testing.T) {
	controller := &VMSnapshotController{}

	req := httptest.NewRequest(http.MethodPost, "/v2/tenants/demo/services/demo/vm-snapshots", bytes.NewBufferString(`{"description":"demo"}`))
	req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("service_id"), "service-1"))
	req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("tenant_id"), "tenant"))
	recorder := httptest.NewRecorder()

	controller.CreateVMSnapshot(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

// capability_id: rainbond.vm-storage.snapshot-http-rejection
func TestVMSnapshotControllerPreservesStorageRejection(t *testing.T) {
	for _, status := range []string{"unsupported", "unknown"} {
		t.Run(status, func(t *testing.T) {
			cap := vmstorage.SnapshotSet{Capability: vmstorage.Capability{Status: status, ReasonCode: "participating_volume_unverified", Message: "cannot verify"}, Volumes: []vmstorage.VolumeCapability{{Key: "data", Capability: vmstorage.Capability{Status: status, ReasonCode: "read_failed"}}}}
			c := &VMSnapshotController{createSnapshot: func(string, *handler.VMSnapshotRequest) (*handler.VMSnapshotStatus, error) {
				return nil, &vmstorage.UnavailableError{Capability: cap}
			}}
			req := httptest.NewRequest(http.MethodPost, "/snapshot", bytes.NewBufferString(`{"name":"test"}`))
			req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("service_id"), "service-1"))
			req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("tenant_id"), "tenant"))
			w := httptest.NewRecorder()
			c.CreateVMSnapshot(w, req)
			expected := http.StatusConflict
			if status == "unknown" {
				expected = http.StatusServiceUnavailable
			}
			if w.Code != expected {
				t.Fatalf("got %d", w.Code)
			}
			var body struct {
				Bean struct {
					Capability vmstorage.SnapshotSet `json:"snapshot_capability"`
				} `json:"bean"`
			}
			if json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Bean.Capability.Status != status || len(body.Bean.Capability.Volumes) != 1 {
				t.Fatalf("lost per-disk reasons: %s", w.Body.String())
			}
		})
	}
}
