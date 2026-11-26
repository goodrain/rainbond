package sourceutil

import "testing"

// capability_id: rainbond.vm-build.native-disk-artifacts
func TestVMPlainDiskFormatsKeepOpticalAndWrappedMediaSeparate(t *testing.T) {
	for _, format := range []string{"qcow2", "raw", "img", "vmdk", "vdi", "vhd", "vhdx", " VHDX "} {
		if !IsVMPlainDiskFormat(format) {
			t.Fatal("supported plain disk format rejected", format)
		}
	}
	for _, format := range []string{"", "iso", "tar", "raw.gz", "vmdk.gz", "vdi.xz", "vpc", "qcow", "unknown"} {
		if IsVMPlainDiskFormat(format) {
			t.Fatal("optical, wrapped or unsupported format entered the plain-disk flow", format)
		}
	}
}
