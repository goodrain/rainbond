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
	// "io/ioutil"
	"os/exec"

	//"github.com/goodrain/rainbond/event"

	//"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
	"os"

	//"github.com/goodrain/rainbond/builder/sources"
	//"github.com/goodrain/rainbond/grctl/clients"
	//"flag"
)

//NewCmdInit grctl reset
func NewCmdReset() cli.Command {
	c := cli.Command{
		Name: "reset",
		Usage: "重置当前节点grctl reset",
		Action: func(c *cli.Context) error {
			resetCurrentNode(c)
			return nil
		},
	}
	return c
}


func resetCurrentNode(c *cli.Context) {

	// stop rainbond services
	fmt.Println("Start stop rainbond services")
	cmd := exec.Command("grclis", "reset")

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
