package copyio

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"syscall"
)

func openRegular(path string) (*os.File, os.FileInfo, error) {
	// Nonblocking open lets the descriptor check reject FIFOs without a writer.
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, err
	}
	raw, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || raw.Nlink != 1 || info.Size() < 1 {
		file.Close()
		return nil, nil, ErrIdentity
	}
	return file, info, nil
}
func hashHex(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

// Scan reads every logical byte and counts the exact chunk bytes copy would write.
// st_blocks is reported as telemetry only; CephFS may derive it from logical size.
func Scan(ctx context.Context, path string, maxBytes int64) (FileResult, error) {
	result := FileResult{}
	file, before, err := openRegular(path)
	if err != nil {
		return result, err
	}
	defer file.Close()
	if before.Size() > maxBytes {
		return result, ErrIdentity
	}
	result.LogicalBytes = before.Size()
	raw := before.Sys().(*syscall.Stat_t)
	result.ReportedStatBlocks = raw.Blocks
	result.UID = raw.Uid
	result.GID = raw.Gid
	result.Mode = uint32(before.Mode().Perm())
	hash := sha256.New()
	buffer := make([]byte, ChunkBytes)
	zero := make([]byte, ChunkBytes)
	head := make([]byte, 0, 4096)
	tail := make([]byte, 0, 4096)
	for result.ReadBytes < before.Size() {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		wanted := int64(len(buffer))
		if remaining := before.Size() - result.ReadBytes; wanted > remaining {
			wanted = remaining
		}
		n, err := io.ReadFull(file, buffer[:wanted])
		result.ReadBytes += int64(n)
		if err != nil {
			return result, err
		}
		data := buffer[:n]
		hash.Write(data)
		if !bytes.Equal(data, zero[:n]) {
			result.NonzeroChunkBytes += int64(n)
		}
		if len(head) < 4096 {
			take := 4096 - len(head)
			if take > n {
				take = n
			}
			head = append(head, data[:take]...)
		}
		if n >= 4096 {
			tail = append(tail[:0], data[n-4096:]...)
		} else {
			tail = append(tail, data...)
			if len(tail) > 4096 {
				tail = tail[len(tail)-4096:]
			}
		}
	}
	extra := make([]byte, 1)
	n, err := file.Read(extra)
	result.ReadBytes += int64(n)
	if n != 0 || err != io.EOF {
		return result, ErrIdentity
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return result, ErrIdentity
	}
	result.SHA256 = hex.EncodeToString(hash.Sum(nil))
	result.HeadSHA256 = hashHex(head)
	result.TailSHA256 = hashHex(tail)
	return result, nil
}
