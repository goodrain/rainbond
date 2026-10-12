package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strings"
)

func uploadBinding(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

// UploadChunkBinding identifies the configured namespace of native upload chunks.
// It excludes credentials and is not a proof of writer coverage or free space.
func (s *Component) UploadChunkBinding() (string, error) {
	if s == nil || s.storageConfig == nil {
		return "", fmt.Errorf("upload storage unavailable")
	}
	if s.storageConfig.StorageType == "s3" {
		endpoint, err := url.Parse(s.storageConfig.S3Endpoint)
		if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || (endpoint.Path != "" && endpoint.Path != "/") {
			return "", fmt.Errorf("upload storage identity unavailable")
		}
		return uploadBinding("s3-upload-chunks", endpoint.Scheme, strings.ToLower(endpoint.Host), "grdata", "package_build/temp/chunks"), nil
	}
	return localUploadChunkBinding("/grdata")
}

func localUploadChunkBinding(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("upload storage root unavailable")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return "", fmt.Errorf("upload storage root unavailable")
	}
	defer root.Close()
	anchored, err := root.Stat(".")
	if err != nil || !os.SameFile(info, anchored) {
		return "", fmt.Errorf("upload storage root changed")
	}
	// Unix device/inode identify the opened root; unsupported platforms stay closed.
	stat := reflect.ValueOf(anchored.Sys())
	if stat.Kind() != reflect.Pointer || stat.IsNil() {
		return "", fmt.Errorf("upload storage identity unavailable")
	}
	stat = stat.Elem()
	if stat.Kind() != reflect.Struct {
		return "", fmt.Errorf("upload storage identity unavailable")
	}
	dev, ino := stat.FieldByName("Dev"), stat.FieldByName("Ino")
	if !dev.IsValid() || !ino.IsValid() || !dev.CanInterface() || !ino.CanInterface() {
		return "", fmt.Errorf("upload storage identity unavailable")
	}
	host, err := os.Hostname()
	if err != nil || host == "" {
		return "", fmt.Errorf("upload storage host unavailable")
	}
	return uploadBinding("local-upload-chunks", host, fmt.Sprint(dev.Interface()), fmt.Sprint(ino.Interface()), "package_build/temp/chunks"), nil
}
