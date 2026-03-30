package registry

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// capability_id: rainbond.registry.manifest-exists-oci
func TestManifestExistsAcceptsOCIManifestTypes(t *testing.T) {
	var acceptHeader string
	reg := &Registry{
		URL: "https://registry.example.com",
		Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/v2/demo/manifests/v1" {
				t.Fatalf("unexpected request path %q", r.URL.Path)
			}
			acceptHeader = r.Header.Get("Accept")
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		})},
		Logf: Quiet,
	}

	exists, err := reg.ManifestExists("demo", "v1")
	if err != nil {
		t.Fatalf("expected manifest existence check to succeed, got error: %v", err)
	}
	if !exists {
		t.Fatal("expected manifest to exist")
	}

	for _, want := range []string{
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.docker.distribution.manifest.list.v2+json",
		"application/vnd.oci.image.index.v1+json",
	} {
		if !strings.Contains(acceptHeader, want) {
			t.Fatalf("expected Accept header to contain %q, got %q", want, acceptHeader)
		}
	}
}
