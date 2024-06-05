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
	"context"
	"fmt"
	"github.com/goodrain/rainbond-operator/util/constants"
	utils "github.com/goodrain/rainbond/util"
	"os"

	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewCmdConfig config command
func NewCmdConfig() cli.Command {
	c := cli.Command{
		Name: "config",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "output,o",
				Usage: "write region api config to file",
			},
			cli.StringFlag{
				Name:  "namespace,ns",
				Usage: "rainbond default namespace",
				Value: utils.GetenvDefault("RBD_NAMESPACE", constants.Namespace),
			},
		},
		Usage: "show region config file",
		Action: func(c *cli.Context) {
			Common(c)
			namespace := c.String("namespace")
			configMap, err := clients.K8SClient.CoreV1().ConfigMaps(namespace).Get(context.Background(), "region-config", metav1.GetOptions{})
			if err != nil {
				showError(err.Error())
			}
			regionConfig := map[string]string{
				"client.pem":          string(configMap.BinaryData["client.pem"]),
				"client.key.pem":      string(configMap.BinaryData["client.key.pem"]),
				"ca.pem":              string(configMap.BinaryData["ca.pem"]),
				"apiAddress":          configMap.Data["apiAddress"],
				"websocketAddress":    configMap.Data["websocketAddress"],
				"defaultDomainSuffix": configMap.Data["defaultDomainSuffix"],
				"defaultTCPHost":      configMap.Data["defaultTCPHost"],
			}
			body, err := yaml.Marshal(regionConfig)
			if err != nil {
				showError(err.Error())
			}
			if c.String("o") != "" {
				file, err := os.OpenFile(c.String("o"), os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					showError(err.Error())
				}
				defer file.Close()
				_, err = file.Write(body)
				if err != nil {
					showError(err.Error())
				}
			} else {
				fmt.Println(string(body))
			}
		},
	}
	return c
}
