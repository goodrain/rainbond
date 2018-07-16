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

// //NewCmdSources 资源相关操作
// func NewCmdSources() cli.Command {
// 	c := cli.Command{
// 		Name:  "sources",
// 		Usage: "自定义资源相关操作。grctl sources [create/delete/update/get] -g NAMESPACE/SOURCEALIAS [commands] [sources]",
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
// 			{
// 				Name:  "update",
// 				Usage: "更新自定义资源。 grctl sources update -g NAMESPACE/SOURCEALIAS -k ENVNAME -v ENVVALUE",
// 				Action: func(c *cli.Context) error {
// 					return sourcesAction(c, "update")
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
// 			{
// 				Name:  "delete",
// 				Usage: "删除自定义资源。 grctl sources delete -g NAMESPACE/SOURCEALIAS -k ENVNAME",
// 				Action: func(c *cli.Context) error {
// 					return sourcesAction(c, "delete")
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
// 				},
// 			},
// 			{
// 				Name:  "get",
// 				Usage: "获取自定义资源。 grctl sources get -g NAMESPACE/SOURCEALIAS -k ENVNAME",
// 				Action: func(c *cli.Context) error {

// 					return sourcesAction(c, "get")
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
// 				},
// 			},
// 		},
// 	}
// 	return c
// }

// func sourcesAction(c *cli.Context, action string) error {
// 	Common(c)
// 	switch action {
// 	case "create", "-c":
// 		return createSource(c)
// 	case "update", "-u":
// 		return updateSource(c)
// 	case "delete", "-d":
// 		return deleteSource(c)
// 	case "get", "-g":
// 		return getSource(c)
// 	}
// 	return fmt.Errorf("Commands wrong, first args must in [create/update/delete/get] or their simplified format")
// }

// func getSourceItems(c *cli.Context, lens int) (string, string, error) {
// 	fmt.Printf("len is %v\n", len(c.Args()))
// 	if len(c.Args()) != lens {
// 		return "", "", fmt.Errorf("Commands nums wrong, need %d args", lens)
// 	}
// 	tenantName := c.Args().Get(1)
// 	sourceAlias := c.Args().Get(2)
// 	logrus.Debugf("tenant_name %s, source_alias %s", tenantName, sourceAlias)
// 	return tenantName, sourceAlias, nil
// }

// func createSource(c *cli.Context) error {
// 	tenantName, sourceAlias, err := checkoutGroup(c)
// 	if err != nil {
// 		return err
// 	}
// 	envName, err := checkoutKV(c, "key")
// 	if err != nil {
// 		return err
// 	}
// 	envVal, err := checkoutKV(c, "value")
// 	if err != nil {
// 		return err
// 	}
// 	sb := &api_model.SoureBody{
// 		EnvName: envName,
// 		EnvVal:  envVal,
// 	}
// 	ss := &api_model.SourceSpec{
// 		Alias:      sourceAlias,
// 		CreateTime: time.Now().Format("2006-01-02 15:04:05"),
// 		Operator:   "grctl",
// 		SourceBody: sb,
// 	}
// 	if err := clients.RegionClient.Tenants().Get(tenantName).DefineSources(ss).PostSource(sourceAlias); err != nil {
// 		return err
// 	}
// 	fmt.Printf("create %s success\n", envName)
// 	return nil
// }

// func updateSource(c *cli.Context) error {
// 	tenantName, sourceAlias, err := checkoutGroup(c)
// 	if err != nil {
// 		return err
// 	}
// 	envName, err := checkoutKV(c, "key")
// 	if err != nil {
// 		return err
// 	}
// 	envVal, err := checkoutKV(c, "value")
// 	if err != nil {
// 		return err
// 	}
// 	sb := &api_model.SoureBody{
// 		EnvName: envName,
// 		EnvVal:  envVal,
// 	}
// 	ss := &api_model.SourceSpec{
// 		Alias:      sourceAlias,
// 		CreateTime: time.Now().Format("2006-01-02 15:04:05"),
// 		Operator:   "grctl",
// 		SourceBody: sb,
// 	}
// 	if err := clients.RegionClient.Tenants().Get(tenantName).DefineSources(ss).PutSource(sourceAlias); err != nil {
// 		return err
// 	}
// 	fmt.Printf("update %s success\n", envName)
// 	return nil
// }

// func deleteSource(c *cli.Context) error {
// 	tenantName, sourceAlias, err := checkoutGroup(c)
// 	if err != nil {
// 		return err
// 	}
// 	envName, err := checkoutKV(c, "key")
// 	if err != nil {
// 		return err
// 	}
// 	sb := &api_model.SoureBody{
// 		EnvName: envName,
// 	}
// 	ss := &api_model.SourceSpec{
// 		Alias:      sourceAlias,
// 		CreateTime: time.Now().Format("2006-01-02 15:04:05"),
// 		Operator:   "grctl",
// 		SourceBody: sb,
// 	}
// 	if err := clients.RegionClient.Tenants().Get(tenantName).DefineSources(ss).DeleteSource(sourceAlias); err != nil {
// 		return err
// 	}
// 	fmt.Printf("delete %s success\n", envName)
// 	return nil
// }

// func getSource(c *cli.Context) error {
// 	tenantName, sourceAlias, err := checkoutGroup(c)
// 	if err != nil {
// 		return err
// 	}
// 	envName, err := checkoutKV(c, "key")
// 	if err != nil {
// 		return err
// 	}
// 	sb := &api_model.SoureBody{
// 		EnvName: envName,
// 	}
// 	ss := &api_model.SourceSpec{
// 		Alias:      sourceAlias,
// 		CreateTime: time.Now().Format("2006-01-02 15:04:05"),
// 		Operator:   "grctl",
// 		SourceBody: sb,
// 	}
// 	resp, err := clients.RegionClient.Tenants().Get(tenantName).DefineSources(ss).GetSource(sourceAlias)
// 	if err != nil {
// 		return err
// 	}
// 	fmt.Printf("resp is %v", string(resp))
// 	return nil
// }

// func checkoutGroup(c *cli.Context) (string, string, error) {
// 	group := c.String("group")
// 	if group == "" {
// 		logrus.Errorf("Incorrect Usage: flag provided but not defined: -group")
// 		return "", "", fmt.Errorf("have no group set, -g TENANTID/SOURCEALIAS")
// 	}
// 	if strings.Contains(group, "/") {
// 		mm := strings.Split(group, "/")
// 		tenantName, sourceAlias := mm[0], mm[1]
// 		return tenantName, sourceAlias, nil
// 	}
// 	logrus.Errorf("format Error, group format must in: -g TENANTID/SOURCEALIAS ")
// 	return "", "", fmt.Errorf("group format wrong")
// }

// func checkoutKV(c *cli.Context, kind string) (string, error) {
// 	value := c.String(kind)
// 	if value == "" {
// 		logrus.Errorf("need %s, --%s ARGV", kind, kind)
// 		return "", fmt.Errorf("have no %s", kind)
// 	}
// 	return value, nil
// }
