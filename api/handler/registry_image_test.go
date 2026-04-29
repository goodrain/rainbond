package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	sourceregistry "github.com/goodrain/rainbond/builder/sources/registry"
)

func TestRegistryImageRepositoriesIgnoresCatalogAuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/_catalog" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errors":[{"code":"UNAUTHORIZED","message":"unauthorized to list catalog"}]}`))
	}))
	defer server.Close()

	action := &ServiceAction{
		registryCli: &sourceregistry.Registry{
			URL: server.URL,
			Client: &http.Client{
				Transport: &sourceregistry.ErrorTransport{Transport: server.Client().Transport},
			},
			Logf: sourceregistry.Quiet,
		},
	}

	repositories, err := action.RegistryImageRepositories("team-a")
	if err != nil {
		t.Fatalf("expected catalog auth failure to be ignored, got error: %v", err)
	}
	if len(repositories) != 0 {
		t.Fatalf("expected no repositories, got %v", repositories)
	}
}
