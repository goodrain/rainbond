package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/api/handler"
	apimodel "github.com/goodrain/rainbond/api/model"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
)

type restoreAttributeHTTPHandler struct {
	handler.ServiceHandler
	err         error
	row         *dbmodel.ComponentK8sAttributes
	calls       int
	componentID string
	tenantID    string
	name        string
}

func (h *restoreAttributeHTTPHandler) GetK8sAttribute(componentID, name string) (*dbmodel.ComponentK8sAttributes, error) {
	h.calls++
	h.componentID, h.name = componentID, name
	return h.row, h.err
}
func (h *restoreAttributeHTTPHandler) CreateK8sAttribute(tenantID, componentID string, attr *apimodel.ComponentK8sAttribute) error {
	h.calls++
	h.tenantID, h.componentID, h.name = tenantID, componentID, attr.Name
	return h.err
}
func (h *restoreAttributeHTTPHandler) UpdateK8sAttribute(componentID string, attr *apimodel.ComponentK8sAttribute) error {
	h.calls++
	h.componentID, h.name = componentID, attr.Name
	return h.err
}
func (h *restoreAttributeHTTPHandler) DeleteK8sAttribute(componentID, name string) error {
	h.calls++
	h.componentID, h.name = componentID, name
	return h.err
}

func restoreAttributeHTTPRequest(method, name string) *http.Request {
	r := httptest.NewRequest(method, "/k8s-attributes", strings.NewReader(fmt.Sprintf(`{"name":%q,"save_type":"string","attribute_value":"PUBLIC_SENTINEL","tenant_id":"body-foreign","component_id":"body-foreign","id":999}`, name)))
	ctx := context.WithValue(r.Context(), ctxutil.ContextKey("service_id"), "server-component")
	ctx = context.WithValue(ctx, ctxutil.ContextKey("tenant_id"), "server-tenant")
	return r.WithContext(ctx)
}

// capability_id: rainbond.vm-restore.public-attribute-http-methods
func TestPrivateAttributeHTTPMethodsUseForbiddenResponse(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			h := &restoreAttributeHTTPHandler{err: fmt.Errorf("private_attribute_code: %w", vmrestore.ErrInternalAttribute)}
			controller := &K8sAttributeController{serviceHandler: h}
			w := httptest.NewRecorder()
			controller.K8sAttributes(w, restoreAttributeHTTPRequest(method, vmrestore.StateKey))
			expectedCalls := 1
			if method == http.MethodPost || method == http.MethodPut {
				expectedCalls = 0
			}
			if w.Code != http.StatusForbidden || h.calls != expectedCalls || expectedCalls == 1 && h.componentID != "server-component" {
				t.Fatal("actual private attribute HTTP method did not return scoped forbidden")
			}
			if strings.Contains(w.Body.String(), "private_attribute_code") || strings.Contains(w.Body.String(), vmrestore.StateKey) {
				t.Fatal("private denial exposed internal details")
			}
		})
	}
}

func TestPublicAttributeHTTPMethodsPreserveOrdinaryResponse(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			h := &restoreAttributeHTTPHandler{row: &dbmodel.ComponentK8sAttributes{Name: "labels", AttributeValue: "PUBLIC_SENTINEL"}}
			controller := &K8sAttributeController{serviceHandler: h}
			w := httptest.NewRecorder()
			controller.K8sAttributes(w, restoreAttributeHTTPRequest(method, "labels"))
			if w.Code != http.StatusOK || h.calls != 1 || h.componentID != "server-component" || h.name != "labels" {
				t.Fatal("ordinary attribute HTTP method changed its behavior or scope")
			}
			if method == http.MethodPost && h.tenantID != "server-tenant" {
				t.Fatal("ordinary attribute create did not use server tenant")
			}
			if method == http.MethodGet && !strings.Contains(w.Body.String(), "PUBLIC_SENTINEL") {
				t.Fatal("ordinary attribute read lost its public value")
			}
		})
	}
}

func TestAttributeHTTPRejectsPersistedCredentialAlias(t *testing.T) {
	h := &restoreAttributeHTTPHandler{row: &dbmodel.ComponentK8sAttributes{Name: "vm_login_password_hash", AttributeValue: "PRIVATE_ATTRIBUTE_SENTINEL"}}
	controller := &K8sAttributeController{serviceHandler: h}
	w := httptest.NewRecorder()
	controller.K8sAttributes(w, restoreAttributeHTTPRequest(http.MethodGet, "vḿ_login_password_hash"))
	if w.Code != http.StatusForbidden || strings.Contains(w.Body.String(), "PRIVATE_ATTRIBUTE_SENTINEL") {
		t.Fatal("credential alias exposed the persisted private attribute")
	}
}

func TestAttributeHTTPDefaultsToExistingServiceHandler(t *testing.T) {
	if (&K8sAttributeController{}).attributeServiceHandler() != handler.GetServiceManager() {
		t.Fatal("default attribute controller changed its service handler")
	}
}
