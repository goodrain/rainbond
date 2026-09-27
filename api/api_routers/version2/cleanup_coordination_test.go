package version2

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// capability_id: rainbond.cleanup.coordination-route-auth
func TestCleanupCoordinationRoutesAlwaysRequireRegionAuthentication(t *testing.T) {
	for _, configured := range []string{"", "isolated-fixture"} {
		t.Setenv("TOKEN", configured)
		router := (&V2{}).cleanupCoordinationRouter()
		for _, path := range []string{"/stores/discover", "/stores/store/participants/registry", "/registry/prepare", "/stores/store/status", "/stores/store/operations", "/stores/store/operations/op/finish", "/stores/store/operations/op/inspect", "/stores/store/operations/op/maintenance/request", "/stores/store/operations/op/maintenance/enter", "/stores/store/operations/op/maintenance/cancel", "/stores/store/operations/op/maintenance/complete", "/stores/store/operations/op/maintenance/restore", "/stores/store/operations/op/maintenance/restored", "/stores/store/operations/op/registry-permit", "/stores/store/operations/op/attempt", "/stores/store/operations/op/attempt/complete", "/stores/store/operations/op/upload", "/stores/store/uploads/lookup", "/stores/store/operations/op/upload/requests", "/stores/store/operations/op/upload/requests/finish"} {
			req := httptest.NewRequest("POST", path, strings.NewReader("["))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != 401 {
				t.Fatalf("%s unauthenticated status=%d", path, response.Code)
			}
			req = httptest.NewRequest("POST", path, strings.NewReader("["))
			req.Header.Set("Authorization", "Token isolated-fixture")
			response = httptest.NewRecorder()
			router.ServeHTTP(response, req)
			want := 401
			if configured != "" {
				want = 400
			}
			if response.Code != want {
				t.Fatalf("%s configured=%t status=%d", path, configured != "", response.Code)
			}
		}
	}
}
