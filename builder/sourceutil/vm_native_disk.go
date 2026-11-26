package sourceutil

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
)

const vmVHDMaxBATBytes = 16 << 20

func invalidVMNativeDisk() error {
	return vmdkError("vm_disk_invalid", "VDI或VHD磁盘元数据无效，请检查源磁盘并重新导出完整单文件")
}

func dependentVMNativeDisk() error {
	return vmdkError("vm_disk_external_dependency", "VDI或VHD包含差分盘或父盘信息，请先合并为独立磁盘再上传")
}

// InspectVMNativeDiskFile rejects dependent VDI/VHD metadata before QEMU opens
// the image. QEMU does not expose all VDI/VHD parent metadata in its JSON, and
// its VPC driver can accept differencing disks without resolving their parent.
// Fixed VHDs retain their footer and original bytes; CDI probes them as RAW.
func InspectVMNativeDiskFile(name string) error {
	info, err := os.Lstat(name)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return vmdkError("vm_disk_unsafe_source", "镜像必须是普通文件，不能使用符号链接或目录")
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(info, opened) {
		return invalidVMNativeDisk()
	}
	head := make([]byte, 512)
	n, err := f.ReadAt(head, 0)
	if err != nil && err != io.EOF {
		return err
	}
	head = head[:n]
	// A structured outer disk may end with guest data containing a VHD footer.
	// Its own format validator owns that metadata; do not reinterpret the guest
	// footer as a fixed VHD container.
	if len(head) >= 4 && bytes.Equal(head[:4], []byte{'Q', 'F', 'I', 0xfb}) ||
		len(head) >= 8 && string(head[:8]) == "vhdxfile" {
		return nil
	}
	tail := make([]byte, 512)
	if info.Size() >= 512 {
		if _, err = f.ReadAt(tail, info.Size()-512); err != nil {
			return err
		}
	}
	vdi := vmNativeVDIMagic(head)
	vhd := vmNativeVHDMagic(head) || vmNativeVHDMagic(tail)
	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".vdi" && !vdi || ext == ".vhd" && !vhd {
		return invalidVMNativeDisk()
	}
	if vdi {
		if len(head) < 512 || binary.LittleEndian.Uint32(head[68:]) != 0x00010001 {
			return invalidVMNativeDisk()
		}
		kind := binary.LittleEndian.Uint32(head[76:])
		if kind != 1 && kind != 2 || vmNativeNonzero(head[424:456]) {
			return dependentVMNativeDisk()
		}
		return nil
	}
	if vhd {
		return inspectVMVHD(f, head, tail, uint64(info.Size()))
	}
	return nil
}

func vmNativeVDIMagic(data []byte) bool {
	return len(data) >= 68 && binary.LittleEndian.Uint32(data[64:68]) == 0xbeda107f
}

func vmNativeVHDMagic(data []byte) bool {
	return len(data) >= 8 && string(data[:8]) == "conectix"
}

func vmNativeDiskPrefix(data []byte) bool {
	return vmNativeVDIMagic(data) || vmNativeVHDMagic(data) || len(data) >= 8 && string(data[:8]) == "vhdxfile"
}

func vmNativeNonzero(data []byte) bool {
	for _, value := range data {
		if value != 0 {
			return true
		}
	}
	return false
}

func vmNativeChecksumValid(data []byte, offset int) bool {
	var sum uint32
	for i, value := range data {
		if i < offset || i >= offset+4 {
			sum += uint32(value)
		}
	}
	return ^sum == binary.BigEndian.Uint32(data[offset:offset+4])
}

func inspectVMVHD(f *os.File, head, footer []byte, size uint64) error {
	if size < 1024 || size%512 != 0 || !vmNativeVHDMagic(footer) || !vmNativeChecksumValid(footer, 64) ||
		binary.BigEndian.Uint32(footer[12:]) != 0x00010000 {
		return invalidVMNativeDisk()
	}
	kind := binary.BigEndian.Uint32(footer[60:])
	if kind != 2 && kind != 3 {
		return dependentVMNativeDisk()
	}
	if vmNativeVHDMagic(head) && !bytes.Equal(head, footer) {
		return invalidVMNativeDisk()
	}
	current := binary.BigEndian.Uint64(footer[48:])
	if current == 0 || current%512 != 0 || current > math.MaxInt64-512 {
		return invalidVMNativeDisk()
	}
	headerOffset := binary.BigEndian.Uint64(footer[16:])
	if kind == 2 {
		if headerOffset != math.MaxUint64 || current+512 != size {
			return invalidVMNativeDisk()
		}
		return nil
	}
	footerStart := size - 512
	if !vmNativeVHDMagic(head) || headerOffset < 512 || headerOffset%512 != 0 ||
		headerOffset > footerStart || footerStart-headerOffset < 1024 {
		return invalidVMNativeDisk()
	}
	header := make([]byte, 1024)
	if _, err := f.ReadAt(header, int64(headerOffset)); err != nil {
		return err
	}
	if string(header[:8]) != "cxsparse" || binary.BigEndian.Uint32(header[24:]) != 0x00010000 ||
		!vmNativeChecksumValid(header, 36) {
		return invalidVMNativeDisk()
	}
	if vmNativeNonzero(header[40:56]) || vmNativeNonzero(header[64:768]) {
		return dependentVMNativeDisk()
	}
	tableOffset := binary.BigEndian.Uint64(header[16:])
	entries := uint64(binary.BigEndian.Uint32(header[28:]))
	blockSize := uint64(binary.BigEndian.Uint32(header[32:]))
	if entries == 0 || entries > vmVHDMaxBATBytes/4 || blockSize < 512 || blockSize&(blockSize-1) != 0 ||
		entries < (current+blockSize-1)/blockSize {
		return invalidVMNativeDisk()
	}
	tableBytes := entries * 4
	tablePadded := (tableBytes + 511) &^ uint64(511)
	if tableOffset%512 != 0 || tableOffset < headerOffset+1024 || tableOffset > footerStart ||
		tablePadded > footerStart-tableOffset {
		return invalidVMNativeDisk()
	}
	bitmapBytes := (((blockSize/512+7)/8 + 511) / 512) * 512
	blockBytes := bitmapBytes + blockSize
	dataStart := tableOffset + tablePadded
	// Stream a bounded BAT without allocating memory from attacker-controlled
	// entry counts. An out-of-file allocated block must not become guest zeros.
	buffer := make([]byte, 4096)
	for done := uint64(0); done < tableBytes; {
		chunk := min(uint64(len(buffer)), tableBytes-done)
		if _, err := f.ReadAt(buffer[:chunk], int64(tableOffset+done)); err != nil {
			return err
		}
		for i := uint64(0); i < chunk; i += 4 {
			sector := binary.BigEndian.Uint32(buffer[i : i+4])
			if sector == math.MaxUint32 {
				continue
			}
			offset := uint64(sector) * 512
			if offset < dataStart || blockBytes > footerStart || offset > footerStart-blockBytes {
				return invalidVMNativeDisk()
			}
		}
		done += chunk
	}
	return nil
}
