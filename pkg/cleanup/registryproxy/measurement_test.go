//go:build linux || darwin

package registryproxy

import (
	"os"
	"path/filepath"
	"testing"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
)

// capability_id: rainbond.cleanup.verified-storage-measurement
func TestMeasurementRequiresBoundStorageAndRejectsSymlinks(t *testing.T) {
	root := t.TempDir()
	binding := coordination.StorageRegistration{StorageID: "owned", Generation: "one", VolumeUID: "volume", RootPath: "/registry"}
	if _, err := MeasureStorage(root, binding); err == nil {
		t.Fatal("unregistered storage measured")
	}
	if err := InitializeStorageIdentity(root, binding); err != nil {
		t.Fatal(err)
	}
	measured, err := MeasureStorage(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, _ := binding.Fingerprint()
	if measured.Protocol != 1 || measured.StorageID != binding.StorageID || measured.BindingFingerprint != fingerprint || measured.TotalBytes == 0 || measured.FreeBytes > measured.TotalBytes || measured.AvailableBytes > measured.FreeBytes || measured.FilesystemID == "" {
		t.Fatal("measurement lacks a valid physical storage binding", measured)
	}
	changed := binding
	changed.Generation = "replacement"
	if _, err := MeasureStorage(root, changed); err == nil {
		t.Fatal("measurement accepted another storage generation")
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	if _, err := MeasureStorage(link, binding); err == nil {
		t.Fatal("measurement followed an unverified root symlink")
	}
}
