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
	"io"
	"os"

	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

//Exec exec interface
type Exec interface {
	Run() error
	WaitingStop() bool
}

type execContext struct {
	clientContext *ClientContext
	tty, pty      *os.File
	Stdin         io.Reader
	Stdout        io.Writer
	Stderr        io.Writer
	kubeRequest   *restclient.Request
	config        *restclient.Config
	closed        bool
}

//NewExecContext new exec Context
func NewExecContext(clientContext *ClientContext, tty *os.File, kubeRequest *restclient.Request, config *restclient.Config) Exec {
	return &execContext{
		clientContext: clientContext,
		Stdin:         tty,
		Stdout:        tty,
		Stderr:        tty,
		kubeRequest:   kubeRequest,
		config:        config,
	}
}

//NewExecContextByStd -
func NewExecContextByStd(clientContext *ClientContext, Stdin io.Reader, Stdout, Stderr io.Writer, kubeRequest *restclient.Request, config *restclient.Config) Exec {
	return &execContext{
		clientContext: clientContext,
		Stdin:         Stdin,
		Stdout:        Stdout,
		Stderr:        Stderr,
		kubeRequest:   kubeRequest,
		config:        config,
	}
}
func (e *execContext) WaitingStop() bool {
	if e.closed {
		return false
	}
	return true
}

func (e *execContext) Close() {
	e.tty.Close()
}

func (e *execContext) Run() error {
	defer e.Close()
	defer func() { e.closed = true }()
	exec, err := remotecommand.NewSPDYExecutor(e.config, "POST", e.kubeRequest.URL())
	if err != nil {
		return fmt.Errorf("create executor failure %s", err.Error())
	}
	if err := exec.Stream(remotecommand.StreamOptions{
		Stdin:             e.Stdin,
		Stdout:            e.Stdout,
		Stderr:            e.Stderr,
		Tty:               false,
		TerminalSizeQueue: e.clientContext,
	}); err != nil {
		return fmt.Errorf("executor stream failure %s", err.Error())
	}
	return nil
}
