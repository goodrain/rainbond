package upload

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type fileManager interface {
	convert(string) error
	SetFilename(*originalFile)
	ToJson() map[string]interface{}
}

type fileBaseManager struct {
	Dir      *dirManager
	Version  string
	Filename string
}

// Return fileManager for given base mime and version.
func newFileManager(dm *dirManager, version string) fileManager {
	fbm := &fileBaseManager{Dir: dm, Version: version}
	fdm := &fileDefaultManager{fileBaseManager: fbm}
	return fdm
}

func (fbm *fileBaseManager) SetFilename(file *originalFile) {
		fbm.Filename = file.Filename
}

func (fbm *fileBaseManager) Filepath() string {
	return filepath.Join(fbm.Dir.Abs(), fbm.Filename)
}

func (fbm *fileBaseManager) Url() string {
	return filepath.Join(fbm.Dir.Path, fbm.Filename)
}

// copyFile copies a file from src to dst. If src and dst files exist, and are
// the same, then return success. Otherwise copy the file contents from src to dst.
func (fbm *fileBaseManager) copyFile(src, dst string) error {
	sfi, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !sfi.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		// FIXME
		return fmt.Errorf("CopyFile: non-regular source file %s (%q)", sfi.Name(), sfi.Mode().String())
	}
	dfi, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		if !(dfi.Mode().IsRegular()) {
			return fmt.Errorf("CopyFile: non-regular destination file %s (%q)", dfi.Name(), dfi.Mode().String())
		}
		if os.SameFile(sfi, dfi) {
			return err
		}
	}
	if err := fbm.copyFileContents(src, dst); err != nil {
		return err
	}
	return nil
}

// copyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
func (fbm *fileBaseManager) copyFileContents(src, dst string) error {
	var err error
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	err = out.Sync()
	return err
}

func seconds() int64 {
	t := time.Now()
	return int64(t.Hour()*3600 + t.Minute()*60 + t.Second())
}
