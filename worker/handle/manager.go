// Copyright (C) 2nilfmt.Errorf("a")4-2nilfmt.Errorf("a")8 Goodrain Co., Ltd.
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

package handle

import (
	"context"
	"fmt"
	"reflect"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/worker/appm/controller"
	"github.com/goodrain/rainbond/worker/appm/conversion"
	"github.com/goodrain/rainbond/worker/appm/store"
	"github.com/goodrain/rainbond/worker/discover/model"
)

//Manager manager
type Manager struct {
	ctx               context.Context
	c                 option.Config
	store             store.Storer
	dbmanager         db.Manager
	controllerManager *controller.Manager
}

//NewManager now handle
func NewManager(ctx context.Context,
	config option.Config,
	store store.Storer,
	controllerManager *controller.Manager) *Manager {

	return &Manager{
		ctx:               ctx,
		c:                 config,
		dbmanager:         db.GetManager(),
		store:             store,
		controllerManager: controllerManager,
	}
}

//ErrCallback do not handle this task
var ErrCallback = fmt.Errorf("callback task to mq")

func (m *Manager) checkCount() bool {
	if m.controllerManager.GetControllerSize() > m.c.MaxTasks {
		return true
	}
	return false
}

//AnalystToExec analyst exec
func (m *Manager) AnalystToExec(task *model.Task) error {
	if task == nil {
		return nil
	}
	//max worker count check
	if m.checkCount() {
		return ErrCallback
	}
	if !m.store.Ready() {
		return ErrCallback
	}
	switch task.Type {
	case "start":
		logrus.Info("start a 'start' task worker")
		return m.startExec(task)
	case "stop":
		logrus.Info("start a 'stop' task worker")
		return m.stopExec(task)
	case "restart":
		logrus.Info("start a 'restart' task worker")
		return m.restartExec(task)
	case "horizontal_scaling":
		logrus.Info("start a 'horizontal_scaling' task worker")
		return m.horizontalScalingExec(task)
	case "vertical_scaling":
		logrus.Info("start a 'vertical_scaling' task worker")
		return m.verticalScalingExec(task)
	case "rolling_upgrade":
		logrus.Info("start a 'rolling_upgrade' task worker")
		return m.rollingUpgradeExec(task)
	case "apply_rule":
		logrus.Info("start a 'apply_rule' task worker")
		return m.applyRuleExec(task)
	default:
		return nil
	}
}

