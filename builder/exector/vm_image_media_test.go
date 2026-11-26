package exector

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeVMImageMediaFixture(t *testing.T, name string, data []byte) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func vmISOFixture() []byte {
	data := make([]byte, 64<<10)
	copy(data[16*2048:], []byte{1, 'C', 'D', '0', '0', '1', 1})
	return data
}

// capability_id: rainbond.vm-build.content-media-routing
func TestISOContentRenamedAsDiskUsesInstallerMedia(t *testing.T) {
	for _, name := range []string{"installer.img", "installer.raw", "installer.qcow2", "installer.iso"} {
		t.Run(name, func(t *testing.T) {
			dockerfile, boundDisk, err := renderVMSourceDockerfileChecked(writeVMImageMediaFixture(t, name, vmISOFixture()), name)
			if err != nil || boundDisk {
				t.Fatalf("ISO must not require a registry root import: %v %v", boundDisk, err)
			}
			if !strings.Contains(dockerfile, "COPY --chown=107:107 "+name+" /disk/") || strings.Contains(dockerfile, "qemu-img") {
				t.Fatalf("ISO must retain its optical-media artifact: %s", dockerfile)
			}
		})
	}
}

// capability_id: rainbond.vm-build.content-media-routing
func TestStructuredDiskRenamedAsISOStillRequiresValidationAndBinding(t *testing.T) {
	for _, magic := range [][]byte{{'Q', 'F', 'I', 0xfb}, []byte("vhdxfile")} {
		data := vmISOFixture()
		copy(data, magic)
		name := "misnamed.iso"
		dockerfile, boundDisk, err := renderVMSourceDockerfileChecked(writeVMImageMediaFixture(t, name, data), name)
		if err != nil || !boundDisk || !strings.Contains(dockerfile, "qemu-img info --output=json") ||
			!strings.Contains(dockerfile, "COPY "+name+" /work/source.img") {
			t.Fatalf("structured disk must keep validation and its owned root: %v %v %s", boundDisk, err, dockerfile)
		}
	}
}

// capability_id: rainbond.vm-build.content-media-routing
func TestMediaProbeDoesNotBypassDiskDependencyChecks(t *testing.T) {
	for _, kind := range []string{"vmdk", "vdi-parent"} {
		data := vmISOFixture()
		if kind == "vmdk" {
			copy(data, "KDMV") // Invalid VMDK must fail before ISO classification.
		} else {
			binary.LittleEndian.PutUint32(data[64:], 0xbeda107f)
			binary.LittleEndian.PutUint32(data[68:], 0x00010001)
			binary.LittleEndian.PutUint32(data[76:], 4)
		}
		_, _, err := renderVMSourceDockerfileChecked(writeVMImageMediaFixture(t, "misnamed.img", data), "misnamed.img")
		if err == nil {
			t.Fatal("an optical marker must not bypass invalid/dependent disk checks", kind)
		}
	}
}

// capability_id: rainbond.vm-build.content-media-routing
func TestUnknownAndWrappedMediaKeepExistingBuildRouting(t *testing.T) {
	var gz bytes.Buffer
	writer := gzip.NewWriter(&gz)
	if _, err := writer.Write(make([]byte, 64<<10)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name     string
		data     []byte
		bound    bool
		fragment string
	}{
		{"plain.img", make([]byte, 64<<10), true, "qemu-img info --output=json"},
		{"legacy.iso", []byte("UDF or old ISO retains suffix routing"), false, "COPY --chown=107:107 legacy.iso /disk/"},
		{"disk.raw.gz", gz.Bytes(), false, "COPY disk.raw.gz /disk/disk.img.gz"},
		{"installer.iso.gz", gz.Bytes(), false, "COPY --chown=107:107 installer.iso.gz /disk/"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dockerfile, boundDisk, err := renderVMSourceDockerfileChecked(writeVMImageMediaFixture(t, tt.name, tt.data), tt.name)
			if err != nil || boundDisk != tt.bound || !strings.Contains(dockerfile, tt.fragment) {
				t.Fatalf("legacy media routing changed: %v %v %s", boundDisk, err, dockerfile)
			}
		})
	}
}
