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

package server

import (
	"os"
	"sort"

	version "github.com/goodrain/rainbond/cmd"
	"github.com/goodrain/rainbond/grctl/cmd"
	"github.com/urfave/cli"
)

var App *cli.App

//Run Run
func Run() error {
	App = cli.NewApp()
	App.Version = version.GetVersion()
	App.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Value: "",
			Usage: "default <USER_HOME>/.rbd/grctl.yaml",
		},
	}
	sort.Sort(cli.FlagsByName(App.Flags))
	sort.Sort(cli.CommandsByName(App.Commands))
	App.Commands = cmd.GetCmds()
	return App.Run(os.Args)
}
