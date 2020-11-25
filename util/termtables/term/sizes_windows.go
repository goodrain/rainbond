// Copyright 2013 Apcera Inc. All rights reserved.

// +build windows

package term

// Used when we have no other source for getting platform-specific information
// about the terminal sizes available.

import (
	"os"
	"syscall"
	"unsafe"
)

// Based on source from from golang.org/x/crypto/ssh/terminal/util_windows.go
var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procGetConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
)

type (
	short int16
	word  uint16

	coord struct {
		x short
		y short
	}
	smallRect struct {
		left   short
		top    short
		right  short
		bottom short
	}
	consoleScreenBufferInfo struct {
		size              coord
		cursorPosition    coord
		attributes        word
		window            smallRect
		maximumWindowSize coord
	}
)

// GetTerminalWindowSize returns the width and height of a terminal in Windows.
func GetTerminalWindowSize(file *os.File) (*Size, error) {
	var info consoleScreenBufferInfo
	_, _, e := syscall.Syscall(procGetConsoleScreenBufferInfo.Addr(), 2, file.Fd(), uintptr(unsafe.Pointer(&info)), 0)
	if e != 0 {
		return nil, error(e)
	}
	return &Size{
		Lines:   int(info.size.y),
		Columns: int(info.size.x),
	}, nil
}
