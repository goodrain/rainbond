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

// //NewCmdPlugin 插件相关操作
// func NewCmdPlugin() cli.Command {
// 	c := cli.Command{
// 		Name:  "plugin",
// 		Usage: "插件相关操作。grctl plugin [create/delete/update/build] NAMESPACE PLUGIN_ID [commands] [sources]",
// 		Subcommands: []cli.Command{
// 			{
// 				Name:  "create",
// 				Usage: "创建自定义资源。 grctl sources create -g NAMESPACE/SOURCEALIAS -k ENVNAME -v ENVVALUE",
// 				Action: func(c *cli.Context) error {
// 					return sourcesAction(c, "create")
// 				},
// 				Flags: []cli.Flag{
// 					cli.StringFlag{
// 						Name:  "group, g",
// 						Usage: "--group/-g NAMESPACE/SOURCEALIAS",
// 					},
// 					cli.StringFlag{
// 						Name:  "key, k",
// 						Usage: "自定义资源名，-k ENVNAME",
// 					},
// 					cli.StringFlag{
// 						Name:  "value, v",
// 						Usage: "自定义资源值，-v ENVVALUE",
// 					},
// 				},
// 			},
// 		},
// 	}
// 	return c
// }
// func pluginAction(c *cli.Context) error {
// 	action := c.Args().First()
// 	switch action {
// 	case "create", "-c":
// 		return createPlugin(c)
// 	case "update", "-u":
// 		return updatePlugin(c)
// 	case "delete", "-d":
// 		return deletePlugin(c)
// 	}
// 	return fmt.Errorf("Commands wrong, first args must in [create/update/delete] or their simplified form")
// }

// func getItems(c *cli.Context, lens int) (string, string, *api_model.CreatePluginStruct, error) {
// 	if len(c.Args()) != lens {
// 		return "", "", nil, fmt.Errorf("Commands nums wrong, need %d args", lens)
// 	}
// 	var cps api_model.CreatePluginStruct
// 	tenantName := c.Args().Get(1)
// 	pluginID := c.Args().Get(2)
// 	if len(c.Args()) > 3 {
// 		infos := c.Args().Get(4)
// 		if err := ffjson.Unmarshal([]byte(infos), &cps.Body); err != nil {
// 			return "", "", nil, err
// 		}
// 		return tenantName, pluginID, &cps, nil
// 	}
// 	return tenantName, pluginID, nil, nil
// }

// func createPlugin(c *cli.Context) error {
// 	//args 5
// 	return nil
// }

// func updatePlugin(c *cli.Context) error {
// 	//args 5
// 	return nil
// }

// func deletePlugin(c *cli.Context) error {
// 	//args 3
// 	return nil
// }
