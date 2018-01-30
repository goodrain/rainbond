
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

/*
Copyright 2017 The Goodrain Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import (
	"github.com/goodrain/rainbond/cmd/builder/option"
	api_option "github.com/goodrain/rainbond/cmd/api/option"
	"github.com/goodrain/rainbond/pkg/builder/discover"
	"github.com/goodrain/rainbond/pkg/builder/exector"
	"github.com/goodrain/rainbond/pkg/api/handler"
	"github.com/goodrain/rainbond/pkg/db/config"
	"github.com/goodrain/rainbond/pkg/event"
	"os"
	"os/signal"
	"syscall"

	"github.com/Sirupsen/logrus"
	"net/http"
	"github.com/goodrain/rainbond/pkg/builder/api"
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
	if err := event.NewManager(event.EventConfig{EventLogServers: s.Config.EventLogServers}); err != nil {
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

	r:=api.APIServer()
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
