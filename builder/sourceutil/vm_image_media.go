package sourceutil

import (
	"bytes"
	"io"
	"os"
)

// VMImageMedia identifies trusted content routing without choosing a disk format.
type VMImageMedia string

const (
	// VMImageMediaUnknown preserves legacy filename routing, including wrappers.
	VMImageMediaUnknown VMImageMedia = "unknown"
	// VMImageMediaDisk identifies a structured disk that requires validation.
	VMImageMediaDisk VMImageMedia = "disk"
	// VMImageMediaISO identifies unstructured ISO9660 optical media.
	VMImageMediaISO VMImageMedia = "iso"
)

// InspectVMImageMediaFile routes media after the disk dependency preflights.
// Structured disk headers/footers take precedence over an embedded ISO marker.
// Compressed and archived inputs remain on their existing transport paths.
func InspectVMImageMediaFile(name string) (VMImageMedia, error) {
	original, err := os.Lstat(name)
	if err != nil {
		return VMImageMediaUnknown, err
	}
	if !original.Mode().IsRegular() {
		return VMImageMediaUnknown, vmdkError("vm_disk_unsafe_source", "镜像必须是普通文件，不能使用符号链接或目录")
	}
	file, err := os.Open(name)
	if err != nil {
		return VMImageMediaUnknown, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return VMImageMediaUnknown, err
	}
	if !os.SameFile(original, opened) {
		return VMImageMediaUnknown, vmdkError("vm_disk_unsafe_source", "镜像文件在检查时已变化")
	}

	head := make([]byte, 512)
	n, err := file.ReadAt(head, 0)
	if err != nil && err != io.EOF {
		return VMImageMediaUnknown, err
	}
	head = head[:n]
	if compressedVMWrapper(head) || vmTarHeader(head) {
		return VMImageMediaUnknown, nil
	}
	tail := make([]byte, 512)
	if opened.Size() >= int64(len(tail)) {
		if _, err = file.ReadAt(tail, opened.Size()-int64(len(tail))); err != nil {
			return VMImageMediaUnknown, err
		}
	}
	if bytes.HasPrefix(head, []byte{'Q', 'F', 'I', 0xfb}) || vmNativeDiskPrefix(head) || vmNativeVHDMagic(tail) {
		return VMImageMediaDisk, nil
	}

	descriptor := make([]byte, 7)
	n, err = file.ReadAt(descriptor, 16*2048)
	if err != nil && err != io.EOF {
		return VMImageMediaUnknown, err
	}
	if n == len(descriptor) && bytes.Equal(descriptor[1:6], []byte("CD001")) && descriptor[6] == 1 &&
		(descriptor[0] <= 3 || descriptor[0] == 255) {
		return VMImageMediaISO, nil
	}
	return VMImageMediaUnknown, nil
}
