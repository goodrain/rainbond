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
	"strings"

	"github.com/apcera/termtables"
	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/goodrain/rainbond/node/api/model"
	"github.com/urfave/cli"
	"github.com/goodrain/rainbond/node/nodem/service"
	"io/ioutil"
	"os"
	"path/filepath"
)

//NewCmdConfigs 全局配置相关命令
func NewCmdConfigs() cli.Command {
	c := cli.Command{
		Name:  "conf",
		Usage: "集群和服务配置相关工具",
		Subcommands: []cli.Command{
			cli.Command{
				Name:  "gen",
				Usage: "generate systemd configs by service list yaml file.",
				Action: func(c *cli.Context) error {
					yamlFile := c.Args().First()
					services, err := service.LoadServicesFromLocal(yamlFile)
					if err != nil {
						println("Can not parse the yaml file: ", err.Error())
						return err
					}

					dirName := filepath.Dir(yamlFile)
					ext := filepath.Ext(yamlFile)
					baseName := filepath.Base(yamlFile)
					baseName = baseName[:len(baseName)-len(ext)]
					dirName = fmt.Sprintf("%s/%s", dirName, baseName)
					os.Mkdir(dirName, 0755)

					for _, s := range services {
						content := service.ToConfig(s)
						configFile := fmt.Sprintf("%s/%s.service", dirName, s.Name)
						err := ioutil.WriteFile(configFile, []byte(content), 0644)
						if err != nil {
							println("Can not write service to file: ", err.Error())
						}
					}

					println("Successful to generate service in: ", dirName)

					return nil
				},
			},
			cli.Command{
				Name:  "get",
				Usage: "get all datacenter configs",
				Action: func(c *cli.Context) error {
					configs, err := clients.RegionClient.Configs().Get()
					if err != nil {
						return err
					}
					taskTable := termtables.CreateTable()
					taskTable.AddHeaders("Name", "CNName", "ValueType", "Value")
					for _, config := range configs.Configs {
						taskTable.AddRow(config.Name, config.CNName, config.ValueType, config.Value)
					}
					fmt.Println(taskTable.Render())
					return nil
				},
			},
			cli.Command{
				Name:  "put",
				Usage: "put database configs",
				Action: func(c *cli.Context) error {
					key := c.Args().Get(0)
					value := c.Args().Get(1)
					configs, err := clients.RegionClient.Configs().Get()
					if err != nil {
						return err
					}
					gc := configs.Get(key)
					if gc == nil {
						gcnew := model.ConfigUnit{
							Name: key,
						}
						configs.Add(gcnew)
						gc = configs.Get(key)
					}
					if strings.Contains(value, ",") {
						vas := strings.Split(value, ",")
						gc.ValueType = "array"
						gc.Value = vas
					} else {
						gc.ValueType = "string"
						gc.Value = value
					}
					err = clients.RegionClient.Configs().Put(configs)
					if err != nil {
						return err
					}
					fmt.Printf("configs %s put success \n", key)
					return nil
				},
			},
		},
	}
	return c
}
