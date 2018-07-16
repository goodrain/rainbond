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

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"

	"github.com/goodrain/rainbond/event"
	//"github.com/docker/docker/api/types"
)

//ShowExec ShowExec
func ShowExec(command string, params []string, logger event.Logger) error {
	cmd := exec.Command(command, params...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, _ := cmd.StderrPipe()
	errC := cmd.Start()
	if errC != nil {
		if logger != nil {
			logger.Error(fmt.Sprintf("builder:%v", errC), map[string]string{"step": "build-exector"})
		}
		return errC
	}
	reader := bufio.NewReader(stdout)
	go func() {
		for {
			line, errL := reader.ReadString('\n')
			if errL != nil || io.EOF == errL {
				break
			}
			if logger != nil {
				logger.Info(fmt.Sprintf("builder:%v", line), map[string]string{"step": "build-exector"})
			}
		}
	}()
	errW := cmd.Wait()
	if errW != nil {
		go func() {
			readerr := bufio.NewReader(stderr)
			for {
				line, errL := readerr.ReadString('\n')
				if errL != nil || io.EOF == errL {
					break
				}
				if logger != nil {
					logger.Error(fmt.Sprintf("builder err:%v", line), map[string]string{"step": "build-exector"})
				}
			}
		}()
		return errW
	}
	return nil
}
