// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

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

package exector

/*
Copyright 2017 The Goodrain Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import (
	"github.com/goodrain/rainbond/util"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

//Worker 工作器
type Worker struct {
	cmd  *exec.Cmd
	user string
}

func (w *Worker) run(timeout time.Duration) ([]byte, error) {
	stdout := &bytes.Buffer{}
	isTimeout, err := util.CmdRunWithTimeout(w.cmd, timeout)
	if err != nil {
		return nil, workerErr(err, stdout.Bytes())
	}
	if isTimeout {
		return nil, fmt.Errorf("exec worker timeout")
	}
	return stdout.Bytes(), nil
}

//NewWorker 创建一个worker
func NewWorker(cmdpath, user string, envs []string, in []byte) *Worker {

	stdout := &bytes.Buffer{}
	c := &exec.Cmd{
		Env:    envs,
		Path:   "/usr/bin/python",
		Args:   []string{"python", cmdpath},
		Stdin:  bytes.NewBuffer(in),
		Stdout: stdout,
		Stderr: os.Stderr,
	}
	return &Worker{cmd: c, user: user}
}

//Error Error
type Error struct {
	Code    uint   `json:"code"`
	Msg     string `json:"msg"`
	Details string `json:"details,omitempty"`
}

func workerErr(err error, output []byte) error {
	if _, ok := err.(*exec.ExitError); ok {
		emsg := Error{}
		if perr := json.Unmarshal(output, &emsg); perr != nil {
			return fmt.Errorf("netplugin failed but error parsing its diagnostic message %q: %v", string(output), perr)
		}
		details := ""
		if emsg.Details != "" {
			details = fmt.Sprintf("; %v", emsg.Details)
		}
		return fmt.Errorf("%v%v", emsg.Msg, details)
	}
	return err
}
