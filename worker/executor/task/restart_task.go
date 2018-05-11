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

package task

import (
	status "github.com/goodrain/rainbond/appruntimesync/client"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/appm"
	"github.com/goodrain/rainbond/worker/discover/model"

	"github.com/Sirupsen/logrus"
)

type restartTask struct {
	modelTask   model.RestartTaskBody
	taskID      string
	logger      event.Logger
	taskManager *TaskManager
	serviceType string
	stopChan    chan struct{}
}

func (s *restartTask) RunSuccess() {
	//设置应用状态为运行中
	s.taskManager.statusManager.SetStatus(s.modelTask.ServiceID, "running")
	s.logger.Info("应用重新启动任务完成", map[string]string{"step": "last", "status": "success"})
}

//RunError 如果有错误发生，回滚移除可能创建的资源
func (s *restartTask) RunError(e error) {
	if e == appm.ErrTimeOut {
		//TODO:
		//应用启动超时，怎么处理？
		//是否关闭应用？
		//暂时不自动关闭
		s.logger.Error("应用重启超时，请稍等并注意应用状态", map[string]string{"step": "callback", "status": "timeout"})
		return
	}
	s.logger.Info("开始移除Service", map[string]string{"step": "worker-executor", "status": "starting"})
	err := s.taskManager.appm.StopService(s.modelTask.ServiceID, s.logger)
	if err != nil {
		s.logger.Error("移除Service发生错误"+err.Error(), map[string]string{"step": "worker-executor", "status": "failure"})
	} else {
		s.logger.Info("移除Service完成", map[string]string{"step": "worker-executor", "status": "success"})
	}
	switch s.serviceType {
	case dbmodel.TypeStatefulSet:
		s.logger.Info("开始移除StatefulSet", map[string]string{"step": "worker-executor", "status": "starting"})
		err = s.taskManager.appm.StopStatefulSet(s.modelTask.ServiceID, s.logger)
		if err != nil {
			s.logger.Info("移除StatefulSet失败"+err.Error(), map[string]string{"step": "worker-executor", "status": "failure"})
		} else {
			s.logger.Error("移除StatefulSet完成", map[string]string{"step": "worker-executor", "status": "success"})
		}
		break
	case dbmodel.TypeDeployment:
		s.logger.Info("开始移除Deployment", map[string]string{"step": "worker-executor", "status": "starting"})
		err = s.taskManager.appm.StopDeployment(s.modelTask.ServiceID, s.logger)
		if err != nil {
			s.logger.Info("移除Deployment失败"+err.Error(), map[string]string{"step": "worker-executor", "status": "failure"})
		} else {
			s.logger.Error("移除Deployment完成", map[string]string{"step": "worker-executor", "status": "success"})
		}
		break
	case dbmodel.TypeReplicationController:
		s.logger.Info("开始移除ReplicationController", map[string]string{"step": "worker-executor", "status": "starting"})
		err = s.taskManager.appm.StopReplicationController(s.modelTask.ServiceID, s.logger)
		if err != nil {
			s.logger.Error("移除ReplicationController失败"+err.Error(), map[string]string{"step": "worker-executor", "status": "failure"})
		} else {
			s.logger.Info("移除ReplicationController完成", map[string]string{"step": "worker-executor", "status": "success"})
		}
		break
	}
	//设置应用状态为已关闭
	s.taskManager.statusManager.SetStatus(s.modelTask.ServiceID, "closed")
	s.logger.Error("重启失败，请重试", map[string]string{"step": "callback", "status": "failure"})
}

func (s *restartTask) BeforeRun() {
	s.taskManager.statusManager.SetStatus(s.modelTask.ServiceID, status.UPGRADE)
	label, err := db.GetManager().TenantServiceLabelDao().GetTenantServiceTypeLabel(s.modelTask.ServiceID)
	if err == nil && label != nil {
		if label.LabelValue == util.StatefulServiceType {
			s.serviceType = dbmodel.TypeStatefulSet
		}
		if label.LabelValue == util.StatelessServiceType {
			//s.serviceType = dbmodel.TypeDeployment
			s.serviceType = dbmodel.TypeReplicationController
		}
	}
}

