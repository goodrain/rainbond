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

type stopTask struct {
	modelTask   model.StopTaskBody
	taskID      string
	logger      event.Logger
	taskManager *TaskManager
	serviceType string
}

func (s *stopTask) RunSuccess() {
	//设置应用状态为已关闭
	s.taskManager.statusManager.SetStatus(s.modelTask.ServiceID, status.CLOSED)
	s.logger.Info("应用关闭任务完成", map[string]string{"step": "last", "status": "success"})
}

//RunError 如果有错误发生，回滚移除可能创建的资源
func (s *stopTask) RunError(e error) {
	if e == appm.ErrTimeOut {
		//TODO:
		//应用关闭超时，怎么处理？
		//超时说明部署信息已删除，设置应用状态为已关闭
		s.taskManager.statusManager.SetStatus(s.modelTask.ServiceID, status.CLOSED)
		return
	}
	// if e == appm.ErrNotDeploy {
	// 	return
	// }
	//处理应用状态，如果是调用k8s api出错，应用并未删除
	//如果不处理应用状态，应用可能一直保持关闭中状态
	//主动监测应用状态
	s.taskManager.statusManager.CheckStatus(s.modelTask.ServiceID)
}

func (s *stopTask) BeforeRun() {
	s.taskManager.statusManager.SetStatus(s.modelTask.ServiceID, status.STOPPING)
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

func (s *stopTask) AfterRun() {
	s.logger.Info("应用关闭任务工作器退出", map[string]string{"step": "worker-executor", "status": "success"})
	event.GetManager().ReleaseLogger(s.logger)
	logrus.Info("stop worker run complete.")
}

func (s *stopTask) Stop() {
	s.logger.Info("应用关闭任务工作器退出", map[string]string{"step": "worker-executor", "status": "success"})
	//不支持
}

func (s *stopTask) RollBack() {
	//不支持
}

func (s *stopTask) Run() error {
	s.logger.Info("应用关闭任务开始执行", map[string]string{"step": "worker-executor", "status": "starting"})
	s.logger.Info("开始移除Service", map[string]string{"step": "worker-executor", "status": "starting"})
	err := s.taskManager.appm.StopService(s.modelTask.ServiceID, s.logger)
	if err != nil {
		s.logger.Error("移除Service发生错误"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
		return err
	}
	s.logger.Info("移除Service完成", map[string]string{"step": "worker-executor", "status": "success"})
	switch s.serviceType {
	case dbmodel.TypeStatefulSet:
		s.logger.Info("开始移除StatefulSet", map[string]string{"step": "worker-executor", "status": "starting"})
		err = s.taskManager.appm.StopStatefulSet(s.modelTask.ServiceID, s.logger)
		if err != nil && err.Error() != appm.ErrNotDeploy.Error() {
			s.logger.Info("移除StatefulSet失败"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return err
		}
		s.logger.Error("移除StatefulSet完成", map[string]string{"step": "worker-executor", "status": "success"})
		break
	case dbmodel.TypeDeployment:
		s.logger.Info("开始移除Deployment", map[string]string{"step": "worker-executor", "status": "starting"})
		err = s.taskManager.appm.StopDeployment(s.modelTask.ServiceID, s.logger)
		if err != nil && err.Error() != appm.ErrNotDeploy.Error() {
			s.logger.Info("移除Deployment失败"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return err
		}
		s.logger.Error("移除Deployment完成", map[string]string{"step": "worker-executor", "status": "success"})
		break
	case dbmodel.TypeReplicationController:
		s.logger.Info("开始移除ReplicationController", map[string]string{"step": "worker-executor", "status": "starting"})
		err = s.taskManager.appm.StopReplicationController(s.modelTask.ServiceID, s.logger)
		if err != nil && err.Error() != appm.ErrNotDeploy.Error() {
			s.logger.Info("移除ReplicationController失败"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return err
		}
		s.logger.Info("移除ReplicationController完成", map[string]string{"step": "worker-executor", "status": "success"})
		break
	}
	return nil
}
func (s *stopTask) TaskID() string {
	return s.taskID
}
func (s *stopTask) Logger() event.Logger {
	return s.logger
}
