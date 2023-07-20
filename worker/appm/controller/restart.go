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
	"context"
	"fmt"
	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/appm/conversion"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

type restartController struct {
	stopChan     chan struct{}
	cfg          option.Config
	controllerID string
	appService   []v1.AppService
	manager      *Manager
	ctx          context.Context
}

func (s *restartController) Begin() {
	var wait sync.WaitGroup
	for _, service := range s.appService {
		wait.Add(1)
		go func(service v1.AppService) {
			defer wait.Done()
			service.Logger.Info("App runtime begin restart app service "+service.ServiceAlias, event.GetLoggerOption("starting"))
			if err := s.restartOne(service); err != nil {
				logrus.Errorf("restart service %s failure %s", service.ServiceAlias, err.Error())
			} else {
				service.Logger.Info(fmt.Sprintf("restart service %s success", service.ServiceAlias), event.GetLastLoggerOption())
			}
		}(service)
	}
	wait.Wait()
	s.manager.callback(s.controllerID, nil)
}
func (s *restartController) restartOne(app v1.AppService) error {
	//Restart the control set timeout interval is 5m
	stopController := &stopController{
		manager:      s.manager,
		waiting:      time.Minute * 5,
		ctx:          s.ctx,
		controllerID: s.controllerID,
	}
	if err := stopController.stopOne(app); err != nil {
		if err != ErrWaitTimeOut {
			app.Logger.Error(util.Translation("(restart)stop service error"), event.GetCallbackLoggerOption())
			return err
		}
		//waiting app closed,max wait 40 second
		var waiting = 20
		for waiting > 0 {
			storeAppService := s.manager.store.GetAppService(app.ServiceID)
			if storeAppService == nil || storeAppService.IsClosed() {
				break
			}
			waiting--
			time.Sleep(time.Second * 2)
		}
	}
	startController := startController{
		manager:      s.manager,
		ctx:          s.ctx,
		controllerID: s.controllerID,
	}
	newAppService, err := conversion.InitAppService(s.cfg.SharedStorageClass, false, db.GetManager(), app.ServiceID, app.ExtensionSet)
	if err != nil {
		logrus.Errorf("Application model init create failure:%s", err.Error())
		app.Logger.Error(util.Translation("(restart)Application model init create failure"), event.GetCallbackLoggerOption())
		return fmt.Errorf("application model init create failure,%s", err.Error())
	}
	newAppService.Logger = app.Logger
	//regist new app service
	s.manager.store.RegistAppService(newAppService)
	if err := startController.startOne(*newAppService); err != nil {
		if err != ErrWaitTimeOut {
			app.Logger.Error(util.Translation("start service error"), event.GetCallbackLoggerOption())
			return err
		}
	}
	return nil
}
func (s *restartController) Stop() error {
	close(s.stopChan)
	return nil
}
