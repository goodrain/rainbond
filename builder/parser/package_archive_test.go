package parser

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// capability_id: rainbond.package-build.select-archive-over-extracted-content
func TestSelectPackageArchiveSkipsExtractedDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "dist"), 0755); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(dir, "dist789.zip")
	if err := os.WriteFile(archivePath, []byte("zip data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0644); err != nil {
		t.Fatal(err)
	}

	got, ext, err := selectPackageArchive(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != archivePath {
		t.Fatalf("selectPackageArchive() path = %q, want %q", got, archivePath)
	}
	if ext != ".zip" {
		t.Fatalf("selectPackageArchive() ext = %q, want .zip", ext)
	}
}

// capability_id: rainbond.package-build.select-latest-archive
func TestSelectPackageArchiveUsesLatestArchiveForSecondUpload(t *testing.T) {
	dir := t.TempDir()
	oldArchive := filepath.Join(dir, "dist.zip")
	newArchive := filepath.Join(dir, "dist789.zip")
	if err := os.WriteFile(oldArchive, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newArchive, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}

	oldTime := time.Now().Add(-time.Hour)
	newTime := time.Now()
	if err := os.Chtimes(oldArchive, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newArchive, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	got, ext, err := selectPackageArchive(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != newArchive {
		t.Fatalf("selectPackageArchive() path = %q, want latest archive %q", got, newArchive)
	}
	if ext != ".zip" {
		t.Fatalf("selectPackageArchive() ext = %q, want .zip", ext)
	}
}

// capability_id: rainbond.package-build.clean-extracted-content
func TestCleanPackageExtractDirKeepsSelectedArchiveOnly(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "dist789.zip")
	if err := os.WriteFile(archivePath, []byte("zip data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dist.zip"), []byte("old zip"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "dist"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := cleanPackageExtractDir(dir, archivePath); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("selected archive should remain: %v", err)
	}
	for _, name := range []string{"dist.zip", "dist", "index.html"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s should be removed, stat err: %v", name, err)
		}
	}
}
