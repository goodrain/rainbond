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

package handle

import (
	"context"
	"time"

	status "github.com/goodrain/rainbond/appruntimesync/client"
	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/worker/discover/model"
	"github.com/goodrain/rainbond/worker/executor"

	"github.com/Sirupsen/logrus"
)

//Manager manager
type Manager struct {
	ctx           context.Context
	c             option.Config
	execManager   executor.Manager
	statusManager *status.AppRuntimeSyncClient
}

//NewManager now handle
func NewManager(ctx context.Context, config option.Config, execManager executor.Manager, statusManager *status.AppRuntimeSyncClient) *Manager {

	return &Manager{
		ctx:           ctx,
		c:             config,
		execManager:   execManager,
		statusManager: statusManager,
	}
}

func (m *Manager) checkCount() bool {
	logrus.Debugf("message nums is %v in mq, max is %v", m.execManager.WorkerCount(), m.c.MaxTasks)
	if m.execManager.WorkerCount() > m.c.MaxTasks {
		return true
	}
	return false
}

//AnalystToExec analyst exec
func (m *Manager) AnalystToExec(task *model.Task) int {
	if task == nil {
		logrus.Error("AnalystToExec receive a nil task")
		return 1
	}
	//max worker count check
	if m.checkCount() {
		return 9
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
		return 0
	}
}

/*
return 值定义
0	任务执行成功
1	任务执行失败
9	任务数量达到上限
*/

func (m *Manager) startExec(task *model.Task) int {
	body, ok := task.Body.(model.StartTaskBody)
	if !ok {
		logrus.Errorf("start body convert to taskbody error")
		return 1
	}
	logger := event.GetManager().GetLogger(body.EventID)
	curStatus := m.statusManager.GetStatus(body.ServiceID)
	if curStatus == "unknow" {
		logger.Error("应用实时状态获取失败", map[string]string{"step": "callback", "status": "failure"})
		event.GetManager().ReleaseLogger(logger)
		return 1
	}
	if curStatus != status.STOPPING && curStatus != status.CLOSED && curStatus != status.UNDEPLOY {
		logger.Info("应用状态未关闭，无需进行启动操作", map[string]string{"step": "last", "status": "success"})
		event.GetManager().ReleaseLogger(logger)
		return 0
	}
	ttask := m.execManager.TaskManager().NewStartTask(task, logger)
	err := m.execManager.AddTask(ttask)
	if err != nil {
		logrus.Errorf("start task <start> error. %v", err)
		logger.Error("启动应用任务创建失败", map[string]string{"step": "callback", "status": "failure"})
		event.GetManager().ReleaseLogger(logger)
		return 1
	}
	logger.Info("启动应用任务创建成功", map[string]string{"step": "worker-handle", "status": "success"})
	logrus.Infof("service(%s) start working is running.", body.ServiceID)
	return 0
}

func (m *Manager) stopExec(task *model.Task) int {
	body, ok := task.Body.(model.StopTaskBody)
	if !ok {
		logrus.Errorf("stop body convert to taskbody error")
		return 1
	}
	logger := event.GetManager().GetLogger(body.EventID)
	curStatus := m.statusManager.GetStatus(body.ServiceID)
	if curStatus == "unknow" {
		logger.Error("应用实时状态获取失败", map[string]string{"step": "callback", "status": "failure"})
		event.GetManager().ReleaseLogger(logger)
		return 1
	}
	if curStatus == status.STOPPING {
		logger.Info("应用正在关闭中，请勿重复操作", map[string]string{"step": "last", "status": "success"})
		event.GetManager().ReleaseLogger(logger)
		return 0
	}
	if curStatus == status.CLOSED {
		logger.Info("应用已关闭，请勿重复操作", map[string]string{"step": "last", "status": "success"})
		db.GetManager().K8sDeployReplicationDao().DeleteK8sDeployReplicationByService(body.ServiceID)
		event.GetManager().ReleaseLogger(logger)
		return 0
	}
	if curStatus == status.UNDEPLOY {
		logger.Info("应用未部署，无需关闭", map[string]string{"step": "last", "status": "success"})
		event.GetManager().ReleaseLogger(logger)
		return 0
	}

	ttask := m.execManager.TaskManager().NewStopTask(task, event.GetManager().GetLogger(body.EventID))
	err := m.execManager.AddTask(ttask)
	if err != nil {
		logrus.Errorf("start task <stop> error. %v", err)
		logger.Error("关闭应用任务创建失败", map[string]string{"step": "callback", "status": "failure"})
		event.GetManager().ReleaseLogger(logger)
		return 1
	}
	logger.Info("关闭应用任务创建成功", map[string]string{"step": "worker-handle", "status": "success"})
	return 0
}

