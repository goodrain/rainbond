package exector

import (
	"compress/gzip"
	"fmt"
	"io"
	"syscall"
)

// Bound disk preparation per builder, without occupying ordinary build slots.
var vmDeliveryBuildSlots = make(chan struct{}, 1)

type vmDeliveryDiskWriter struct {
	destination io.Writer
	available   func() (uint64, error)
	written     int64
	nextCheck   int64
}

func (w *vmDeliveryDiskWriter) Write(data []byte) (int, error) {
	if w.written >= w.nextCheck {
		free, err := w.available()
		if err != nil {
			return 0, err
		}
		if free < (1<<30)+uint64(len(data)) {
			return 0, fmt.Errorf("insufficient temporary space for VM delivery image")
		}
		w.nextCheck = w.written + (64 << 20)
	}
	n, err := w.destination.Write(data)
	w.written += int64(n)
	return n, err
}

// Preserve compressed bytes while checking the entire gzip stream and trailer.
// A clean HTTP EOF must not turn a truncated disk into a successful artifact.
func copyVMDeliveryDisk(destination io.Writer, source io.Reader, available func() (uint64, error)) (int64, error) {
	writer := &vmDeliveryDiskWriter{destination: destination, available: available}
	reader, err := gzip.NewReader(io.TeeReader(source, writer))
	if err != nil {
		return writer.written, err
	}
	_, err = io.Copy(io.Discard, reader)
	closeErr := reader.Close()
	if err == nil {
		err = closeErr
	}
	return writer.written, err
}

func vmDeliveryAvailableSpace(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return stat.Bavail * uint64(stat.Bsize), nil
}
