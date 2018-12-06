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
	"context"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/Sirupsen/logrus"

	"github.com/goodrain/rainbond/cmd/windowsutil/option"
	"github.com/goodrain/rainbond/util/windows"
	"github.com/spf13/pflag"
)

func main() {
	if runtime.GOOS != "windows" {
		logrus.Infof("only support windows system")
		return
	}
	conf := option.Config{}
	conf.AddFlags(pflag.CommandLine)
	if !conf.Check() {
		return
	}
	pflag.Parse()
	ctx, cancel := context.WithCancel(context.Background())
	shell := strings.Split(conf.RunShell, " ")
	cmd := exec.CommandContext(ctx, shell[0], shell[1:]...)
	startFunc := func() error {
		go cmd.Start()
		//step finally: listen Signal
		term := make(chan os.Signal)
		signal.Notify(term, os.Interrupt, syscall.SIGTERM)
		select {
		case si := <-term:
			logrus.Warn("Received SIGTERM, exiting gracefully...")
			return cmd.Process.Signal(si)
		}
		logrus.Info("See you next time!")
		return nil
	}
	stopFunc := func() error {
		cancel()
		return nil
	}
	if err := windows.RunAsService(conf.ServiceName, startFunc, stopFunc, conf.Debug); err != nil {
		logrus.Fatalf("run command failure %s", err.Error())
	}
}
