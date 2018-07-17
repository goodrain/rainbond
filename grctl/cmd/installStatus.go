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
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/urfave/cli"
)

func GetCommand(status bool) []cli.Command {
	c := []cli.Command{
		{
			Name:  "compute",
			Usage: "安装计算节点 compute -h",
			Flags: []cli.Flag{
				cli.StringSliceFlag{
					Name:  "nodes",
					Usage: "hostID1 hostID2 ...,空表示全部",
				},
			},
			Action: func(c *cli.Context) error {
				return Task(c, "check_compute_services", status)
			},
			Subcommands: []cli.Command{
				{
					Name:  "storage_client",
					Usage: "step 1 storage_client",
					Action: func(c *cli.Context) error {
						return Task(c, "install_storage_client", status)
					},
				},
				{
					Name:  "kubelet",
					Usage: "need storage_client",
					Action: func(c *cli.Context) error {
						return Task(c, "install_kubelet", status)
					},
				},
				{
					Name:  "network_compute",
					Usage: "need storage_client,kubelet",
					Action: func(c *cli.Context) error {
						return Task(c, "install_network_compute", status)
					},
				},
			},
		},
		{
			Name:  "manage_base",
			Usage: "安装管理节点基础服务。 manage_base -h",
			Flags: []cli.Flag{
				cli.StringSliceFlag{
					Name:  "nodes",
					Usage: "hostID1 hostID2 ...,空表示全部",
				},
			},
			Action: func(c *cli.Context) error {
				return Task(c, "check_manage_base_services", status)
			},
			Subcommands: []cli.Command{
				{
					Name:  "docker",
					Usage: "step 1 安装docker",
					Action: func(c *cli.Context) error {
						return Task(c, "install_docker", status)
					},
				},
				{
					Name:  "db",
					Usage: "step 2 安装db",
					Action: func(c *cli.Context) error {
						return Task(c, "install_db", status)
					},
				},
				{
					Name:  "base_plugins",
					Usage: "step 3 基础插件",
					Action: func(c *cli.Context) error {
						return Task(c, "install_base_plugins", status)
					},
				},
				{
					Name:  "acp_plugins",
					Usage: "step 4 acp插件",
					Action: func(c *cli.Context) error {
						return Task(c, "install_acp_plugins", status)
					},
				},
			},
		},
		{
			Name:  "manage",
			Usage: "安装管理节点。 manage -h",
			Flags: []cli.Flag{
				cli.StringSliceFlag{
					Name:  "nodes",
					Usage: "hostID1 hostID2 ...,空表示全部",
				},
			},
			Subcommands: []cli.Command{
				{
					Name:  "storage",
					Usage: "step 1 安装存储",
					Action: func(c *cli.Context) error {
						return Task(c, "install_storage", status)
					},
				},
				{
					Name:  "k8s",
					Usage: "need storage",
					Action: func(c *cli.Context) error {
						return Task(c, "install_k8s", status)
					},
				},
				{
					Name:  "network",
					Usage: "need storage,k8s",
					Action: func(c *cli.Context) error {
						return Task(c, "install_network", status)
					},
				},
				{
					Name:  "plugins",
					Usage: "need storage,k8s,network",
					Action: func(c *cli.Context) error {
						return Task(c, "install_plugins", status)
					},
				},
			},
			Action: func(c *cli.Context) error {
				return Task(c, "check_manage_services", status)
			},
		},
	}
	return c
}

func NewCmdInstall() cli.Command {
	c := cli.Command{
		Name:  "install",
		Usage: "安装命令相关子命令。grctl install  -h",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  "nodes",
				Usage: "hostID1 hostID2 ...,空表示全部",
			},
		},
		Subcommands: GetCommand(false),
	}
	return c
}

//func NewCmdStatus() cli.Command {
//	c:=cli.Command{
//		Name:  "status",
//		Usage: "状态命令相关子命令。grctl status  -h",
//		Flags: []cli.Flag{
//			cli.StringSliceFlag{
//				Name:  "nodes",
//				Usage: "hostID1 hostID2 ...,空表示全部",
//			},
//		},
//		Subcommands:GetCommand(true),
//	}
//	return c
//}

func Status(task string, nodes []string) {
	checkFail := 0
	lastState := ""
	set := make(map[string]bool)
	for _, v := range nodes {
		set[v] = true
	}
	fmt.Printf("%s task is start\n", task)
	lastState = "Start"
	for checkFail < 3 {
		time.Sleep(3 * time.Second)
		taskE, err := clients.RegionClient.Tasks().Get(task)
		if err != nil {

			logrus.Warnf("error get task %s ,details %s,retry", task, err.String())
			checkFail += 1
			continue
		}
		//status,error:=clients.NodeClient.Tasks().Status(task)
		status, err := clients.RegionClient.Tasks().GetTaskStatus(task)
		if err != nil || status == nil {
			logrus.Warnf("error get task %s status,details %s,retry", task, err.String())
			checkFail += 1
			continue
		}
		for k, v := range status {
			//不是当前任务需要检测的status
			if !set[k] {
				fmt.Print("..")
				continue
			}
			if strings.Contains(v.Status, "error") || strings.Contains(v.CompleStatus, "Failure") || strings.Contains(v.CompleStatus, "Unknow") {
				checkFail += 1
				fmt.Errorf("error executing task %s", task)
				for _, v := range taskE.OutPut {
					if set[v.NodeID] {
						fmt.Printf("on %s :\n %s", v.NodeID, v.Body)
					}
				}
				os.Exit(1)
			}
			if lastState != v.Status {
				fmt.Printf("task %s is %s\n", task, v.Status)
			} else {
				fmt.Print("..")
			}
			lastState = v.Status
			if v.Status == "complete" || v.CompleStatus == "Success" {
				fmt.Printf("task %s is %s %s\n", task, v.Status, v.CompleStatus)
				lastState = v.Status
				taskFinished := taskE
				var nextTasks []string
				for _, v := range taskFinished.OutPut {
					if !set[v.NodeID] {
						continue
					}
					for _, sv := range v.Status {
						if sv.NextTask == nil || len(sv.NextTask) == 0 {
							continue
						} else {
							for _, v := range sv.NextTask {
								nextTasks = append(nextTasks, v)
							}
						}
					}
				}
				if len(nextTasks) > 0 {
					fmt.Printf("next will install %v \n", nextTasks)
					for _, v := range nextTasks {
						Status(v, nodes)
					}
				}
				return
			}

		}
		checkFail = 0
	}
}

func Task(c *cli.Context, task string, status bool) error {
	nodes := c.StringSlice("nodes")
	if len(nodes) == 0 {
		return fmt.Errorf("nodes can not be empty")
	}
	err := clients.RegionClient.Tasks().Exec(task, nodes)
	if err != nil {
		logrus.Errorf("error exec task:%s,details %s", task, err.Error())
		return err
	}
	Status(task, nodes)
	return nil
}
