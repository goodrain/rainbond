package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/config/configs"
)

// capability_id: rainbond.storage.upload-chunk-binding
func TestUploadStorageBindingChangesWithTargetWithoutExposingCredentials(t *testing.T) {
	a := &Component{storageConfig: &configs.StorageConfig{StorageType: "s3", S3Endpoint: "http://owned.invalid:9000", S3AccessKeyID: "fixture-user", S3SecretAccessKey: "fixture-not-exported"}}
	first, err := a.UploadChunkBinding()
	if err != nil || len(first) != 64 {
		t.Fatal(err)
	}
	a.storageConfig.S3SecretAccessKey = "rotated-fixture"
	same, err := a.UploadChunkBinding()
	if err != nil || same != first {
		t.Fatal("credential rotation changed storage scope", err)
	}
	a.storageConfig.S3Endpoint = "http://different.invalid:9000"
	other, err := a.UploadChunkBinding()
	if err != nil || other == first {
		t.Fatal("storage endpoint change not detected", err)
	}
	root := t.TempDir()
	local, err := localUploadChunkBinding(root)
	if err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(t.TempDir(), "new")
	if err := os.Mkdir(replacement, 0700); err != nil {
		t.Fatal(err)
	}
	changed, err := localUploadChunkBinding(replacement)
	if err != nil || changed == local {
		t.Fatal("local filesystem identity not bound", err)
	}
	if _, err := localUploadChunkBinding(filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing root created or accepted")
	}
}
