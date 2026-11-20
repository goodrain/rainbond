package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/util/bcode"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/pkg/vmstorage"
)

// capability_id: rainbond.vm-storage.existing-snapshot-capability-http
func TestExistingSnapshotCapabilityHTTPUsesOnlyAuthenticatedScope(t *testing.T) {
	called := 0
	c := &ExistingVMSnapshotCapabilityController{inspect: func(ctx context.Context, tenant, service string) (*handler.ExistingVMSnapshotCapability, error) {
		called++
		if tenant != "tenant-auth" || service != "service-auth" {
			t.Fatalf("client scope trusted: %s/%s", tenant, service)
		}
		return &handler.ExistingVMSnapshotCapability{Capability: vmstorage.Capability{Status: "unknown", ReasonCode: "pvc_identity_unverified", Message: "not verified"}, Operation: "create_snapshot", Complete: true, Volumes: []handler.ExistingSnapshotDisk{{VolumeCapability: vmstorage.VolumeCapability{Key: "manual42", Capability: vmstorage.Capability{Status: "unknown", ReasonCode: "pvc_identity_unverified"}}}}}, nil
	}}
	req := httptest.NewRequest(http.MethodGet, "/vm-snapshot-capability?namespace=other&claim=foreign&tenant_id=other", strings.NewReader(`{"namespace":"other","claim":"foreign"}`))
	ctx := context.WithValue(req.Context(), ctxutil.ContextKey("tenant_id"), "tenant-auth")
	ctx = context.WithValue(ctx, ctxutil.ContextKey("service_id"), "service-auth")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	c.Get(w, req)
	if w.Code != 200 || called != 1 {
		t.Fatalf("HTTP%d calls%d", w.Code, called)
	}
	var body struct {
		Bean handler.ExistingVMSnapshotCapability `json:"bean"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Bean.Supported || body.Bean.Status != "unknown" || !body.Bean.Complete || body.Bean.Operation != "create_snapshot" || len(body.Bean.Volumes) != 1 {
		t.Fatalf("lost read-only contract: %s", w.Body.String())
	}
}

// capability_id: rainbond.vm-storage.existing-snapshot-capability-http
func TestExistingSnapshotCapabilityHTTPRejectsMissingAndForeignContext(t *testing.T) {
	c := &ExistingVMSnapshotCapabilityController{inspect: func(context.Context, string, string) (*handler.ExistingVMSnapshotCapability, error) {
		t.Fatal("unauthenticated inspect called")
		return nil, nil
	}}
	w := httptest.NewRecorder()
	c.Get(w, httptest.NewRequest(http.MethodGet, "/?tenant_id=claimed&service_id=claimed", nil))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	c.inspect = func(context.Context, string, string) (*handler.ExistingVMSnapshotCapability, error) {
		return nil, &bcode.Code{Status: 404, Code: 404, Message: "组件不存在"}
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), ctxutil.ContextKey("tenant_id"), "tenant")
	ctx = context.WithValue(ctx, ctxutil.ContextKey("service_id"), "foreign-service")
	w = httptest.NewRecorder()
	c.Get(w, req.WithContext(ctx))
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
}
