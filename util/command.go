// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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

package util

import (
	"bytes"
	"io"
	"os/exec"
)

//PipeCommand PipeCommand
type PipeCommand struct {
	stack                    []*exec.Cmd
	finalStdout, finalStderr io.Reader
	pipestack                []*io.PipeWriter
}

//NewPipeCommand new pipe commands
func NewPipeCommand(stack ...*exec.Cmd) (*PipeCommand, error) {
	var errorbuffer bytes.Buffer
	pipestack := make([]*io.PipeWriter, len(stack)-1)
	i := 0
	for ; i < len(stack)-1; i++ {
		stdinpipe, stdoutpipe := io.Pipe()
		stack[i].Stdout = stdoutpipe
		stack[i].Stderr = &errorbuffer
		stack[i+1].Stdin = stdinpipe
		pipestack[i] = stdoutpipe
	}
	finalStdout, err := stack[i].StdoutPipe()
	if err != nil {
		return nil, err
	}
	finalStderr, err := stack[i].StderrPipe()
	if err != nil {
		return nil, err
	}
	pipeCommand := &PipeCommand{
		stack:       stack,
		pipestack:   pipestack,
		finalStdout: finalStdout,
		finalStderr: finalStderr,
	}
	return pipeCommand, nil
}

//Run Run
func (p *PipeCommand) Run() error {
	return call(p.stack, p.pipestack)
}

//GetFinalStdout get final command stdout reader
func (p *PipeCommand) GetFinalStdout() io.Reader {
	return p.finalStdout
}

//GetFinalStderr get final command stderr reader
func (p *PipeCommand) GetFinalStderr() io.Reader {
	return p.finalStderr
}

func call(stack []*exec.Cmd, pipes []*io.PipeWriter) (err error) {
	if stack[0].Process == nil {
		if err = stack[0].Start(); err != nil {
			return err
		}
	}
	if len(stack) > 1 {
		if err = stack[1].Start(); err != nil {
			return err
		}
		defer func() {
			if err == nil {
				pipes[0].Close()
				err = call(stack[1:], pipes[1:])
			}
		}()
	}
	return stack[0].Wait()
}
