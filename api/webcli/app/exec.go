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
	"sync"
	"syscall"
	"time"
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
	mu          sync.Mutex
	streamReady chan error // 传递 Stream 启动状态(nil表示成功)
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
		streamReady: make(chan error, 1),
	}
	if err := ec.Run(); err != nil {
		tty.Close()
		pty.Close()
		return nil, err
	}

	// 等待 Stream 准备就绪或超时
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	select {
	case err := <-ec.streamReady:
		if err != nil {
			tty.Close()
			pty.Close()
			return nil, fmt.Errorf("stream failed to start: %w", err)
		}
		return ec, nil
	case <-timeout.C:
		// 超时仍未准备好，记录警告但继续
		logrus.Warnf("stream did not signal ready within timeout, proceeding anyway")
		return ec, nil
	}
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

		// 使用一个信号来标记 Stream 是否已启动
		streamStarted := make(chan struct{})

		go func() {
			// 给一个小延迟确保 Stream 开始执行
			time.Sleep(50 * time.Millisecond)
			select {
			case e.streamReady <- nil: // 通知已准备好
			default:
			}
			close(streamStarted)
		}()

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
				// 如果 Stream 快速失败,尝试发送错误
				select {
				case e.streamReady <- err:
				case <-streamStarted:
					// 已经发送了准备信号,记录错误即可
				}
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
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil
	}
	e.closed = true

	// 关闭所有资源
	var ttyErr, ptyErr error
	if e.tty != nil {
		ttyErr = e.tty.Close()
	}
	if e.pty != nil {
		ptyErr = e.pty.Close()
	}

	if ttyErr != nil {
		return ttyErr
	}
	return ptyErr
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
