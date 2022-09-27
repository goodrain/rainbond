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
	"fmt"
	"github.com/urfave/cli"
	"os/exec"
)

//NewCmdDomain domain cmd
//v5.2 need refactoring
func NewCmdDomain() cli.Command {
	c := cli.Command{
		Name:  "domain",
		Usage: "cluster httpdomain",
		Subcommands: []cli.Command{
			{
				Name:  "get",
				Usage: "get httpdomain",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "ip",
						Usage: "ip address",
					},
					cli.StringFlag{
						Name:  "domain",
						Usage: "domain",
					},
				},
				Action: func(c *cli.Context) error {
					ip := c.String("ip")
					if len(ip) == 0 {
						fmt.Println("ip must not null")
						return nil
					}
					domain := c.String("domain")
					cmd := exec.Command("bash", "/opt/rainbond/bin/.domain.sh", ip, domain)
					outbuf := bytes.NewBuffer(nil)
					cmd.Stdout = outbuf
					cmd.Run()
					out := outbuf.String()
					fmt.Println(out)
					return nil
				},
			},
		},
	}
	return c
}
