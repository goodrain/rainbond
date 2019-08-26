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
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/worker/appm/controller"
	"github.com/goodrain/rainbond/worker/appm/conversion"
	"github.com/goodrain/rainbond/worker/appm/store"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/goodrain/rainbond/worker/discover/model"
)

//Manager manager
type Manager struct {
	ctx               context.Context
	c                 option.Config
	store             store.Storer
	dbmanager         db.Manager
	controllerManager *controller.Manager

	startCh *channels.RingChannel
}

//NewManager now handle
func NewManager(ctx context.Context,
	config option.Config,
	store store.Storer,
	controllerManager *controller.Manager,
	startCh *channels.RingChannel) *Manager {

	return &Manager{
		ctx:               ctx,
		c:                 config,
		dbmanager:         db.GetManager(),
		store:             store,
		controllerManager: controllerManager,
		startCh:           startCh,
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
	case "apply_plugin_config":
		logrus.Info("start a 'apply_plugin_config' task worker")
		return m.applyPluginConfig(task)
	default:
		logrus.Warning("task can not execute because no type is identified")
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
		logger.Info("Application is not closed, can not start", event.GetLastLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return nil
	}
	newAppService, err := conversion.InitAppService(m.dbmanager, body.ServiceID, body.Configs)
	if err != nil {
		logrus.Errorf("Application init create failure:%s", err.Error())
		logger.Error(fmt.Sprintf("应用初始化服务元数据模型失败, %s", err.Error()), event.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("Application init create failure")
	}
	newAppService.Logger = logger
	//regist new app service
	m.store.RegistAppService(newAppService)
	err = m.controllerManager.StartController(controller.TypeStartController, *newAppService)
	if err != nil {
		logrus.Errorf("Application run  start controller failure:%s", err.Error())
		logger.Error(fmt.Sprintf("应用运行启动服务控制器失败,%s", err.Error()), event.GetCallbackLoggerOption())
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
		logger.Info("Application is closed, can not stop", event.GetLastLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return nil
	}
	appService.Logger = logger
	for k, v := range body.Configs {
		appService.ExtensionSet[k] = v
	}
	err := m.controllerManager.StartController(controller.TypeStopController, *appService)
	if err != nil {
		logrus.Errorf("Application run  stop controller failure:%s", err.Error())
		logger.Info(fmt.Sprintf("应用运行停止服务控制器失败, %s", err.Error()), event.GetCallbackLoggerOption())
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
		logger.Info("Application is closed, can not stop", event.GetLastLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return nil
	}
	appService.Logger = logger
	for k, v := range body.Configs {
		appService.ExtensionSet[k] = v
	}
	//first stop app
	err := m.controllerManager.StartController(controller.TypeRestartController, *appService)
	if err != nil {
		logrus.Errorf("Application run restart controller failure:%s", err.Error())
		logger.Info(fmt.Sprintf("应用运行服务重启控制器失败, %s", err.Error()), event.GetCallbackLoggerOption())
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
		logger.Error(fmt.Sprintf("获取应用基础信息失败,%s", err.Error()), event.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		logrus.Errorf("horizontal_scaling get rc error. %v", err)
		return fmt.Errorf("a")
	}
	appService := m.store.GetAppService(body.ServiceID)
	if appService == nil || appService.IsClosed() {
		logger.Info("service is closed,no need handle", event.GetLastLoggerOption())
		return nil
	}
	appService.Logger = logger
	appService.Replicas = service.Replicas
	err = m.controllerManager.StartController(controller.TypeScalingController, *appService)
	if err != nil {
		logrus.Errorf("Application run  scaling controller failure:%s", err.Error())
		logger.Info(fmt.Sprintf("应用运行扩展控制器失败, %s", err.Error()), event.GetCallbackLoggerOption())
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
		logger.Error(fmt.Sprintf("获取应用基础信息失败,%s", err.Error()), event.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("vertical_scaling get rc error. %v", err)
	}
	appService := m.store.GetAppService(body.ServiceID)
	if appService == nil || appService.IsClosed() {
		logger.Info("service is closed,no need handle", event.GetLastLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return nil
	}
	appService.ContainerCPU = service.ContainerCPU
	appService.ContainerMemory = service.ContainerMemory
	appService.Logger = logger
	newAppService, err := conversion.InitAppService(m.dbmanager, body.ServiceID, nil)
	if err != nil {
		logrus.Errorf("Application init create failure:%s", err.Error())
		logger.Error(fmt.Sprintf("应用初始化创建失败,%s", err.Error()), event.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("Application init create failure")
	}
	newAppService.Logger = logger
	appService.SetUpgradePatch(newAppService)
	err = m.controllerManager.StartController(controller.TypeUpgradeController, *newAppService)
	if err != nil {
		logrus.Errorf("Application run  vertical scaling(upgrade) controller failure:%s", err.Error())
		logger.Info(fmt.Sprintf("应用运行垂直扩展(升级)控制器失败, %s", err.Error()), event.GetCallbackLoggerOption())
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
	newAppService, err := conversion.InitAppService(m.dbmanager, body.ServiceID, body.Configs)
	if err != nil {
		logrus.Errorf("Application init create failure:%s", err.Error())
		logger.Error(fmt.Sprintf("应用初始化创建失败, %s", err.Error()), event.GetCallbackLoggerOption())
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
			logger.Info(fmt.Sprintf("应用运行启动控制器失败,%s", err.Error()), event.GetCallbackLoggerOption())
			event.GetManager().ReleaseLogger(logger)
			return fmt.Errorf("Application start failure")
		}
		logrus.Infof("service(%s) %s working is running.", body.ServiceID, "start")
		return nil
	}
	if err := oldAppService.SetUpgradePatch(newAppService); err != nil {
		if err.Error() == "no upgrade" {
			logger.Info("Application no change no need upgrade.", event.GetLastLoggerOption())
			return nil
		}
		logrus.Errorf("Application get upgrade info error:%s", err.Error())
		logger.Error(fmt.Sprintf("应用获取升级信息失败,%s", err.Error()), event.GetCallbackLoggerOption())
		return nil
	}
	//if service already deploy,upgrade it:
	err = m.controllerManager.StartController(controller.TypeUpgradeController, *newAppService)
	if err != nil {
		logrus.Errorf("Application run  upgrade controller failure:%s", err.Error())
		logger.Info(fmt.Sprintf("应用运行升级控制器失败,%s", err.Error()), event.GetCallbackLoggerOption())
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
	svc, err := db.GetManager().TenantServiceDao().GetServiceByID(body.ServiceID)
	if err != nil {
		logrus.Errorf("error get TenantServices: %v", err)
		return fmt.Errorf("error get TenantServices: %v", err)
	}
	logger := event.GetManager().GetLogger(body.EventID)
	oldAppService := m.store.GetAppService(body.ServiceID)
	logrus.Debugf("body action: %s", body.Action)
	if svc.Kind != dbmodel.ServiceKindThirdParty.String() && !strings.HasPrefix(body.Action, "port") {
		if oldAppService == nil || oldAppService.IsClosed() {
			logrus.Debugf("service is closed, no need handle")
			logger.Info("service is closed,no need handle", event.GetLastLoggerOption())
			event.GetManager().ReleaseLogger(logger)
			return nil
		}
	}
	var newAppService *v1.AppService
	if svc.Kind == dbmodel.ServiceKindThirdParty.String() {
		newAppService, err = conversion.InitAppService(m.dbmanager, body.ServiceID, nil,
			"ServiceSource", "TenantServiceBase", "TenantServiceRegist")
	} else {
		newAppService, err = conversion.InitAppService(m.dbmanager, body.ServiceID, nil)
	}
	if err != nil {
		logrus.Errorf("Application init create failure:%s", err.Error())
		logger.Error(fmt.Sprintf("应用初始化创建失败, %s", err.Error()), event.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("Application init create failure")
	}
	newAppService.Logger = logger
	newAppService.SetDeletedResources(oldAppService)
	// update k8s resources
	err = m.controllerManager.StartController(controller.TypeApplyRuleController, *newAppService)
	if err != nil {
		logrus.Errorf("Application apply rule controller failure:%s", err.Error())
		return fmt.Errorf("Application apply rule controller failure:%s", err.Error())
	}

	if svc.Kind == dbmodel.ServiceKindThirdParty.String() && strings.HasPrefix(body.Action, "port") {
		if oldAppService == nil {
			m.store.RegistAppService(newAppService)
		}
		if body.Action == "port-open" {
			m.startCh.In() <- &v1.Event{
				Type:    v1.StartEvent,
				Sid:     body.ServiceID,
				Port:    body.Port,
				IsInner: body.IsInner,
			}
		}
		if body.Action == "port-close" {
			if !db.GetManager().TenantServicesPortDao().HasOpenPort(body.ServiceID) {
				m.startCh.In() <- &v1.Event{
					Type: v1.StopEvent,
					Sid:  body.ServiceID,
				}
			}
		}
	}

	return nil
}

//applyPluginConfig apply service plugin config
func (m *Manager) applyPluginConfig(task *model.Task) error {
	body, ok := task.Body.(*model.ApplyPluginConfigTaskBody)
	if !ok {
		logrus.Errorf("Can't convert %s to *model.ApplyPluginConfigTaskBody", reflect.TypeOf(task.Body))
		return fmt.Errorf("Can't convert %s to *model.ApplyPluginConfigTaskBody", reflect.TypeOf(task.Body))
	}
	oldAppService := m.store.GetAppService(body.ServiceID)
	if oldAppService == nil || oldAppService.IsClosed() {
		logrus.Debugf("service is closed,no need handle")
		return nil
	}
	newApp, err := conversion.InitAppService(m.dbmanager, body.ServiceID, nil, "ServiceSource", "TenantServiceBase", "TenantServicePlugin")
	if err != nil {
		logrus.Errorf("Application apply plugin config controller failure:%s", err.Error())
		return err
	}
	err = m.controllerManager.StartController(controller.TypeApplyConfigController, *newApp)
	if err != nil {
		logrus.Errorf("Application apply plugin config controller failure:%s", err.Error())
		return fmt.Errorf("Application apply plugin config controller failure:%s", err.Error())
	}
	return nil
}
