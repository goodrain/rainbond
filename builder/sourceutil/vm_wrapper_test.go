package sourceutil

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
)

func vmTarFixture(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	w := tar.NewWriter(&out)
	if err := w.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
func vmGzipFixture(t *testing.T, data []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	w := gzip.NewWriter(&out)
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
func vmBase64Fixture(t *testing.T, value string) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// capability_id: rainbond.vm-build.vmdk-self-contained
func TestVMWrapperCannotHideVMDKFromPreflight(t *testing.T) {
	xzVMDK := vmBase64Fixture(t, "/Td6WFoAAATm1rRGAgAhARYAAAB0L+Wj4AP/ABBdACWRBd4YN7fSIEV5vSuMiwAA4mBIJJej3CEAASyACAAAAO2JacexxGf7AgAAAAAEWVo=")
	for name, data := range map[string][]byte{
		"disguised.raw.gz":      vmGzipFixture(t, validVMDKFixture("")),
		"disguised.raw.xz":      xzVMDK,
		"nested-gzip.raw.gz":    vmGzipFixture(t, vmGzipFixture(t, validVMDKFixture(""))),
		"nested-xz.raw.gz":      vmGzipFixture(t, xzVMDK),
		"compressed-member.tar": vmTarFixture(t, "disk.qcow2.gz", vmGzipFixture(t, validVMDKFixture(""))),
		"disguised.tar.xz":      vmBase64Fixture(t, "/Td6WFoAAATm1rRGAgAhARYAAAB0L+Wj4Cf/AF9dADIaSs1g6R4NvSA5ejJPZAuSqyVa3iVC9PZ3ZOpCpwAA8W3qsYcZCH/4UxUb9bAuZRpmvRxotWJ7jsDEkb2VZOrcvlGVAOo5RWkeSQsl4q2kEpOQ2c+W+OthYTrTiVQAAADrhVsonllSJgABe4BQAAAA75Zb2LHEZ/sCAAAAAARZWg=="),
		"disguised.tar":         vmTarFixture(t, "disk.img", validVMDKFixture("")),
		"disguised.tar.gz":      vmGzipFixture(t, vmTarFixture(t, "disk.qcow2", validVMDKFixture(""))),
		"descriptor.tar":        vmTarFixture(t, "disk.img", []byte(strings.Repeat("# padding\n", 100)+"version=1\ncreateType=\"monolithicFlat\"\nparentCID=ffffffff\n")),
		"encoding-first.tar.gz": vmGzipFixture(t, vmTarFixture(t, "disk.qcow2", []byte("# Disk DescriptorFile\nencoding=\"UTF-8\"\nversion=1\nparentCID=ffffffff\n"))),
	} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(p, data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := InspectVMDKFile(p); err == nil {
				t.Fatal("wrapped VMDK bypassed preflight")
			}
		})
	}
}

// capability_id: rainbond.vm-build.vmdk-self-contained
func TestVMWrapperRetainsExistingRawAndQCOWArchives(t *testing.T) {
	raw := make([]byte, 1024)
	xzRaw := vmBase64Fixture(t, "/Td6WFoAAATm1rRGAgAhARYAAAB0L+Wj4AP/AAtdAABv/f//o7d/RjIAAAAMJ2kgl2N4wwABJ4AIAAAALnmurbHEZ/sCAAAAAARZWg==")
	for name, data := range map[string][]byte{"disk.raw.gz": vmGzipFixture(t, raw), "disk.raw.xz": xzRaw, "disk.tar": vmTarFixture(t, "disk.qcow2", raw), "disk.tar.gz": vmGzipFixture(t, vmTarFixture(t, "disk.img", raw))} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(p, data, 0600); err != nil {
				t.Fatal(err)
			}
			recognized, err := InspectVMDKFile(p)
			if recognized || err != nil {
				t.Fatal("legacy disk/archive rejected", recognized, err)
			}
		})
	}
}

// capability_id: rainbond.vm-build.vmdk-self-contained
func TestVMWrapperBoundsNamesTypesAndXZDictionary(t *testing.T) {
	largeDictionary := vmBase64Fixture(t, "/Td6WFoAAATm1rRGAgAhARYAAAB0L+Wj4AP/AAtdAABv/f//o7d/RjIAAAAMJ2kgl2N4wwABJ4AIAAAALnmurbHEZ/sCAAAAAARZWg==")
	largeDictionary[16] = 40
	binary.LittleEndian.PutUint32(largeDictionary[20:24], crc32.ChecksumIEEE(largeDictionary[12:20]))
	for name, data := range map[string][]byte{"traversal.tar": vmTarFixture(t, "../disk.raw", make([]byte, 512)), "huge-dictionary.raw.xz": largeDictionary, "corrupt.raw.xz": {0xfd, '7', 'z', 'X', 'Z', 0}, "corrupt.raw.gz": {0x1f, 0x8b, 0}} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(p, data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := InspectVMDKFile(p); err == nil {
				t.Fatal("unsafe wrapper accepted")
			}
		})
	}
	errReader := &vmInspectionReader{reader: bytes.NewReader([]byte("test")), deadline: time.Now().Add(-time.Second)}
	if _, err := errReader.Read(make([]byte, 1)); err == nil {
		t.Fatal("inspection deadline ignored")
	}
	if err := vmWrapperReadError(errors.New("decode failure")); err == nil {
		t.Fatal("decoder errors must propagate")
	}
}

// capability_id: rainbond.vm-build.vmdk-self-contained
func TestVMWrapperRejectsBzipAndZstdIncludingSkippableFrames(t *testing.T) {
	tarData := vmTarFixture(t, "disk.vmdk", validVMDKFixture(""))
	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer encoder.Close()
	zstdData := encoder.EncodeAll(tarData, nil)
	for name, data := range map[string][]byte{"bzip.tar": vmBase64Fixture(t, "QlpoOTFBWSZTWQJliEAAAHF/gMiAAAFAAV+ABAoBQGSqHgAICCAAVDSjRkGBqMgPIJJTIaaMgAAfbQKEIMnoQiXU4FdjVIEMDHNzknMKbIqwhi49TGZt5WtmhLf9/IiYKxdiIgLi7kinChIATLEIAA=="), "zstd.tar": zstdData, "skippable.tar": append([]byte{0x50, 0x2a, 0x4d, 0x18, 0, 0, 0, 0}, zstdData...)} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(p, data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := InspectVMDKFile(p); err == nil {
				t.Fatal("uninspected compression reached legacy ADD")
			}
		})
	}
}
