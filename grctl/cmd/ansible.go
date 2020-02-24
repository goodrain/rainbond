// RAINBOND, Application Management Platform
// Copyright (C) 2014-2019 Goodrain Co., Ltd.

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
	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/urfave/cli"

	ansibleUtil "github.com/goodrain/rainbond/util/ansible"
)

//NewCmdAnsible ansible config cmd
func NewCmdAnsible() cli.Command {
	c := cli.Command{
		Name:   "ansible",
		Usage:  "Manage the ansible environment",
		Hidden: true,
		Subcommands: []cli.Command{
			cli.Command{
				Name:  "hosts",
				Usage: "Manage the ansible hosts config environment",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "hosts-file-path",
						Usage: "hosts file path",
						Value: "/opt/rainbond/rainbond-ansible/inventory/hosts",
					},
					cli.StringFlag{
						Name:  "config-file-path",
						Usage: "install config path",
						Value: "/opt/rainbond/rainbond-ansible/scripts/installer/global.sh",
					},
				},
				Action: func(c *cli.Context) error {
					Common(c)
					hosts, err := clients.RegionClient.Nodes().List()
					handleErr(err)
					return WriteHostsFile(c.String("hosts-file-path"), c.String("config-file-path"), hosts)
				},
			},
		},
	}
	return c
}

//WriteHostsFile write hosts file
func WriteHostsFile(filePath, installConfPath string, hosts []*client.HostNode) error {
	//get node list from api without condition list.
	//so will get condition
	for i := range hosts {
		nodeWithCondition, _ := clients.RegionClient.Nodes().Get(hosts[i].ID)
		if nodeWithCondition != nil {
			hosts[i] = nodeWithCondition
		}
	}
	return ansibleUtil.WriteHostsFile(filePath, installConfPath, hosts)
}
