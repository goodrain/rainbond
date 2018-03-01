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

package exector

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"

	"github.com/goodrain/rainbond/pkg/event"

	"github.com/Sirupsen/logrus"
	//"github.com/docker/docker/api/types"
	"github.com/docker/engine-api/types"
)

func (e *exectorManager) DockerPull(image string) error {
	_, err := e.DockerClient.ImagePull(context.Background(), image, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (e *exectorManager) DockerPush(image string) error {
	_, err := e.DockerClient.ImagePush(context.Background(), image, types.ImagePushOptions{})
	if err != nil {
		return err
	}
	return nil
}

//ShowExec ShowExec
func ShowExec(command string, params []string, logger ...event.Logger) error {
	cmd := exec.Command(command, params...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, _ := cmd.StderrPipe()
	errC := cmd.Start()
	if errC != nil {
		logrus.Debugf(fmt.Sprintf("builder: %v", errC))
		logger[0].Error(fmt.Sprintf("builder:%v", errC), map[string]string{"step": "build-exector"})
		return errC
	}
	reader := bufio.NewReader(stdout)
	go func() {
		for {
			line, errL := reader.ReadString('\n')
			if errL != nil || io.EOF == errL {
				break
			}
			//fmt.Print(line)
			logrus.Debugf(fmt.Sprintf("builder: %v", line))
			logger[0].Debug(fmt.Sprintf("builder:%v", line), map[string]string{"step": "build-exector"})
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
				logrus.Errorf(fmt.Sprintf("builder err: %v", line))
				logger[0].Error(fmt.Sprintf("builder err:%v", line), map[string]string{"step": "build-exector"})
			}
		}()
		return errW
	}
	return nil
}