//startExec exec start service task
func (m *Manager) startExec(task *model.Task) error {
	body, ok := task.Body.(model.StartTaskBody)
	if !ok {
		logrus.Errorf("start body convert to taskbody error")
		return fmt.Errorf("start body convert to taskbody error")
	}
	logger := event.GetManager().GetLogger(body.EventID)
	appService := m.store.GetAppService(body.ServiceID)
	if appService != nil && !appService.IsClosed() {
		logger.Info("Application is not closed, can not start", controller.GetLastLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return nil
	}
	newAppService, err := conversion.InitAppService(m.dbmanager, body.ServiceID)
	if err != nil {
		logrus.Errorf("Application init create failure:%s", err.Error())
		logger.Error("Application init create failure", controller.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("Application init create failure")
	}
	newAppService.Logger = logger
	//regist new app service
	m.store.RegistAppService(newAppService)
	err = m.controllerManager.StartController(controller.TypeStartController, *newAppService)
	if err != nil {
		logrus.Errorf("Application run  start controller failure:%s", err.Error())
		logger.Error("Application run start controller failure", controller.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("Application start failure")
	}
	logrus.Infof("service(%s) %s working is running.", body.ServiceID, "start")
	return nil
}

func (m *Manager) stopExec(task *model.Task) error {
	body, ok := task.Body.(model.StopTaskBody)
	if !ok {
		logrus.Errorf("stop body convert to taskbody error")
		return fmt.Errorf("stop body convert to taskbody error")
	}
	logger := event.GetManager().GetLogger(body.EventID)
	appService := m.store.GetAppService(body.ServiceID)
	if appService == nil {
		logger.Info("Application is closed, can not stop", controller.GetLastLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return nil
	}
	appService.Logger = logger
	err := m.controllerManager.StartController(controller.TypeStopController, *appService)
	if err != nil {
		logrus.Errorf("Application run  stop controller failure:%s", err.Error())
		logger.Info("Application run stop controller failure", controller.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("Application stop failure")
	}
	logrus.Infof("service(%s) %s working is running.", body.ServiceID, "stop")
	return nil
}

func (m *Manager) restartExec(task *model.Task) error {
	body, ok := task.Body.(model.RestartTaskBody)
	if !ok {
		logrus.Errorf("stop body convert to taskbody error")
		return fmt.Errorf("stop body convert to taskbody error")
	}
	logger := event.GetManager().GetLogger(body.EventID)
	appService := m.store.GetAppService(body.ServiceID)
	if appService == nil {
		logger.Info("Application is closed, can not stop", controller.GetLastLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return nil
	}
	appService.Logger = logger
	//first stop app
	err := m.controllerManager.StartController(controller.TypeRestartController, *appService)
	if err != nil {
		logrus.Errorf("Application run restart controller failure:%s", err.Error())
		logger.Info("Application run restart controller failure", controller.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("Application restart failure")
	}
	logrus.Infof("service(%s) %s working is running.", body.ServiceID, "restart")
	return nil
}

func (m *Manager) horizontalScalingExec(task *model.Task) error {
	body, ok := task.Body.(model.HorizontalScalingTaskBody)
	if !ok {
		logrus.Errorf("horizontal_scaling body convert to taskbody error")
		return fmt.Errorf("a")
	}
	logger := event.GetManager().GetLogger(body.EventID)
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(body.ServiceID)
	if err != nil {
		logger.Error("Get app base info failure", controller.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		logrus.Errorf("horizontal_scaling get rc error. %v", err)
		return fmt.Errorf("a")
	}
	appService := m.store.GetAppService(body.ServiceID)
	if appService == nil || appService.IsClosed() {
		logger.Info("service is closed,no need handle", controller.GetLastLoggerOption())
		return nil
	}
	appService.Logger = logger
	appService.Replicas = service.Replicas
	err = m.controllerManager.StartController(controller.TypeScalingController, *appService)
	if err != nil {
		logrus.Errorf("Application run  scaling controller failure:%s", err.Error())
		logger.Info("Application run scaling controller failure", controller.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("Application scaling failure")
	}
	logrus.Infof("service(%s) %s working is running.", body.ServiceID, "scaling")
	return nil
}

func (m *Manager) verticalScalingExec(task *model.Task) error {
	body, ok := task.Body.(model.VerticalScalingTaskBody)
	if !ok {
		logrus.Errorf("vertical_scaling body convert to taskbody error")
		return fmt.Errorf("vertical_scaling body convert to taskbody error")
	}
	logger := event.GetManager().GetLogger(body.EventID)
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(body.ServiceID)
	if err != nil {
		logrus.Errorf("vertical_scaling get rc error. %v", err)
		logger.Error("Get app base info failure", controller.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("vertical_scaling get rc error. %v", err)
	}
	appService := m.store.GetAppService(body.ServiceID)
	if appService == nil || appService.IsClosed() {
		logger.Info("service is closed,no need handle", controller.GetLastLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return nil
	}
	appService.ContainerCPU = service.ContainerCPU
	appService.ContainerMemory = service.ContainerMemory
	appService.Logger = logger
	newAppService, err := conversion.InitAppService(m.dbmanager, body.ServiceID)
	if err != nil {
		logrus.Errorf("Application init create failure:%s", err.Error())
		logger.Error("Application init create failure", controller.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("Application init create failure")
	}
	newAppService.Logger = logger
	appService.SetUpgradePatch(newAppService)
	err = m.controllerManager.StartController(controller.TypeUpgradeController, *appService)
	if err != nil {
		logrus.Errorf("Application run  vertical scaling(upgrade) controller failure:%s", err.Error())
		logger.Info("Application run vertical scaling(upgrade) controller failure", controller.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("Application vertical scaling(upgrade) failure")
	}
	logrus.Infof("service(%s) %s working is running.", body.ServiceID, "vertical scaling")
	return nil
}

func (m *Manager) rollingUpgradeExec(task *model.Task) error {
	body, ok := task.Body.(model.RollingUpgradeTaskBody)
	if !ok {
		logrus.Error("rolling_upgrade body convert to taskbody error", task.Body)
		return fmt.Errorf("rolling_upgrade body convert to taskbody error")
	}
	logger := event.GetManager().GetLogger(body.EventID)
	newAppService, err := conversion.InitAppService(m.dbmanager, body.ServiceID)
	if err != nil {
		logrus.Errorf("Application init create failure:%s", err.Error())
		logger.Error("Application init create failure", controller.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("Application init create failure")
	}
	newAppService.Logger = logger
	oldAppService := m.store.GetAppService(body.ServiceID)
	// if service not deploy,start it
	if oldAppService == nil || oldAppService.IsClosed() {
		//regist new app service
		m.store.RegistAppService(newAppService)
		err = m.controllerManager.StartController(controller.TypeStartController, *newAppService)
		if err != nil {
			logrus.Errorf("Application run  start controller failure:%s", err.Error())
			logger.Info("Application run start controller failure", controller.GetCallbackLoggerOption())
			event.GetManager().ReleaseLogger(logger)
			return fmt.Errorf("Application start failure")
		}
		logrus.Infof("service(%s) %s working is running.", body.ServiceID, "start")
		return nil
	}
	if err := oldAppService.SetUpgradePatch(newAppService); err != nil {
		if err.Error() == "no upgrade" {
			logger.Info("Application no change no need upgrade.", controller.GetLastLoggerOption())
			return nil
		}
		logrus.Errorf("Application get upgrade info error:%s", err.Error())
		logger.Error(fmt.Sprintf("Application get upgrade info error:%s", err.Error()), controller.GetCallbackLoggerOption())
		return nil
	}
	oldAppService.Logger = logger
	oldAppService.DeployVersion = newAppService.DeployVersion
	//if service already deploy,upgrade it:
	err = m.controllerManager.StartController(controller.TypeUpgradeController, *oldAppService)
	if err != nil {
		logrus.Errorf("Application run  upgrade controller failure:%s", err.Error())
		logger.Info("Application run upgrade controller failure", controller.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("Application upgrade failure")
	}
	logrus.Infof("service(%s) %s working is running.", body.ServiceID, "upgrade")
	return nil
}

func (m *Manager) applyRuleExec(task *model.Task) error {
	body, ok := task.Body.(*model.ApplyRuleTaskBody)
	if !ok {
		logrus.Errorf("Can't convert %s to *model.ApplyRuleTaskBody", reflect.TypeOf(task.Body))
		return fmt.Errorf("Can't convert %s to *model.ApplyRuleTaskBody", reflect.TypeOf(task.Body))
	}
	logger := event.GetManager().GetLogger(body.EventID)
	oldAppService := m.store.GetAppService(body.ServiceID)
	if oldAppService == nil || oldAppService.IsClosed() {
		logrus.Debugf("service is closed,no need handle")
		logger.Info("service is closed,no need handle", controller.GetLastLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return nil
	}
	newAppService, err := conversion.InitAppService(m.dbmanager, body.ServiceID)
	if err != nil {
		logrus.Errorf("Application init create failure:%s", err.Error())
		logger.Error("Application init create failure", controller.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("Application init create failure")
	}
	newAppService.Logger = logger
	newAppService.SetDelIngsSecrets(oldAppService)
	// update k8s resources
	err = m.controllerManager.StartController(controller.TypeApplyRuleController, *newAppService)
	if err != nil {
		logrus.Errorf("Application apply rule controller failure:%s", err.Error())
		return fmt.Errorf("Application apply rule controller failure:%s", err.Error())
	}
	return nil
}
