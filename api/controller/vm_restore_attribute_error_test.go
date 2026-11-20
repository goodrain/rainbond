package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/pkg/vmrestore"
)

// capability_id: rainbond.vm-restore.public-attribute-http
func TestPrivateAttributeUsesForbiddenResponse(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/k8s-attributes", nil)
	w := httptest.NewRecorder()
	returnK8sAttributeError(r, w, vmrestore.ErrInternalAttribute)
	if w.Code != http.StatusForbidden {
		t.Fatal("internal attribute denial was not HTTP403")
	}
}

func TestPublicAttributeErrorPreservesExistingBadRequestResponse(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/k8s-attributes", nil)
	w := httptest.NewRecorder()
	returnK8sAttributeError(r, w, bcode.NewBadRequest("public_attribute_fixed_code"))
	if w.Code != http.StatusBadRequest {
		t.Fatal("public attribute error mapping changed")
	}
}
