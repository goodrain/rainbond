package sourceutil

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func vmImageMediaFixture() []byte {
	data := make([]byte, 64<<10)
	copy(data[16*2048:], []byte{1, 'C', 'D', '0', '0', '1', 1})
	return data
}

func vmImageMediaFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

// capability_id: rainbond.vm-build.content-media-routing
func TestVMImageMediaDetectsISOWithoutTrustingExtension(t *testing.T) {
	for _, name := range []string{"installer.iso", "installer.img", "installer.raw", "installer.qcow2"} {
		got, err := InspectVMImageMediaFile(vmImageMediaFile(t, name, vmImageMediaFixture()))
		if err != nil || got != VMImageMediaISO {
			t.Fatalf("%s ISO content must select optical media: %v %v", name, got, err)
		}
	}
}

// capability_id: rainbond.vm-build.content-media-routing
func TestVMImageMediaPrioritizesStructuredDiskMagic(t *testing.T) {
	for _, kind := range []string{"qcow", "vdi", "vhdx", "vhd-header", "vhd-footer"} {
		t.Run(kind, func(t *testing.T) {
			data := vmImageMediaFixture()
			switch kind {
			case "qcow":
				copy(data, []byte{'Q', 'F', 'I', 0xfb})
			case "vdi":
				copy(data, nativeVDIFixture())
			case "vhdx":
				copy(data, "vhdxfile")
			case "vhd-header":
				copy(data, "conectix")
			case "vhd-footer":
				footer := nativeVHDFixture(true, false)
				copy(data[len(data)-512:], footer[len(footer)-512:])
			}
			got, err := InspectVMImageMediaFile(vmImageMediaFile(t, "misnamed.iso", data))
			if err != nil || got != VMImageMediaDisk {
				t.Fatalf("structured disk must win over an embedded CD001 marker: %v %v", got, err)
			}
		})
	}
}

// capability_id: rainbond.vm-build.content-media-routing
func TestVMImageMediaKeepsUnknownAndWrappedInputsOnLegacyPath(t *testing.T) {
	var gz, archive bytes.Buffer
	writer := gzip.NewWriter(&gz)
	if _, err := writer.Write(vmImageMediaFixture()); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	tarWriter := tar.NewWriter(&archive)
	if err := tarWriter.WriteHeader(&tar.Header{Name: "disk.raw", Mode: 0600, Size: 512}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(make([]byte, 512)); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	udf := make([]byte, 64<<10)
	copy(udf[16*2048:], []byte{0, 'B', 'E', 'A', '0', '1', 1})
	for name, data := range map[string][]byte{
		"legacy.iso": udf, "short.img": []byte("legacy"), "empty.img": {},
		"disk.raw.gz": gz.Bytes(), "disk.tar": archive.Bytes(),
	} {
		got, err := InspectVMImageMediaFile(vmImageMediaFile(t, name, data))
		if err != nil || got != VMImageMediaUnknown {
			t.Fatalf("%s must retain legacy routing: %v %v", name, got, err)
		}
	}
}

// capability_id: rainbond.vm-build.content-media-routing
func TestVMImageMediaRejectsUnsafeFiles(t *testing.T) {
	if _, err := InspectVMImageMediaFile(filepath.Join(t.TempDir(), "missing.img")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file error lost: %v", err)
	}
	if _, err := InspectVMImageMediaFile(t.TempDir()); err == nil {
		t.Fatal("directory must not become image media")
	}
	target := vmImageMediaFile(t, "installer.iso", vmImageMediaFixture())
	link := filepath.Join(t.TempDir(), "link.img")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectVMImageMediaFile(link); err == nil {
		t.Fatal("symlink must not become trusted image media")
	}
}
