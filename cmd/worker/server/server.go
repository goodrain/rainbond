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

package server

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/pkg/db"
	"github.com/goodrain/rainbond/pkg/db/config"
	"github.com/goodrain/rainbond/pkg/event"
	"github.com/goodrain/rainbond/pkg/status"
	"github.com/goodrain/rainbond/pkg/worker/appm"
	"github.com/goodrain/rainbond/pkg/worker/discover"
	"github.com/goodrain/rainbond/pkg/worker/executor"
	"github.com/goodrain/rainbond/pkg/worker/monitor"

	"github.com/Sirupsen/logrus"
)

//Run start run
func Run(s *option.Worker) error {
	errChan := make(chan error)
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

	if err := event.NewManager(event.EventConfig{EventLogServers: s.Config.EventLogServers}); err != nil {
		return err
	}
	defer event.CloseManager()

	//step 2 : create status watching
	statusManager := status.NewManager(s.Config)
	if err := statusManager.Start(); err != nil {
		return err
	}
	defer statusManager.Stop()

	appmm, err := appm.NewManager(s.Config, statusManager)
	if err != nil {
		return err
	}
	defer appmm.Stop()

	if s.RunMode == "sync" {
		go appmm.SyncData()
		go statusManager.SyncStatus()
	}
	//step 3 : create executor module
	executorManager, err := executor.NewManager(s.Config, statusManager, appmm)
	if err != nil {
		return err
	}
	executorManager.Start()
	defer executorManager.Stop()
	//step 4 : create discover module
	taskManager := discover.NewTaskManager(s.Config, executorManager, statusManager)
	if err := taskManager.Start(); err != nil {
		return err
	}
	defer taskManager.Stop()

	//step 5 :create application use resource exporter.
	exporterManager := monitor.NewManager(s.Config, statusManager)
	if err := exporterManager.Start(); err != nil {
		return err
	}
	defer exporterManager.Stop()

	logrus.Info("worker begin running...")
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
