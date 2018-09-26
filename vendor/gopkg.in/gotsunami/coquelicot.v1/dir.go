package coquelicot

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Directory mananger
type dirManager struct {
	Root string
	Path string
}

// Prepare dirManager given root, mime.
func createDir(root, mime string) (*dirManager, error) {
	dm := newDirManager(root)

	dm.CalcPath(mime)
	if err := dm.create(); err != nil {
		return nil, err
	}

	return dm, nil
}

// Check path and return dirManager.
func checkDir(root, path string) (*dirManager, error) {
	dm := newDirManager(root)

	if m, _ := filepath.Match("/[a-z]*/[0-9]*/[0-9a-z]*/[0-9a-z]*", path); m != true {
		return nil, errors.New("dir: path does not match the pattern")
	}
	dm.Path = path

	return dm, nil
}

// newDirManager returns a new dirManager given a root.
func newDirManager(root string) *dirManager {
	return &dirManager{Root: root}
}

// Return absolute path for directory
func (dm *dirManager) Abs() string {
	return filepath.Join(dm.Root, dm.Path)
}

// Create directory obtained by concatenating the root and path.
func (dm *dirManager) create() error {
	return os.MkdirAll(dm.Root+dm.Path, 0755)
}

// Generate path given mime and date.
func (dm *dirManager) CalcPath(mime string) {
	h := sha1.New()
	io.WriteString(h, fmt.Sprintf("%d", time.Now().UnixNano()))
	dm.Path = fmt.Sprintf("/%x", h.Sum(nil))
}

func yearDay(t time.Time) string {
	return strconv.FormatInt(int64(t.YearDay()), 36)
}

func containerName(t time.Time) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano())).Intn(1000)
	seconds := t.Hour()*3600 + t.Minute()*60 + t.Second()

	return strconv.FormatInt(int64(seconds*1000+r), 36)
}
