package websocket

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/api/controller"
)

func TestChunkUploadPreflightAllowsConsoleHeaders(t *testing.T) {
	chunkController := &controller.ChunkUploadController{}
	req := httptest.NewRequest(http.MethodOptions, "/component/events/event-id/upload/init", nil)
	req.Header.Set("Origin", "http://console.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "content-type,authorization,x-team-name,x-region-name,x-requested-with")
	rec := httptest.NewRecorder()

	chunkController.HandleOptions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://console.example" {
		t.Fatalf("expected allow origin to echo request origin, got %q", got)
	}

	allowedHeaders := rec.Header().Get("Access-Control-Allow-Headers")
	for _, header := range []string{"content-type", "authorization", "x-team-name", "x-region-name", "x-requested-with"} {
		if !containsHeaderToken(allowedHeaders, header) {
			t.Fatalf("expected Access-Control-Allow-Headers %q to contain %q", allowedHeaders, header)
		}
	}
}

func TestPackageBuildCORSMiddlewareHandlesUnknownRoutePreflight(t *testing.T) {
	handler := packageBuildCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	req := httptest.NewRequest(http.MethodOptions, "/component/events/event-id/package_build/component/events/event-id/upload/init", nil)
	req.Header.Set("Origin", "http://console.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected unknown-route preflight status 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://console.example" {
		t.Fatalf("expected CORS allow origin on unknown-route preflight, got %q", got)
	}
}

func containsHeaderToken(headerValue, target string) bool {
	for _, token := range strings.Split(headerValue, ",") {
		if strings.EqualFold(strings.TrimSpace(token), target) {
			return true
		}
	}
	return false
}
