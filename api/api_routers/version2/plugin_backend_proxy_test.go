package version2

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/pkg/interceptors"
)

// capability_id: rainbond.plugin.backend-proxy-contract
func TestPluginBackendProxyContract(t *testing.T) {
	tests := []struct {
		method string
		path   string
		status int
		body   string
	}{
		{http.MethodGet, "v1/models", http.StatusOK, `{"data":[{"id":"team/model"}]}`},
		{http.MethodPost, "v1/messages", http.StatusTooManyRequests, `{"type":"error","error":{"type":"rate_limit_error"}}`},
		{http.MethodPut, "api/v1/ai-engine/model-apis/api-1", http.StatusConflict, `{"code":409,"msg":"in use","data":null}`},
		{http.MethodDelete, "api/v1/ai-engine/sources/source-1", http.StatusNoContent, ""},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			const query = "api_id=first&api_id=second&trace=a%2Fb&name=hello+world"
			const requestBody = `{"model":"team/model","stream":true}`
			headers := map[string]string{
				"Authorization":            "region-token",
				"X-Original-Authorization": "Bearer client-key",
				"X-AI-Team-Name":           "team-a",
				"X-AI-Region-Name":         "region-b",
				"X-AI-Team-Namespace":      "team-a-ns",
				"X-AI-Actor-ID":            "42",
				"Anthropic-Version":        "2023-06-01",
				"Content-Type":             "application/json",
			}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method || r.URL.Path != "/"+tt.path || r.URL.RawQuery != query {
					t.Errorf("upstream target = %s %s, want %s /%s?%s", r.Method, r.URL, tt.method, tt.path, query)
				}
				for name, want := range headers {
					if got := r.Header.Get(name); got != want {
						t.Errorf("upstream %s = %q, want %q", name, got, want)
					}
				}
				body, err := io.ReadAll(r.Body)
				if err != nil || string(body) != requestBody {
					t.Errorf("upstream body = %q, error = %v", body, err)
				}
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.Header().Set("Retry-After", "7")
				w.Header().Set("X-Request-ID", "upstream-request")
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			}))
			t.Cleanup(upstream.Close)
			router := pluginBackendTestRouter(t, upstream.URL)
			req := httptest.NewRequest(tt.method, "/v2/platform/backend/plugins/rainbond-ai-engine/"+tt.path+"?"+query,
				strings.NewReader(requestBody))
			for name, value := range headers {
				req.Header.Set(name, value)
			}
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			if resp.Code != tt.status || resp.Body.String() != tt.body {
				t.Fatalf("response = %d %q, want %d %q", resp.Code, resp.Body.String(), tt.status, tt.body)
			}
			for name, want := range map[string]string{
				"Content-Type": "application/json; charset=utf-8", "Retry-After": "7", "X-Request-ID": "upstream-request",
			} {
				if got := resp.Header().Get(name); got != want {
					t.Errorf("response %s = %q, want %q", name, got, want)
				}
			}
		})
	}
}

func TestPluginBackendProxyStreamsPastShortTimeoutAndCancels(t *testing.T) {
	const first = "event: content_block_delta\ndata: {\"delta\":"
	const second = "{\"text\":\"hello\"}}\n\n"
	sendSecond := make(chan struct{})
	canceled := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = io.WriteString(w, first)
		w.(http.Flusher).Flush()
		select {
		case <-sendSecond:
			_, _ = io.WriteString(w, second)
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			close(canceled)
			return
		}
		<-r.Context().Done()
		close(canceled)
	}))
	t.Cleanup(upstream.Close)
	proxy := httptest.NewServer(pluginBackendTestRouter(t, upstream.URL))
	t.Cleanup(proxy.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, proxy.URL+"/v2/platform/backend/plugins/rainbond-ai-engine/v1/messages",
		strings.NewReader(`{"stream":true}`))
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("Content-Type"); got != "text/event-stream; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	readPluginChunk(t, resp.Body, first)
	if time.Since(started) > 5*time.Second {
		t.Fatal("first partial SSE chunk was buffered")
	}
	// Exceed both historical 15s and 20s proxy limits while the stream stays open.
	timer := time.NewTimer(21 * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	close(sendSecond)
	readPluginChunk(t, resp.Body, second)
	cancel()
	_ = resp.Body.Close()
	select {
	case <-canceled:
	case <-time.After(3 * time.Second):
		t.Fatal("client cancellation did not close the upstream request")
	}
}

