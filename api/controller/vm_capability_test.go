package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goodrain/rainbond/api/handler"
)

// capability_id: rainbond.vm-capabilities.password-bootstrap-env-gate
func TestVMCapabilityControllerReturnsPasswordBootstrapGate(t *testing.T) {
	controller := &VMCapabilityController{
		buildCapabilities: func() (*handler.VMCapability, error) {
			return &handler.VMCapability{PasswordBootstrapSupported: true}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v2/tenants/demo/vm/capabilities", nil)
	recorder := httptest.NewRecorder()
	controller.GetCapabilities(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	var response struct {
		Bean json.RawMessage `json:"bean"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal("unmarshal capability response")
	}
	var bean map[string]json.RawMessage
	if err := json.Unmarshal(response.Bean, &bean); err != nil {
		t.Fatal("unmarshal capability bean")
	}
	field, ok := bean["password_bootstrap_supported"]
	if !ok {
		t.Fatal("password capability field missing from JSON")
	}
	var supported bool
	if err := json.Unmarshal(field, &supported); err != nil {
		t.Fatal("unmarshal password capability field")
	}
	if !supported {
		t.Fatal("controller did not serialize the enabled password capability")
	}
}

func TestVMCapabilityControllerGetCapabilities(t *testing.T) {
	controller := &VMCapabilityController{
		buildCapabilities: func() (*handler.VMCapability, error) {
			return &handler.VMCapability{
				ChunkUploadSupported: true,
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
