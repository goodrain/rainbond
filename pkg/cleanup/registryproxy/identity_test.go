package registryproxy

import (
	"os"
	"path/filepath"
	"testing"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
)

// capability_id: rainbond.cleanup.storage-identity
func TestStorageIdentityIsBoundAndNeverOverwritten(t *testing.T) {
	root := t.TempDir()
	binding := coordination.StorageRegistration{StorageID: "store", Generation: "one", VolumeUID: "volume", RootPath: "/var/lib/registry"}
	if err := InitializeStorageIdentity(root, binding); err != nil {
		t.Fatal(err)
	}
	if err := VerifyStorageIdentity(root, binding); err != nil {
		t.Fatal(err)
	}
	if err := InitializeStorageIdentity(root, binding); err != nil {
		t.Fatal("idempotent initialization failed", err)
	}
	changed := binding
	changed.VolumeUID = "replacement"
	if err := InitializeStorageIdentity(root, changed); err == nil {
		t.Fatal("existing identity overwritten")
	}
	if err := VerifyStorageIdentity(root, binding); err != nil {
		t.Fatal("original identity lost", err)
	}
}
func TestStorageIdentityRejectsSymlinksAndUnrelatedDirectories(t *testing.T) {
	binding := coordination.StorageRegistration{StorageID: "store", Generation: "one", VolumeUID: "volume", RootPath: "/var/lib/registry"}
	t.Run("unrelated-data", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "business-data"), []byte("owned test fixture"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := InitializeStorageIdentity(root, binding); err == nil {
			t.Fatal("unrelated data root accepted")
		}
		if _, err := os.Stat(filepath.Join(root, ".rainbond-cleanup")); !os.IsNotExist(err) {
			t.Fatal("modified unrelated directory")
		}
	})
	t.Run("metadata-link", func(t *testing.T) {
		root, other := t.TempDir(), t.TempDir()
		if err := os.Symlink(other, filepath.Join(root, ".rainbond-cleanup")); err != nil {
			t.Fatal(err)
		}
		if err := InitializeStorageIdentity(root, binding); err == nil {
			t.Fatal("metadata symlink followed")
		}
		entries, err := os.ReadDir(other)
		if err != nil || len(entries) != 0 {
			t.Fatal("wrote through symlink")
		}
	})
	t.Run("root-link", func(t *testing.T) {
		root, other := t.TempDir(), t.TempDir()
		link := filepath.Join(root, "link")
		if err := os.Symlink(other, link); err != nil {
			t.Fatal(err)
		}
		if err := InitializeStorageIdentity(link, binding); err == nil {
			t.Fatal("root symlink followed")
		}
	})
}

func TestConcurrentStorageIdentityInitializationIsAtomic(t *testing.T) {
	root := t.TempDir()
	binding := coordination.StorageRegistration{StorageID: "store", Generation: "one", VolumeUID: "volume", RootPath: "/var/lib/registry"}
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { results <- InitializeStorageIdentity(root, binding) }()
	}
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if err := VerifyStorageIdentity(root, binding); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".rainbond-cleanup"))
	if err != nil || len(entries) != 1 || entries[0].Name() != "identity.json" {
		t.Fatal("temporary metadata not cleaned up")
	}
}
