package sourceutil

import "strings"

// IsVMPlainDiskFormat identifies uncompressed disk inputs imported through CDI.
// IMG is a filename convention; Builder validates the actual content format.
func IsVMPlainDiskFormat(format string) bool {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "qcow2", "raw", "img", "vmdk", "vdi", "vhd", "vhdx":
		return true
	default:
		return false
	}
}
