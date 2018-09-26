package coquelicot

import (
	"os"
)

type fileDefaultManager struct {
	*fileBaseManager
	Size int64
}

func (fdm *fileDefaultManager) convert(src string, convert string) error {
	return fdm.rawCopy(src, convert)
}

func (fdm *fileDefaultManager) ToJson() map[string]interface{} {
	return map[string]interface{}{
		"url":      fdm.Url(),
		"filename": fdm.Filename,
		"size":     fdm.Size,
	}
}

func (fdm *fileDefaultManager) rawCopy(src, convert string) error {
	if err := fdm.copyFile(src, fdm.Filepath()); err != nil {
		return err
	}

	f, err := os.Open(fdm.Filepath())
	if err != nil {
		return err
	}
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	fdm.Size = fi.Size()

	return nil
}
