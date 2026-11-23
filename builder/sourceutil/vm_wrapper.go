package sourceutil

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/xi2/xz"
)

func wrappedVMDK() error {
	return vmdkError("vm_vmdk_wrapped", "压缩或归档中的VMDK不支持直接导入，请先解压并合并成单个自包含VMDK再上传")
}
func invalidVMWrapper() error {
	return vmdkError("vm_image_archive_invalid", "镜像压缩流或归档无效、超出安全限制，请重新生成单文件镜像或归档")
}

// Preserve compressed RAW/QCOW2 paths without hiding a VMDK from preflight.
// Inspect streams only; no archive member is extracted on the host.
func inspectVMWrapper(f *os.File, head []byte, _ int64) (bool, error) {
	if unsupportedVMWrapper(head) {
		return true, vmdkError("vm_image_compression_unsupported", "当前不支持Bzip2或Zstd镜像包，请改用未压缩镜像、gzip或xz")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return false, err
	}
	deadline := time.Now().Add(10 * time.Minute)
	input := &vmInspectionReader{reader: f, deadline: deadline}
	var reader io.Reader
	if len(head) >= 2 && bytes.Equal(head[:2], []byte{0x1f, 0x8b}) {
		z, err := gzip.NewReader(input)
		if err != nil {
			return true, invalidVMWrapper()
		}
		defer z.Close()
		reader = z
	} else if len(head) >= 6 && bytes.Equal(head[:6], []byte{0xfd, '7', 'z', 'X', 'Z', 0}) {
		// xi2/xz's argument is a true dictionary maximum for every stream,
		// unlike readers which treat DictCap as an allocation minimum.
		z, err := xz.NewReader(input, 64<<20)
		if err != nil {
			return true, invalidVMWrapper()
		}
		reader = z
	} else if vmTarHeader(head) {
		return true, inspectVMTar(f)
	} else {
		return false, nil
	}
	buffered := bufio.NewReaderSize(&vmInspectionReader{reader: reader, deadline: deadline}, vmDiskProbeBytes)
	prefix, err := buffered.Peek(vmDiskProbeBytes)
	if err != nil && err != io.EOF {
		return true, vmWrapperReadError(err)
	}
	if vmVMDKPrefix(prefix) {
		return true, wrappedVMDK()
	}
	if compressedVMWrapper(prefix) {
		return true, invalidVMWrapper()
	}
	if !vmTarHeader(prefix) {
		return true, nil
	}
	return true, inspectVMTar(buffered)
}

func inspectVMTar(reader io.Reader) error {
	tr := tar.NewReader(reader)
	var total int64
	for count := 0; ; count++ {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return vmWrapperReadError(err)
		}
		if count >= 256 || len(h.Name) > 1024 || path.IsAbs(h.Name) || path.Clean(h.Name) == ".." || strings.HasPrefix(path.Clean(h.Name), "../") || strings.Contains(h.Name, `\`) {
			return invalidVMWrapper()
		}
		if h.Typeflag == tar.TypeDir {
			continue
		}
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA || h.Size < 0 || h.Size > 2<<40 || total > 2<<40-h.Size {
			return invalidVMWrapper()
		}
		total += h.Size
		if strings.HasSuffix(strings.ToLower(h.Name), ".vmdk") {
			return wrappedVMDK()
		}
		data := make([]byte, min(h.Size, int64(vmDiskProbeBytes)))
		n, err := io.ReadFull(tr, data)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return vmWrapperReadError(err)
		}
		if vmVMDKPrefix(data[:n]) {
			return wrappedVMDK()
		}
		if vmTarHeader(data[:n]) || unsupportedVMWrapper(data[:n]) || n >= 2 && bytes.Equal(data[:2], []byte{0x1f, 0x8b}) || n >= 6 && bytes.Equal(data[:6], []byte{0xfd, '7', 'z', 'X', 'Z', 0}) {
			return invalidVMWrapper()
		}
	}
}

func vmWrapperReadError(err error) error {
	if errors.Is(err, xz.ErrMemlimit) {
		return vmdkError("vm_image_xz_dictionary_limit", "XZ字典超过64MiB，请以较小字典重新压缩或上传未压缩镜像")
	}
	return invalidVMWrapper()
}

type vmInspectionReader struct {
	reader   io.Reader
	deadline time.Time
}

func (r *vmInspectionReader) Read(data []byte) (int, error) {
	if time.Now().After(r.deadline) {
		return 0, invalidVMWrapper()
	}
	return r.reader.Read(data)
}

func unsupportedVMWrapper(data []byte) bool {
	if len(data) >= 3 && string(data[:3]) == "BZh" {
		return true
	}
	if len(data) < 4 {
		return false
	}
	magic := binary.LittleEndian.Uint32(data[:4])
	return magic == 0xfd2fb528 || magic&0xfffffff0 == 0x184d2a50
}

func compressedVMWrapper(data []byte) bool {
	return unsupportedVMWrapper(data) || len(data) >= 2 && bytes.Equal(data[:2], []byte{0x1f, 0x8b}) || len(data) >= 6 && bytes.Equal(data[:6], []byte{0xfd, '7', 'z', 'X', 'Z', 0})
}

func vmVMDKPrefix(data []byte) bool {
	return len(data) >= 4 && (string(data[:4]) == "KDMV" || string(data[:4]) == "COWD") || looksLikeVMDKDescriptor(data)
}

func vmTarHeader(data []byte) bool {
	if len(data) < 512 {
		return false
	}
	want, err := strconv.ParseInt(strings.Trim(string(data[148:156]), " \x00"), 8, 64)
	if err != nil {
		return false
	}
	var sum int64
	for i, b := range data[:512] {
		if i >= 148 && i < 156 {
			sum += 32
		} else {
			sum += int64(b)
		}
	}
	return want == sum
}
