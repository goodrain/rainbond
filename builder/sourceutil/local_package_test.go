package sourceutil

import (
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/pkg/component/storage"
)

// capability_id: rainbond.vm-run.local-package-storage-download

type fakeStorage struct {
	downloadCalls int
	downloadFn    func(srcDir, dstDir string) error
}

func (f *fakeStorage) MkdirAll(path string) error { return nil }

func (f *fakeStorage) Unzip(archive, target string, currentDirectory bool) error { return nil }

func (f *fakeStorage) ReadDir(dirName string) ([]string, error) { return nil, nil }

func (f *fakeStorage) ServeFile(w http.ResponseWriter, r *http.Request, filePath string) {}

func (f *fakeStorage) SaveFile(fileName string, reader multipart.File) error { return nil }

func (f *fakeStorage) UploadFileToFile(src string, dst string, logger event.Logger) error { return nil }

func (f *fakeStorage) DownloadDirToDir(srcDir, dstDir string) error {
	f.downloadCalls++
	if f.downloadFn != nil {
		return f.downloadFn(srcDir, dstDir)
	}
	return nil
}

func (f *fakeStorage) DownloadFileToDir(srcFile, dstDir string) error { return nil }

func (f *fakeStorage) ReadFile(filePath string) (storage.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (f *fakeStorage) SaveChunk(sessionID string, chunkIndex int, reader multipart.File) (string, error) {
	return "", nil
}

func (f *fakeStorage) MergeChunks(sessionID string, outputPath string, totalChunks int) error { return nil }

func (f *fakeStorage) ChunkExists(sessionID string, chunkIndex int) bool { return false }

func (f *fakeStorage) CleanupChunks(sessionID string) error { return nil }

func (f *fakeStorage) GetChunkDir(sessionID string) string { return "" }

func TestReadLocalPackageDirFallsBackToStorageDownload(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "vm-image")
	originalPrefixes := localPackageSourcePrefixes
	localPackageSourcePrefixes = []string{filepath.Dir(sourcePath)}
	t.Cleanup(func() {
		localPackageSourcePrefixes = originalPrefixes
	})

	component := storage.New()
	component.StorageCli = &fakeStorage{
		downloadFn: func(srcDir, dstDir string) error {
			if srcDir != sourcePath {
				t.Fatalf("expected srcDir %q, got %q", sourcePath, srcDir)
			}
			if dstDir != sourcePath {
				t.Fatalf("expected dstDir %q, got %q", sourcePath, dstDir)
			}
			if err := os.MkdirAll(dstDir, 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(dstDir, "ubuntu.qcow2"), []byte("image"), 0o644)
		},
	}

	fileInfos, err := ReadLocalPackageDir(sourcePath)
	if err != nil {
		t.Fatalf("expected storage fallback to succeed, got error: %v", err)
	}
	if len(fileInfos) != 1 {
		t.Fatalf("expected 1 file after storage fallback, got %d", len(fileInfos))
	}
	if got := fileInfos[0].Name(); got != "ubuntu.qcow2" {
		t.Fatalf("expected ubuntu.qcow2, got %q", got)
	}
}

func TestReadLocalPackageDirUsesExistingDirectoryWithoutStorageDownload(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "vm-image")
	originalPrefixes := localPackageSourcePrefixes
	localPackageSourcePrefixes = []string{filepath.Dir(sourcePath)}
	t.Cleanup(func() {
		localPackageSourcePrefixes = originalPrefixes
	})

	if err := os.MkdirAll(sourcePath, 0o755); err != nil {
		t.Fatalf("create source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourcePath, "ubuntu.qcow2"), []byte("image"), 0o644); err != nil {
		t.Fatalf("write test image: %v", err)
	}

	fake := &fakeStorage{}
	component := storage.New()
	component.StorageCli = fake

	fileInfos, err := ReadLocalPackageDir(sourcePath)
	if err != nil {
		t.Fatalf("expected existing directory to be readable, got error: %v", err)
	}
	if len(fileInfos) != 1 {
		t.Fatalf("expected 1 file, got %d", len(fileInfos))
	}
	if fake.downloadCalls != 0 {
		t.Fatalf("expected no storage download, got %d calls", fake.downloadCalls)
	}
}
