// RAINBOND, Application Management Platform
// Copyright (C) 2014-2020 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package app

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/barnettZQG/gotty/server"
	"github.com/kr/pty"
	"github.com/sirupsen/logrus"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

type execContext struct {
	tty, pty    *os.File
	kubeRequest *restclient.Request
	config      *restclient.Config
	sizeUpdate  chan remotecommand.TerminalSize
	closed      bool
}

// NewExecContext new exec Context
func NewExecContext(kubeRequest *restclient.Request, config *restclient.Config) (server.Slave, error) {
	pty, tty, err := pty.Open()
	if err != nil {
		logrus.Errorf("open pty failure %s", err.Error())
		return nil, err
	}
	ec := &execContext{
		tty:         tty,
		pty:         pty,
		kubeRequest: kubeRequest,
		config:      config,
		sizeUpdate:  make(chan remotecommand.TerminalSize, 2),
	}
	if err := ec.Run(); err != nil {
		return nil, err
	}
	return ec, nil
}

func (e *execContext) WaitingStop() bool {
	if e.closed {
		return false
	}
	return true
}

func (e *execContext) Run() error {
	exec, err := remotecommand.NewSPDYExecutor(e.config, "POST", e.kubeRequest.URL())
	if err != nil {
		return fmt.Errorf("create executor failure %s", err.Error())
	}
	go func() {
		out := CreateOut(e.tty)
		t := out.SetTTY()
		t.Safe(func() error {
			defer e.Close()
			if err := exec.Stream(remotecommand.StreamOptions{
				Stdin:             out.Stdin,
				Stdout:            out.Stdout,
				Stderr:            nil,
				Tty:               true,
				TerminalSizeQueue: e,
			}); err != nil {
				logrus.Errorf("executor stream failure %s", err.Error())
				return err
			}
			return nil
		})

	}()
	return nil
}

func (e *execContext) Read(p []byte) (n int, err error) {
	return e.pty.Read(p)
}

func (e *execContext) Write(p []byte) (n int, err error) {
	return e.pty.Write(p)
}

func (e *execContext) Close() error {
	return e.tty.Close()
}

func (e *execContext) WindowTitleVariables() map[string]interface{} {
	return map[string]interface{}{}
}

func (e *execContext) Next() *remotecommand.TerminalSize {
	size, ok := <-e.sizeUpdate
	if !ok {
		return nil
	}
	logrus.Infof("width %d height %d", size.Width, size.Height)
	return &size
}

func (e *execContext) ResizeTerminal(width int, height int) error {
	logrus.Infof("set width %d height %d", width, height)
	e.sizeUpdate <- remotecommand.TerminalSize{
		Width:  uint16(width),
		Height: uint16(height),
	}
	window := struct {
		row uint16
		col uint16
		x   uint16
		y   uint16
	}{
		uint16(height),
		uint16(width),
		0,
		0,
	}
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		e.pty.Fd(),
		syscall.TIOCSWINSZ,
		uintptr(unsafe.Pointer(&window)),
	)
	if errno != 0 {
		return errno
	}
	return nil
}
