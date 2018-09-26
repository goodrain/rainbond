// Package coquelicot provides (chunked) file upload capability (with resume).
package coquelicot

type Storage struct {
	output    string
	verbosity int
}

// FIXME: global for now
var makeThumbnail bool

func (s *Storage) StorageDir() string {
	return s.output
}

func NewStorage(rootDir string) *Storage {
	return &Storage{output: rootDir}
}
