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

package main

import (
	"os"
	"sort"

	"github.com/sirupsen/logrus"

	version "github.com/goodrain/rainbond/cmd"
	"github.com/goodrain/rainbond/cmd/init-probe/cmd"
	"github.com/urfave/cli"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		version.ShowVersion("init-probe")
	}
	App := cli.NewApp()
	App.Version = version.GetVersion()
	App.Flags = []cli.Flag{}
	App.Commands = cmd.GetCmds()
	sort.Sort(cli.FlagsByName(App.Flags))
	sort.Sort(cli.CommandsByName(App.Commands))
	if err := App.Run(os.Args); err != nil {
		logrus.Errorf("probe cmd run failure. %s", err.Error())
		os.Exit(1)
	}
}
