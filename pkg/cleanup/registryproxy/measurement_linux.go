package registryproxy

import "golang.org/x/sys/unix"

func measurementBlockSize(fs *unix.Statfs_t) uint64 {
	if fs.Frsize > 0 {
		return uint64(fs.Frsize)
	}
	if fs.Bsize > 0 {
		return uint64(fs.Bsize)
	}
	return 0
}
