package exector

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/event"
)

// capability_id: rainbond.vm-run.build-media-paths
// capability_id: rainbond.vm-publish.qcow2-image-build
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

func TestRenderVMDockerfileUsesQCOW2ConversionForGzipRawExport(t *testing.T) {
	dockerfile, err := renderVMDockerfile("disk.img.gz")
	if err != nil {
		t.Fatalf("render gzip raw dockerfile: %v", err)
	}
	expected := "FROM quay.io/kubevirt/cdi-importer:v1.65.0 AS convert\nWORKDIR /work\nCOPY disk.img.gz /work/source.img.gz\nRUN gzip -dc /work/source.img.gz > /work/source.img && /usr/bin/qemu-img convert -p -f raw -O qcow2 -c /work/source.img /work/rootdisk.qcow2 && rm -f /work/source.img /work/source.img.gz\nFROM scratch\nCOPY --from=convert --chown=107:107 /work/rootdisk.qcow2 /disk/\n"
	if dockerfile != expected {
		t.Fatalf("unexpected gzip raw dockerfile: %q", dockerfile)
	}
}

func TestRenderVMDockerfileUsesQCOW2ConversionForRawDisk(t *testing.T) {
	dockerfile, err := renderVMDockerfile("rootdisk.img")
	if err != nil {
		t.Fatalf("render raw disk dockerfile: %v", err)
	}
	expected := "FROM quay.io/kubevirt/cdi-importer:v1.65.0 AS convert\nWORKDIR /work\nCOPY rootdisk.img /work/source.img\nRUN /usr/bin/qemu-img convert -p -f raw -O qcow2 -c /work/source.img /work/rootdisk.qcow2 && rm -f /work/source.img\nFROM scratch\nCOPY --from=convert --chown=107:107 /work/rootdisk.qcow2 /disk/\n"
	if dockerfile != expected {
		t.Fatalf("unexpected raw disk dockerfile: %q", dockerfile)
	}
}

func TestVMBuildItemLocalImageNameUsesLocalRegistryPrefix(t *testing.T) {
	item := &VMBuildItem{Image: "tenant-ns:windows-root"}

	got := item.localImageName()

	if got != "goodrain.me/tenant-ns:windows-root" {
		t.Fatalf("unexpected local image name: %q", got)
	}
}

func TestMyDownloaderHandlesUnknownContentLength(t *testing.T) {
	downloader := &MyDownloader{
		Reader: strings.NewReader("vm-image-content"),
		Total:  0,
		Pace:   10,
	}
	var dst bytes.Buffer

	n, err := io.Copy(&dst, downloader)

	if err != nil {
		t.Fatalf("copy with unknown content length failed: %v", err)
	}
	if n != int64(len("vm-image-content")) {
		t.Fatalf("unexpected copied bytes: %d", n)
	}
	if dst.String() != "vm-image-content" {
		t.Fatalf("unexpected copied content: %q", dst.String())
	}
}

func TestDownloadFileUsesVMExportTokenHeader(t *testing.T) {
	tokenCh := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenCh <- r.Header.Get("x-kubevirt-export-token")
		_, _ = w.Write([]byte("vm-image-content"))
	}))
	defer server.Close()
	downloadDir := t.TempDir()

	err := downloadFile(downloadDir, server.URL+"/disk.img.gz", "download-token", event.NewLogger("evt-token", nil))

	if err != nil {
		t.Fatalf("download file failed: %v", err)
	}
	if got := <-tokenCh; got != "download-token" {
		t.Fatalf("expected token header, got %q", got)
	}
	content, err := os.ReadFile(filepath.Join(downloadDir, "disk.img.gz"))
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(content) != "vm-image-content" {
		t.Fatalf("unexpected downloaded content: %q", string(content))
	}
}
