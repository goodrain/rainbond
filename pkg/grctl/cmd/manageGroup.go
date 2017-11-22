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
	"encoding/json"
	"github.com/goodrain/rainbond/pkg/grctl/clients"
)
func NewCmdCheckManageBaseServices() cli.Command {
	c:=cli.Command{
		Name:  "install_manage_base",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  "nodes",
				Usage: "10.0.0.2 10.0.0.3",
			},
			cli.StringFlag{
				Name:  "status",
				Value:"",
				Usage: "查看任务状态",
			},
		},
		Usage: "安装管理节点基础服务。grctl install_manage_base -h",
		Action: func(c *cli.Context) error {
			return Task(c,"check_manage_base_services")
			//Common(c)
			//return checkBaseManage(c)
		},
	}
	return c
}
func NewCmdCheckManageServices() cli.Command {
	c:=cli.Command{
		Name:  "install_manage",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  "nodes",
				Usage: "10.0.0.2 10.0.0.3",
			},
			cli.StringFlag{
				Name:  "status",
				Value:"",
				Usage: "查看任务状态",
			},
		},
		Usage: "安装管理节点。grctl install_manage -h",
		Action: func(c *cli.Context) error {
			return Task(c,"check_manage_services")
			//Common(c)
			//return checkManage(c)
		},
	}
	return c
}

func NewCmdBaseManageGroup() cli.Command {
	c:=cli.Command{
		Name:  "install_manage_base_model",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  "nodes",
				Usage: "10.0.0.2 10.0.0.3,空表示全部",
			},
			cli.StringFlag{
				Name:  "status",
				Value:"",
				Usage: "查看任务状态",
			},
		},
		Subcommands:[]cli.Command{
			{
				Name:  "docker",
				Usage: "step 1 安装docker",
				Action: func(c *cli.Context) error {
					return Task(c,"install_docker")
				},
			},
			{
				Name:  "db",
				Usage: "step 2 安装db",
				Action: func(c *cli.Context) error {
					return Task(c,"install_db")
				},

			},
			{
				Name:  "base_plugins",
				Usage: "step 3 基础插件",
				Action: func(c *cli.Context) error {
					return Task(c,"install_base_plugins")
				},

			},
			{
				Name:  "acp_plugins",
				Usage: "step 4 acp插件",
				Action: func(c *cli.Context) error {
					return Task(c,"install_acp_plugins")
				},

			},
		},
		Usage: "安装管理节点单模块基础服务。grctl install_manage_base_model -h",
	}
	return c
}
func NewCmdManageGroup() cli.Command {
	c:=cli.Command{
		Name:  "install_manage_model",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  "nodes",
				Usage: "10.0.0.2 10.0.0.3,空表示全部",
			},
			cli.StringFlag{
				Name:  "status",
				Value:"",
				Usage: "查看任务状态",
			},
		},
		Subcommands:[]cli.Command{
			{
				Name:  "storage",
				Usage: "step 1 安装存储",
				Action: func(c *cli.Context) error {
					return Task(c,"install_storage")
				},

			},
			{
				Name:  "k8s",
				Usage: "need storage",
				Action: func(c *cli.Context) error {
					return Task(c,"install_k8s")
				},

			},
			{
				Name:  "network",
				Usage: "need storage,k8s",
				Action: func(c *cli.Context) error {
					return Task(c,"install_network")
				},

			},
			{
				Name:  "plugins",
				Usage: "need storage,k8s,network",
				Action: func(c *cli.Context) error {
					return Task(c,"install_plugins")
				},

			},
		},
		Usage: "安装管理节点单模块。grctl install_manage_model -h",
	}
	return c
}

func Task(c *cli.Context,task string) error   {
	if c.Bool("status"){

		status,err:=clients.NodeClient.Tasks().Get(task).Status()
		if err != nil {
			return err
		}
		a:=status.Status
		b,_:=json.Marshal(a)
		logrus.Infof(string(b))
		return nil
	}

	nodes:=c.StringSlice("nodes")
	err:=clients.NodeClient.Tasks().Get(task).Exec(nodes)
	if err != nil {
		logrus.Errorf("error exec task:%s",task)
		return err
	}
	return nil
}
