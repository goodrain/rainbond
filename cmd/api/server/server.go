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

	"github.com/goodrain/rainbond/api/controller"
	"github.com/goodrain/rainbond/api/db"
	"github.com/goodrain/rainbond/api/discover"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/server"
	"github.com/goodrain/rainbond/cmd/api/option"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/worker/client"

	"github.com/Sirupsen/logrus"
)

//Run start run
func Run(s *option.APIServer) error {
	errChan := make(chan error)
	//启动服务发现
	if _, err := discover.CreateEndpointDiscover(s.Config.EtcdEndpoint); err != nil {
		return err
	}
	//创建db manager
	if err := db.CreateDBManager(s.Config); err != nil {
		logrus.Debugf("create db manager error, %v", err)
		return err
	}
	//创建event manager
	if err := db.CreateEventManager(s.Config); err != nil {
		logrus.Debugf("create event manager error, %v", err)
	}

	if err := event.NewManager(event.EventConfig{
		EventLogServers: s.Config.EventLogServers,
		DiscoverAddress: s.Config.EtcdEndpoint,
	}); err != nil {
		return err
	}
	defer event.CloseManager()
	//create app status client
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cli, err := client.NewClient(ctx, client.AppRuntimeSyncClientConf{
		EtcdEndpoints: s.Config.EtcdEndpoint,
	})
	if err != nil {
		logrus.Errorf("create app status client error, %v", err)
		return err
	}
	//初始化 middleware
	handler.InitProxy(s.Config)
	//创建handle
	if err := handler.InitHandle(s.Config, cli); err != nil {
		logrus.Errorf("init all handle error, %v", err)
		return err
	}
	//创建v2Router manager
	if err := controller.CreateV2RouterManager(s.Config, cli); err != nil {
		logrus.Errorf("create v2 route manager error, %v", err)
	}
	// 启动api
	apiManager := server.NewManager(s.Config)
	if err := apiManager.Start(); err != nil {
		return err
	}
	defer apiManager.Stop()
	logrus.Info("api router is running...")

	//step finally: listen Signal
	term := make(chan os.Signal)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)
	select {
	case s := <-term:
		logrus.Infof("Received a Signal  %s, exiting gracefully...", s.String())
	case err := <-errChan:
		logrus.Errorf("Received a error %s, exiting gracefully...", err.Error())
	}
	logrus.Info("See you next time!")
	return nil
}
