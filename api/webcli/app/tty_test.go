// RAINBOND, Application Management Platform
// Copyright (C) 2014-2020 Goodrain Co., Ltd.

package app

import (
	"testing"

	"github.com/kr/pty"
	"k8s.io/client-go/tools/remotecommand"
)

// capability_id: rainbond.webcli.terminal-resize
func TestResizeTerminalQueuesWindowSize(t *testing.T) {
	ptyFile, ttyFile, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer ptyFile.Close()
	defer ttyFile.Close()

	ec := &execContext{
		pty:        ptyFile,
		tty:        ttyFile,
		sizeUpdate: make(chan remotecommand.TerminalSize, 1),
	}

	if err := ec.ResizeTerminal(120, 40); err != nil {
		t.Fatal(err)
	}

	size := ec.Next()
	if size == nil {
		t.Fatal("expected terminal size update")
	}
	if size.Width != 120 || size.Height != 40 {
		t.Fatalf("unexpected size: %+v", size)
	}
}
