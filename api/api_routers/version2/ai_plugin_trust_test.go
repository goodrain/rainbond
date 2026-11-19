package version2

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// This synthetic vector is shared with Console's HMAC implementation. It is
// never a runtime cluster credential.
const aiPlatformFixtureToken = "04d2bf8e9084ae34a9e41113bc003fea0bb0f5df6d983ac66b3ff66d5d7e86df"

// capability_id: rainbond.plugin.ai-management-platform-trust
func TestAIManagementProxyRequiresVerifiedRegionIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, configured, authorization string
		wantStatus                      int
	}{
		{"configured token", "region-fixture-token", "Token region-fixture-token", http.StatusOK},
		{"configured bearer", "region-fixture-token", "Bearer region-fixture-token", http.StatusOK},
		{"missing authorization", "region-fixture-token", "", http.StatusUnauthorized},
		{"wrong authorization", "region-fixture-token", "Token wrong-fixture", http.StatusUnauthorized},
		{"malformed authorization", "region-fixture-token", "Token region-fixture-token extra", http.StatusUnauthorized},
		{"token unconfigured", "", "Token region-fixture-token", http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TOKEN", tc.configured)
			var attempts atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts.Add(1)
				if r.Header.Get("X-AI-Platform-Token") != aiPlatformFixtureToken {
					t.Error("management request did not receive the derived platform credential")
				}
				if r.Header.Get("Authorization") != "" {
					t.Error("region master credential reached the plugin")
				}
				for key, value := range map[string]string{
					"X-AI-Team-Name": "team-a", "X-AI-Region-Name": "region-b",
					"X-AI-Actor-ID": "42", "X-AI-Can-Read-Secrets": "true",
				} {
					if r.Header.Get(key) != value {
						t.Errorf("verified management context lost header %s", key)
					}
				}
				_, _ = io.WriteString(w, `{"code":200,"msg":"success","data":{}}`)
			}))
			defer upstream.Close()
			proxy := httptest.NewServer(pluginBackendTestRouter(t, upstream.URL))
			defer proxy.Close()
			req, err := http.NewRequest(http.MethodGet,
				proxy.URL+"/v2/platform/backend/plugins/rainbond-ai-engine/api/v1/ai-engine/api-keys/1/secret", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", tc.authorization)
			// Even the correct plugin credential cannot substitute for Region auth.
			req.Header.Set("X-AI-Platform-Token", aiPlatformFixtureToken)
			req.Header.Set("X-AI-Team-Name", "team-a")
			req.Header.Set("X-AI-Region-Name", "region-b")
			req.Header.Set("X-AI-Actor-ID", "42")
			req.Header.Set("X-AI-Can-Read-Secrets", "true")
			response, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, tc.wantStatus)
			}
			wantAttempts := int32(0)
			if tc.wantStatus == http.StatusOK {
				wantAttempts = 1
			}
			if attempts.Load() != wantAttempts {
				t.Fatalf("upstream attempts = %d, want %d", attempts.Load(), wantAttempts)
			}
			if strings.Contains(string(body), "region-fixture-token") || strings.Contains(string(body), aiPlatformFixtureToken) {
				t.Fatal("response exposed an authentication credential")
			}
		})
	}
}

func TestAIPlatformCredentialDoesNotReachUnprotectedRoutes(t *testing.T) {
	t.Setenv("TOKEN", "region-fixture-token")
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "v1/messages"},
		{http.MethodGet, "v1/models"},
		{http.MethodGet, "healthz"},
		{http.MethodGet, "static/main.js"},
		{http.MethodGet, "api/v1/ai-engine/media/signed-asset?signature=fixture&expires=123"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-AI-Platform-Token") != "" || r.Header.Get("Authorization") != "" {
					t.Error("unprotected AI route received platform credentials")
				}
				if r.Header.Get("X-Original-Authorization") != "Bearer business-fixture" || r.Header.Get("x-api-key") != "business-fixture" {
					t.Error("public business credentials changed")
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer upstream.Close()
			req := httptest.NewRequest(tc.method, "/v2/platform/backend/plugins/rainbond-ai-engine/"+tc.path, nil)
			req.Header.Set("Authorization", "Token region-fixture-token")
			req.Header.Set("X-AI-Platform-Token", "forged-fixture")
			req.Header.Set("X-Original-Authorization", "Bearer business-fixture")
			req.Header.Set("x-api-key", "business-fixture")
			response := httptest.NewRecorder()
			pluginBackendTestRouter(t, upstream.URL).ServeHTTP(response, req)
			if response.Code != http.StatusNoContent {
				t.Fatalf("status = %d", response.Code)
			}
		})
	}
}

func TestAIPlatformTrustDoesNotExemptOtherMediaOperations(t *testing.T) {
	t.Setenv("TOKEN", "region-fixture-token")
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "api/v1/ai-engine/media/signed-asset"},
		{http.MethodGet, "api/v1/ai-engine/media/signed-asset/other"},
		{http.MethodGet, "api/v1/ai-engine/media/"},
		{http.MethodGet, "api/v1/ai-engine/media/."},
		{http.MethodGet, "api/v1/ai-engine/media/.."},
		{http.MethodGet, "api/v1/ai-engine"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/v2/platform/backend/plugins/rainbond-ai-engine/"+tc.path, nil)
			req.Header.Set("X-AI-Platform-Token", "forged-fixture")
			response := httptest.NewRecorder()
			pluginBackendTestRouter(t, "http://127.0.0.1:1").ServeHTTP(response, req)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want unauthorized", response.Code)
			}
		})
	}
}

func TestOtherPluginCredentialForwardingIsUnchanged(t *testing.T) {
	t.Setenv("TOKEN", "region-fixture-token")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Token region-fixture-token" || r.Header.Get("X-AI-Platform-Token") != "other-plugin-fixture" {
			t.Error("unrelated plugin header defaults changed")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	req := httptest.NewRequest(http.MethodGet, "/v2/platform/backend/plugins/other-plugin/api/v1/ai-engine/model-apis", nil)
	req.Header.Set("Authorization", "Token region-fixture-token")
	req.Header.Set("X-AI-Platform-Token", "other-plugin-fixture")
	response := httptest.NewRecorder()
	pluginBackendTestRouter(t, upstream.URL).ServeHTTP(response, req)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d", response.Code)
	}
}
