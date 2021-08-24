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
	"os"
	"os/signal"
	"syscall"

	"github.com/goodrain/rainbond/cmd"

	"github.com/goodrain/rainbond/cmd/monitor/option"
	"github.com/goodrain/rainbond/monitor"
	"github.com/goodrain/rainbond/monitor/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		cmd.ShowVersion("monitor")
	}
	c := option.NewConfig()
	c.AddFlag(pflag.CommandLine)
	c.AddPrometheusFlag(pflag.CommandLine)
	pflag.Parse()

	c.CompleteConfig()

	// start prometheus daemon and watching tis status in all time, exit monitor process if start failed
	p := prometheus.NewManager(c)

	// start watching prometheus config from kube api, and update modify to prometheus config
	m, err := monitor.NewMonitor(c, p)
	if err != nil {
		logrus.Fatalf("new monitor module failure %s", err.Error())
	}
	m.Start()
	defer m.Stop()

	errChan := make(chan error, 1)
	defer close(errChan)
	p.StartDaemon(errChan)
	defer p.StopDaemon()

	//step finally: listen Signal
	term := make(chan os.Signal, 1)
	defer close(term)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)

	select {
	case <-term:
		logrus.Warn("Received SIGTERM, exiting monitor gracefully...")
	case err := <-errChan:
		if err != nil {
			logrus.Errorf("Received a error %s from prometheus, exiting monitor gracefully...", err.Error())
		}
	}
	logrus.Info("See you next time!")
}
