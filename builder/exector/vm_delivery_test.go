package exector

import (
	"bytes"
	"compress/gzip"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/event"
)

// capability_id: rainbond.vm-delivery.isolated-image
func TestVMDeliveryBuildNeverReusesSourceVersionOrWritesIt(t *testing.T) {
	item, err := NewImageShareItem([]byte(`{"service_id":"source","share_info":{"deploy_version":"original","image_info":{"vm_capture_id":"delivery-one","vm_image_source":"https://export.test/disk.img.gz"}}}`), nil, nil, "", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	build := item.vmDeliveryBuild()
	if build == nil || !build.SkipVersionStore || build.DeployVersion == "original" || build.Image == "source:original" {
		t.Fatalf("capture can overwrite or reuse source build: %#v", build)
	}
	if build.Image != "source:delivery-one" {
		t.Fatalf("capture is not idempotently named: %q", build.Image)
	}
	// A detached artifact build must not attempt a version DAO lookup.
	if err := build.storeVersionInfo("registry.test/captured:v1"); err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.vm-delivery.isolated-image
func TestVMDeliveryDownloadPreservesCompressedHTTPBytes(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write([]byte("captured OS and installed software")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(compressed.Bytes())
	}))
	defer server.Close()
	directory := t.TempDir()
	if err := downloadFile(directory, server.URL+"/disk.img.gz", "", event.NewLogger("delivery-download-test", nil), map[string]interface{}{"delivery_capture": true}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(directory, "disk.img.gz"))
	if err != nil || !bytes.Equal(content, compressed.Bytes()) {
		t.Fatal("HTTP transport changed the captured artifact", err)
	}
}

// capability_id: rainbond.vm-delivery.isolated-image
func TestVMDeliveryCopyChecksGzipIntegrityAndFreeSpace(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write([]byte("captured-disk-marker")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	available := func() (uint64, error) { return 4 << 30, nil }
	var destination bytes.Buffer
	n, err := copyVMDeliveryDisk(&destination, bytes.NewReader(compressed.Bytes()), available)
	if err != nil || n != int64(compressed.Len()) || !bytes.Equal(destination.Bytes(), compressed.Bytes()) {
		t.Fatal("compressed disk changed", n, err)
	}
	if _, err := copyVMDeliveryDisk(&bytes.Buffer{}, bytes.NewReader(compressed.Bytes()[:compressed.Len()-4]), available); err == nil {
		t.Fatal("truncated disk was accepted")
	}
	destination.Reset()
	if _, err := copyVMDeliveryDisk(&destination, bytes.NewReader(compressed.Bytes()), func() (uint64, error) { return 1, nil }); err == nil || destination.Len() != 0 {
		t.Fatal("download consumed exhausted temporary storage")
	}
	if _, err := copyVMDeliveryDisk(&destination, bytes.NewReader(compressed.Bytes()), func() (uint64, error) { return 0, errors.New("storage unavailable") }); err == nil {
		t.Fatal("storage observation failure was ignored")
	}
}
