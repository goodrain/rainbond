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
)


func NewCmdBatchStop() cli.Command {
	c:=cli.Command{
		Name:  "batchstop",
		Usage: "批量停止租户应用。grctl batchstop tenant_name",
		Action: func(c *cli.Context) error {
			Common(c)
			return stopTenantService(c)
		},
	}
	return c
}
func stopTenantService(c *cli.Context) error  {
	//GET /v2/tenants/{tenant_name}/services/{service_alias}
	//POST /v2/tenants/{tenant_name}/services/{service_alias}/stop

	tenantID := c.Args().First()
	eventID:=c.Args().Get(1)
	services:=clients.RegionClient.Tenants().Get(tenantID).Services().List()

	for _,service:=range services{
		err:=clients.RegionClient.Tenants().Get(tenantID).Services().Stop(service.ServiceAlias,eventID)
		//err = region.StopService(service["service_id"].(string), service["deploy_version"].(string))
		if err != nil {
			logrus.Error("停止应用失败:" + err.Error())
			return err
		}
	}
	return nil
}


