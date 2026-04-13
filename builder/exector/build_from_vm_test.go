package exector

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/builder/sourceutil"
	"github.com/goodrain/rainbond/event"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// capability_id: rainbond.vm-run.remote-package-download-export-cert
func TestDownloadFileUsesVMExportCert(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET download, got %s", req.Method)
		}
		payload := []byte("vm")
		w.Header().Set("Content-Length", "2")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	restoreDynamicClient := sourceutil.SetVMExportDynamicClientProviderForTest(func() dynamic.Interface {
		return newFakeVMExportDynamicClient(t, server.URL+"/disk.img.gz", mustEncodePEMCertificate(t, server.Certificate()))
	})
	defer restoreDynamicClient()

	targetDir := t.TempDir()
	if err := downloadFile(targetDir, server.URL+"/disk.img.gz", event.GetTestLogger()); err != nil {
		t.Fatalf("expected download to succeed with vm export cert, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "disk.img.gz")); err != nil {
		t.Fatalf("expected downloaded file to exist: %v", err)
	}
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
