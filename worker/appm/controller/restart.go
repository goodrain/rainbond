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

package controller

import (
	"fmt"
	"sync"
	"time"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/worker/appm/conversion"

	"github.com/Sirupsen/logrus"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
)

type restartController struct {
	stopChan     chan struct{}
	controllerID string
	appService   []v1.AppService
	manager      *Manager
}

func (s *restartController) Begin() {
	var wait sync.WaitGroup
	for _, service := range s.appService {
		go func(service v1.AppService) {
			wait.Add(1)
			defer wait.Done()
			service.Logger.Info("App runtime begin restart app service "+service.ServiceAlias, getLoggerOption("starting"))
			if err := s.restartOne(service); err != nil {
				logrus.Errorf("restart service %s failure %s", service.ServiceAlias, err.Error())
			} else {
				service.Logger.Info(fmt.Sprintf("restart service %s success", service.ServiceAlias), GetLastLoggerOption())
			}
		}(service)
	}
	wait.Wait()
	s.manager.callback(s.controllerID, nil)
}
func (s *restartController) restartOne(app v1.AppService) error {
	stopController := stopController{
		manager: s.manager,
	}
	if err := stopController.stopOne(app); err != nil {
		app.Logger.Error("(Restart)Stop app failure %s,you could waiting stoped and manual start it", GetCallbackLoggerOption())
		return err
	}
	//sleep 3 secode
	time.Sleep(time.Second * 3)

	startController := startController{
		manager: s.manager,
	}
	newAppService, err := conversion.InitAppService(db.GetManager(), app.ServiceID)
	if err != nil {
		logrus.Errorf("Application model init create failure:%s", err.Error())
		app.Logger.Error("Application model init create failure", GetCallbackLoggerOption())
		return fmt.Errorf("Application model init create failure,%s", err.Error())
	}
	newAppService.Logger = app.Logger
	//regist new app service
	s.manager.store.RegistAppService(newAppService)
	if err := startController.startOne(*newAppService); err != nil {
		app.Logger.Error(fmt.Sprintf("(Restart)Start app failure %s,you could waiting it start success.or manual stop it", newAppService.ServiceAlias), GetCallbackLoggerOption())
		return err
	}
	return nil
}
func (s *restartController) Stop() error {
	close(s.stopChan)
	return nil
}