func TestPluginBackendProxyDoesNotReplayPOST(t *testing.T) {
	for _, failure := range []string{"http-error", "transport-error"} {
		t.Run(failure, func(t *testing.T) {
			var attempts atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts.Add(1)
				_, _ = io.Copy(io.Discard, r.Body)
				if failure == "transport-error" {
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					_ = conn.Close()
					return
				}
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = io.WriteString(w, `{"error":{"message":"unavailable"}}`)
			}))
			t.Cleanup(upstream.Close)
			req := httptest.NewRequest(http.MethodPost, "/v2/platform/backend/plugins/rainbond-ai-engine/v1/chat/completions",
				strings.NewReader(`{"model":"team/model"}`))
			req.Header.Set("Idempotency-Key", "client-supplied")
			resp := httptest.NewRecorder()
			pluginBackendTestRouter(t, upstream.URL).ServeHTTP(resp, req)
			wantStatus := http.StatusServiceUnavailable
			if failure == "transport-error" {
				wantStatus = http.StatusBadGateway
			}
			if resp.Code != wantStatus || attempts.Load() != 1 {
				t.Fatalf("status = %d, attempts = %d; want %d and one attempt", resp.Code, attempts.Load(), wantStatus)
			}
		})
	}
}

func TestPluginBackendProxyPreservesTruncatedStream(t *testing.T) {
	for _, plugin := range []string{"rainbond-ai-engine", "other-plugin"} {
		t.Run(plugin, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, "data: partial\n\n")
				w.(http.Flusher).Flush()
				panic(http.ErrAbortHandler)
			}))
			t.Cleanup(upstream.Close)
			proxy := httptest.NewServer(pluginBackendTestRouter(t, upstream.URL))
			t.Cleanup(proxy.Close)
			client := &http.Client{Timeout: 3 * time.Second}
			resp, err := client.Post(proxy.URL+"/v2/platform/backend/plugins/"+plugin+"/v1/messages", "application/json",
				strings.NewReader(`{"stream":true}`))
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if string(body) != "data: partial\n\n" {
				t.Fatalf("truncated response body = %q", body)
			}
			if plugin == "rainbond-ai-engine" && err == nil {
				t.Fatal("truncated AI stream returned a clean EOF")
			}
			if plugin != "rainbond-ai-engine" && err != nil {
				t.Fatalf("other plugin's existing abort behavior changed: %v", err)
			}
		})
	}
}

func pluginBackendTestRouter(t *testing.T, backend string) http.Handler {
	t.Helper()
	router := chi.NewRouter()
	router.Use(interceptors.Recoverer)
	router.Use(interceptors.Timeout(5 * time.Second))
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete} {
		router.MethodFunc(method, "/v2/platform/backend/plugins/{plugin_name}/*", func(w http.ResponseWriter, r *http.Request) {
			servePluginBackend(w, r, func(name string) (*v1alpha1.RBDPlugin, error) {
				plugin := testRBDPlugin("")
				plugin.Name = name
				plugin.Spec.BackendService = backend
				return plugin, nil
			})
		})
	}
	return router
}

func readPluginChunk(t *testing.T, body io.Reader, want string) {
	t.Helper()
	chunk := make([]byte, len(want))
	if _, err := io.ReadFull(body, chunk); err != nil {
		t.Fatal(err)
	}
	if string(chunk) != want {
		t.Fatalf("SSE chunk = %q, want %q", chunk, want)
	}
}
