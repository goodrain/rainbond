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
	"encoding/json"
	"github.com/goodrain/rainbond/pkg/node/api/model"
)

func NewCmdAddNode() cli.Command {
	c:=cli.Command{
		Name:  "add_node",
		Usage: "添加节点。grctl add_node '{}'(jsoned host node)",
		Action: func(c *cli.Context) error {
			Common(c)
			return addNode(c)
		},
	}
	return c
}
func NewCmdCheckComputeServices() cli.Command {
	c:=cli.Command{
		Name:  "install_compute",
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
		Usage: "安装计算节点。grctl install_compute -h",
		Action: func(c *cli.Context) error {
			return Task(c,"check_compute_services")
		},
	}
	return c
}

func NewCmdComputeGroup() cli.Command {
	c:=cli.Command{
		Name:  "install_compute_model",
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
				Name:  "storage_client",
				Usage: "step 1 storage_client",
				Action: func(c *cli.Context) error {
					return Task(c,"install_storage_client")
				},
			},
			{
				Name:  "kubelet",
				Usage: "need storage_client",
				Action: func(c *cli.Context) error {
					return Task(c,"install_kubelet")
				},
			},
			{
				Name:  "network_compute",
				Usage: "need storage_client,kubelet",
				Action: func(c *cli.Context) error {
					return Task(c,"install_network_compute")
				},
			},
		},
		Usage: "安装计算节点单模块。grctl install_compute_model -h",
	}
	return c
}
func addNode(c *cli.Context) error{

	jsoned:=c.Args().First()
	var node model.APIHostNode
	err:=json.Unmarshal([]byte(jsoned),&node)
	if err != nil {
		logrus.Errorf("error unmarshal input json host node")
		return err
	}
	clients.NodeClient.Nodes().Add(&node)
	return nil
}

