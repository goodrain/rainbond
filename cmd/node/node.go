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

package main

import (
	"fmt"
	"os"

	"github.com/goodrain/rainbond/cmd"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/cmd/node/server"
	"github.com/goodrain/rainbond/node/nodem/controller"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		cmd.ShowVersion("node")
	}
	option.Init()

	if option.Config.ServiceManager == "systemd" {
		if err := controller.StartRequiresSystemd(option.Config); err != nil {
			fmt.Fprintf(os.Stderr, "failed to start requires service: %v", err)
			os.Exit(1)
		}
	}

	if err := server.Run(option.Config); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
