package controller

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
)

func TestVMExportControllerCreateVMExport(t *testing.T) {
	controller := &VMExportController{
		createExport: func(serviceID string, req *handler.VMExportRequest) (*handler.VMExportStatus, error) {
			if serviceID != "service-1" {
				t.Fatalf("unexpected service id %s", serviceID)
			}
			if req.Name != "exp-1" {
				t.Fatalf("unexpected request %#v", req)
			}
			return &handler.VMExportStatus{ExportName: "exp-1"}, nil
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/v2/tenants/demo/services/demo/vm-exports", bytes.NewBufferString(`{"name":"exp-1","description":"demo"}`))
	req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("service_id"), "service-1"))
	recorder := httptest.NewRecorder()

	controller.CreateVMExport(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestVMExportControllerCreateVMExportRequiresName(t *testing.T) {
	controller := &VMExportController{}

	req := httptest.NewRequest(http.MethodPost, "/v2/tenants/demo/services/demo/vm-exports", bytes.NewBufferString(`{"description":"demo"}`))
	req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("service_id"), "service-1"))
	recorder := httptest.NewRecorder()

	controller.CreateVMExport(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestVMExportControllerGetVMExport(t *testing.T) {
	controller := &VMExportController{
		getExport: func(serviceID, exportName string) (*handler.VMExportStatus, error) {
			if serviceID != "service-1" {
				t.Fatalf("unexpected service id %s", serviceID)
			}
			if exportName != "exp-1" {
				t.Fatalf("unexpected export name %s", exportName)
			}
			return &handler.VMExportStatus{ExportName: "exp-1", Phase: "Ready"}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v2/tenants/demo/services/demo/vm-exports/exp-1", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("service_id"), "service-1"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "exp-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	recorder := httptest.NewRecorder()

	controller.GetVMExport(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}
