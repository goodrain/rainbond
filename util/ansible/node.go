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

package ansible

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/goodrain/rainbond/util"
)

//NodeInstallOption node install option
type NodeInstallOption struct {
	HostRole   string
	HostName   string
	InternalIP string
	RootPass   string // ssh login password
	KeyPath    string // ssh login key path
	NodeID     string
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	loginValue string
	linkModel  string
}

//RunNodeInstallCmd install node
func RunNodeInstallCmd(option NodeInstallOption) (err error) {
	installNodeShellPath := os.Getenv("INSTALL_NODE_SHELL_PATH")
	if installNodeShellPath == "" {
		installNodeShellPath = "/opt/rainbond/rainbond-ansible/scripts/node.sh"
	}
	// ansible file must exists
	if ok, _ := util.FileExists(installNodeShellPath); !ok {
		return fmt.Errorf("install node scripts is not found")
	}
	// ansible's param can't send nil nor empty string
	if err := preCheckNodeInstall(&option); err != nil {
		return err
	}
	line := fmt.Sprintf("'%s' -r '%s' -i '%s' -t '%s' -k '%s' -u '%s'",
		installNodeShellPath, option.HostRole, option.InternalIP, option.linkModel, option.loginValue, option.NodeID)
	cmd := exec.Command("bash", "-c", line)
	cmd.Stdin = option.Stdin
	cmd.Stdout = option.Stdout
	cmd.Stderr = option.Stderr
	return cmd.Run()
}

// check param
func preCheckNodeInstall(option *NodeInstallOption) error {
	if strings.TrimSpace(option.HostRole) == "" {
		return fmt.Errorf("install node failed, install scripts needs param hostRole")
	}
	if strings.TrimSpace(option.InternalIP) == "" {
		return fmt.Errorf("install node failed, install scripts needs param internalIP")
	}
	//login key path first, and then rootPass, so keyPath and RootPass can't all be empty
	if strings.TrimSpace(option.KeyPath) == "" {
		if strings.TrimSpace(option.RootPass) == "" {
			return fmt.Errorf("install node failed, install scripts needs login key path or login password")
		}
		option.loginValue = strings.TrimSpace(option.RootPass)
		option.linkModel = "pass"
	} else {
		option.loginValue = strings.TrimSpace(option.KeyPath)
		option.linkModel = "key"
	}
	if strings.TrimSpace(option.NodeID) == "" {
		return fmt.Errorf("install node failed, install scripts needs param nodeID")
	}
	return nil
}
