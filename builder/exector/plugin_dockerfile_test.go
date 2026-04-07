package exector

import (
	"os"
	"path/filepath"
	"testing"
)

// capability_id: rainbond.plugin-build.detect-dockerfile
func TestCheckDockerfile(t *testing.T) {
	root := t.TempDir()
	if checkDockerfile(root) {
		t.Fatal("expected no dockerfile")
	}

	if err := os.WriteFile(filepath.Join(root, "Dockerfile"), []byte("FROM busybox\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !checkDockerfile(root) {
		t.Fatal("expected dockerfile to be detected")
	}
}
