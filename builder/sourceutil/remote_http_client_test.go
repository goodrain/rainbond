package sourceutil

import (
	"crypto/tls"
	"net/http"
	"testing"
)

// capability_id: rainbond.vm-publish.http-artifact-image-build
func TestNewRemotePackageHTTPClientSkipsTLSVerifyForVMExportService(t *testing.T) {
	client := NewRemotePackageHTTPClient("https://virt-export-vm-root.default.svc/volumes/manual22/disk.img.gz")
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected http transport, got %T", client.Transport)
	}
	if transport.TLSClientConfig == nil || !transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("expected vm export service download client to skip internal self-signed cert verification")
	}
}

func TestNewRemotePackageHTTPClientKeepsTLSVerifyForPublicHTTPS(t *testing.T) {
	client := NewRemotePackageHTTPClient("https://example.com/disk.img.gz")
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected http transport, got %T", client.Transport)
	}
	if transport.TLSClientConfig != nil && transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("expected public https client to keep certificate verification enabled")
	}
}

func TestNewRemotePackageHTTPClientKeepsTLSVerifyForLookalikeExternalHost(t *testing.T) {
	client := NewRemotePackageHTTPClient("https://virt-export-vm-root.default.svc.evil.com/volumes/manual22/disk.img.gz")
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected http transport, got %T", client.Transport)
	}
	if transport.TLSClientConfig != nil && transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("expected lookalike external host to keep certificate verification enabled")
	}
}

func TestCloneTransportDoesNotMutateDefaultTransportTLSConfig(t *testing.T) {
	oldDefault := http.DefaultTransport
	http.DefaultTransport = &http.Transport{TLSClientConfig: &tls.Config{}}
	defer func() {
		http.DefaultTransport = oldDefault
	}()

	client := NewRemotePackageHTTPClient("https://virt-export-vm-root.default.svc.cluster.local/volumes/manual22/disk.img.gz")
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected http transport, got %T", client.Transport)
	}
	if transport.TLSClientConfig == nil || !transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("expected vm export service client to skip cert verification")
	}
	defaultTransport := http.DefaultTransport.(*http.Transport)
	if defaultTransport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("expected default transport TLS config to remain unchanged")
	}
}
