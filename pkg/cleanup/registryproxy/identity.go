//go:build linux || darwin

package registryproxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	"golang.org/x/sys/unix"
)

// ErrStorageIdentity hides file contents and refuses an unverified data root.
var ErrStorageIdentity = errors.New("registry storage identity unavailable or changed")

const identityDirectory = ".rainbond-cleanup"
const identityFile = "identity.json"

type identityMarker struct {
	Protocol int                              `json:"protocol"`
	Binding  coordination.StorageRegistration `json:"binding"`
}

func openIdentityRoot(root string) (int, error) {
	fd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, ErrStorageIdentity
	}
	duplicate, err := unix.Dup(fd)
	if err != nil {
		unix.Close(fd)
		return -1, ErrStorageIdentity
	}
	file := os.NewFile(uintptr(duplicate), "registry-root")
	entries, readErr := file.ReadDir(4)
	file.Close()
	if readErr != nil && readErr != io.EOF {
		unix.Close(fd)
		return -1, ErrStorageIdentity
	}
	for _, entry := range entries {
		if !entry.IsDir() || (entry.Name() != "docker" && entry.Name() != "lost+found" && entry.Name() != identityDirectory) {
			unix.Close(fd)
			return -1, ErrStorageIdentity
		}
	}
	if len(entries) > 3 {
		unix.Close(fd)
		return -1, ErrStorageIdentity
	}
	return fd, nil
}
func readIdentity(directory int, expected coordination.StorageRegistration) error {
	fd, err := unix.Openat(directory, identityFile, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), "registry-identity")
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return ErrStorageIdentity
	}
	raw, err := io.ReadAll(io.LimitReader(file, 8193))
	if err != nil || len(raw) > 8192 {
		return ErrStorageIdentity
	}
	var marker identityMarker
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&marker) != nil || decoder.Decode(&struct{}{}) != io.EOF || marker.Protocol != 1 || marker.Binding != expected {
		return ErrStorageIdentity
	}
	return nil
}

// VerifyStorageIdentity only observes an existing marker. It cannot initialize
// or rebind storage. The expected volume identity must come from the control plane.
func VerifyStorageIdentity(root string, expected coordination.StorageRegistration) error {
	if _, err := expected.Fingerprint(); err != nil {
		return ErrStorageIdentity
	}
	rootFD, err := openIdentityRoot(root)
	if err != nil {
		return err
	}
	defer unix.Close(rootFD)
	directory, err := unix.Openat(rootFD, identityDirectory, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return ErrStorageIdentity
	}
	defer unix.Close(directory)
	if err := readIdentity(directory, expected); err != nil {
		return ErrStorageIdentity
	}
	return nil
}

// InitializeStorageIdentity is an installer action on a verified Registry mount.
// It creates only owned metadata, publishes atomically without replacement and
// never converts an existing mismatched identity into the requested identity.
func InitializeStorageIdentity(root string, expected coordination.StorageRegistration) error {
	if _, err := expected.Fingerprint(); err != nil {
		return ErrStorageIdentity
	}
	rootFD, err := openIdentityRoot(root)
	if err != nil {
		return err
	}
	defer unix.Close(rootFD)
	if err := unix.Mkdirat(rootFD, identityDirectory, 0755); err != nil && err != unix.EEXIST {
		return ErrStorageIdentity
	}
	directory, err := unix.Openat(rootFD, identityDirectory, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return ErrStorageIdentity
	}
	defer unix.Close(directory)
	existing := readIdentity(directory, expected)
	if existing == nil {
		if unix.Fsync(directory) != nil || unix.Fsync(rootFD) != nil {
			return ErrStorageIdentity
		}
		return nil
	}
	if !errors.Is(existing, unix.ENOENT) {
		return ErrStorageIdentity
	}
	id, err := coordination.NewActivationRevision()
	if err != nil {
		return ErrStorageIdentity
	}
	temporary := "identity-" + id + ".tmp"
	fd, err := unix.Openat(directory, temporary, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return ErrStorageIdentity
	}
	defer unix.Unlinkat(directory, temporary, 0)
	file := os.NewFile(uintptr(fd), "registry-identity-temporary")
	raw, err := json.Marshal(identityMarker{Protocol: 1, Binding: expected})
	if err == nil {
		_, err = file.Write(raw)
	}
	if err == nil {
		err = file.Chmod(0444)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		return ErrStorageIdentity
	}
	if err := unix.Linkat(directory, temporary, directory, identityFile, 0); err != nil && err != unix.EEXIST {
		return ErrStorageIdentity
	}
	if err := readIdentity(directory, expected); err != nil {
		return ErrStorageIdentity
	}
	if unix.Fsync(directory) != nil || unix.Fsync(rootFD) != nil {
		return ErrStorageIdentity
	}
	return nil
}
