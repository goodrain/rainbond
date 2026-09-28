package storage

import (
	"context"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MeasureUploadEvent measures current package objects and extracted files within
// one server-derived event prefix. It does not measure historical object versions.
func (s3s *S3Storage) MeasureUploadEvent(ctx context.Context, eventID string) (UploadChunkUsage, error) {
	if !chunkSessionIdentity.MatchString(eventID) || s3s.s3Client == nil {
		return UploadChunkUsage{}, fmt.Errorf("invalid upload event scope")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	objects, err := s3s.listUploadObjects(ctx, "package_build/temp/events/"+eventID+"/")
	if err != nil {
		return UploadChunkUsage{}, err
	}
	return measureUploadObjects(objects)
}

// MeasureUploadEvent observes an event tree without following links outside it.
// A missing directory is unknown, not proof that the event has no stored data.
func (l *LocalStorage) MeasureUploadEvent(ctx context.Context, eventID string) (UploadChunkUsage, error) {
	if !chunkSessionIdentity.MatchString(eventID) {
		return UploadChunkUsage{}, fmt.Errorf("invalid upload event scope")
	}
	return measureLocalUploadEvent(ctx, filepath.Join("/grdata/package_build/temp/events", eventID))
}

func measureLocalUploadEvent(ctx context.Context, path string) (UploadChunkUsage, error) {
	if err := ctx.Err(); err != nil {
		return UploadChunkUsage{}, err
	}
	before, err := os.Lstat(path)
	if err != nil || !before.IsDir() {
		return UploadChunkUsage{}, fmt.Errorf("upload package directory unavailable")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return UploadChunkUsage{}, fmt.Errorf("upload package directory unavailable")
	}
	defer root.Close()
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(before, opened) {
		return UploadChunkUsage{}, fmt.Errorf("upload package directory changed")
	}
	usage := UploadChunkUsage{}
	visited := 0
	err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("upload package inventory unavailable")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		visited++
		if visited > 10001 || strings.Count(name, "/") > 32 {
			return fmt.Errorf("upload package inventory limit exceeded")
		}
		info, err := root.Lstat(name)
		if err != nil {
			return fmt.Errorf("upload package entry unavailable")
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > math.MaxInt64-usage.Bytes {
			return fmt.Errorf("upload package size unavailable")
		}
		usage.Bytes += info.Size()
		usage.Objects++
		return nil
	})
	if err != nil {
		return UploadChunkUsage{}, err
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		return UploadChunkUsage{}, fmt.Errorf("upload package directory changed")
	}
	return usage, nil
}
