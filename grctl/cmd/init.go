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
	"io/ioutil"
	"os/exec"

	"github.com/goodrain/rainbond/event"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"

	//"github.com/goodrain/rainbond/grctl/clients"

	"os"

	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/grctl/clients"
)

//NewCmdInit grctl init
func NewCmdInit() cli.Command {
	c := cli.Command{
		Name: "init",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "type",
				Usage: "node type: manage or compute",
				Value: "manage",
			},
			cli.StringFlag{
				Name:  "work_dir",
				Usage: "clone source code to the work directory",
				Value: "/opt/rainbond/install",
			},
			cli.StringFlag{
				Name:  "iip",
				Usage: "manage01 local ip",
				Value: "",
			},
			cli.StringFlag{
				Name:  "eip",
				Usage: "manage01 public ip",
				Value: "0.0.0.0",
			},
			cli.StringFlag{
				Name:  "rainbond-version",
				Usage: "Choose a specific Rainbond version for the control plane. (default v3.7)",
				Value: "v3.7",
			},
			cli.StringFlag{
				Name:  "rainbond-install-repostoiry",
				Usage: "Set install rainbond code git repostory address",
				Value: "https://github.com/goodrain/rainbond-install.git",
			},
			cli.StringFlag{
				Name:  "install-type",
				Usage: "defalut online.",
				Value: "online",
			},
			cli.StringFlag{
				Name:  "domain",
				Usage: "defalut custom apps domain.",
				Value: "",
			},
			cli.BoolFlag{
				Name:   "test",
				Usage:  "use test shell",
				Hidden: true,
			},
		},
		Usage: "初始化集群。grctl init cluster",
		Action: func(c *cli.Context) error {
			initCluster(c)
			return nil
		},
	}
	return c
}

//NewCmdInstallStatus install status
func NewCmdInstallStatus() cli.Command {
	c := cli.Command{
		Name: "install_status",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "taskID",
				Usage: "install_k8s,空则自动寻找",
			},
		},
		Usage: "获取task执行状态。grctl install_status",
		Action: func(c *cli.Context) error {
			taskID := c.String("taskID")
			if taskID == "" {
				tasks, err := clients.RegionClient.Tasks().List()
				if err != nil {
					logrus.Errorf("error get task list,details %s", err.Error())
					return nil
				}
				for _, v := range tasks {
					for _, vs := range v.Status {
						if vs.Status == "start" || vs.Status == "create" {
							//Status(v.ID)
							return nil
						}

					}
				}
			} else {
				//Status(taskID)
			}
			return nil
		},
	}
	return c
}

func initCluster(c *cli.Context) {
	// check if the rainbond is already installed
	fmt.Println("Checking install enviremant.")
	_, err := os.Stat("/opt/rainbond/.rainbond.success")
	if err == nil {
		println("Rainbond is already installed, if you whant reinstall, then please delete the file: /opt/rainbond/.rainbond.success")
		return
	}

	// download source code from github if in online model
	if c.String("install-type") == "online" {
		fmt.Println("Download rainbond install package.")
		csi := sources.CodeSourceInfo{
			RepositoryURL: c.String("rainbond-install-repostoiry"),
			Branch:        c.String("rainbond-version"),
		}
		os.RemoveAll(c.String("work_dir"))
		os.MkdirAll(c.String("work_dir"), 0755)
		_, err := sources.GitClone(csi, c.String("work_dir"), event.GetTestLogger(), 5)
		if err != nil {
			println(err.Error())
			return
		}
	}

	// start setup script to install rainbond
	fmt.Println("Begin init cluster first node,please don't exit,wait install")
	cmd := exec.Command("bash", "-c", fmt.Sprintf("cd %s ; ./setup.sh %s %s %s", c.String("work_dir"), c.String("install-type"), c.String("eip"), c.String("domain")))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err = cmd.Run()
	if err != nil {
		println(err.Error())
		return
	}

	ioutil.WriteFile("/opt/rainbond/.rainbond.success", []byte(c.String("rainbond-version")), 0644)

	return
}
