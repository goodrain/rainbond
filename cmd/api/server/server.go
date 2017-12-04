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

	"github.com/goodrain/rainbond/cmd/api/option"
	"github.com/goodrain/rainbond/pkg/api/controller"
	"github.com/goodrain/rainbond/pkg/api/db"
	"github.com/goodrain/rainbond/pkg/api/discover"
	"github.com/goodrain/rainbond/pkg/api/handler"
	"github.com/goodrain/rainbond/pkg/api/server"
	"github.com/goodrain/rainbond/pkg/event"

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

	if err := event.NewManager(event.EventConfig{EventLogServers: s.Config.EventLogServers}); err != nil {
		return err
	}
	defer event.CloseManager()

	//TODO:
	//创建mq manager
	//创建k8s manager

	//CreateEventHandler create event handler
	if err := handler.CreateEventHandler(s.Config); err != nil {
		logrus.Errorf("create event handler manager error, %v", err)
		return err
	}
	//创建Servie manager
	if err := handler.CreateServiceManger(s.Config); err != nil {
		logrus.Errorf("create servie manager error, %v", err)
		return err
	}
	//创建Plugin manager
	if err := handler.CreatePluginHandler(s.Config); err != nil {
		logrus.Errorf("create plugin manager error, %v", err)
		return err
	}
	//创建Tenant manager
	if err := handler.CreateTenantManger(s.Config); err != nil {
		logrus.Errorf("create tenant manager error, %v", err)
		return err
	}
	//创建NetRule manager
	if err := handler.CreateNetRulesHandler(s.Config); err != nil {
		logrus.Errorf("create net-rule manager error, %v", err)
		return err
	}
	//创建sources manager
	if err := handler.CreateSourcesHandler(s.Config); err != nil {
		logrus.Errorf("create sources manager error, %v", err)
		return err
	}
	if err := handler.CreateCloudHandler(s.Config); err != nil {
		logrus.Errorf("create cloud auth manager error, %v", err)
		return err
	}
	if err := handler.CreateTokenIdenHandler(s.Config); err != nil {
		logrus.Errorf("create token identification mannager error, %v", err)
		return err
	}
	//初始化token信息
	if err := handler.GetTokenIdenHandler().InitTokenMap(); err != nil {
		logrus.Errorf("init token records error, %v", err)
		return err
	}
	//创建license manager
	// if err := handler.CreateLicenseManger(); err != nil {
	// 	logrus.Errorf("create tenant manager error, %v", err)
	//}
	//创建license验证 manager
	// if err := handler.CreateLicensesInfoManager(); err != nil {
	// 	logrus.Errorf("create license check manager error, %v", err)
	// }
	//创建v2Router manager
	if err := controller.CreateV2RouterManager(s.Config); err != nil {
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
		logrus.Errorf("Received a Signal  %s, exiting gracefully...", s.String())
	case err := <-errChan:
		logrus.Errorf("Received a error %s, exiting gracefully...", err.Error())
	}
	logrus.Info("See you next time!")
	return nil
}
