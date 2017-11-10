// Copyright 2013 Apcera Inc. All rights reserved.

// +build !windows

package term

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

// ErrGetWinsizeFailed indicates that the system call to extract the size of
// a Unix tty from the kernel failed.
var ErrGetWinsizeFailed = errors.New("term: syscall.TIOCGWINSZ failed")

// GetTerminalWindowSize returns the terminal size maintained by the kernel
// for a Unix TTY, passed in as an *os.File.  This information can be seen
// with the stty(1) command, and changes in size (eg, terminal emulator
// resized) should trigger a SIGWINCH signal delivery to the foreground process
// group at the time of the change, so a long-running process might reasonably
// watch for SIGWINCH and arrange to re-fetch the size when that happens.
func GetTerminalWindowSize(file *os.File) (*Size, error) {
	// Based on source from from golang.org/x/crypto/ssh/terminal/util.go
	var dimensions [4]uint16
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, file.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&dimensions)), 0, 0, 0); err != 0 {
		return nil, err
	}

	return &Size{
		Lines:   int(dimensions[0]),
		Columns: int(dimensions[1]),
	}, nil
}
