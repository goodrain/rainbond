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
	"os/signal"
	"syscall"

	"github.com/goodrain/rainbond/builder/discover"
	"github.com/goodrain/rainbond/builder/exector"
	"github.com/goodrain/rainbond/builder/monitor"
	"github.com/goodrain/rainbond/cmd/builder/option"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/event"

	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/builder/api"
	"github.com/goodrain/rainbond/builder/clean"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	discoverv2 "github.com/goodrain/rainbond/discover.v2"
)

//Run start run
func Run(s *option.Builder) error {
	errChan := make(chan error)
	//init mysql
	dbconfig := config.Config{
		DBType:              s.Config.DBType,
		MysqlConnectionInfo: s.Config.MysqlConnectionInfo,
		EtcdEndPoints:       s.Config.EtcdEndPoints,
		EtcdTimeout:         s.Config.EtcdTimeout,
	}
	if err := db.CreateManager(dbconfig); err != nil {
		return err
	}
	if err := event.NewManager(event.EventConfig{
		EventLogServers: s.Config.EventLogServers,
		DiscoverAddress: s.Config.EtcdEndPoints,
	}); err != nil {
		return err
	}
	defer event.CloseManager()
	exec, err := exector.NewManager(dbconfig)
	if err != nil {
		return err
	}
	if err := exec.Start(); err != nil {
		return err
	}
	defer exec.Stop()
	dis := discover.NewTaskManager(s.Config, exec)
	if err := dis.Start(); err != nil {
		return err
	}
	defer dis.Stop()

	cle, err := clean.CreateCleanManager()
	if err != nil {
		return err
	}
	if err := cle.Start(errChan); err != nil {
		return err
	}
	defer cle.Stop()

	keepalive, err := discoverv2.CreateKeepAlive(s.Config.EtcdEndPoints, "builder",
		"", s.Config.HostIP, s.Config.APIPort)
	if err != nil {
		return err
	}
	if err := keepalive.Start(); err != nil {
		return err
	}
	defer keepalive.Stop()

	exporter := monitor.NewExporter()
	prometheus.MustRegister(exporter)
	r := api.APIServer()
	r.Handle(s.Config.PrometheusMetricPath, promhttp.Handler())
	logrus.Info("builder api listen port 3228")
	go http.ListenAndServe(":3228", r)

	logrus.Info("builder begin running...")
	//step finally: listen Signal
	term := make(chan os.Signal)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)
	select {
	case <-term:
		logrus.Warn("Received SIGTERM, exiting gracefully...")
	case err := <-errChan:
		logrus.Errorf("Received a error %s, exiting gracefully...", err.Error())
	}
	logrus.Info("See you next time!")
	return nil
}
