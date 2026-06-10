package controller

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi"
)

// capability_id: rainbond.api.kubeblocks.adapter-service-namespace
func TestKubeBlocksAdapterBaseURLUsesPluginNamespace(t *testing.T) {
	want := "http://kb-adapter-rbdplugin.rbd-plugins.svc:8080"
	if blockMechanicaBaseURL != want {
		t.Fatalf("blockMechanicaBaseURL = %q, want %q", blockMechanicaBaseURL, want)
	}
}

// capability_id: rainbond.api.kubeblocks.backup-repo-mutation-proxy
func TestKubeBlocksBackupRepoMutationProxy(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantMethod string
		wantPath   string
		handler    func(*KubeBlocksController) http.HandlerFunc
	}{
		{
			name:       "create backup repo",
			method:     http.MethodPost,
			path:       "/kubeblocks/backup-repos",
			body:       `{"name":"team-prod"}`,
			wantMethod: http.MethodPost,
			wantPath:   "/v1/backuprepos",
			handler:    func(c *KubeBlocksController) http.HandlerFunc { return c.CreateBackupRepo },
		},
		{
			name:       "update backup repo",
			method:     http.MethodPut,
			path:       "/kubeblocks/backup-repos/team-prod",
			body:       `{"bucket":"backup"}`,
			wantMethod: http.MethodPut,
			wantPath:   "/v1/backuprepos/team-prod",
			handler:    func(c *KubeBlocksController) http.HandlerFunc { return c.UpdateBackupRepo },
		},
		{
			name:       "delete backup repo",
			method:     http.MethodDelete,
			path:       "/kubeblocks/backup-repos/team-prod",
			wantMethod: http.MethodDelete,
			wantPath:   "/v1/backuprepos/team-prod",
			handler:    func(c *KubeBlocksController) http.HandlerFunc { return c.DeleteBackupRepo },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath, gotBody string
			restore := replaceDefaultTransport(func(req *http.Request) (*http.Response, error) {
				gotMethod = req.Method
				gotPath = req.URL.Path
				if req.Body != nil {
					bodyBytes, _ := io.ReadAll(req.Body)
					gotBody = string(bodyBytes)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"bean":{"ok":true}}`)),
				}, nil
			})
			defer restore()

			controller := &KubeBlocksController{}
			router := chi.NewRouter()
			switch tt.method {
			case http.MethodPost:
				router.Post("/kubeblocks/backup-repos", tt.handler(controller))
			case http.MethodPut:
				router.Put("/kubeblocks/backup-repos/{name}", tt.handler(controller))
			case http.MethodDelete:
				router.Delete("/kubeblocks/backup-repos/{name}", tt.handler(controller))
			}

			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status code = %d, body = %s", rec.Code, rec.Body.String())
			}
			if gotMethod != tt.wantMethod {
				t.Fatalf("method = %s, want %s", gotMethod, tt.wantMethod)
			}
			if gotPath != tt.wantPath {
				t.Fatalf("path = %s, want %s", gotPath, tt.wantPath)
			}
			if tt.body != "" && gotBody != tt.body {
				t.Fatalf("body = %s, want %s", gotBody, tt.body)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func replaceDefaultTransport(fn roundTripFunc) func() {
	old := http.DefaultTransport
	http.DefaultTransport = fn
	return func() {
		http.DefaultTransport = old
	}
}
