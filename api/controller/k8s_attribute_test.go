package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
)

// capability_id: rainbond.vm-runtime.password-hash-read-denial
func TestK8sAttributeControllerDoesNotReturnVMLoginHash(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet,
		"/v2/tenants/demo/services/service-vm/k8s-attributes", strings.NewReader(`{"name":"vm_login_password_hash"}`))
	request = request.WithContext(context.WithValue(request.Context(), ctxutil.ContextKey("service_id"), "service-vm"))
	response := httptest.NewRecorder()

	(&K8sAttributeController{}).getK8sAttributes(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("sensitive VM login attribute read status = %d, want %d", response.Code, http.StatusForbidden)
	}
	if strings.Contains(response.Body.String(), "vm_login_password_hash") || strings.Contains(response.Body.String(), "$6$") {
		t.Fatal("VM login hash read returned sensitive data")
	}
}
