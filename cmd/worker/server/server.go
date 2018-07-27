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
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/goodrain/rainbond/appruntimesync"
	"github.com/goodrain/rainbond/appruntimesync/client"
	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/worker/appm"
	"github.com/goodrain/rainbond/worker/discover"
	"github.com/goodrain/rainbond/worker/executor"
	"github.com/goodrain/rainbond/worker/monitor"

	"net/http"
	_ "net/http/pprof"

	"github.com/Sirupsen/logrus"
)

//Run start run
func Run(s *option.Worker) error {
	errChan := make(chan error, 2)
	dbconfig := config.Config{
		DBType:              s.Config.DBType,
		MysqlConnectionInfo: s.Config.MysqlConnectionInfo,
		EtcdEndPoints:       s.Config.EtcdEndPoints,
		EtcdTimeout:         s.Config.EtcdTimeout,
	}
	//step 1:db manager init ,event log client init
	if err := db.CreateManager(dbconfig); err != nil {
		return err
	}
	defer db.CloseManager()

	if err := event.NewManager(event.EventConfig{
		EventLogServers: s.Config.EventLogServers,
		DiscoverAddress: s.Config.EtcdEndPoints,
	}); err != nil {
		return err
	}
	defer event.CloseManager()

	//step 2 : create and start app runtime module
	ars := appruntimesync.CreateAppRuntimeSync(s.Config)
	go ars.Start(errChan)
	defer ars.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	statusClient, err := client.NewClient(ctx, client.AppRuntimeSyncClientConf{
		EtcdEndpoints: s.Config.EtcdEndPoints,
	})
	if err != nil {
		return err
	}
	appmm, err := appm.NewManager(s.Config, statusClient)
	if err != nil {
		return err
	}
	defer appmm.Stop()

	if s.RunMode == "sync" {
		go appmm.SyncData()
	}
	//step 3 : create executor module
	executorManager, err := executor.NewManager(s.Config, statusClient, appmm)
	if err != nil {
		return err
	}
	executorManager.Start()
	defer executorManager.Stop()
	//step 4 : create discover module
	taskManager := discover.NewTaskManager(s.Config, executorManager, statusClient)
	if err := taskManager.Start(); err != nil {
		return err
	}
	defer taskManager.Stop()

	//step 5 :create application use resource exporter.
	exporterManager := monitor.NewManager(s.Config, statusClient)
	if err := exporterManager.Start(); err != nil {
		return err
	}
	defer exporterManager.Stop()

	//step 6 :enable pprof api
	logrus.Info("pprof api listen port 3229")
	go http.ListenAndServe(":3229", nil)

	logrus.Info("worker begin running...")

	//step finally: listen Signal
	term := make(chan os.Signal)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)
	select {
	case <-term:
		logrus.Warn("Received SIGTERM, exiting gracefully...")
	case err := <-errChan:
		if err != nil {
			logrus.Errorf("Received a error %s, exiting gracefully...", err.Error())
		}
	}
	logrus.Info("See you next time!")
	return nil
}
