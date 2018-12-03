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
	"io/ioutil"
	"net/http"
	"os"

	"github.com/goodrain/rainbond/cmd"
	httputil "github.com/goodrain/rainbond/util/http"
)

//ParseClientCommnad parse client command
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
		case "reg":

		case "run":

		}
	}
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
	bb, _ := ioutil.ReadAll(res.Body)
	fmt.Println(string(bb))
	return nil
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
