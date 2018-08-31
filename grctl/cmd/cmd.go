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

package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	conf "github.com/goodrain/rainbond/cmd/grctl/option"
	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/urfave/cli"
)

//GetCmds GetCmds
func GetCmds() []cli.Command {
	cmds := []cli.Command{}
	cmds = append(cmds, NewCmdService())

	cmds = append(cmds, NewCmdTenant())
	cmds = append(cmds, NewCmdNode())
	cmds = append(cmds, NewCmdCluster())
	cmds = append(cmds, NewCmdExec())
	cmds = append(cmds, NewCmdInit())
	cmds = append(cmds, NewCmdShow())
	cmds = append(cmds, NewCmdAlerting())
	cmds = append(cmds, NewCmdNotificationEvent())

	//task相关命令
	//cmds = append(cmds, NewCmdTasks())
	//数据中心配置相关命令
	cmds = append(cmds, NewCmdConfigs())

	//cmds = append(cmds, NewCmdComputeGroup())
	//cmds = append(cmds, NewCmdInstall())
	//cmds = append(cmds, NewCmdInstallStatus())

	cmds = append(cmds, NewCmdDomain())
	// source build test
	cmds = append(cmds, NewSourceBuildCmd())

	//cmds = append(cmds, NewCmdBaseManageGroup())
	//cmds = append(cmds, NewCmdManageGroup())

	//cmds = append(cmds, NewCmdSources())
	//cmds = append(cmds, NewCmdCloudAuth())
	//cmds = append(cmds, NewCmdRegionNode())
	//cmds = append(cmds, NewCmdTest())
	//cmds = append(cmds, NewCmdPlugin())
	//todo
	return cmds
}

//Common Common
func Common(c *cli.Context) {
	config, err := conf.LoadConfig(c)
	if err != nil {
		logrus.Warn("Load config file error.", err.Error())
	}
	if err := clients.InitClient(c.GlobalString("kubeconfig")); err != nil {
		logrus.Errorf("error config k8s,details %s", err.Error())
	}
	//clients.SetInfo(config.RegionAPI.URL, config.RegionAPI.Token)
	if err := clients.InitRegionClient(config.RegionAPI); err != nil {
		logrus.Fatal("error config region")
	}
}

// fatal prints the message (if provided) and then exits. If V(2) or greater,
// glog.Fatal is invoked for extended information.
func fatal(msg string, code int) {
	if len(msg) > 0 {
		// add newline if needed
		if !strings.HasSuffix(msg, "\n") {
			msg += "\n"
		}
		fmt.Fprint(os.Stderr, msg)
	}
	os.Exit(code)
}
