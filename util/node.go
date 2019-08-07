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
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/Sirupsen/logrus"
)

type NodeInstallOption struct {
	HostRole   string
	HostName   string
	InternalIP string
	LinkModel  string
	RootPass   string // ssh login password
	KeyPath    string // ssh login key path
	NodeID     string
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	loginValue string
}

func RunNodeInstallCmd(option NodeInstallOption) (err error) {
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

	line := fmt.Sprintf(installNodeShellPath+" %s %s %s %s %s %s",
		option.HostRole, option.HostName, option.InternalIP, option.LinkModel, option.loginValue, option.NodeID)

	cmd := exec.Command("bash", "-c", line)
	cmd.Stdin = option.Stdin
	cmd.Stdout = option.Stdout
	cmd.Stderr = option.Stderr

	err = cmd.Start()
	if err != nil {
		logrus.Errorf("install node failed")
		return err
	}

	err = cmd.Wait()
	if err != nil {
		logrus.Errorf("install node finished with error : %v", err.Error())
	}

	return
}

// check param
func preCheckNodeInstall(option NodeInstallOption) (err error) {
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

	//login key path first, and then rootPass, so keyPath and RootPass can't all be empty
	if strings.TrimSpace(option.KeyPath) == "" {
		if strings.TrimSpace(option.RootPass) == "" {
			err = fmt.Errorf("install node failed, install scripts needs login key path or login password")
			logrus.Error(err)
			return
		}
		option.loginValue = strings.TrimSpace(option.RootPass)
	}
	option.loginValue = strings.TrimSpace(option.KeyPath)

	if strings.TrimSpace(option.NodeID) == "" {
		err = fmt.Errorf("install node failed, install scripts needs param nodeID")
		logrus.Error(err)
		return
	}
	return
}
