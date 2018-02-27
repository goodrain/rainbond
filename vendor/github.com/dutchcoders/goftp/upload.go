// Package goftp upload helper
package goftp

import (
	"os"
	"path/filepath"
)

func (ftp *FTP) copyDir(localPath string) error {
	fullPath, err := filepath.Abs(localPath)
	if err != nil {
		return err
	}

	pwd, err := ftp.Pwd()
	if err != nil {
		return err
	}

	walkFunc := func(path string, fi os.FileInfo, err error) error {
		// Stop upon error
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(fullPath, path)
		if err != nil {
			return err
		}
		switch {
		case fi.IsDir():
			// Walk calls walkFn on root as well
			if path == fullPath {
				return nil
			}
			if err = ftp.Mkd(relPath); err != nil {
				if _, err = ftp.List(relPath + "/"); err != nil {
					return err
				}
			}
		case fi.Mode()&os.ModeSymlink == os.ModeSymlink:
			fInfo, err := os.Stat(path)
			if err != nil {
				return err
			}
			if fInfo.IsDir() {
				err = ftp.Mkd(relPath)
				return err
			} else if fInfo.Mode()&os.ModeType != 0 {
				// ignore other special files
				return nil
			}
			fallthrough
		case fi.Mode()&os.ModeType == 0:
			if err = ftp.copyFile(path, pwd+"/"+relPath); err != nil {
				return err
			}
		default:
			// Ignore other special files
		}

		return nil
	}

	return filepath.Walk(fullPath, walkFunc)
}

func (ftp *FTP) copyFile(localPath, serverPath string) (err error) {
	var file *os.File
	if file, err = os.Open(localPath); err != nil {
		return err
	}
	defer file.Close()
	if err := ftp.Stor(serverPath, file); err != nil {
		return err
	}

	return nil
}

// Upload a file, or recursively upload a directory.
// Only normal files and directories are uploaded.
// Symlinks are not kept but treated as normal files/directories if targets are so.
func (ftp *FTP) Upload(localPath string) (err error) {
	fInfo, err := os.Stat(localPath)
	if err != nil {
		return err
	}

	switch {
	case fInfo.IsDir():
		return ftp.copyDir(localPath)
	case fInfo.Mode()&os.ModeType == 0:
		return ftp.copyFile(localPath, filepath.Base(localPath))
	default:
		// Ignore other special files
	}

	return nil
}
