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
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	api_model "github.com/goodrain/rainbond/pkg/api/model"
	"github.com/goodrain/rainbond/pkg/grctl/clients"
	node_model "github.com/goodrain/rainbond/pkg/node/api/model"
	"github.com/urfave/cli"
)

//NewCmdSources 资源相关操作
func NewCmdSources() cli.Command {
	c := cli.Command{
		Name: "sources",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "create, c",
				Usage: "创建自定义资源。 grctl sources -c NAMESPACE SOURCE_ALIAS -k ENV_NAME -v ENV_VALUE",
			},
			cli.BoolFlag{
				Name:  "update, u",
				Usage: "更新自定义资源。 grctl sources -u NAMESPACE SOURCE_ALIAS -k ENV_NAME -v ENV_VALUE",
			},
			cli.BoolFlag{
				Name:  "delete, d",
				Usage: "删除自定义资源。 grctl sources -d NAMESPACE SOURCE_ALIAS -k ENV_NAME",
			},
			cli.BoolFlag{
				Name:  "get, g",
				Usage: "获取自定义资源。 grctl sources -g NAMESPACE SOURCE_ALIAS -k ENV_NAME",
			},
			cli.StringFlag{
				Name:  "k",
				Usage: "自定义资源名。-k ENV_NAME",
			},
			cli.StringFlag{
				Name:  "v",
				Usage: "-v ENV_VALUE",
			},
		},
		Usage: "自定义资源相关操作。grctl plugin [create/delete/update/get] NAMESPACE SOURCE_ALIAS [commands] [sources]",
		Action: func(c *cli.Context) error {
			Common(c)
			return sourcesAction(c)
		},
	}
	return c
}
func sourcesAction(c *cli.Context) error {
	action := c.Args().First()
	switch action {
	case "create", "-c":
		return createSource(c)
	case "update", "-u":
		return updateSource(c)
	case "delete", "-d":
		return deleteSource(c)
	case "get", "-g":
		return getSource(c)
	}
	return fmt.Errorf("Commands wrong, first args must in [create/update/delete/get] or their simplified format")
}

func getSourceItems(c *cli.Context, lens int) (string, string, error) {
	if len(c.Args()) != lens {
		return "", "", fmt.Errorf("Commands nums wrong, need %d args", lens)
	}
	tenantName := c.Args().Get(1)
	sourceAlias := c.Args().Get(2)
	return tenantName, sourceAlias, nil
}

func createSource(c *cli.Context) error {
	fmt.Println("create source success.")
	tenantName, sourceAlias, err := getSourceItems(c, 7)
	if err != nil {
		logrus.Errorf("params error, %v", err)
	}
	envName := c.String("k")
	envVal := c.String("v")
	sb := &api_model.SoureBody{
		EnvName: envName,
		EnvVal:  envVal,
	}
	ss := &api_model.SourceSpec{
		Alias:      sourceAlias,
		CreateTime: time.Now().Format("2006-01-02 15:04:05"),
		Operator:   "grctl",
		SourceBody: sb,
	}
	if err := clients.RegionClient.Tenants().Get(tenantName).DefineSources(ss).PostSource(sourceAlias); err != nil {
		return err
	}
	return nil
}

func updateSource(c *cli.Context) error {
	fmt.Println("update source success.")
	tenantName, sourceAlias, err := getSourceItems(c, 7)
	if err != nil {
		logrus.Errorf("params error, %v", err)
	}
	envName := c.String("k")
	envVal := c.String("v")
	sb := &api_model.SoureBody{
		EnvName: envName,
		EnvVal:  envVal,
	}
	ss := &api_model.SourceSpec{
		Alias:      sourceAlias,
		CreateTime: time.Now().Format("2006-01-02 15:04:05"),
		Operator:   "grctl",
		SourceBody: sb,
	}
	if err := clients.RegionClient.Tenants().Get(tenantName).DefineSources(ss).PutSource(sourceAlias); err != nil {
		return err
	}
	return nil
}

func deleteSource(c *cli.Context) error {
	fmt.Println("delete source success.")
	tenantName, sourceAlias, err := getSourceItems(c, 5)
	if err != nil {
		logrus.Errorf("params error, %v", err)
	}
	envName := c.String("k")
	sb := &api_model.SoureBody{
		EnvName: envName,
	}
	ss := &api_model.SourceSpec{
		Alias:      sourceAlias,
		CreateTime: time.Now().Format("2006-01-02 15:04:05"),
		Operator:   "grctl",
		SourceBody: sb,
	}
	if err := clients.RegionClient.Tenants().Get(tenantName).DefineSources(ss).DeleteSource(sourceAlias); err != nil {
		return err
	}
	return nil
}

func getSource(c *cli.Context) error {
	fmt.Println("get source success.")
	tenantName, sourceAlias, err := getSourceItems(c, 5)
	if err != nil {
		logrus.Errorf("params error, %v", err)
	}
	envName := c.String("k")
	sb := &api_model.SoureBody{
		EnvName: envName,
	}
	ss := &api_model.SourceSpec{
		Alias:      sourceAlias,
		CreateTime: time.Now().Format("2006-01-02 15:04:05"),
		Operator:   "grctl",
		SourceBody: sb,
	}
	resp, err := clients.RegionClient.Tenants().Get(tenantName).DefineSources(ss).GetSource(sourceAlias)
	if err != nil {
		return err
	}
	switch sourceAlias {
	case node_model.DOWNSTREAM:
		fmt.Printf("resp is %v", string(resp))
	case node_model.UPSTREAM:
		fmt.Printf("resp is %v", string(resp))
	default:
		fmt.Printf("resp is %v", string(resp))
	}
	return nil
}
