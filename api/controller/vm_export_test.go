package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	dbmodel "github.com/goodrain/rainbond/db/model"
)

func TestVMExportControllerStartVMExport(t *testing.T) {
	controller := &VMExportController{
		startExport: func(serviceID, exportID string, req *handler.VMExportRequest) (*handler.VMExportStatus, error) {
			if serviceID != "service-1" {
				t.Fatalf("unexpected service id %s", serviceID)
			}
			if exportID != "service-1" {
				t.Fatalf("unexpected export id %s", exportID)
			}
			if req.Name != "snapshot-1" {
				t.Fatalf("unexpected request %#v", req)
			}
			return &handler.VMExportStatus{
				ExportID: exportID,
				Status:   "exporting",
			}, nil
		},
		setEventStatus: func(ctx context.Context, status dbmodel.EventStatus) error {
			if status != dbmodel.EventStatusSuccess {
				t.Fatalf("unexpected event status %s", status)
			}
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/v2/tenants/demo/services/demo/vm-exports", bytes.NewBufferString(`{"name":"snapshot-1","export_all_disks":true}`))
	req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("service_id"), "service-1"))
	req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("event"), &dbmodel.ServiceEvent{EventID: "evt-1"}))
	recorder := httptest.NewRecorder()

	controller.StartVMExport(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestVMExportControllerStartVMExportSnapshotSource(t *testing.T) {
	controller := &VMExportController{
		startExport: func(serviceID, exportID string, req *handler.VMExportRequest) (*handler.VMExportStatus, error) {
			if req.SourceKind != "snapshot" {
				t.Fatalf("expected snapshot source, got %#v", req)
			}
			if req.SnapshotName != "snap-1" {
				t.Fatalf("expected snapshot name, got %#v", req)
			}
			return &handler.VMExportStatus{
				ExportID: exportID,
				Status:   "exporting",
			}, nil
		},
		setEventStatus: func(ctx context.Context, status dbmodel.EventStatus) error {
			if status != dbmodel.EventStatusSuccess {
				t.Fatalf("unexpected event status %s", status)
			}
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/v2/tenants/demo/services/demo/vm-exports", bytes.NewBufferString(`{"name":"snapshot-1","source_kind":"snapshot","snapshot_name":"snap-1"}`))
	req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("service_id"), "service-1"))
	req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("event"), &dbmodel.ServiceEvent{EventID: "evt-1"}))
	recorder := httptest.NewRecorder()

	controller.StartVMExport(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestVMExportControllerStartVMExportSnapshotSourceRequiresName(t *testing.T) {
	controller := &VMExportController{}

	req := httptest.NewRequest(http.MethodPost, "/v2/tenants/demo/services/demo/vm-exports", bytes.NewBufferString(`{"name":"snapshot-1","source_kind":"snapshot"}`))
	req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("service_id"), "service-1"))
	req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("event"), &dbmodel.ServiceEvent{EventID: "evt-1"}))
	recorder := httptest.NewRecorder()

	controller.StartVMExport(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestVMExportControllerStartVMExportClosedGuard(t *testing.T) {
	controller := &VMExportController{
		startExport: func(serviceID, exportID string, req *handler.VMExportRequest) (*handler.VMExportStatus, error) {
			return nil, handler.ErrServiceNotClosed
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/v2/tenants/demo/services/demo/vm-exports", bytes.NewBufferString(`{"name":"snapshot-1"}`))
	req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("service_id"), "service-1"))
	req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("event"), &dbmodel.ServiceEvent{EventID: "evt-1"}))
	recorder := httptest.NewRecorder()

	controller.StartVMExport(recorder, req)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", recorder.Code)
	}
}

