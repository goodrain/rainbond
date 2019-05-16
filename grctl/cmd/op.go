// RAINBOND, Application Management Platform
// Copyright (C) 2014-2019 Goodrain Co., Ltd.

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
	//"fmt"
	// "io/ioutil"
	"fmt"
	"os/exec"

	//"github.com/goodrain/rainbond/event"

	//"github.com/Sirupsen/logrus"
	"os"

	"github.com/urfave/cli"
	//"github.com/goodrain/rainbond/builder/sources"
	//"github.com/goodrain/rainbond/grctl/clients"
	//"flag"
)

//NewCmdOp grctl op
func NewCmdOp() cli.Command {
	c := cli.Command{
		Name:  "op",
		Usage: "cluster operations",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "task",
				Usage: "op task type: network",
				Value: "network",
			},
			cli.StringFlag{
				Name:  "dnk",
				Usage: "Default network type: calico,flannel",
				Value: "calico",
			},
			cli.StringFlag{
				Name:  "mnk",
				Usage: "Modify network type: flannel,calico",
				Value: "calico",
			},
		},
		Action: func(c *cli.Context) error {
			OpTools(c)
			return nil
		},
	}
	return c
}

//OpTools grctl op
func OpTools(c *cli.Context) {

	cmd := exec.Command("bash", "-c", fmt.Sprintf("cd /opt/rainbond/rainbond-ansible; ./scripts/op/%s.sh %s %s", c.String("task"), c.String("dnk"), c.String("mnk")))

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	if err != nil {
		println(err.Error())
		return
	}
	return
}
