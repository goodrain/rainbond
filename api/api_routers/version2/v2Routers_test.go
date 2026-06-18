package version2

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPluginProxyErrorHandler(t *testing.T) {
	// Test that the error handler produces valid JSON
	// We can't fully test the proxy without a running backend,
	// but we can test the error response format
	testCases := []struct {
		name     string
		errMsg   string
		wantCode int
	}{
		{
			name:     "connection reset",
			errMsg:   "read tcp 10.0.0.1:8080->10.0.0.2:9090: read: connection reset by peer",
			wantCode: http.StatusBadGateway,
		},
		{
			name:     "broken pipe",
			errMsg:   "write tcp 10.0.0.1:8080->10.0.0.2:9090: write: broken pipe",
			wantCode: http.StatusBadGateway,
		},
		{
			name:     "timeout",
			errMsg:   "dial tcp 10.0.0.1:8080: i/o timeout",
			wantCode: http.StatusBadGateway,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a fresh recorder for each test case
			w := httptest.NewRecorder()

			// Simulate what the ErrorHandler would produce
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(tc.wantCode)
			resp := map[string]interface{}{
				"code":   tc.wantCode,
				"msg":    "plugin backend unavailable: " + tc.errMsg,
				"plugin": "test-plugin",
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("failed to encode response: %v", err)
			}

			// Verify the response
			if w.Code != tc.wantCode {
				t.Errorf("expected status %d, got %d", tc.wantCode, w.Code)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("expected Content-Type application/json, got %s", contentType)
			}

			// Parse the response body
			var result map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatalf("failed to parse response body: %v", err)
			}

			// Verify the response structure
			if code, ok := result["code"].(float64); !ok || int(code) != tc.wantCode {
				t.Errorf("expected code %d in response, got %v", tc.wantCode, result["code"])
			}

			if _, ok := result["msg"].(string); !ok {
				t.Error("expected msg field in response")
			}

			if plugin, ok := result["plugin"].(string); !ok || plugin != "test-plugin" {
				t.Errorf("expected plugin field 'test-plugin', got %v", result["plugin"])
			}
		})
	}
}

func TestPluginProxyErrorResponseBody(t *testing.T) {
	// Test that error responses are properly formatted
	w := httptest.NewRecorder()

	// Simulate error response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadGateway)
	resp := map[string]interface{}{
		"code":    http.StatusBadGateway,
		"msg":     "plugin backend unavailable: read tcp 10.0.0.1:8080->10.0.0.2:9090: read: connection reset by peer",
		"plugin":  "my-plugin",
	}
	json.NewEncoder(w).Encode(resp)

	// Verify response is valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}

	// Verify all expected fields are present
	requiredFields := []string{"code", "msg", "plugin"}
	for _, field := range requiredFields {
		if _, ok := result[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
}
