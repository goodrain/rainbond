package sourceutil

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const vmdkSector = 512
const vmDiskProbeBytes = 16 << 10
const vmdkKnownFlags = uint32(1 | 2 | 4 | 1<<16 | 1<<17)

// VMDKSourceError gives callers a stable reason and an actionable instruction,
// without echoing untrusted descriptor paths or encryption metadata.
type VMDKSourceError struct{ Code, Instruction string }

func (e *VMDKSourceError) Error() string       { return e.Code + ": " + e.Instruction }
func vmdkError(code, instruction string) error { return &VMDKSourceError{code, instruction} }
func invalidVMDK() error {
	return vmdkError("vm_vmdk_invalid", "VMDK文件头或磁盘元数据无效，请检查源磁盘并重新导出")
}
func dependentVMDK() error {
	return vmdkError("vm_vmdk_external_dependency", "VMDK包含父盘或外部分片，请正常关机后在VMware中合并为自包含单文件再上传")
}
func unsupportedVMDK() error {
	return vmdkError("vm_vmdk_unsupported_layout", "当前VMDK布局未支持，请导出自包含单文件monolithicSparse或streamOptimized磁盘")
}

// InspectVMDKFile runs before qemu can inspect an untrusted disk. The sparse
// header must unambiguously select the one input file, not a descriptor or
// external extent. Content magic, not a user-supplied suffix, is authoritative.
func InspectVMDKFile(name string) (bool, error) {
	info, err := os.Lstat(name)
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return true, vmdkError("vm_vmdk_unsafe_source", "镜像必须是普通文件，不能使用符号链接或目录")
	}
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return false, err
	}
	if !os.SameFile(info, opened) {
		return true, invalidVMDK()
	}
	header := make([]byte, 512)
	n, err := f.ReadAt(header, 0)
	if err != nil && err != io.EOF {
		return false, err
	}
	header = header[:n]
	if handled, err := inspectVMWrapper(f, header, info.Size()); handled || err != nil {
		return false, err
	}
	required := strings.HasSuffix(strings.ToLower(filepath.Base(name)), ".vmdk")
	if len(header) < 4 {
		if required {
			return true, invalidVMDK()
		}
		return false, nil
	}
	if string(header[:4]) == "COWD" {
		return true, unsupportedVMDK()
	}
	if string(header[:4]) != "KDMV" {
		probe := make([]byte, min(info.Size(), int64(vmDiskProbeBytes)))
		if _, err = f.ReadAt(probe, 0); err != nil {
			return false, err
		}
		if looksLikeVMDKDescriptor(probe) {
			return true, vmdkError("vm_vmdk_descriptor_only", "当前文件只是VMDK描述符，请在VMware中合并并导出包含全部数据的单文件磁盘")
		}
		if required {
			return true, invalidVMDK()
		}
		return false, nil
	}
	if len(header) != 512 {
		return true, invalidVMDK()
	}
	if err = validateVMDKHeader(header, info.Size()); err != nil {
		return true, err
	}
	size := binary.LittleEndian.Uint64(header[36:])
	descriptor := make([]byte, int(size)*vmdkSector)
	if _, err = f.ReadAt(descriptor, vmdkSector); err != nil {
		return true, invalidVMDK()
	}
	end := bytes.IndexByte(descriptor, 0)
	if end < 0 || end >= 20*vmdkSector {
		return true, invalidVMDK()
	}
	if len(bytes.Trim(descriptor[end:], "\x00")) != 0 {
		return true, invalidVMDK()
	}
	kind, err := validateVMDKDescriptor(string(descriptor[:end]), binary.LittleEndian.Uint64(header[12:]))
	if err != nil {
		return true, err
	}
	compressed := binary.LittleEndian.Uint16(header[77:]) == 1
	if (kind == "streamOptimized") != compressed {
		return true, unsupportedVMDK()
	}
	if binary.LittleEndian.Uint64(header[56:]) == math.MaxUint64 {
		if !compressed || kind != "streamOptimized" || info.Size() < 1536 {
			return true, invalidVMDK()
		}
		footer := make([]byte, 1536)
		if _, err = f.ReadAt(footer, info.Size()-1536); err != nil {
			return true, invalidVMDK()
		}
		if binary.LittleEndian.Uint32(footer[8:]) != 0 || binary.LittleEndian.Uint32(footer[12:]) != 3 || !bytes.Equal(footer[1024:1040], make([]byte, 16)) {
			return true, invalidVMDK()
		}
		last := footer[512:1024]
		if string(last[:4]) != "KDMV" || validateVMDKHeader(last, info.Size()) != nil || binary.LittleEndian.Uint64(last[56:]) == math.MaxUint64 {
			return true, invalidVMDK()
		}
		for _, p := range [][2]int{{4, 48}, {64, 79}} {
			if !bytes.Equal(header[p[0]:p[1]], last[p[0]:p[1]]) {
				return true, invalidVMDK()
			}
		}
	}
	return true, nil
}

