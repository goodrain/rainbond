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

package util

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
)

type NodeInstallOption struct {
	HostRole   string
	HostName   string
	InternalIP string
	LinkModel  string
	RootPass   string
	KeyPath    string
	NodeID     string
	Stdin      *os.File
}

func RunNodeInstallCmd(option NodeInstallOption, logChan chan string) (err error) {
	installNodeShellPath := os.Getenv("INSTALL_NODE_SHELL_PATH")
	if installNodeShellPath == "" {
		installNodeShellPath = "/opt/rainbond/rainbond-ansible/scripts/node.sh"
	}

	// ansible file must exists
	if ok, _ := FileExists(installNodeShellPath); !ok {
		err = fmt.Errorf("install node scripts is not found")
		logrus.Error(err)
		return err
	}

	// ansible's param can't send nil nor empty string
	if err = preCheckNodeInstall(option); err != nil {
		return
	}

	line := fmt.Sprintf(installNodeShellPath+" %s %s %s %s %s %s %s",
		option.HostRole, option.HostName, option.InternalIP, option.LinkModel, option.RootPass, option.KeyPath, option.NodeID)

	cmd := exec.Command("bash", "-c", line)
	cmd.Stdin = option.Stdin

	stderr, err := cmd.StderrPipe()
	if err != nil {
		logrus.Errorf("install node failed")
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logrus.Errorf("install node failed")
		return err
	}

	wg := sync.WaitGroup{}

	// for another log
	reader := bufio.NewReader(stdout)
	wg.Add(1)
	go func() {
		for {
			line, err := reader.ReadString('\n')
			if err == io.EOF {
				break
			}
			if err != nil {
				logrus.Errorf("install node failed")
				return
			}

			logChan <- line
		}
		wg.Done()
	}()

	readerStderr := bufio.NewReader(stderr)
	wg.Add(1)
	go func() {
		for {
			line, err := readerStderr.ReadString('\n')
			if err == io.EOF {
				break
			}
			if err != nil {
				logrus.Error("install node failed")
				return
			}
			logChan <- line
		}
		wg.Done()
	}()

	err = cmd.Start()
	if err != nil {
		logrus.Errorf("install node failed")
		return err
	}

	err = cmd.Wait()
	if err != nil {
		logrus.Errorf("install node finished with error : %v", err.Error())
	}
	// wait clse logChan
	wg.Wait()
	close(logChan)
	return
}

func preCheckNodeInstall(option NodeInstallOption) (err error) {
	// TODO check param
	if strings.TrimSpace(option.HostRole) == "" {
		err = fmt.Errorf("install node failed, install scripts needs param hostRole")
		logrus.Error(err)
		return
	}
	if strings.TrimSpace(option.HostName) == "" {
		err = fmt.Errorf("install node failed, install scripts needs param hostName")
		logrus.Error(err)
		return
	}
	if strings.TrimSpace(option.InternalIP) == "" {
		err = fmt.Errorf("install node failed, install scripts needs param internalIP")
		logrus.Error(err)
		return
	}
	if strings.TrimSpace(option.LinkModel) == "" {
		err = fmt.Errorf("install node failed, install scripts needs param linkModel")
		logrus.Error(err)
		return
	}
	if strings.TrimSpace(option.RootPass) == "" {
		err = fmt.Errorf("install node failed, install scripts needs param rootPass")
		logrus.Error(err)
		return
	}
	// if rootPass is not empty then keyPath can be empty
	// if strings.TrimSpace(option.KeyPath) == "" {
	// 	err = fmt.Errorf("install node failed, install scripts needs param keyPath")
	// 	logrus.Error(err)
	// 	return
	// }
	if strings.TrimSpace(option.NodeID) == "" {
		err = fmt.Errorf("install node failed, install scripts needs param nodeID")
		logrus.Error(err)
		return
	}
	return
}
