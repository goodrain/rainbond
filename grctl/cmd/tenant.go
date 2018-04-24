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

	"github.com/Sirupsen/logrus"
	"github.com/apcera/termtables"
	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/urfave/cli"
)

//NewCmdTenant tenant cmd
func NewCmdTenant() cli.Command {
	c := cli.Command{
		Name:  "tenant",
		Usage: "grctl tenant -h",
		Subcommands: []cli.Command{
			cli.Command{
				Name:  "get",
				Usage: "get all app details by specified tenant name",
				Action: func(c *cli.Context) error {
					Common(c)
					return getTenantInfo(c)
				},
			},
			cli.Command{
				Name:  "res",
				Usage: "get tenant resource details by specified tenant name",
				Action: func(c *cli.Context) error {
					Common(c)
					return findTenantResourceUsage(c)
				},
			},
			cli.Command{
				Name:  "batchstop",
				Usage: "batch stop app by specified tenant name",
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "f",
						Usage: "Continuous log output",
					},
					cli.StringFlag{
						Name:  "event_log_server",
						Usage: "event log server address",
					},
				},
				Action: func(c *cli.Context) error {
					Common(c)
					return stopTenantService(c)
				},
			},
		},
	}
	return c
}

// grctrl tenant TENANT_NAME
func getTenantInfo(c *cli.Context) error {
	tenantID := c.Args().First()

	services, err := clients.RegionClient.Tenants().Get(tenantID).Services().List()
	handleErr(err)
	if services != nil {
		table := termtables.CreateTable()
		table.AddHeaders("租户ID", "服务ID", "服务别名", "应用状态", "Deploy版本")
		for _, service := range services {
			table.AddRow(service.TenantID, service.ServiceID, service.ServiceAlias, service.CurStatus, service.DeployVersion)
		}
		fmt.Println(table.Render())
		return nil
	}
	logrus.Error("get nothing")
	return nil
}
func findTenantResourceUsage(c *cli.Context) error {
	tenantID := c.Args().First()
	services, err := clients.RegionClient.Tenants().Get(tenantID).Services().List()
	handleErr(err)
	var cpuUsage float32
	var cpuUnit float32 = 1000
	var memoryUsage int64
	for _, service := range services {
		cpuUsage += float32(service.ContainerCPU)
		memoryUsage += int64(service.ContainerMemory)
	}
	fmt.Printf("租户 %s 占用CPU : %v 核; 占用Memory : %d M", tenantID, cpuUsage/cpuUnit, memoryUsage)
	fmt.Println()
	return nil
}
