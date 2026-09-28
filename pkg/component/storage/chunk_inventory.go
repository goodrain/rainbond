package storage

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"time"
)

// UploadChunkUsage describes observed current chunk objects, not reclaimable bytes.
// It excludes retained object versions and grants no deletion authority.
type UploadChunkUsage struct {
	Bytes   int64 `json:"bytes"`
	Objects int   `json:"objects"`
}

// MeasureUploadChunks reads a complete bounded inventory of one exact upload scope.
// Callers must preserve an error as unknown size rather than report zero bytes.
func (s3s *S3Storage) MeasureUploadChunks(ctx context.Context, sessionID string) (UploadChunkUsage, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	objects, err := s3s.listChunkObjects(ctx, sessionID)
	if err != nil {
		return UploadChunkUsage{}, err
	}
	usage := UploadChunkUsage{}
	for _, object := range objects {
		if object.Size == nil || *object.Size < 0 || *object.Size > math.MaxInt64-usage.Bytes {
			return UploadChunkUsage{}, fmt.Errorf("upload chunk size unavailable")
		}
		usage.Bytes += *object.Size
		usage.Objects++
	}
	return usage, nil
}

// MeasureUploadChunks observes local chunk files without treating a missing mount
// or directory as an empty upload. The observation is not a deletion fence.
func (l *LocalStorage) MeasureUploadChunks(ctx context.Context, sessionID string) (UploadChunkUsage, error) {
	if !chunkSessionIdentity.MatchString(sessionID) {
		return UploadChunkUsage{}, fmt.Errorf("invalid upload chunk inventory scope")
	}
	return measureLocalChunkDirectory(ctx, l.GetChunkDir(sessionID))
}

func measureLocalChunkDirectory(ctx context.Context, path string) (UploadChunkUsage, error) {
	if err := ctx.Err(); err != nil {
		return UploadChunkUsage{}, err
	}
	before, err := os.Lstat(path)
	if err != nil || !before.IsDir() {
		return UploadChunkUsage{}, fmt.Errorf("upload chunk directory unavailable")
	}
	dir, err := os.Open(path)
	if err != nil {
		return UploadChunkUsage{}, fmt.Errorf("upload chunk directory unavailable")
	}
	defer dir.Close()
	opened, err := dir.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return UploadChunkUsage{}, fmt.Errorf("upload chunk directory changed")
	}
	entries, err := dir.Readdir(10001)
	if err != nil && err != io.EOF {
		return UploadChunkUsage{}, fmt.Errorf("upload chunk inventory unavailable")
	}
	if len(entries) > 10000 {
		return UploadChunkUsage{}, fmt.Errorf("upload chunk inventory limit exceeded")
	}
	usage := UploadChunkUsage{}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return UploadChunkUsage{}, err
		}
		if !entry.Mode().IsRegular() || entry.Size() < 0 || entry.Size() > math.MaxInt64-usage.Bytes {
			return UploadChunkUsage{}, fmt.Errorf("upload chunk size unavailable")
		}
		usage.Bytes += entry.Size()
		usage.Objects++
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		return UploadChunkUsage{}, fmt.Errorf("upload chunk directory changed")
	}
	return usage, nil
}
