package version2

import (
	"net/http"
	"testing"
)

// capability_id: rainbond.plugin-proxy.restore-original-authorization
func TestRestorePluginBackendAuthorizationUsesOriginalAuthorization(t *testing.T) {
	header := http.Header{}
	header.Set("Authorization", "region-token")
	header.Set("X-Original-Authorization", "Bearer sk-test")

	restorePluginBackendAuthorization(header)

	if got := header.Get("Authorization"); got != "Bearer sk-test" {
		t.Fatalf("expected Authorization to be restored to original bearer token, got %q", got)
	}
	if got := header.Get("X-Original-Authorization"); got != "" {
		t.Fatalf("expected X-Original-Authorization to be removed before plugin backend, got %q", got)
	}
}

// capability_id: rainbond.plugin-proxy.restore-original-authorization
func TestRestorePluginBackendAuthorizationKeepsRegionAuthorizationWithoutOriginal(t *testing.T) {
	header := http.Header{}
	header.Set("Authorization", "region-token")

	restorePluginBackendAuthorization(header)

	if got := header.Get("Authorization"); got != "region-token" {
		t.Fatalf("expected Authorization to stay unchanged, got %q", got)
	}
}
