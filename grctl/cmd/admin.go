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
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/urfave/cli"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/goodrain/rainbond/grctl/clients"
)

//NewCmdAdmin -
func NewCmdAdmin() cli.Command {
	c := cli.Command{
		Name:  "admin",
		Usage: "rainbond administrator manage cmd",
		Subcommands: []cli.Command{
			{
				Name:  "reset-password",
				Usage: "reset password of rainbond administrator",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name: "password,pass",
					},
				},
				Action: resetAdminPassword,
			},
		},
	}
	return c
}

func getNewPass(c *cli.Context) string {
	pass := c.Args().First()
	if pass == "" {
		pass = c.String("pass")
	}
	if pass == "" {
		showError("please set new pass, eg: grctl admin reset-admin-password 12345678")
	}
	return pass
}

func resetAdminPassword(c *cli.Context) {
	// init kubernetes client
	Common(c)

	// prepare url
	url := getURL()

	// prepare new password
	pass := getNewPass(c)
	data := map[string]interface{}{"password": pass}

	// prepare expect data
	var expect respStruct

	// do request
	if err := doRequest(http.MethodPut, url, data, &expect); err != nil {
		showError(fmt.Sprintf("do request failed: %s", err.Error()))
	}

	// handle custom code
	if expect.Code != 200 {
		showError(fmt.Sprintf("reset failed: %s", expect.Msg))
	}

	// success
	showSuccessMsg("success")
}

func getURL() string {
	// get openapi service address and port
	services, err := clients.K8SClient.CoreV1().Services(clients.RainbondNamespace).List(metav1.ListOptions{LabelSelector: "key=rainbond-openapi-admin"})
	if err != nil {
		showError(fmt.Sprintf("get openapi service failed: %s", err.Error()))
	}
	if services == nil || len(services.Items) == 0 {
		showError("can't found operator svc")
	}
	svc := services.Items[0]
	addr := svc.Spec.ClusterIP
	port := 1234
	if len(svc.Spec.Ports) > 0 {
		port = int(svc.Spec.Ports[0].Port)
	}
	// format url
	return fmt.Sprintf("http://%s:%d/admin/reset-password", addr, port)

}

type respStruct struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func doRequest(method, url string, data map[string]interface{}, expect interface{}) error {
	bs, _ := json.Marshal(data)
	req, err := http.NewRequest(method, url, bytes.NewBuffer(bs))
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// handle http code
	if resp.StatusCode != 200 {
		return fmt.Errorf("do request failed: http code is: %d", resp.StatusCode)
	}

	if err := json.Unmarshal(body, expect); err != nil {
		return err
	}
	return nil
}
