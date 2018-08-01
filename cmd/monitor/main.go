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
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/cmd/monitor/option"
	"github.com/goodrain/rainbond/monitor"
	"github.com/goodrain/rainbond/monitor/prometheus"
	"github.com/spf13/pflag"
	"os"
	"os/signal"
	"syscall"
	"github.com/goodrain/rainbond/monitor/api"
	"github.com/goodrain/rainbond/monitor/api/controller"
	"net/http"
)

func main() {
	c := option.NewConfig()
	c.AddFlag(pflag.CommandLine)
	c.AddPrometheusFlag(pflag.CommandLine)
	pflag.Parse()

	c.CompleteConfig()

	// start prometheus daemon and watching tis status in all time, exit monitor process if start failed
	a := prometheus.NewRulesManager(c)
	p := prometheus.NewManager(c, a)
	controllerManager := controller.NewControllerManager(a,p)

	errChan := make(chan error, 1)
	defer close(errChan)
	p.StartDaemon(errChan)
	defer p.StopDaemon()

	// register prometheus address to etcd cluster
	p.Registry.Start()
	defer p.Registry.Stop()

	// start watching components from etcd, and update modify to prometheus config
	m := monitor.NewMonitor(c, p)
	m.Start()
	defer m.Stop()

	r := api.APIServer(controllerManager)
	logrus.Info("monitor api listen port 3329")
	go http.ListenAndServe(":3329", r)

	//step finally: listen Signal
	term := make(chan os.Signal)
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
