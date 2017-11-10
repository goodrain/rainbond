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
	"os"
	conf "github.com/goodrain/rainbond/cmd/grctl/option"
	"github.com/goodrain/rainbond/pkg/grctl/clients"
)

func GetCmds() []cli.Command {
	cmds:=[]cli.Command{}
	cmds=append(cmds,NewCmdBatchStop())
	cmds=append(cmds,NewCmdStartService())
	cmds=append(cmds,NewCmdStopService())
	cmds=append(cmds,NewCmdTenant())
	cmds=append(cmds,NewCmdTenantRes())
	cmds=append(cmds,NewCmdNode())
	cmds=append(cmds,NewCmdNodeRes())
	//todo
	return cmds
}
func Common(c *cli.Context) {
	config, err := conf.LoadConfig(c)
	if err != nil {
		logrus.Error("Load config file error.", err.Error())
		os.Exit(1)
	}
	//if err := db.InitDB(*config.RegionMysql); err != nil {
	//	os.Exit(1)
	//}
	if err := clients.InitClient(*config.Kubernets); err != nil {
		os.Exit(1)
	}
	//clients.SetInfo(config.RegionAPI.URL, config.RegionAPI.Token)
	clients.InitRegionClient(*config.RegionAPI)
}