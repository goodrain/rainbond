package sourceutil

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func nativeVDIFixture() []byte {
	data := make([]byte, 512)
	binary.LittleEndian.PutUint32(data[64:], 0xbeda107f)
	binary.LittleEndian.PutUint32(data[68:], 0x00010001)
	binary.LittleEndian.PutUint32(data[72:], 0x180)
	binary.LittleEndian.PutUint32(data[76:], 1)
	return data
}

func nativeChecksum(data []byte, offset int) {
	clear(data[offset : offset+4])
	var sum uint32
	for _, value := range data {
		sum += uint32(value)
	}
	binary.BigEndian.PutUint32(data[offset:], ^sum)
}

func nativeVHDFooter(kind uint32, size uint64) []byte {
	data := make([]byte, 512)
	copy(data, "conectix")
	binary.BigEndian.PutUint32(data[8:], 2)
	binary.BigEndian.PutUint32(data[12:], 0x00010000)
	binary.BigEndian.PutUint64(data[16:], math.MaxUint64)
	if kind == 3 {
		binary.BigEndian.PutUint64(data[16:], 512)
	}
	copy(data[28:], "qem2")
	binary.BigEndian.PutUint64(data[40:], size)
	binary.BigEndian.PutUint64(data[48:], size)
	binary.BigEndian.PutUint32(data[60:], kind)
	nativeChecksum(data, 64)
	return data
}

func nativeVHDFixture(fixed, allocated bool) []byte {
	if fixed {
		return append(make([]byte, 512), nativeVHDFooter(2, 512)...)
	}
	size := 2560
	if allocated {
		size += 512 + 2<<20
	}
	data := make([]byte, size)
	footer := nativeVHDFooter(3, 2<<20)
	copy(data, footer)
	copy(data[len(data)-512:], footer)
	header := data[512:1536]
	copy(header, "cxsparse")
	binary.BigEndian.PutUint64(header[8:], math.MaxUint64)
	binary.BigEndian.PutUint64(header[16:], 1536)
	binary.BigEndian.PutUint32(header[24:], 0x00010000)
	binary.BigEndian.PutUint32(header[28:], 1)
	binary.BigEndian.PutUint32(header[32:], 2<<20)
	nativeChecksum(header, 36)
	binary.BigEndian.PutUint32(data[1536:], math.MaxUint32)
	if allocated {
		binary.BigEndian.PutUint32(data[1536:], 4)
	}
	return data
}

