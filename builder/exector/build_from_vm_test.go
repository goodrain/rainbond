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
	"time"

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
	expected := "FROM quay.io/libpod/busybox:latest AS gzip\nFROM quay.io/kubevirt/cdi-importer:v1.65.0 AS convert\nWORKDIR /work\nCOPY --from=gzip /bin/busybox /usr/local/bin/busybox\nCOPY disk.img.gz /work/source.img.gz\nRUN /usr/local/bin/busybox gzip -dc /work/source.img.gz > /work/source.img && /usr/bin/qemu-img convert -p -f raw -O qcow2 -c /work/source.img /work/rootdisk.qcow2 && rm -f /work/source.img /work/source.img.gz\nFROM scratch\nCOPY --from=convert --chown=107:107 /work/rootdisk.qcow2 /disk/\n"
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

func TestMyDownloaderLogsUnknownSizeProgress(t *testing.T) {
	logger := &recordingLogger{}
	now := time.Unix(100, 0)
	downloader := &MyDownloader{
		Reader:           strings.NewReader("vm-image-content"),
		Total:            0,
		Logger:           logger,
		Pace:             10,
		ProgressInterval: time.Hour,
		NextProgressByte: 5,
		Now: func() time.Time {
			return now
		},
	}
	var dst bytes.Buffer

	_, err := io.Copy(&dst, downloader)

	if err != nil {
		t.Fatalf("copy with unknown content length failed: %v", err)
	}
	if len(logger.infos) == 0 {
		t.Fatal("expected progress log for unknown content length")
	}
	if !strings.Contains(logger.infos[0], "downloaded") || !strings.Contains(logger.infos[0], "total size unknown") {
		t.Fatalf("unexpected progress log: %q", logger.infos[0])
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

func TestDownloadFileOverwritesExistingPartialFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("new-vm-image-content"))
	}))
	defer server.Close()
	downloadDir := t.TempDir()
	existingPath := filepath.Join(downloadDir, "disk.img.gz")
	if err := os.WriteFile(existingPath, []byte("partial-stale-content"), 0644); err != nil {
		t.Fatalf("write existing partial file: %v", err)
	}

	err := downloadFile(downloadDir, server.URL+"/disk.img.gz", "", event.NewLogger("evt-overwrite", nil))

	if err != nil {
		t.Fatalf("download file failed: %v", err)
	}
	content, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(content) != "new-vm-image-content" {
		t.Fatalf("expected stale content to be overwritten, got %q", string(content))
	}
}

func TestVMRemoteImageSourceDirUsesEventID(t *testing.T) {
	got := vmRemoteImageSourceDir("service-a", "event-b")

	if got != "/grdata/package_build/temp/events/service-a/event-b" {
		t.Fatalf("unexpected remote image source dir: %q", got)
	}
}

type recordingLogger struct {
	infos []string
}

func (l *recordingLogger) Info(message string, info map[string]string) {
	l.infos = append(l.infos, message)
}

func (l *recordingLogger) Error(message string, info map[string]string) {}

func (l *recordingLogger) Debug(message string, info map[string]string) {}

func (l *recordingLogger) Event() string {
	return "test"
}

func (l *recordingLogger) CreateTime() time.Time {
	return time.Unix(0, 0)
}

func (l *recordingLogger) GetChan() chan []byte {
	return nil
}

func (l *recordingLogger) SetChan(ch chan []byte) {}

func (l *recordingLogger) GetWriter(step, level string) event.LoggerWriter {
	return discardLoggerWriter{}
}

type discardLoggerWriter struct{}

func (discardLoggerWriter) SetFormat(format map[string]interface{}) {}

func (discardLoggerWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
