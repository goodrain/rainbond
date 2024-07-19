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
	"github.com/goodrain/rainbond/cmd"
	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/cmd/worker/server"
	"github.com/grafana/pyroscope-go"
	"os"

	_ "net/http/pprof"

	"github.com/spf13/pflag"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		cmd.ShowVersion("worker")
	}

	serverAddr := "http://127.0.0.1:4040"
	if os.Getenv("PYROSCOPE_URL") != "" {
		serverAddr = os.Getenv("PYROSCOPE_URL")
	}
	pyroscope.Start(pyroscope.Config{
		ApplicationName: "rbd-worker",

		// replace this with the address of pyroscope server
		ServerAddress: serverAddr,

		// you can disable logging by setting this to nil
		Logger: nil,

		// you can provide static tags via a map:
		Tags: map[string]string{"hostname": os.Getenv("HOSTNAME")},

		ProfileTypes: []pyroscope.ProfileType{
			// these profile types are enabled by default:
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,

			// these profile types are optional:
			pyroscope.ProfileGoroutines,
			pyroscope.ProfileMutexCount,
			pyroscope.ProfileMutexDuration,
			pyroscope.ProfileBlockCount,
			pyroscope.ProfileBlockDuration,
		},
	})
	s := option.NewWorker()
	s.AddFlags(pflag.CommandLine)
	pflag.Parse()
	s.SetLog()
	if err := s.CheckEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if err := server.Run(s); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