func validateVMDKHeader(h []byte, size int64) error {
	if len(h) != 512 || size < 512 || size%512 != 0 {
		return invalidVMDK()
	}
	version := binary.LittleEndian.Uint32(h[4:])
	flags := binary.LittleEndian.Uint32(h[8:])
	capacity := binary.LittleEndian.Uint64(h[12:])
	grain := binary.LittleEndian.Uint64(h[20:])
	offset := binary.LittleEndian.Uint64(h[28:])
	length := binary.LittleEndian.Uint64(h[36:])
	gtes := binary.LittleEndian.Uint32(h[44:])
	overhead := binary.LittleEndian.Uint64(h[64:])
	compression := binary.LittleEndian.Uint16(h[77:])
	if version < 1 || version > 3 || flags & ^vmdkKnownFlags != 0 {
		return unsupportedVMDK()
	}
	if capacity == 0 || capacity > 1<<32 || grain == 0 || grain > 1<<21 || grain&(grain-1) != 0 || gtes == 0 || gtes > 512 || gtes&(gtes-1) != 0 || offset != 1 || length == 0 || length > 20 || length+offset > uint64(size/512) || overhead > uint64(size/512) {
		return invalidVMDK()
	}
	if h[72] != 0 {
		return vmdkError("vm_vmdk_unclean", "VMDK仍带未正常关闭标记，请先正常关闭虚拟机，再重新复制磁盘")
	}
	if compression > 1 || (compression == 1) != (flags&(1<<16) != 0) || compression == 1 && flags&(1<<17) == 0 {
		return unsupportedVMDK()
	}
	if flags&1 != 0 && !bytes.Equal(h[73:77], []byte{10, 32, 13, 10}) {
		return invalidVMDK()
	}
	l1bytes := ((capacity + uint64(gtes)*grain - 1) / (uint64(gtes) * grain)) * 4
	for _, p := range []int{48, 56} {
		position := binary.LittleEndian.Uint64(h[p:])
		if p == 48 && flags&2 == 0 {
			continue
		}
		if position == math.MaxUint64 && p == 56 {
			continue
		}
		if position < offset+length || position > uint64(size/512) || l1bytes > uint64(size)-position*512 {
			return invalidVMDK()
		}
	}
	return nil
}

var vmdkExtent = regexp.MustCompile(`^RW[ \t]+([0-9]+)[ \t]+SPARSE[ \t]+"([^"\r\n]+)"[ \t]*$`)
var vmdkHexCID = regexp.MustCompile(`^[0-9A-Fa-f]{1,8}$`)

func looksLikeVMDKDescriptor(data []byte) bool {
	value := strings.TrimSpace(strings.TrimPrefix(string(data), "\xef\xbb\xbf"))
	if strings.Contains(value, "Disk DescriptorFile") {
		return true
	}
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pair := strings.SplitN(line, "=", 2)
		if len(pair) == 2 {
			switch strings.TrimSpace(pair[0]) {
			case "version", "CID", "parentCID", "createType", "encoding":
				return true
			}
		}
	}
	return false
}

func validateVMDKDescriptor(text string, capacity uint64) (string, error) {
	lower := strings.ToLower(text)
	// qemu's VMDK parent lookup searches the raw text, including comments.
	if strings.Contains(lower, "parentfilenamehint") {
		return "", dependentVMDK()
	}
	if strings.Contains(lower, "encrypt") {
		return "", vmdkError("vm_vmdk_encrypted", "请先在VMware中解密磁盘，再导出自包含单文件VMDK")
	}
	for _, b := range []byte(text) {
		if b < 32 && b != '\r' && b != '\n' && b != '\t' {
			return "", invalidVMDK()
		}
	}
	values := map[string]string{}
	extents := 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "RW ") || strings.HasPrefix(line, "RW\t") || strings.HasPrefix(line, "RDONLY") || strings.HasPrefix(line, "NOACCESS") {
			m := vmdkExtent.FindStringSubmatch(line)
			if m == nil {
				return "", dependentVMDK()
			}
			sectors, err := strconv.ParseUint(m[1], 10, 64)
			if err != nil || sectors != capacity {
				return "", invalidVMDK()
			}
			if m[2] == "." || m[2] == ".." || strings.ContainsAny(m[2], `/\:`) {
				return "", dependentVMDK()
			}
			extents++
			continue
		}
		pair := strings.SplitN(line, "=", 2)
		if len(pair) != 2 {
			return "", invalidVMDK()
		}
		key, value := strings.TrimSpace(pair[0]), strings.TrimSpace(pair[1])
		if _, exists := values[key]; exists {
			return "", invalidVMDK()
		}
		switch key {
		case "version", "CID", "parentCID", "createType", "encoding":
		default:
			if !strings.HasPrefix(key, "ddb.") {
				return "", unsupportedVMDK()
			}
		}
		values[key] = value
	}
	if extents != 1 {
		return "", dependentVMDK()
	}
	if values["version"] != "1" || !vmdkHexCID.MatchString(values["CID"]) {
		return "", invalidVMDK()
	}
	if !strings.EqualFold(values["parentCID"], "ffffffff") {
		return "", dependentVMDK()
	}
	typeName, err := strconv.Unquote(values["createType"])
	if err != nil {
		return "", invalidVMDK()
	}
	if typeName != "monolithicSparse" && typeName != "streamOptimized" {
		return "", unsupportedVMDK()
	}
	return typeName, nil
}
