package controller

// capability_id: rainbond.vm-snapshot.http-bounded-contract

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/pkg/vmrestore"
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

func newNativeRestoreHTTPRequest(method, path, body, snapshot, requestID string) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	route := chi.NewRouteContext()
	route.URLParams.Add("name", snapshot)
	if requestID != "" {
		route.URLParams.Add("request_id", requestID)
	}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, route)
	ctx = context.WithValue(ctx, ctxutil.ContextKey("service_id"), "service-1")
	ctx = context.WithValue(ctx, ctxutil.ContextKey("tenant_id"), "tenant")
	return req.WithContext(ctx)
}

// capability_id: rainbond.vm-restore.native-http
func TestVMSnapshotControllerStartsAndReadsNativeRestore(t *testing.T) {
	created := 0
	controller := &VMSnapshotController{
		startNativeRestore: func(_ context.Context, serviceID, snapshot, requestID string) (vmrestore.NativeRestoreDTO, error) {
			created++
			if serviceID != "service-1" || snapshot != "snap-1" || requestID != "request-a" {
				t.Fatalf("unexpected native restore scope: %s %s %s", serviceID, snapshot, requestID)
			}
			return vmrestore.NativeRestoreDTO{RestoreName: "restore-a", SnapshotName: snapshot, RequestID: requestID, Phase: "pending"}, nil
		},
		getNativeRestore: func(_ context.Context, serviceID, snapshot, requestID string) (vmrestore.NativeRestoreDTO, error) {
			return vmrestore.NativeRestoreDTO{RestoreName: "restore-a", SnapshotName: snapshot, RequestID: requestID, Phase: "succeeded", Complete: true}, nil
		},
	}
	post := httptest.NewRecorder()
	controller.RestoreVMSnapshot(post, newNativeRestoreHTTPRequest(http.MethodPost, "/restore", `{"request_id":"request-a"}`, "snap-1", ""))
	if post.Code != http.StatusAccepted || created != 1 {
		t.Fatalf("native restore was not accepted: %d %s", post.Code, post.Body.String())
	}
	get := httptest.NewRecorder()
	controller.GetVMSnapshotRestore(get, newNativeRestoreHTTPRequest(http.MethodGet, "/restore/request-a", "", "snap-1", "request-a"))
	if get.Code != http.StatusOK {
		t.Fatalf("native restore status failed: %d %s", get.Code, get.Body.String())
	}
}

// capability_id: rainbond.vm-restore.native-http
func TestVMSnapshotControllerRejectsInvalidNativeRestoreRequest(t *testing.T) {
	controller := &VMSnapshotController{}
	for _, body := range []string{`{}`, `{"request_id":"../bad"}`, `{"request_id":"request-a","power_after":"running"}`} {
		recorder := httptest.NewRecorder()
		controller.RestoreVMSnapshot(recorder, newNativeRestoreHTTPRequest(http.MethodPost, "/restore", body, "snap-1", ""))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("invalid native restore request accepted: %s => %d", body, recorder.Code)
		}
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
