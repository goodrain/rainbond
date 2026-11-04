package parser

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePackageWorkspaceArchive(t *testing.T, filename, prefix string, contents map[string]string) {
	t.Helper()
	file, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasSuffix(strings.ToLower(filename), ".zip") {
		writer := zip.NewWriter(file)
		for name, content := range contents {
			header := &zip.FileHeader{Name: prefix + name, Method: zip.Deflate}
			header.SetMode(0644)
			entry, err := writer.CreateHeader(header)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.WriteString(entry, content); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
	} else {
		var output io.Writer = file
		var compressed *gzip.Writer
		if !strings.HasSuffix(filename, ".tar") {
			compressed = gzip.NewWriter(file)
			output = compressed
		}
		writer := tar.NewWriter(output)
		for name, content := range contents {
			if err := writer.WriteHeader(&tar.Header{Name: prefix + name, Mode: 0644, Size: int64(len(content))}); err != nil {
				t.Fatal(err)
			}
			if _, err := io.WriteString(writer, content); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if compressed != nil {
			if err := compressed.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.cnb.package-workspace-isolation
func TestPreparePackageBuildWorkspace(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ext    string
		prefix string
	}{
		{name: "zip", ext: ".zip"},
		{name: "uppercase zip", ext: ".ZIP"},
		{name: "zip with common root", ext: ".zip", prefix: "project/"},
		{name: "tar", ext: ".tar"},
		{name: "tar gz", ext: ".tar.gz"},
		{name: "tgz", ext: ".tgz"},
		{name: "gz", ext: ".gz"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := t.TempDir()
			name := "uploaded" + tc.ext
			contents := map[string]string{
				"package.json":                 `{"name":"original"}`,
				"package-lock.json":            `{"lockfileVersion":3}`,
				"node_modules/from-upload.txt": "user supplied",
				".npmrc":                       "registry=https://registry.npmmirror.com",
				"frontend/package.json":        `{"name":"nested-project"}`,
			}
			writePackageWorkspaceArchive(t, filepath.Join(source, name), tc.prefix, contents)
			before, err := os.ReadFile(filepath.Join(source, name))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(source, "generated.zip"), []byte("not the selected archive"), 0644); err != nil {
				t.Fatal(err)
			}
			first, releaseFirst, err := PreparePackageBuildWorkspace(source, name)
			if err != nil {
				t.Fatal(err)
			}
			defer releaseFirst()
			if first == source || filepath.Dir(first) != filepath.Dir(source) {
				t.Fatalf("unexpected isolated workspace: %s", first)
			}
			for name, want := range contents {
				got, err := os.ReadFile(filepath.Join(first, name))
				if err != nil || string(got) != want {
					t.Errorf("archive content %s = %q, %v; want %q", name, got, err, want)
				}
			}
			if err := os.WriteFile(filepath.Join(first, "generated.txt"), []byte("first build"), 0644); err != nil {
				t.Fatal(err)
			}
			second, releaseSecond, err := PreparePackageBuildWorkspace(source, name)
			if err != nil {
				t.Fatal(err)
			}
			defer releaseSecond()
			if second == first {
				t.Fatal("parallel/repeated builds share a workspace")
			}
			if _, err := os.Stat(filepath.Join(second, "generated.txt")); !os.IsNotExist(err) {
				t.Fatalf("other build output was copied: %v", err)
			}
			releaseFirst()
			releaseFirst()
			if _, err := os.Stat(first); !os.IsNotExist(err) {
				t.Fatalf("cleanup did not remove its workspace: %v", err)
			}
			if _, err := os.Stat(second); err != nil {
				t.Fatalf("cleanup removed a different build workspace: %v", err)
			}
			after, err := os.ReadFile(filepath.Join(source, name))
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("uploaded archive changed: %v", err)
			}
		})
	}
	for _, ext := range []string{".jar", ".war"} {
		t.Run("standalone "+ext, func(t *testing.T) {
			source := t.TempDir()
			name := "application" + ext
			if err := os.WriteFile(filepath.Join(source, name), []byte("artifact"), 0644); err != nil {
				t.Fatal(err)
			}
			dir, cleanup, err := PreparePackageBuildWorkspace(source, name)
			if err != nil {
				t.Fatal(err)
			}
			cleanup()
			if dir != source {
				t.Fatalf("standalone artifact source changed: %s", dir)
			}
			if _, err := os.Stat(filepath.Join(source, name)); err != nil {
				t.Fatalf("standalone artifact was removed: %v", err)
			}
		})
	}
}

// capability_id: rainbond.cnb.package-workspace-isolation
func TestPreparePackageBuildWorkspaceRejectsInvalidInput(t *testing.T) {
	for _, tc := range []struct {
		name     string
		file     string
		kind     string
		emptyDir bool
	}{
		{name: "empty source directory", file: "source.zip", kind: "file", emptyDir: true},
		{name: "missing filename"},
		{name: "parent path", file: "../outside.zip"},
		{name: "absolute path", file: "/tmp/outside.zip"},
		{name: "windows path", file: `..\outside.zip`},
		{name: "missing file", file: "missing.zip"},
		{name: "directory", file: "directory.zip", kind: "directory"},
		{name: "symlink", file: "link.zip", kind: "symlink"},
		{name: "unsupported", file: "source.txt", kind: "file"},
		{name: "corrupt zip", file: "source.zip", kind: "file"},
		{name: "corrupt tar", file: "source.tar", kind: "file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := t.TempDir()
			target := filepath.Join(source, tc.file)
			switch tc.kind {
			case "directory":
				if err := os.Mkdir(target, 0755); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				original := filepath.Join(source, "original.txt")
				if err := os.WriteFile(original, []byte("keep"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(original, target); err != nil {
					t.Fatal(err)
				}
			case "file":
				if err := os.WriteFile(target, []byte("invalid archive"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			inputDir := source
			if tc.emptyDir {
				inputDir = ""
			}
			dir, cleanup, err := PreparePackageBuildWorkspace(inputDir, tc.file)
			if cleanup != nil {
				defer cleanup()
			}
			if err == nil || dir != "" {
				t.Errorf("invalid source accepted: %q, %v", dir, err)
			}
			entries, err := os.ReadDir(filepath.Dir(source))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".cnb-package-") {
					t.Errorf("failed extraction leaked temporary directory: %s", entry.Name())
				}
			}
			if _, err := os.Stat(source); err != nil {
				t.Fatalf("invalid input cleanup removed original source: %v", err)
			}
		})
	}
}
