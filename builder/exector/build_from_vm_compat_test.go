package exector

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/goodrain/rainbond/event"
)

// capability_id: rainbond.vm-build.safe-url-filename
func TestVMDownloadUsesDecodedURLPathName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("public-vm-disk-content"))
	}))
	defer server.Close()
	for _, tt := range []struct{ suffix, name string }{
		{"/%E4%B9%8C%E7%8F%AD%E5%9B%BE.img?download=/other.iso#ignored.qcow2", "乌班图.img"},
		{"/folder/ubuntu%2ERAW?download=1", "ubuntu.RAW"},
		{"/乌班图.qcow2#other/file.iso", "乌班图.qcow2"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := downloadFile(dir, server.URL+tt.suffix, "", event.GetTestLogger(), nil); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(filepath.Join(dir, tt.name))
			if err != nil || string(content) != "public-vm-disk-content" {
				t.Fatalf("decoded download content: %v", err)
			}
			if _, err := renderVMDockerfile(tt.name); err != nil {
				t.Fatalf("downloaded filename must remain buildable: %v", err)
			}
		})
	}
}

func TestVMDownloadRejectsUnsafeURLNamesBeforeRequest(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte("must-not-download"))
	}))
	defer server.Close()
	for _, suffix := range []string{
		"", "/", "/folder/", "/%2Fdisk.img", "/%5Cdisk.img", "/%2E%2E%2Fdisk.img",
		"/%00disk.img", "/disk%0ARUN.img", "/disk%20name.img", "/disk%24HOME.img",
		"/--from=other.img", "/disk%60cmd%60.img", "/%252Fdisk.img",
	} {
		t.Run(suffix, func(t *testing.T) {
			dir := t.TempDir()
			if err := downloadFile(dir, server.URL+suffix, "", event.GetTestLogger(), nil); err == nil {
				t.Fatal("unsafe name must fail")
			}
			files, err := os.ReadDir(dir)
			if err != nil || len(files) != 0 {
				t.Fatal("rejected URL must not create a file")
			}
		})
	}
	if requests.Load() != 0 {
		t.Fatal("invalid URL filename must be rejected before a network request")
	}
}

// capability_id: rainbond.vm-run.raw-image-format
func TestVMNativeRawBuildStrategies(t *testing.T) {
	for _, name := range []string{"disk.raw", "disk.RAW", "磁盘.raw"} {
		t.Run(name, func(t *testing.T) {
			got, err := renderVMDockerfile(name)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(got, "COPY "+name+" /work/source.img\n") || !strings.Contains(got, "qemu-img info --output=json") {
				t.Fatal("native raw must use content-aware disk conversion")
			}
		})
	}
	for _, name := range []string{"disk.raw.gz", "disk.RAW.GZ", "磁盘.raw.gz"} {
		t.Run(name, func(t *testing.T) {
			got, err := renderVMDockerfile(name)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(got, "COPY "+name+" /disk/disk.img.gz\n") || !strings.Contains(got, "RUN ln -sf") || strings.Contains(got, "qemu-img") {
				t.Fatal("gzip raw must retain the gzip HTTP artifact strategy")
			}
		})
	}
	for _, name := range []string{"disk.raw.xz", "disk.RAW.XZ", "磁盘.raw.xz"} {
		t.Run(name, func(t *testing.T) {
			got, err := renderVMDockerfile(name)
			if err != nil {
				t.Fatal(err)
			}
			if got != "FROM scratch\nADD --chown=107:107 "+name+" /disk/\n" {
				t.Fatal("xz raw must retain the compressed disk ADD strategy")
			}
		})
	}
}
