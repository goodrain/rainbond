package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goodrain/rainbond/api/handler"
)

func TestVMCapabilityControllerGetCapabilities(t *testing.T) {
	controller := &VMCapabilityController{
		buildCapabilities: func() (*handler.VMCapability, error) {
			return &handler.VMCapability{
				ChunkUploadSupported: true,
				NetworkModes:         []string{"random", "fixed"},
			}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v2/tenants/demo/vm/capabilities", nil)
	recorder := httptest.NewRecorder()

	controller.GetCapabilities(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestVMCapabilityControllerGetCapabilitiesError(t *testing.T) {
	controller := &VMCapabilityController{
		buildCapabilities: func() (*handler.VMCapability, error) {
			return nil, errors.New("boom")
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v2/tenants/demo/vm/capabilities", nil)
	recorder := httptest.NewRecorder()

	controller.GetCapabilities(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", recorder.Code)
	}
}
