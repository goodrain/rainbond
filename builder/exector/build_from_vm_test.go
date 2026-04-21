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

// capability_id: rainbond.vm-run.build-media-paths
func TestResolveVMBuildMediaDistinguishesISOAndDiskImages(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		want     vmBuildMedia
	}{
		{name: "plain iso", fileName: "ubuntu.iso", want: vmBuildMediaISO},
		{name: "plain qcow2", fileName: "ubuntu.qcow2", want: vmBuildMediaDisk},
		{name: "plain img", fileName: "ubuntu.img", want: vmBuildMediaDisk},
		{name: "gzip disk export", fileName: "disk.img.gz", want: vmBuildMediaDisk},
		{name: "xz disk export", fileName: "disk.qcow2.xz", want: vmBuildMediaDisk},
		{name: "tar disk export", fileName: "disk.tar", want: vmBuildMediaDisk},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveVMBuildMedia(tt.fileName)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestResolveVMBuildMediaRejectsUnknownFormats(t *testing.T) {
	if _, err := resolveVMBuildMedia("ubuntu.vmdk"); err == nil {
		t.Fatal("expected unknown vm media format to fail")
	}
}

func TestRenderVMDockerfileUsesDedicatedTemplatesPerMedia(t *testing.T) {
	isoDockerfile, err := renderVMDockerfile("installer.iso")
	if err != nil {
		t.Fatalf("render iso dockerfile: %v", err)
	}
	if isoDockerfile != "FROM scratch\nCOPY --chown=107:107 installer.iso /disk/\n" {
		t.Fatalf("unexpected iso dockerfile: %q", isoDockerfile)
	}

	diskDockerfile, err := renderVMDockerfile("rootdisk.qcow2")
	if err != nil {
		t.Fatalf("render qcow2 dockerfile: %v", err)
	}
	if diskDockerfile != "FROM scratch\nADD --chown=107:107 rootdisk.qcow2 /disk/\n" {
		t.Fatalf("unexpected disk dockerfile: %q", diskDockerfile)
	}
}

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

// capability_id: rainbond.vm-run.remote-package-download-export-token
func TestDownloadFileUsesVMExportToken(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if got := req.Header.Get("x-kubevirt-export-token"); got != "secret-token" {
			t.Fatalf("expected export token header, got %q", got)
		}
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET download, got %s", req.Method)
		}
		payload := []byte("vm")
		w.Header().Set("Content-Length", "2")
		_, _ = w.Write(payload)
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

	targetDir := t.TempDir()
	if err := downloadFile(targetDir, server.URL+"/disk.img.gz", event.GetTestLogger()); err != nil {
		t.Fatalf("expected download to succeed with vm export token, got %v", err)
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