func (m *Manager) restartExec(task *model.Task) int {
	body, ok := task.Body.(model.RestartTaskBody)
	if !ok {
		logrus.Errorf("restart body convert to taskbody error")
		return 1
	}
	logger := event.GetManager().GetLogger(body.EventID)
	curStatus := m.statusManager.GetStatus(body.ServiceID)
	if curStatus == "unknow" {
		logger.Error("应用实时状态获取失败，稍后操作", map[string]string{"step": "callback", "status": "failure"})
		event.GetManager().ReleaseLogger(logger)
		return 1
	}
	if curStatus == status.STOPPING {
		logger.Info("应用正在关闭中，请勿重复操作", map[string]string{"step": "last", "status": "success"})
		event.GetManager().ReleaseLogger(logger)
		return 0
	}
	if curStatus == status.CLOSED {
		logger.Info("应用已关闭，请直接启动应用", map[string]string{"step": "last", "status": "success"})
		event.GetManager().ReleaseLogger(logger)
		return 0
	}
	if curStatus == status.UNDEPLOY {
		logger.Info("应用未部署，无法进行重启", map[string]string{"step": "last", "status": "success"})
		event.GetManager().ReleaseLogger(logger)
		return 0
	}
	ttask := m.execManager.TaskManager().NewRestartTask(task, event.GetManager().GetLogger(body.EventID))
	err := m.execManager.AddTask(ttask)
	if err != nil {
		logrus.Errorf("start task <restart> error. %v", err)
		logger.Error("重启应用任务创建失败", map[string]string{"step": "callback", "status": "failure"})
		event.GetManager().ReleaseLogger(logger)
		return 1
	}
	logger.Info("重启应用任务创建成功", map[string]string{"step": "worker-handle", "status": "success"})
	return 0
}

func (m *Manager) horizontalScalingExec(task *model.Task) int {
	body, ok := task.Body.(model.HorizontalScalingTaskBody)
	if !ok {
		logrus.Errorf("horizontal_scaling body convert to taskbody error")
		return 1
	}
	logger := event.GetManager().GetLogger(body.EventID)
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(body.ServiceID)
	if err != nil {
		logger.Error("获取应用信息失败", map[string]string{"step": "callback", "status": "failure"})
		event.GetManager().ReleaseLogger(logger)
		logrus.Errorf("horizontal_scaling get rc error. %v", err)
		return 1
	}
	oldReplicas := int32(service.Replicas)
	//newReplicas 超过3w时存储问题
	service.Replicas = int(body.Replicas)
	err = db.GetManager().TenantServiceDao().UpdateModel(service)
	if err != nil {
		logrus.Errorf("horizontal_scaling set new replicas error. %v", err)
		logger.Error("更新应用信息失败", map[string]string{"step": "callback", "status": "failure"})
		event.GetManager().ReleaseLogger(logger)
		return 1
	}
	ttask := m.execManager.TaskManager().NewHorizontalScalingTask(
		task,
		oldReplicas,
		event.GetManager().GetLogger(body.EventID),
	)
	logger.Info("水平伸缩元数据设置成功", map[string]string{"step": "worker-handle", "status": "success"})
	err = m.execManager.AddTask(ttask)
	if err != nil {
		logrus.Errorf("start task <horizontal_scaling> error. %v", err)
		logger.Error("水平伸缩任务创建失败", map[string]string{"step": "callback", "status": "failure"})
		event.GetManager().ReleaseLogger(logger)
		return 1
	}
	logger.Info("水平伸缩任务创建成功", map[string]string{"step": "worker-handle", "status": "success"})
	return 0
}

