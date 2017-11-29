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

package cmd
import (
	"github.com/urfave/cli"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/pkg/grctl/clients"
	"time"
	"fmt"
)

func GetCommand(status bool)[]cli.Command  {
	c:=[]cli.Command{
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
				return Task(c,"check_compute_services",status)
			},
			Subcommands:[]cli.Command{
				{
					Name:  "storage_client",
					Usage: "step 1 storage_client",
					Action: func(c *cli.Context) error {
						return Task(c,"install_storage_client",status)
					},
				},
				{
					Name:  "kubelet",
					Usage: "need storage_client",
					Action: func(c *cli.Context) error {
						return Task(c,"install_kubelet",status)
					},
				},
				{
					Name:  "network_compute",
					Usage: "need storage_client,kubelet",
					Action: func(c *cli.Context) error {
						return Task(c,"install_network_compute",status)
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
				return Task(c,"check_manage_base_services",status)
			},
			Subcommands:[]cli.Command{
				{
					Name:  "docker",
					Usage: "step 1 安装docker",
					Action: func(c *cli.Context) error {
						return Task(c,"install_docker",status)
					},
				},
				{
					Name:  "db",
					Usage: "step 2 安装db",
					Action: func(c *cli.Context) error {
						return Task(c,"install_db",status)
					},

				},
				{
					Name:  "base_plugins",
					Usage: "step 3 基础插件",
					Action: func(c *cli.Context) error {
						return Task(c,"install_base_plugins",status)
					},

				},
				{
					Name:  "acp_plugins",
					Usage: "step 4 acp插件",
					Action: func(c *cli.Context) error {
						return Task(c,"install_acp_plugins",status)
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
			Subcommands:[]cli.Command{
				{
					Name:  "storage",
					Usage: "step 1 安装存储",
					Action: func(c *cli.Context) error {
						return Task(c,"install_storage",status)
					},

				},
				{
					Name:  "k8s",
					Usage: "need storage",
					Action: func(c *cli.Context) error {
						return Task(c,"install_k8s",status)
					},

				},
				{
					Name:  "network",
					Usage: "need storage,k8s",
					Action: func(c *cli.Context) error {
						return Task(c,"install_network",status)
					},

				},
				{
					Name:  "plugins",
					Usage: "need storage,k8s,network",
					Action: func(c *cli.Context) error {
						return Task(c,"install_plugins",status)
					},

				},
			},
			Action:func(c *cli.Context) error {
				return Task(c,"check_manage_services",status)
			},
		},
	}
	return c
}
func NewCmdInstall() cli.Command {
	c:=cli.Command{
		Name:  "install",
		Usage: "安装命令相关子命令。grctl install  -h",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  "nodes",
				Usage: "hostID1 hostID2 ...,空表示全部",
			},
		},
		Subcommands:GetCommand(false),
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

func Status(task string) {
	taskE:=clients.NodeClient.Tasks().Get(task)

	checkFail:=0
	for checkFail<3  {
		time.Sleep(3*time.Second)
		status,err:=taskE.Status()
		if err != nil {
			logrus.Warnf("error get task status,retry")
			checkFail+=1
			logrus.Errorf("error get task status ,details %s",err.Error())
			continue
		}
		checkFail=0
		for _,v:=range status.Status{
			if v.Status!="complete" {
				fmt.Printf("task %s is %s\n",task,v.Status)
				//fmt.Printf("task is %s",v.Status)
				continue
			}else {
				fmt.Printf("task %s is %s %s\n",task,v.Status,v.CompleStatus)
				taskFinished:=clients.NodeClient.Tasks().Get(task)

				var  nextTasks []string
				for _,v:=range taskFinished.Task.OutPut{
					for _,sv:=range v.Status{
						if sv.NextTask == nil ||len(sv.NextTask)==0{
							continue
						}else{
							for _,v:=range sv.NextTask{
								fmt.Println("next:"+v)
								nextTasks=append(nextTasks,v)
							}
						}

					}
				}
				if len(nextTasks) > 0 {
					fmt.Printf("接下来要安装 %v \n",nextTasks)
					for _,v:=range nextTasks{
						Status(v)
					}
				}
				return
			}
		}
	}
}

func Task(c *cli.Context,task string,status bool) error   {

	nodes:=c.StringSlice("nodes")
	taskEntity:=clients.NodeClient.Tasks().Get(task)
	err:=taskEntity.Exec(nodes)
	if err != nil {
		logrus.Errorf("error exec task:%s,details %s",task,err.Error())
		return err
	}
	Status(task)

	return nil
}
