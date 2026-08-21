package version2

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestPluginStaticProxySuccess(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = w.Write([]byte("export const plugin = {};"))
	}))
	t.Cleanup(upstream.Close)

	resp := servePluginStaticRequest(t, func(string) (*v1alpha1.RBDPlugin, error) {
		return testRBDPlugin(upstream.URL), nil
	})

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	if got := resp.Header().Get("Content-Type"); got != "application/javascript; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want upstream JavaScript type", got)
	}
	if got := resp.Body.String(); got != "export const plugin = {};" {
		t.Fatalf("body = %q, want upstream JavaScript", got)
	}
}

func TestPluginStaticProxyDefaultsSuccessContentType(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(upstream.Close)

	resp := servePluginStaticRequest(t, func(string) (*v1alpha1.RBDPlugin, error) {
		return testRBDPlugin(upstream.URL), nil
	})

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	if got := resp.Header().Get("Content-Type"); got != "application/javascript" {
		t.Fatalf("Content-Type = %q, want application/javascript fallback", got)
	}
}

func TestPluginStaticProxyRejectsUpstreamErrors(t *testing.T) {
	tests := []struct {
		name           string
		upstreamStatus int
	}{
		{name: "not found", upstreamStatus: http.StatusNotFound},
		{name: "internal server error", upstreamStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/javascript")
				w.WriteHeader(tt.upstreamStatus)
				_, _ = w.Write([]byte("sensitive upstream error body"))
			}))
			t.Cleanup(upstream.Close)

			resp := servePluginStaticRequest(t, func(string) (*v1alpha1.RBDPlugin, error) {
				return testRBDPlugin(upstream.URL), nil
			})

			if resp.Code != http.StatusBadGateway {
				t.Fatalf("status = %d, want %d", resp.Code, http.StatusBadGateway)
			}
			if got := resp.Header().Get("Content-Type"); strings.Contains(got, "javascript") {
				t.Fatalf("error Content-Type = %q, must not be JavaScript", got)
			}
			if strings.Contains(resp.Body.String(), "sensitive upstream error body") {
				t.Fatalf("response leaked upstream body: %q", resp.Body.String())
			}
		})
	}
}

func TestPluginStaticProxyMapsConnectionFailureToBadGateway(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	frontendService := upstream.URL
	upstream.Close()

	resp := servePluginStaticRequest(t, func(string) (*v1alpha1.RBDPlugin, error) {
		return testRBDPlugin(frontendService), nil
	})

	if resp.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusBadGateway)
	}
	if got := resp.Header().Get("Content-Type"); strings.Contains(got, "javascript") {
		t.Fatalf("error Content-Type = %q, must not be JavaScript", got)
	}
}

func TestPluginStaticProxyMapsTimeoutToBadGateway(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	t.Cleanup(upstream.Close)

	client := &http.Client{Timeout: 20 * time.Millisecond}
	resp := servePluginStaticRequestWithClient(t, func(string) (*v1alpha1.RBDPlugin, error) {
		return testRBDPlugin(upstream.URL), nil
	}, client)

	if resp.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusBadGateway)
	}
	if got := resp.Header().Get("Content-Type"); strings.Contains(got, "javascript") {
		t.Fatalf("timeout Content-Type = %q, must not be JavaScript", got)
	}
}

func TestPluginStaticProxyDoesNotLogUpstreamBody(t *testing.T) {
	const sensitiveBody = "authorization=secret-upstream-token"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(sensitiveBody))
	}))
	t.Cleanup(upstream.Close)

	var logs bytes.Buffer
	logger := logrus.StandardLogger()
	originalOutput := logger.Out
	logger.SetOutput(&logs)
	t.Cleanup(func() {
		logger.SetOutput(originalOutput)
	})

	resp := servePluginStaticRequest(t, func(string) (*v1alpha1.RBDPlugin, error) {
		return testRBDPlugin(upstream.URL), nil
	})

	if resp.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusBadGateway)
	}
	if strings.Contains(logs.String(), sensitiveBody) {
		t.Fatalf("log leaked upstream body: %q", logs.String())
	}
}

func TestPluginStaticProxyMissingFrontendConfiguration(t *testing.T) {
	resp := servePluginStaticRequest(t, func(string) (*v1alpha1.RBDPlugin, error) {
		return testRBDPlugin(""), nil
	})

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusInternalServerError)
	}
	if got := resp.Header().Get("Content-Type"); strings.Contains(got, "javascript") {
		t.Fatalf("error Content-Type = %q, must not be JavaScript", got)
	}
}

func TestPluginStaticProxyClassifiesPluginLookupErrors(t *testing.T) {
	notFound := apierrors.NewNotFound(
		schema.GroupResource{Group: "rainbond.io", Resource: "rbdplugins"},
		"missing-plugin",
	)
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{
			name:       "wrapped Kubernetes not found",
			err:        fmt.Errorf("plugin lookup failed: %w", notFound),
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "generic lookup failure",
			err:        errors.New("Kubernetes API unavailable"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := servePluginStaticRequest(t, func(string) (*v1alpha1.RBDPlugin, error) {
				return nil, tt.err
			})

			if resp.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.Code, tt.wantStatus)
			}
			if got := resp.Header().Get("Content-Type"); strings.Contains(got, "javascript") {
				t.Fatalf("error Content-Type = %q, must not be JavaScript", got)
			}
		})
	}
}

func TestPluginStaticHTTPClientHasTimeout(t *testing.T) {
	if pluginStaticHTTPClient.Timeout <= 0 {
		t.Fatalf("plugin static HTTP client timeout = %s, want a positive timeout", pluginStaticHTTPClient.Timeout)
	}
}

func servePluginStaticRequest(t *testing.T, getPlugin rbdPluginGetter) *httptest.ResponseRecorder {
	return servePluginStaticRequestWithClient(t, getPlugin, pluginStaticHTTPClient)
}

func servePluginStaticRequestWithClient(t *testing.T, getPlugin rbdPluginGetter, client *http.Client) *httptest.ResponseRecorder {
	t.Helper()

	router := chi.NewRouter()
	router.Get("/v2/platform/static/plugins/{plugin_name}", func(w http.ResponseWriter, r *http.Request) {
		servePluginStaticWithClient(w, r, getPlugin, client)
	})
	req := httptest.NewRequest(http.MethodGet, "/v2/platform/static/plugins/test-plugin", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func testRBDPlugin(frontendService string) *v1alpha1.RBDPlugin {
	return &v1alpha1.RBDPlugin{
		ObjectMeta: metav1.ObjectMeta{Name: "test-plugin"},
		Spec: v1alpha1.RBDPluginSpec{
			FrontendService: frontendService,
		},
	}
}