func (m *Manager) verticalScalingExec(task *model.Task) int {
	body, ok := task.Body.(model.VerticalScalingTaskBody)
	if !ok {
		logrus.Errorf("vertical_scaling body convert to taskbody error")
		return 1
	}
	logger := event.GetManager().GetLogger(body.EventID)
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(body.ServiceID)
	if err != nil {
		logrus.Errorf("vertical_scaling get rc error. %v", err)
		logger.Error("获取应用信息失败", map[string]string{"step": "callback", "status": "failure"})
		event.GetManager().ReleaseLogger(logger)
		return 1
	}
	service.ContainerCPU = int(body.ContainerCPU)
	service.ContainerMemory = int(body.ContainerMemory)
	err = db.GetManager().TenantServiceDao().UpdateModel(service)
	if err != nil {
		logrus.Errorf("vertical_scaling set new cpu&memory error. %v", err)
		logger.Error("更新应用信息失败", map[string]string{"step": "callback", "status": "failure"})
		event.GetManager().ReleaseLogger(logger)
		return 1
	}
	curStatus := m.statusManager.GetStatus(body.ServiceID)
	if m.statusManager.IsClosedStatus(curStatus) {
		logger.Error("应用未部署，垂直升级成功", map[string]string{"step": "last", "status": "success"})
		return 0
	}
	ttask := m.execManager.TaskManager().NewRestartTask(
		&model.Task{
			Type: "restart",
			Body: model.RestartTaskBody{
				TenantID:      body.TenantID,
				ServiceID:     body.ServiceID,
				DeployVersion: service.DeployVersion,
				EventID:       body.EventID,
			},
			CreateTime: time.Now(),
			User:       task.User,
		},
		event.GetManager().GetLogger(body.EventID),
	)
	err = m.execManager.AddTask(ttask)
	if err != nil {
		logrus.Errorf("start task <vertical_scaling> error. %v", err)
		logger.Error("垂直伸缩重启任务创建失败", map[string]string{"step": "callback", "status": "failure"})
		event.GetManager().ReleaseLogger(logger)
		return 1
	}
	logger.Info("垂直伸缩重启任务创建成功", map[string]string{"step": "worker-handle", "status": "success"})
	return 0
}

func (m *Manager) rollingUpgradeExec(task *model.Task) int {
	body, ok := task.Body.(model.RollingUpgradeTaskBody)
	if !ok {
		logrus.Error("rolling_upgrade body convert to taskbody error", task.Body)
		return 1
	}
	logger := event.GetManager().GetLogger(body.EventID)
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(body.ServiceID)
	if err != nil {
		logrus.Errorf("rolling_upgrade get rc error. %v", err)
		logger.Error("获取应用信息失败", map[string]string{"step": "callback", "status": "failure"})
		event.GetManager().ReleaseLogger(logger)
		return 1
	}
	if service.DeployVersion == body.NewDeployVersion {
		logger.Error("应用版本无变化，无需升级", map[string]string{"step": "last", "status": "success"})
		event.GetManager().ReleaseLogger(logger)
		return 0
	}
	service.DeployVersion = body.NewDeployVersion
	err = db.GetManager().TenantServiceDao().UpdateModel(service)
	if err != nil {
		logrus.Errorf("rolling_upgrade set new deploy version error. %v", err)
		logger.Error("更新应用信息失败", map[string]string{"step": "callback", "status": "failure"})
		event.GetManager().ReleaseLogger(logger)
		return 1
	}

	t := m.execManager.TaskManager().NewRollingUpgradeTask(
		task,
		event.GetManager().GetLogger(body.EventID),
	)
	err = m.execManager.AddTask(t)
	if err != nil {
		logrus.Errorf("start task <rolling_upgrade> error. %v", err)
		logger.Error("滚动升级任务创建失败", map[string]string{"step": "callback", "status": "failure"})
		event.GetManager().ReleaseLogger(logger)
		return 1
	}
	logger.Info("应用滚动升级任务创建成功", map[string]string{"step": "worker-handle", "status": "success"})
	return 0
}
