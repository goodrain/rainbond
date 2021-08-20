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
	"github.com/goodrain/rainbond/grctl/cluster"
	"github.com/urfave/cli"
)

//NewCmdCluster cmd for cluster
func NewCmdCluster() cli.Command {
	c := cli.Command{
		Name:  "cluster",
		Usage: "Cluster management commands",
		Subcommands: []cli.Command{
			{
				Name:  "upgrade",
				Usage: "upgrade cluster",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "namespace, ns",
						Usage: "rainbond default namespace",
						Value: "rbd-system",
					},
					cli.StringFlag{
						Name:     "new-version",
						Usage:    "the new version of rainbond cluster",
						Required: true,
					},
				},
				Action: func(c *cli.Context) error {
					Common(c)
					cluster, err := cluster.NewCluster(c.String("namespace"), c.String("new-version"))
					if err != nil {
						return err
					}
					return cluster.Upgrade()
				},
			},
		},
	}
	return c
}