func TestVMExportControllerGetVMExportStatus(t *testing.T) {
	controller := &VMExportController{
		getExportStatus: func(serviceID, exportID string) (*handler.VMExportStatus, error) {
			if serviceID != "service-1" {
				t.Fatalf("unexpected service id %s", serviceID)
			}
			if exportID != "evt-1" {
				t.Fatalf("unexpected export id %s", exportID)
			}
			return &handler.VMExportStatus{
				ExportID: exportID,
				Status:   "ready",
			}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v2/tenants/demo/services/demo/vm-exports/evt-1", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("service_id"), "service-1"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("export_id", "evt-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	recorder := httptest.NewRecorder()

	controller.GetVMExportStatus(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := body["bean"]; !ok {
		t.Fatalf("expected bean in response, got %s", recorder.Body.String())
	}
}

func TestVMExportControllerDeleteVMExport(t *testing.T) {
	controller := &VMExportController{
		deleteExport: func(serviceID, exportID string) error {
			if serviceID != "service-1" {
				t.Fatalf("unexpected service id %s", serviceID)
			}
			if exportID != "service-1" {
				t.Fatalf("unexpected export id %s", exportID)
			}
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/v2/tenants/demo/services/demo/vm-exports/service-1", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("service_id"), "service-1"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("export_id", "service-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	recorder := httptest.NewRecorder()

	controller.DeleteVMExport(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestVMExportControllerDeleteVMExportError(t *testing.T) {
	controller := &VMExportController{
		deleteExport: func(serviceID, exportID string) error {
			return errors.New("boom")
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/v2/tenants/demo/services/demo/vm-exports/service-1", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("service_id"), "service-1"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("export_id", "service-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	recorder := httptest.NewRecorder()

	controller.DeleteVMExport(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", recorder.Code)
	}
}

func TestVMExportControllerGetVMExportStatusError(t *testing.T) {
	controller := &VMExportController{
		getExportStatus: func(serviceID, exportID string) (*handler.VMExportStatus, error) {
			return nil, errors.New("boom")
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v2/tenants/demo/services/demo/vm-exports/evt-1", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("service_id"), "service-1"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("export_id", "evt-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	recorder := httptest.NewRecorder()

	controller.GetVMExportStatus(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", recorder.Code)
	}
}

func TestVMExportControllerPersistVMExport(t *testing.T) {
	controller := &VMExportController{
		persistExport: func(serviceID, exportID string, req *handler.VMExportPersistRequest) (*handler.VMExportPersistStatus, error) {
			if serviceID != "service-1" {
				t.Fatalf("unexpected service id %s", serviceID)
			}
			if exportID != "evt-1" {
				t.Fatalf("unexpected export id %s", exportID)
			}
			if req.AssetID != 101 {
				t.Fatalf("unexpected request %#v", req)
			}
			return &handler.VMExportPersistStatus{
				ExportID: "evt-1",
				Status:   "ready",
			}, nil
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/v2/tenants/demo/services/demo/vm-exports/evt-1/persist", bytes.NewBufferString(`{"asset_id":101,"asset_name":"demo-vm-export"}`))
	req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("service_id"), "service-1"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("export_id", "evt-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	recorder := httptest.NewRecorder()

	controller.PersistVMExport(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestVMExportControllerBuildVMAssetRestorePlan(t *testing.T) {
	controller := &VMExportController{
		restorePlan: func(req *handler.VMAssetRestorePlanRequest) (*handler.VMAssetRestorePlan, error) {
			if req.Manifest == nil || req.Manifest.RootDiskKey != "rootdisk" {
				t.Fatalf("unexpected request %#v", req)
			}
			return &handler.VMAssetRestorePlan{
				BootSourceFormat: "disk",
				DiskImports: []handler.VMAssetRestoreDiskImport{
					{VolumeName: "disk", DiskKey: "rootdisk", ImageURL: "https://signed.example/root.qcow2"},
				},
			}, nil
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/v2/tenants/demo/vm-assets/restore-plan", bytes.NewBufferString(`{"manifest":{"version":"v1","root_disk_key":"rootdisk","disks":[]}}`))
	recorder := httptest.NewRecorder()

	controller.BuildVMAssetRestorePlan(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}
