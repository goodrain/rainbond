package copyio

import (
	"bytes"
	"context"
	"io"
	"os"
	"syscall"
)

type byteWriter interface{ Write([]byte) (int, error) }

// writeBounded checks the worst case before each syscall and counts short writes.
func writeBounded(writer byteWriter, data []byte, completed *int64, limit int64) error {
	for len(data) > 0 {
		if int64(len(data)) > limit-*completed {
			return ErrWriteBudget
		}
		n, err := writer.Write(data)
		if n < 0 || n > len(data) {
			return io.ErrShortWrite
		}
		*completed += int64(n)
		data = data[n:]
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}
func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

// ZeroScanCopy skips only chunks actually read as all-zero. It never invokes
// copy_file_range, reflink, cp, shell commands or source-file writes.
func ZeroScanCopy(ctx context.Context, source, target string, size, limit int64) (int64, int64, error) {
	input, before, err := openRegular(source)
	if err != nil {
		return 0, 0, err
	}
	defer input.Close()
	if size < 1 || before.Size() != size || limit < 0 {
		return 0, 0, ErrIdentity
	}
	fd, err := syscall.Open(target, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return 0, 0, err
	}
	output := os.NewFile(uintptr(fd), target)
	defer output.Close()
	buffer := make([]byte, ChunkBytes)
	zero := make([]byte, ChunkBytes)
	var offset, written, skipped int64
	for offset < size {
		if err := ctx.Err(); err != nil {
			return written, skipped, err
		}
		wanted := int64(len(buffer))
		if size-offset < wanted {
			wanted = size - offset
		}
		n, err := io.ReadFull(input, buffer[:wanted])
		if err != nil {
			return written, skipped, err
		}
		data := buffer[:n]
		if bytes.Equal(data, zero[:n]) {
			if _, err := output.Seek(int64(n), io.SeekCurrent); err != nil {
				return written, skipped, err
			}
			skipped += int64(n)
		} else if err := writeBounded(output, data, &written, limit); err != nil {
			return written, skipped, err
		}
		offset += int64(n)
	}
	extra := make([]byte, 1)
	if n, err := input.Read(extra); n != 0 || err != io.EOF {
		return written, skipped, ErrIdentity
	}
	after, err := input.Stat()
	if err != nil || !os.SameFile(before, after) || after.Size() != size || !after.ModTime().Equal(before.ModTime()) {
		return written, skipped, ErrIdentity
	}
	if err := output.Truncate(size); err != nil {
		return written, skipped, err
	}
	if err := output.Sync(); err != nil {
		return written, skipped, err
	}
	return written, skipped, nil
}
