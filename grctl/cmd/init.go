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

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
	//"github.com/goodrain/rainbond/grctl/clients"

	"os"

	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/grctl/clients"
	"net/http"
	"net"
	"time"
)

//NewCmdInit grctl init
func NewCmdInit() cli.Command {
	c := cli.Command{
		Name: "init",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "etcd",
				Usage: "etcd ip,127.0.0.1",
			},
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
				Name:  "mip",
				Usage: "当前节点内网IP, 10.0.0.1",
			},
			cli.StringFlag{
				Name:  "repo_ver",
				Usage: "repo version,3.4",
				Value: "master",
			},
			cli.StringFlag{
				Name:  "install_type",
				Usage: "online or offline",
				Value: "online",
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
	_, err := os.Stat("/opt/rainbond/rainbond.success")
	if err == nil {
		println("Rainbond is already installed, if you whant reinstall, then please delete the file: /tmp/rainbond.success")
		return
	}

	// download source code from github if in online model
	if c.String("install_type") == "online" {
		fmt.Println("Download rainbond install package.")
		csi := sources.CodeSourceInfo{
			RepositoryURL: "https://github.com/goodrain/rainbond-install.git",
			Branch:        c.String("repo_ver"),
		}
		os.RemoveAll(c.String("work_dir"))
		os.MkdirAll(c.String("work_dir"), 0755)
		_, err := sources.GitClone(csi, c.String("work_dir"), nil, 5)
		if err != nil {
			println(err.Error())
			return
		}
	}

	// start setup script to install rainbond
	fmt.Println("Begin init cluster first node,please don't exit,wait install")
	cmd := exec.Command("bash", "-c", fmt.Sprintf("cd %s ; ./setup.sh %s %s", c.String("work_dir"), c.String("install_type"), c.String("repo_ver")))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		println(err.Error())
		return
	}

	fmt.Println("Waiting WEB UI started.")
	index := 1
	for {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:7070", time.Second)
		println("waiting web ui is started: ", err.Error())
		if err == nil {
			conn.Close()
			break
		}
		index++
		if index > 30 {
			println("Install complete but WEB UI is can not access, please manual check node status by `grctl node list`")
			return
		}
	}

	_, err = http.Get("http://127.0.0.1:7070")
	if err != nil {
		println("Install complete but WEB UI is can not access, please manual check node status by `grctl node list`")
		return
	}

	ioutil.WriteFile("/opt/rainbond/rainbond.success", []byte(c.String("repo_ver")), 0644)

	fmt.Println("Init manage node successful, next you can:")
	fmt.Println("	access WEB UI: http://127.0.0.1:7070")
	fmt.Println("	add compute node: grctl node add -h")
	fmt.Println("	online compute node: grctl node up -h")

	return
}
