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

	"github.com/goodrain/rainbond/worker/appm/controller"

	"github.com/goodrain/rainbond/worker/appm/conversion"

	"github.com/goodrain/rainbond/worker/appm/store"

	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/worker/discover/model"

	"github.com/Sirupsen/logrus"
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
	appService := m.store.GetAppServiceWithoutCreaterID(body.ServiceID, body.DeployVersion)
	if appService != nil && appService.GetDeployStatus() {
		logger.Info("Application is not closed, can not start", map[string]string{"step": "last", "status": "success"})
		event.GetManager().ReleaseLogger(logger)
		return nil
	}
	newAppService, err := conversion.InitAppService(m.dbmanager, body.ServiceID)
	if err != nil {
		logrus.Errorf("Application init create failure:%s", err.Error())
		logger.Info("Application init create failure", map[string]string{"step": "callback", "status": "faliure"})
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("Application init create failure")
	}
	newAppService.Logger = logger
	//regist new app service
	m.store.RegistAppService(newAppService)
	err = m.controllerManager.StartController(controller.TypeStartController, *newAppService)
	if err != nil {
		logrus.Errorf("Application run  start controller failure:%s", err.Error())
		logger.Info("Application run start controller failure", map[string]string{"step": "callback", "status": "faliure"})
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("Application start failure")
	}
	logrus.Infof("service(%s) start working is running.", body.ServiceID)
	return nil
}

func (m *Manager) stopExec(task *model.Task) error {
	body, ok := task.Body.(model.StopTaskBody)
	if !ok {
		logrus.Errorf("stop body convert to taskbody error")
		return fmt.Errorf("stop body convert to taskbody error")
	}
	logger := event.GetManager().GetLogger(body.EventID)
	appService := m.store.GetAppServices(body.ServiceID)
	if appService == nil {
		logger.Info("Application is closed, can not stop", map[string]string{"step": "last", "status": "success"})
		event.GetManager().ReleaseLogger(logger)
		return nil
	}
	for _, app := range appService {
		app.Logger = logger
		err := m.controllerManager.StartController(controller.TypeStopController, *app)
		if err != nil {
			logrus.Errorf("Application run  stop controller failure:%s", err.Error())
			logger.Info("Application run stop controller failure", map[string]string{"step": "callback", "status": "faliure"})
			event.GetManager().ReleaseLogger(logger)
			return fmt.Errorf("Application stop failure")
		}
	}
	logrus.Infof("service(%s) stop working is running.", body.ServiceID)
	return nil
}

func (m *Manager) restartExec(task *model.Task) error {
	body, ok := task.Body.(model.RestartTaskBody)
	if !ok {
		logrus.Errorf("stop body convert to taskbody error")
		return fmt.Errorf("stop body convert to taskbody error")
	}
	logger := event.GetManager().GetLogger(body.EventID)
	appService := m.store.GetAppServiceWithoutCreaterID(body.ServiceID, body.DeployVersion)
	if appService == nil {
		logger.Info("Application is closed, can not stop", map[string]string{"step": "last", "status": "success"})
		event.GetManager().ReleaseLogger(logger)
		return nil
	}
	err := m.controllerManager.StartController(controller.TypeStopController, *appService)
	if err != nil {
		logrus.Errorf("Application run  stop controller failure:%s", err.Error())
		logger.Info("Application run stop controller failure", map[string]string{"step": "callback", "status": "faliure"})
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("Application stop failure")
	}
	logrus.Infof("service(%s) stop working is running.", body.ServiceID)
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
		logger.Error("获取应用信息失败", map[string]string{"step": "callback", "status": "failure"})
		event.GetManager().ReleaseLogger(logger)
		logrus.Errorf("horizontal_scaling get rc error. %v", err)
		return fmt.Errorf("a")
	}
	oldReplicas := int32(service.Replicas)
	//newReplicas 超过3w时存储问题
	service.Replicas = int(body.Replicas)
	err = db.GetManager().TenantServiceDao().UpdateModel(service)
	if err != nil {
		logrus.Errorf("horizontal_scaling set new replicas error. %v", err)
		logger.Error("更新应用信息失败", map[string]string{"step": "callback", "status": "failure"})
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("a")
	}
	fmt.Println(oldReplicas)
	logger.Info("水平伸缩任务创建成功", map[string]string{"step": "worker-handle", "status": "success"})
	return nil
}

func (m *Manager) verticalScalingExec(task *model.Task) error {
	body, ok := task.Body.(model.VerticalScalingTaskBody)
	if !ok {
		logrus.Errorf("vertical_scaling body convert to taskbody error")
		return fmt.Errorf("a")
	}
	logger := event.GetManager().GetLogger(body.EventID)
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(body.ServiceID)
	if err != nil {
		logrus.Errorf("vertical_scaling get rc error. %v", err)
		logger.Error("获取应用信息失败", map[string]string{"step": "callback", "status": "failure"})
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("a")
	}
	service.ContainerCPU = int(body.ContainerCPU)
	service.ContainerMemory = int(body.ContainerMemory)
	err = db.GetManager().TenantServiceDao().UpdateModel(service)
	if err != nil {
		logrus.Errorf("vertical_scaling set new cpu&memory error. %v", err)
		logger.Error("更新应用信息失败", map[string]string{"step": "callback", "status": "failure"})
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("a")
	}
	logger.Info("垂直伸缩重启任务创建成功", map[string]string{"step": "worker-handle", "status": "success"})
	return nil
}

func (m *Manager) rollingUpgradeExec(task *model.Task) error {
	_, ok := task.Body.(model.RollingUpgradeTaskBody)
	if !ok {
		logrus.Error("rolling_upgrade body convert to taskbody error", task.Body)
		return fmt.Errorf("a")
	}
	return nil
}
