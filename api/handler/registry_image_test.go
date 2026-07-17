package handler

import (
	"fmt"
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

// capability_id: rainbond.registry.delete-vm-image-manifest
func TestDeleteRegistryImageManifestDeletesInternalVMImage(t *testing.T) {
	const digest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	var deleted bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodHead && r.URL.Path == "/v2/team-a/manifests/vm-image":
			w.Header().Set("Docker-Content-Digest", digest)
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete && r.URL.Path == fmt.Sprintf("/v2/team-a/manifests/%s", digest):
			deleted = true
			w.WriteHeader(http.StatusAccepted)
		default:
			t.Fatalf("unexpected registry request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	action := &ServiceAction{
		registryCli: &sourceregistry.Registry{
			URL:    server.URL,
			Client: server.Client(),
			Logf:   sourceregistry.Quiet,
		},
	}

	result, err := action.DeleteRegistryImageManifest("team-a:vm-image")

	if err != nil {
		t.Fatalf("expected delete to succeed, got error: %v", err)
	}
	if result.Repository != "team-a" || result.Tag != "vm-image" || !result.Deleted {
		t.Fatalf("unexpected delete result: %#v", result)
	}
	if !deleted {
		t.Fatalf("expected registry manifest to be deleted")
	}
}

// capability_id: rainbond.registry.delete-vm-image-manifest
func TestDeleteRegistryImageManifestRejectsExternalRegistry(t *testing.T) {
	action := &ServiceAction{
		registryCli: &sourceregistry.Registry{
			URL:    "http://goodrain.me",
			Client: http.DefaultClient,
			Logf:   sourceregistry.Quiet,
		},
	}

	_, err := action.DeleteRegistryImageManifest("registry.example.com/team-a:vm-image")

	if err == nil {
		t.Fatalf("expected external registry image to be rejected")
	}
	if err.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", err.Code)
	}
}

// capability_id: rainbond.registry.delete-vm-image-manifest
func TestDeleteRegistryImageManifestTreatsMissingManifestAsDeleted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead || r.URL.Path != "/v2/team-a/manifests/missing-image" {
			t.Fatalf("unexpected registry request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":[{"code":"MANIFEST_UNKNOWN","message":"manifest unknown"}]}`))
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

	result, err := action.DeleteRegistryImageManifest("team-a:missing-image")

	if err != nil {
		t.Fatalf("expected missing manifest to be ignored, got error: %v", err)
	}
	if result.Deleted || result.Reason != "not_found" {
		t.Fatalf("expected not_found result, got %#v", result)
	}
}
