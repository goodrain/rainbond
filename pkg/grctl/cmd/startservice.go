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
	"strings"
	"errors"
)


func NewCmdStartService() cli.Command {
	c:=cli.Command{
		Name:  "start",
		Usage: "启动应用 grctl start goodrain/gra564a1 eventID",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "f",
				Usage: "添加此参数日志持续输出。",
			},
			cli.StringFlag{
				Name:  "event_log_server",
				Usage: "event log server address",
			},
		},
		Action: func(c *cli.Context) error {
			Common(c)
			return startService(c)
		},

	}
	return c
}
func NewCmdStopService() cli.Command {
	c:=cli.Command{
		Name:  "stop",
		Usage: "启动应用 grctl stop goodrain/gra564a1 eventID",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "f",
				Usage: "添加此参数日志持续输出。",
			},
			cli.StringFlag{
				Name:  "event_log_server",
				Usage: "event log server address",
			},
		},
		Action: func(c *cli.Context) error {
			Common(c)
			return stopService(c)
		},
	}
	return c
}
func startService(c *cli.Context) error  {
	//GET /v2/tenants/{tenant_name}/services/{service_alias}
	//POST /v2/tenants/{tenant_name}/services/{service_alias}/stop

	// goodrain/gra564a1
	serviceAlias := c.Args().First()
	info := strings.Split(serviceAlias, "/")

	eventID:=c.Args().Get(1)


	service:=clients.RegionClient.Tenants().Get(info[0]).Services().Get(info[1])
	if service==nil {
		return errors.New("应用不存在:"+info[1])
	}
	err:=clients.RegionClient.Tenants().Get(info[0]).Services().Start(info[1],eventID)
	if c.Bool("f") {
		server := "127.0.0.1:6363"
		if c.String("event_log_server") != "" {
			server = c.String("event_log_server")
		}
		GetEventLogf(eventID,server)
	}

	//err = region.StopService(service["service_id"].(string), service["deploy_version"].(string))
	if err != nil {
		logrus.Error("启动应用失败:" + err.Error())
		return err
	}
	return nil
}


func stopService(c *cli.Context) error {

	serviceAlias := c.Args().First()
	info := strings.Split(serviceAlias, "/")

	eventID:=c.Args().Get(1)
	service:=clients.RegionClient.Tenants().Get(info[0]).Services().Get(info[1])
	if service==nil {
		return errors.New("应用不存在:"+info[1])
	}
	err:=clients.RegionClient.Tenants().Get(info[0]).Services().Stop(info[1],eventID)
	if c.Bool("f") {
		server := "127.0.0.1:6363"
		if c.String("event_log_server") != "" {
			server = c.String("event_log_server")
		}
		GetEventLogf(eventID,server)
	}
	if err != nil {
		logrus.Error("停止应用失败:" + err.Error())
		return err
	}
	return nil
}