func (s *restartTask) AfterRun() {
	s.logger.Info("应用重新启动任务工作器退出", map[string]string{"step": "worker-executor", "status": "success"})
	event.GetManager().ReleaseLogger(s.logger)
	logrus.Info("restart worker run complete.")
}
func (s *restartTask) Stop() {
	s.logger.Info("应用重新启动任务工作器退出", map[string]string{"step": "worker-executor", "status": "success"})
	//不支持
}
func (s *restartTask) RollBack() {
	//不支持
}
func (s *restartTask) Run() error {
	s.logger.Info("应用重新启动任务开始执行", map[string]string{"step": "worker-executor", "status": "starting"})
	switch s.serviceType {
	case dbmodel.TypeStatefulSet:
		s.logger.Info("开始移除Service", map[string]string{"step": "worker-executor", "status": "starting"})
		err := s.taskManager.appm.StopService(s.modelTask.ServiceID, s.logger)
		if err != nil {
			s.logger.Error("移除Service发生错误"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return err
		}
		s.logger.Info("移除Service完成", map[string]string{"step": "worker-executor", "status": "success"})

		s.logger.Info("开始移除StatefulSet", map[string]string{"step": "worker-executor", "status": "starting"})
		err = s.taskManager.appm.StopStatefulSet(s.modelTask.ServiceID, s.logger)
		if err != nil && err.Error() != "time out" {
			s.logger.Info("移除StatefulSet失败"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return err
		}
		s.logger.Error("移除StatefulSet完成", map[string]string{"step": "worker-executor", "status": "success"})
		//step 2 启动应用
		s.logger.Info("应用部署类型为有状态应用", map[string]string{"step": "worker-executor"})
		//设置应用状态为启动中
		s.taskManager.statusManager.SetStatus(s.modelTask.ServiceID, status.STARTING)
		if err := s.startStatefulSet(); err != nil {
			return err
		}
		break
	case dbmodel.TypeDeployment:
		s.logger.Info("开始移除Service", map[string]string{"step": "worker-executor", "status": "starting"})
		err := s.taskManager.appm.StopService(s.modelTask.ServiceID, s.logger)
		if err != nil {
			s.logger.Error("移除Service发生错误"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return err
		}
		s.logger.Info("移除Service完成", map[string]string{"step": "worker-executor", "status": "success"})

		s.logger.Info("开始移除Deployment", map[string]string{"step": "worker-executor", "status": "starting"})
		err = s.taskManager.appm.StopDeployment(s.modelTask.ServiceID, s.logger)
		if err != nil && err.Error() != "time out" {
			s.logger.Info("移除Deployment失败"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return err
		}
		s.logger.Error("移除Deployment完成", map[string]string{"step": "worker-executor", "status": "success"})
		s.logger.Info("应用部署类型为无状态应用", map[string]string{"step": "worker-executor"})
		s.taskManager.statusManager.SetStatus(s.modelTask.ServiceID, status.STARTING)
		if err := s.startDeployment(); err != nil {
			return err
		}
		break
	case dbmodel.TypeReplicationController:
		s.logger.Info("开始重启ReplicationController", map[string]string{"step": "worker-executor", "status": "starting"})
		rc, upgradeError := s.taskManager.appm.RollingUpgradeReplicationControllerCompatible(s.modelTask.ServiceID, s.stopChan, s.logger)
		if upgradeError != nil && upgradeError.Error() != appm.ErrTimeOut.Error() {
			logrus.Error(upgradeError.Error())
			s.logger.Info("应用重启升级发生错误", map[string]string{"step": "worker-executor", "status": "failure"})
			return upgradeError
		}
		err := s.taskManager.appm.UpdateService(s.modelTask.ServiceID, s.logger, rc.Name, dbmodel.TypeReplicationController)
		if err != nil {
			logrus.Error(err.Error())
			s.logger.Info("应用Service重启发生错误", map[string]string{"step": "worker-executor", "status": "failure"})
			return err
		}

		break
	}
	return nil
}
func (s *restartTask) startStatefulSet() error {
	s.logger.Info("有状态应用启动任务开始执行", map[string]string{"step": "worker-executor", "status": "starting"})
	_, err := s.taskManager.appm.StartStatefulSet(s.modelTask.ServiceID, s.logger)
	if err != nil && err.Error() != "time out" {
		s.logger.Error("有状态应用启动任务执行失败。"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
		return err
	}
	//有状态服务Service创建改在StartStatefulSet内部完成
	return nil
}
func (s *restartTask) startDeployment() error {
	s.logger.Info("无状态应用启动任务开始执行", map[string]string{"step": "worker-executor", "status": "starting"})
	stateful, err := s.taskManager.appm.StartDeployment(s.modelTask.ServiceID, s.logger)
	if err != nil && err.Error() != "time out" {
		s.logger.Error("无状态应用启动任务执行失败。"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
		return err
	}
	err = s.taskManager.appm.StartService(s.modelTask.ServiceID, s.logger, stateful.Name, dbmodel.TypeDeployment)
	if err != nil {
		s.logger.Error("Service创建执行失败。"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
		return err
	}
	return nil
}
func (s *restartTask) startReplicationController() error {
	s.logger.Info("无状态应用启动任务开始执行", map[string]string{"step": "worker-executor", "status": "starting"})
	stateful, err := s.taskManager.appm.StartReplicationController(s.modelTask.ServiceID, s.logger)
	if err != nil && err.Error() != "time out" {
		s.logger.Error("无状态应用启动任务执行失败。"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
		return err
	}
	err = s.taskManager.appm.StartService(s.modelTask.ServiceID, s.logger, stateful.Name, dbmodel.TypeReplicationController)
	if err != nil {
		s.logger.Error("Service创建执行失败。"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
		return err
	}
	return nil
}

func (s *restartTask) TaskID() string {
	return s.taskID
}
func (s *restartTask) Logger() event.Logger {
	return s.logger
}
