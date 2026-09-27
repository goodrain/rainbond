package registryproxy

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestRegistryRequestClassification(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	for _, tc := range []struct{ method, path, kind, repository, target string }{
		{"GET", "/v2/", "read", "", ""},
		{"GET", "/v2/_catalog?n=100", "read", "", ""},
		{"GET", "/v2/team/app/tags/list", "read", "team/app", ""},
		{"GET", "/v2/team/app/manifests/v1", "read", "team/app", "v1"},
		{"PUT", "/v2/team/app/manifests/v1", "manifest_put", "team/app", "v1"},
		{"PUT", "/v2/team/blobs/uploads/inner/manifests/v1", "manifest_put", "team/blobs/uploads/inner", "v1"},
		{"GET", "/v2/team/manifests/inner/blobs/" + digest, "read", "team/manifests/inner", digest},
		{"DELETE", "/v2/team/app/manifests/" + digest, "manifest_delete", "team/app", digest},
		{"POST", "/v2/team/app/blobs/uploads/", "upload_start", "team/app", ""},
		{"PATCH", "/v2/team/app/blobs/uploads/upload-1?_state=opaque", "upload_part", "team/app", "upload-1"},
		{"PUT", "/v2/team/app/blobs/uploads/upload-1?digest=" + digest, "upload_complete", "team/app", "upload-1"},
		{"DELETE", "/v2/team/app/blobs/uploads/upload-1", "upload_abort", "team/app", "upload-1"},
		{"HEAD", "/v2/team/app/blobs/" + digest, "read", "team/app", digest},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			u, err := url.ParseRequestURI(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ClassifyRequest(tc.method, u)
			if err != nil || got.Kind != tc.kind || got.Repository != tc.repository || got.Target != tc.target || got.Mutating != (tc.method != http.MethodGet && tc.method != http.MethodHead) {
				t.Fatal(got, err)
			}
		})
	}
}

// capability_id: rainbond.cleanup.registry-request-scope
func TestRegistryProxyRejectsAmbiguousPathsAndUnselectedDeletion(t *testing.T) {
	for _, tc := range []struct{ method, path string }{
		{"DELETE", "/v2/app/manifests/v1"},
		{"DELETE", "/v2/app/blobs/sha256:" + strings.Repeat("a", 64)},
		{"PUT", "/v2/app/blobs/uploads/id"},
		{"PATCH", "/v2/app/blobs/uploads/"},
		{"POST", "/v2/app/blobs/uploads/?mount=invalid&from=other"},
		{"PUT", "/v2/app%2Fother/manifests/v1"},
		{"PUT", "/v2/app/../other/manifests/v1"},
		{"PUT", "/v2/app//other/manifests/v1"},
		{"PUT", "/v2/app/manifests/v1?digest=a&digest=b"},
		{"POST", "/v2/_catalog"},
		{"POST", "/debug/anything"},
	} {
		u, err := url.ParseRequestURI(tc.path)
		if err != nil {
			t.Fatal(err)
		}
		if result, err := ClassifyRequest(tc.method, u); err == nil {
			t.Fatalf("accepted %s %s: %#v", tc.method, tc.path, result)
		}
	}
}
func TestRegistryCrossRepositoryMountRetainsSourceScope(t *testing.T) {
	u, _ := url.ParseRequestURI("/v2/team/target/blobs/uploads/?from=team/source&mount=sha256:" + strings.Repeat("a", 64))
	r, err := ClassifyRequest("POST", u)
	if err != nil || r.Repository != "team/target" || r.MountFrom != "team/source" || r.Kind != "upload_start" {
		t.Fatal(r, err)
	}
}
