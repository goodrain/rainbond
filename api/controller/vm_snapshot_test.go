package controller

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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
	recorder := httptest.NewRecorder()

	controller.CreateVMSnapshot(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestVMSnapshotControllerCreateVMSnapshotRequiresName(t *testing.T) {
	controller := &VMSnapshotController{}

	req := httptest.NewRequest(http.MethodPost, "/v2/tenants/demo/services/demo/vm-snapshots", bytes.NewBufferString(`{"description":"demo"}`))
	req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("service_id"), "service-1"))
	recorder := httptest.NewRecorder()

	controller.CreateVMSnapshot(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}