func writeNativeDiskFixture(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

// capability_id: rainbond.vm-build.native-disk-validation
func TestInspectVMNativeDiskAcceptsSelfContainedVDIAndVHD(t *testing.T) {
	for _, name := range []string{"disk.vdi", "renamed.raw", "renamed.img"} {
		t.Run(name, func(t *testing.T) {
			for _, kind := range []uint32{1, 2} {
				data := nativeVDIFixture()
				binary.LittleEndian.PutUint32(data[76:], kind)
				if err := InspectVMNativeDiskFile(writeNativeDiskFixture(t, name, data)); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
	for _, fixed := range []bool{false, true} {
		for _, allocated := range []bool{false, true} {
			data := nativeVHDFixture(fixed, allocated)
			before := append([]byte(nil), data...)
			p := writeNativeDiskFixture(t, "renamed.img", data)
			if err := InspectVMNativeDiskFile(p); err != nil {
				t.Fatal(fixed, allocated, err)
			}
			after, err := os.ReadFile(p)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("native disk validation changed source bytes", err)
			}
		}
	}
}

// capability_id: rainbond.vm-build.native-disk-validation
func TestInspectVMNativeDiskRejectsDependentAndInvalidVDI(t *testing.T) {
	cases := map[string]func([]byte){
		"undo":        func(data []byte) { binary.LittleEndian.PutUint32(data[76:], 3) },
		"difference":  func(data []byte) { binary.LittleEndian.PutUint32(data[76:], 4) },
		"unknown":     func(data []byte) { binary.LittleEndian.PutUint32(data[76:], 0) },
		"link":        func(data []byte) { data[424] = 1 },
		"parent":      func(data []byte) { data[440] = 1 },
		"new-version": func(data []byte) { binary.LittleEndian.PutUint32(data[68:], 0x00020000) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			data := nativeVDIFixture()
			mutate(data)
			if err := InspectVMNativeDiskFile(writeNativeDiskFixture(t, "renamed.raw", data)); err == nil {
				t.Fatal("dependent or unsupported VDI accepted")
			}
		})
	}
	for _, data := range [][]byte{nativeVDIFixture()[:68], make([]byte, 512)} {
		if err := InspectVMNativeDiskFile(writeNativeDiskFixture(t, "disk.VDI", data)); err == nil {
			t.Fatal("invalid declared VDI accepted")
		}
	}
}

// capability_id: rainbond.vm-build.native-disk-validation
func TestInspectVMNativeDiskRejectsDependentAndInvalidVHD(t *testing.T) {
	cases := map[string]func([]byte){
		"difference": func(data []byte) {
			for _, off := range []int{0, len(data) - 512} {
				binary.BigEndian.PutUint32(data[off+60:], 4)
				nativeChecksum(data[off:off+512], 64)
			}
		},
		"footer-checksum": func(data []byte) { data[len(data)-512+64] ^= 1 },
		"head-checksum":   func(data []byte) { data[64] ^= 1 },
		"missing-tail":    func(data []byte) { clear(data[len(data)-512:]) },
		"mismatched-footers": func(data []byte) {
			data[len(data)-512+68] = 1
			nativeChecksum(data[len(data)-512:], 64)
		},
		"header-offset": func(data []byte) {
			for _, off := range []int{0, len(data) - 512} {
				binary.BigEndian.PutUint64(data[off+16:], math.MaxUint64)
				nativeChecksum(data[off:off+512], 64)
			}
		},
		"parent-uuid":     func(data []byte) { data[512+40] = 1; nativeChecksum(data[512:1536], 36) },
		"parent-name":     func(data []byte) { data[512+64] = 1; nativeChecksum(data[512:1536], 36) },
		"parent-locator":  func(data []byte) { data[512+576] = 1; nativeChecksum(data[512:1536], 36) },
		"header-checksum": func(data []byte) { data[512+36] ^= 1 },
		"table-outside-file": func(data []byte) {
			binary.BigEndian.PutUint64(data[512+16:], 1<<63)
			nativeChecksum(data[512:1536], 36)
		},
		"table-overlaps-header": func(data []byte) {
			binary.BigEndian.PutUint64(data[512+16:], 512)
			nativeChecksum(data[512:1536], 36)
		},
		"too-many-entries": func(data []byte) {
			binary.BigEndian.PutUint32(data[512+28:], math.MaxUint32)
			nativeChecksum(data[512:1536], 36)
		},
		"missing-entries": func(data []byte) {
			binary.BigEndian.PutUint32(data[512+28:], 0)
			nativeChecksum(data[512:1536], 36)
		},
		"zero-block-size": func(data []byte) {
			binary.BigEndian.PutUint32(data[512+32:], 0)
			nativeChecksum(data[512:1536], 36)
		},
		"invalid-block-size": func(data []byte) {
			binary.BigEndian.PutUint32(data[512+32:], 1025)
			nativeChecksum(data[512:1536], 36)
		},
		"data-outside-file":      func(data []byte) { binary.BigEndian.PutUint32(data[1536:], math.MaxUint32-1) },
		"data-overlaps-metadata": func(data []byte) { binary.BigEndian.PutUint32(data[1536:], 1) },
		"truncated-data-block":   func(data []byte) { binary.BigEndian.PutUint32(data[1536:], 4) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			data := nativeVHDFixture(false, false)
			mutate(data)
			if err := InspectVMNativeDiskFile(writeNativeDiskFixture(t, "renamed.raw", data)); err == nil {
				t.Fatal("dependent or malformed VHD accepted")
			}
		})
	}
	fixed := nativeVHDFixture(true, false)
	binary.BigEndian.PutUint64(fixed[len(fixed)-512+48:], 1024)
	nativeChecksum(fixed[len(fixed)-512:], 64)
	if err := InspectVMNativeDiskFile(writeNativeDiskFixture(t, "fixed.vhd", fixed)); err == nil {
		t.Fatal("fixed VHD length mismatch accepted")
	}
	if err := InspectVMNativeDiskFile(writeNativeDiskFixture(t, "missing.vhd", make([]byte, 512))); err == nil {
		t.Fatal("declared VHD without magic accepted")
	}
}

// capability_id: rainbond.vm-build.native-disk-validation
func TestInspectVMNativeDiskPreservesOtherFormatsAndRejectsUnsafeFiles(t *testing.T) {
	for _, name := range []string{"disk.raw", "disk.qcow2", "disk.iso", "disk.tar", "disk.raw.gz"} {
		if err := InspectVMNativeDiskFile(writeNativeDiskFixture(t, name, make([]byte, 1024))); err != nil {
			t.Fatal(name, err)
		}
	}
	p := filepath.Join(t.TempDir(), "missing.vdi")
	if err := InspectVMNativeDiskFile(p); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("file error lost", err)
	}
	target := writeNativeDiskFixture(t, "disk.vdi", nativeVDIFixture())
	link := filepath.Join(t.TempDir(), "linked.vdi")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := InspectVMNativeDiskFile(link); err == nil {
		t.Fatal("symlink accepted")
	}
	if err := InspectVMNativeDiskFile(t.TempDir()); err == nil {
		t.Fatal("directory accepted")
	}
}

// capability_id: rainbond.vm-build.native-disk-validation
func TestInspectVMNativeDiskDoesNotTreatGuestVHDFooterAsContainer(t *testing.T) {
	for _, magic := range []string{"QFI\xfb", "vhdxfile"} {
		data := append(make([]byte, 512), nativeVHDFixture(true, false)...)
		copy(data, []byte(magic))
		// This validator owns VDI/VHD metadata. The format-specific QEMU stage
		// validates the outer structured disk, whose guest data may contain VHDs.
		if err := InspectVMNativeDiskFile(writeNativeDiskFixture(t, "outer.img", data)); err != nil {
			t.Fatal("a guest VHD footer must not override the outer disk format", magic, err)
		}
	}
}
