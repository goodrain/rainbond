package parser

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goodrain/rainbond/builder/sourceutil"
	"github.com/goodrain/rainbond/event"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// capability_id: rainbond.vm-run.remote-package-probe
func TestVMServiceParseRemoteURLPrefersHeadProbe(t *testing.T) {
	withDefaultClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		case http.MethodHead:
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(http.NoBody),
				Request:    req,
			}, nil
		case http.MethodGet:
			return nil, errors.New("read: connection reset by peer")
		default:
			t.Fatalf("unexpected request method %s", req.Method)
			return nil, nil
		}
	}))

	parser := CreateVMServiceParse("https://example.com/ubuntu.iso", event.GetTestLogger())

	if errors := parser.Parse(); len(errors) != 0 {
		t.Fatalf("expected no parse errors, got %#v", errors)
	}
}

// capability_id: rainbond.vm-run.remote-package-probe-range-fallback
func TestVMServiceParseRemoteURLFallsBackToRangeGet(t *testing.T) {
	withDefaultClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		case http.MethodHead:
			return &http.Response{
				StatusCode: http.StatusMethodNotAllowed,
				Body:       io.NopCloser(http.NoBody),
				Request:    req,
			}, nil
		case http.MethodGet:
			if got := req.Header.Get("Range"); got != "bytes=0-0" {
				t.Fatalf("expected Range header bytes=0-0, got %q", got)
			}
			return &http.Response{
				StatusCode: http.StatusPartialContent,
				Body:       io.NopCloser(http.NoBody),
				Request:    req,
			}, nil
		default:
			t.Fatalf("unexpected request method %s", req.Method)
			return nil, nil
		}
	}))

	parser := CreateVMServiceParse("https://example.com/ubuntu.iso", event.GetTestLogger())

	if errors := parser.Parse(); len(errors) != 0 {
		t.Fatalf("expected no parse errors, got %#v", errors)
	}
}

// capability_id: rainbond.vm-run.remote-package-probe-export-cert
func TestVMServiceParseRemoteURLUsesVMExportCert(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodHead {
			t.Fatalf("expected HEAD probe, got %s", req.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	restoreDynamicClient := sourceutil.SetVMExportDynamicClientProviderForTest(func() dynamic.Interface {
		return newFakeVMExportDynamicClient(t, server.URL+"/disk.img.gz", mustEncodePEMCertificate(t, server.Certificate()))
	})
	defer restoreDynamicClient()

	parser := CreateVMServiceParse(server.URL+"/disk.img.gz", event.GetTestLogger())

	if errors := parser.Parse(); len(errors) != 0 {
		t.Fatalf("expected no parse errors, got %#v", errors)
	}
}

// capability_id: rainbond.vm-run.remote-package-probe-export-token
func TestVMServiceParseRemoteURLUsesVMExportToken(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if got := req.Header.Get("x-kubevirt-export-token"); got != "secret-token" {
			t.Fatalf("expected export token header, got %q", got)
		}
		if req.Method != http.MethodHead {
			t.Fatalf("expected HEAD probe, got %s", req.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	restoreDynamicClient := sourceutil.SetVMExportDynamicClientProviderForTest(func() dynamic.Interface {
		return newFakeVMExportDynamicClientWithToken(t, server.URL+"/disk.img.gz", mustEncodePEMCertificate(t, server.Certificate()), "export-token")
	})
	defer restoreDynamicClient()
	restoreSecretGetter := sourceutil.SetVMExportSecretGetterForTest(func(namespace, name string) ([]byte, error) {
		if namespace != "default" {
			t.Fatalf("expected default namespace, got %s", namespace)
		}
		if name != "export-token" {
			t.Fatalf("expected export-token secret, got %s", name)
		}
		return []byte("secret-token"), nil
	})
	defer restoreSecretGetter()

	parser := CreateVMServiceParse(server.URL+"/disk.img.gz", event.GetTestLogger())

	if errors := parser.Parse(); len(errors) != 0 {
		t.Fatalf("expected no parse errors, got %#v", errors)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func withDefaultClient(t *testing.T, transport http.RoundTripper) {
	t.Helper()

	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: transport}
	t.Cleanup(func() {
		http.DefaultClient = originalClient
	})
}

func newFakeVMExportDynamicClient(t *testing.T, url, cert string) dynamic.Interface {
	t.Helper()

	gvr := schema.GroupVersionResource{
		Group:    "export.kubevirt.io",
		Version:  "v1beta1",
		Resource: "virtualmachineexports",
	}
	scheme := runtime.NewScheme()
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{gvr: "VirtualMachineExportList"},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "export.kubevirt.io/v1beta1",
				"kind":       "VirtualMachineExport",
				"metadata": map[string]interface{}{
					"name":      "export-1",
					"namespace": "default",
				},
				"status": map[string]interface{}{
					"links": map[string]interface{}{
						"internal": map[string]interface{}{
							"cert": cert,
							"volumes": []interface{}{
								map[string]interface{}{
									"name": "disk",
									"formats": []interface{}{
										map[string]interface{}{
											"format": "gzip",
											"url":    url,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	)
}

func newFakeVMExportDynamicClientWithToken(t *testing.T, url, cert, tokenSecretRef string) dynamic.Interface {
	t.Helper()

	gvr := schema.GroupVersionResource{
		Group:    "export.kubevirt.io",
		Version:  "v1beta1",
		Resource: "virtualmachineexports",
	}
	scheme := runtime.NewScheme()
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{gvr: "VirtualMachineExportList"},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "export.kubevirt.io/v1beta1",
				"kind":       "VirtualMachineExport",
				"metadata": map[string]interface{}{
					"name":      "export-1",
					"namespace": "default",
				},
				"status": map[string]interface{}{
					"tokenSecretRef": tokenSecretRef,
					"links": map[string]interface{}{
						"internal": map[string]interface{}{
							"cert": cert,
							"volumes": []interface{}{
								map[string]interface{}{
									"name": "disk",
									"formats": []interface{}{
										map[string]interface{}{
											"format": "gzip",
											"url":    url,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	)
}

func mustEncodePEMCertificate(t *testing.T, cert *x509.Certificate) string {
	t.Helper()

	if cert == nil {
		t.Fatal("expected tls certificate")
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	if len(pemBytes) == 0 {
		t.Fatal("expected pem certificate bytes")
	}
	return base64.StdEncoding.EncodeToString(pemBytes)
}
