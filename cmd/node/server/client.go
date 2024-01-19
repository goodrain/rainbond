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

package server

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/goodrain/rainbond/util"

	"github.com/sirupsen/logrus"

	"github.com/goodrain/rainbond/event"

	"github.com/goodrain/rainbond/builder/parser"

	"github.com/goodrain/rainbond/node/nodem/service"

	"github.com/goodrain/rainbond/cmd"

	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/urfave/cli"
)

// ParseClientCommnad parse client command
// node service xxx :Operation of the guard component
// node reg : Register the daemon configuration for node
// node run: daemon start node server
func ParseClientCommnad(args []string) {

	if len(args) > 1 {
		switch args[1] {
		case "version":
			cmd.ShowVersion("node")
		case "service":
			controller := controllerServiceClient{}
			if len(args) > 2 {
				switch args[2] {
				case "start":
					if len(args) < 4 {
						fmt.Printf("Parameter error")
					}
					//enable a service
					serviceName := args[3]
					if err := controller.startService(serviceName); err != nil {
						fmt.Printf("start service %s failure %s", serviceName, err.Error())
						os.Exit(1)
					}
					fmt.Printf("start service %s success", serviceName)
					os.Exit(0)
				case "stop":
					if len(args) < 4 {
						fmt.Printf("Parameter error")
					}
					//disable a service
					serviceName := args[3]
					if err := controller.stopService(serviceName); err != nil {
						fmt.Printf("stop service %s failure %s", serviceName, err.Error())
						os.Exit(1)
					}
					fmt.Printf("stop service %s success", serviceName)
					os.Exit(0)
				case "update":
					if err := controller.updateConfig(); err != nil {
						fmt.Printf("update service config failure %s", err.Error())
						os.Exit(1)
					}
					fmt.Printf("update service config success")
					os.Exit(0)
				}
			}
		case "upgrade":
			App := cli.NewApp()
			App.Version = "0.1"
			App.Commands = []cli.Command{
				{
					Name: "upgrade",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "config-dir,c",
							Value: "/opt/rainbond/conf",
							Usage: "service config file dir",
						},
						cli.StringFlag{
							Name:  "new-version,v",
							Value: "",
							Usage: "upgrade target version",
						},
						cli.StringFlag{
							Name:  "image-prefix,p",
							Value: "goodrain.me",
							Usage: "",
						},
						cli.StringSliceFlag{
							Name:  "services,s",
							Value: &cli.StringSlice{"rbd-gateway", "rbd-api", "rbd-chaos", "rbd-mq", "rbd-webcli", "rbd-worker", "rbd-eventlog", "rbd-monitor", "rbd-app-ui"},
							Usage: "Enable supported services",
						},
					},
					Action: upgradeImages,
				},
			}
			sort.Sort(cli.FlagsByName(App.Flags))
			sort.Sort(cli.CommandsByName(App.Commands))
			if err := App.Run(os.Args); err != nil {
				logrus.Errorf("upgrade failure %s", err.Error())
				os.Exit(1)
			}
			logrus.Info("upgrade success")
			os.Exit(0)
		case "run":

		}
	}
}

// upgrade image name
func upgradeImages(ctx *cli.Context) error {
	services := service.LoadServicesWithFileFromLocal(ctx.String("c"))
	for i, serviceList := range services {
		for j, service := range serviceList.Services {
			if util.StringArrayContains(ctx.StringSlice("s"), service.Name) &&
				service.Start != "" && !service.OnlyHealthCheck {
				par := parser.CreateDockerRunOrImageParse("", "", service.Start, nil, event.GetTestLogger(), "")
				par.ParseDockerun(service.Start)
				image := par.GetImage()
				if image.Name == "" {
					continue
				}
				newImage := ctx.String("p") + "/" + service.Name + ":" + ctx.String("v")
				oldImage := func() string {
					if image.IsOfficial() {
						return image.GetRepostory() + ":" + image.GetTag()
					}
					return image.String()
				}()
				services[i].Services[j].Start = strings.Replace(services[i].Services[j].Start, oldImage, newImage, 1)
				logrus.Infof("upgrade %s image from %s to %s", service.Name, oldImage, newImage)
			}
		}
	}
	return service.WriteServicesWithFile(services...)
}

type controllerServiceClient struct {
}

func (c *controllerServiceClient) request(url string) error {
	res, err := http.Post(url, "", nil)
	if err != nil {
		return err
	}
	if res.StatusCode == 200 {
		return nil
	}
	resbody, err := httputil.ParseResponseBody(res.Body, "application/json")
	if err != nil {
		return err
	}
	return fmt.Errorf(resbody.Msg)
}
func (c *controllerServiceClient) startService(serviceName string) error {
	return c.request(fmt.Sprintf("http://127.0.0.1:6100/services/%s/start", serviceName))
}
func (c *controllerServiceClient) stopService(serviceName string) error {
	return c.request(fmt.Sprintf("http://127.0.0.1:6100/services/%s/stop", serviceName))
}
func (c *controllerServiceClient) updateConfig() error {
	return c.request(fmt.Sprintf("http://127.0.0.1:6100/services/update"))
}
