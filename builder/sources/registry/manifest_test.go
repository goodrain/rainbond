package registry

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestManifestDigestV2AcceptsOCIManifest(t *testing.T) {
	const expectedDigest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	reg := &Registry{
		URL: "http://registry.example.com",
		Client: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodHead {
					t.Fatalf("expected HEAD request, got %s", r.Method)
				}
				accept := r.Header.Get("Accept")
				if !strings.Contains(accept, "application/vnd.docker.distribution.manifest.v2+json") {
					t.Fatalf("expected schema2 manifest in Accept header, got %q", accept)
				}
				if !strings.Contains(accept, "application/vnd.oci.image.manifest.v1+json") {
					t.Fatalf("expected OCI manifest in Accept header, got %q", accept)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header: http.Header{
						"Docker-Content-Digest": []string{expectedDigest},
					},
					Body: ioutil.NopCloser(strings.NewReader("")),
				}, nil
			}),
		},
		Logf: Quiet,
	}

	digest, err := reg.ManifestDigestV2("rainbond/app", "latest")
	if err != nil {
		t.Fatalf("expected OCI manifest to be accepted, got error: %v", err)
	}
	if digest.String() != expectedDigest {
		t.Fatalf("expected digest %s, got %s", expectedDigest, digest.String())
	}
}
