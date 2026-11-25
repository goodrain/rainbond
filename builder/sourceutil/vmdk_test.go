package sourceutil

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validVMDKFixture(descriptor string) []byte {
	data := make([]byte, 128*512)
	copy(data, "KDMV")
	binary.LittleEndian.PutUint32(data[4:], 1)
	binary.LittleEndian.PutUint32(data[8:], 3)
	binary.LittleEndian.PutUint64(data[12:], 2048)
	binary.LittleEndian.PutUint64(data[20:], 128)
	binary.LittleEndian.PutUint64(data[28:], 1)
	binary.LittleEndian.PutUint64(data[36:], 20)
	binary.LittleEndian.PutUint32(data[44:], 512)
	binary.LittleEndian.PutUint64(data[48:], 21)
	binary.LittleEndian.PutUint64(data[56:], 26)
	binary.LittleEndian.PutUint64(data[64:], 128)
	copy(data[73:], []byte{10, 32, 13, 10})
	if descriptor == "" {
		descriptor = "# Disk DescriptorFile\nversion=1\nCID=12345678\nparentCID=ffffffff\ncreateType=\"monolithicSparse\"\nRW 2048 SPARSE \"Windows 10 x64.vmdk\"\nddb.adapterType = \"lsilogic\"\n"
	}
	copy(data[512:], descriptor)
	return data
}

// capability_id: rainbond.vm-build.vmdk-self-contained
func TestInspectVMDKAcceptsOnlyBoundedSelfContainedSparseImages(t *testing.T) {
	for _, name := range []string{"windows.vmdk", "windows.img", "windows.qcow2", "磁盘.VMDK"} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(p, validVMDKFixture(""), 0600); err != nil {
				t.Fatal(err)
			}
			isVMDK, err := InspectVMDKFile(p)
			if err != nil || !isVMDK {
				t.Fatal("valid sparse VMDK rejected", isVMDK, err)
			}
		})
	}
	for _, kind := range []string{"parent CID", "parent hint", "comment parent hint", "external extent", "multiple extents", "encrypted", "unknown type", "unknown flags", "zero capacity", "bad descriptor offset", "truncated descriptor", "unknown version", "missing descriptor terminator", "unsafe file type"} {
		t.Run(kind, func(t *testing.T) {
			data := validVMDKFixture("")
			p := filepath.Join(t.TempDir(), "disk.vmdk")
			text := string(data[512 : 512+20*512])
			text = strings.TrimRight(text, "\x00")
			switch kind {
			case "parent CID":
				text = strings.Replace(text, "parentCID=ffffffff", "parentCID=12345678", 1)
			case "parent hint":
				text += "parentFileNameHint=\"/outside/secret.vmdk\"\n"
			case "comment parent hint":
				text += "# parentFileNameHint=\"http://outside.invalid/disk\"\n"
			case "external extent":
				text = strings.Replace(text, "SPARSE \"Windows 10 x64.vmdk\"", "FLAT \"/outside/data.vmdk\" 0", 1)
			case "multiple extents":
				text += "RW 2048 SPARSE \"second.vmdk\"\n"
			case "encrypted":
				text += "ddb.encryption.keySafe = \"opaque\"\n"
			case "unknown type":
				text = strings.Replace(text, "monolithicSparse", "twoGbMaxExtentSparse", 1)
			case "unknown flags":
				binary.LittleEndian.PutUint32(data[8:], 1<<30)
			case "zero capacity":
				binary.LittleEndian.PutUint64(data[12:], 0)
			case "bad descriptor offset":
				binary.LittleEndian.PutUint64(data[28:], 2)
			case "truncated descriptor":
				data = data[:512]
			case "unknown version":
				binary.LittleEndian.PutUint32(data[4:], 99)
			case "missing descriptor terminator":
				for i := 512; i < 512+20*512; i++ {
					data[i] = 'a'
				}
				text = ""
			case "unsafe file type":
				if err := os.Symlink("/outside/secret.vmdk", p); err != nil {
					t.Fatal(err)
				}
			}
			if kind != "missing descriptor terminator" && len(data) > 512 {
				for i := 512; i < 512+20*512; i++ {
					data[i] = 0
				}
				copy(data[512:], text)
			}
			if kind != "unsafe file type" {
				if err := os.WriteFile(p, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := InspectVMDKFile(p); err == nil {
				t.Fatal("unsafe/dependent VMDK accepted", kind)
			}
		})
	}
}

// capability_id: rainbond.vm-build.vmdk-self-contained
func TestInspectVMDKRejectsDescriptorLegacyAndWrongExtensionPayload(t *testing.T) {
	for _, tt := range []struct {
		name           string
		data           []byte
		wantRecognized bool
		wantError      bool
	}{
		{"disk.img", []byte("# Disk DescriptorFile\nversion=1\nparentCID=ffffffff\ncreateType=\"monolithicFlat\"\nRW 2048 FLAT \"disk-flat.vmdk\" 0\n"), true, true},
		{"renamed.qcow2", []byte("# Disk DescriptorFile\nencoding=\"UTF-8\"\nversion=1\nparentCID=ffffffff\ncreateType=\"monolithicFlat\"\nRW 2048 FLAT \"external-flat.vmdk\" 0\n"), true, true},
		{"renamed.img", []byte("encoding=\"UTF-8\"\nversion=1\nparentCID=ffffffff\n"), true, true},
		{"disk.img", []byte("COWD legacy vmfs sparse"), true, true},
		{"disk.vmdk", []byte("not a virtual disk"), true, true},
		{"disk.raw", make([]byte, 1024), false, false},
		{"installer.iso", []byte("legacy ISO fixture"), false, false},
	} {
		t.Run(tt.name+string(tt.data[:4]), func(t *testing.T) {
			p := filepath.Join(t.TempDir(), tt.name)
			if err := os.WriteFile(p, tt.data, 0600); err != nil {
				t.Fatal(err)
			}
			recognized, err := InspectVMDKFile(p)
			if recognized != tt.wantRecognized || (err != nil) != tt.wantError {
				t.Fatal(recognized, err)
			}
		})
	}
}
