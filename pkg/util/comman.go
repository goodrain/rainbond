
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
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/Sirupsen/logrus"
)

//CheckAndCreateDir check and create dir
func CheckAndCreateDir(path string) error {
	if subPathExists, err := FileExists(path); err != nil {
		return fmt.Errorf("Could not determine if subPath %s exists; will not attempt to change its permissions", path)
	} else if !subPathExists {
		// Create the sub path now because if it's auto-created later when referenced, it may have an
		// incorrect ownership and mode. For example, the sub path directory must have at least g+rwx
		// when the pod specifies an fsGroup, and if the directory is not created here, Docker will
		// later auto-create it with the incorrect mode 0750
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to mkdir:%s", path)
		}

		if err := os.Chmod(path, 0755); err != nil {
			return err
		}
	}
	return nil
}

//FileExists check file exist
func FileExists(filename string) (bool, error) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

//CmdRunWithTimeout exec cmd with timeout
func CmdRunWithTimeout(cmd *exec.Cmd, timeout time.Duration) (bool, error) {
	done := make(chan error)
	if cmd.Process != nil { //还原执行状态
		cmd.Process = nil
		cmd.ProcessState = nil
	}
	if err := cmd.Start(); err != nil {
		return false, err
	}
	go func() {
		done <- cmd.Wait()
	}()
	var err error
	select {
	case <-time.After(timeout):
		// timeout
		if err = cmd.Process.Kill(); err != nil {
			logrus.Errorf("failed to kill: %s, error: %s", cmd.Path, err.Error())
		}
		go func() {
			<-done // allow goroutine to exit
		}()
		logrus.Info("process:%s killed", cmd.Path)
		return true, err
	case err = <-done:
		return false, err
	}
}
