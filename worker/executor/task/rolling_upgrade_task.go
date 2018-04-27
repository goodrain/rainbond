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
	"fmt"

	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/appm"
	"github.com/goodrain/rainbond/worker/discover/model"

	status "github.com/goodrain/rainbond/appruntimesync/client"

	"github.com/Sirupsen/logrus"
	"k8s.io/client-go/pkg/api/v1"
)

type rollingUpgradeTask struct {
	modelTask   model.RollingUpgradeTaskBody
	taskID      string
	logger      event.Logger
	taskManager *TaskManager
	serviceType string
	oldVersion  string
	stopChan    chan struct{}
}

func (s *rollingUpgradeTask) RunSuccess() {
	//设置应用状态为运行中
	s.taskManager.statusManager.SetStatus(s.modelTask.ServiceID, status.RUNNING)
	s.logger.Info("应用滚动升级任务完成", map[string]string{"step": "last", "status": "success"})
}

//RunError 如果有错误发生，回滚移除可能创建的资源
func (s *rollingUpgradeTask) RunError(e error) {
	if e == appm.ErrTimeOut {
		//TODO:
		//应用启动超时，怎么处理？
		//是否关闭应用？
		//暂时不自动关闭
		s.logger.Error("应用升级或启动超时，请稍等并注意应用状态", map[string]string{"step": "callback", "status": "timeout"})
	} else {
		//TODO:
		//是否还原到原版本
		if e.Error() == "应用容器重启" {
			s.logger.Error("滚动升级失败，应用发生重启，请查询应用日志", map[string]string{"step": "callback", "status": "failure"})
		} else if e.Error() != "dont't support" {
			s.logger.Error("滚动升级失败，请重试", map[string]string{"step": "callback", "status": "failure"})
		}
	}
	s.taskManager.statusManager.CheckStatus(s.modelTask.ServiceID)
}

func (s *rollingUpgradeTask) BeforeRun() {
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
	if serviceStatus := s.taskManager.statusManager.GetStatus(s.modelTask.ServiceID); s.taskManager.statusManager.IsClosedStatus(serviceStatus) {
		s.taskManager.statusManager.SetStatus(s.modelTask.ServiceID, status.STARTING)
	} else {
		s.taskManager.statusManager.SetStatus(s.modelTask.ServiceID, status.UPGRADE)
	}
}

func (s *rollingUpgradeTask) AfterRun() {
	s.logger.Info("应用滚动升级任务工作器退出", map[string]string{"step": "worker-executor", "status": "success"})
	event.GetManager().ReleaseLogger(s.logger)
	logrus.Info("rolling upgrade worker run complete.")
}
func (s *rollingUpgradeTask) Stop() {
	s.logger.Info("应用滚动升级任务工作器退出", map[string]string{"step": "worker-executor", "status": "success"})
	//发送停止信号
	close(s.stopChan)
}

func (s *rollingUpgradeTask) RollBack() {
	//不支持
}
func (s *rollingUpgradeTask) Run() error {
	s.logger.Info("应用滚动升级任务开始执行", map[string]string{"step": "worker-executor", "status": "starting"})

	var upgradeError error
	switch s.serviceType {
	case dbmodel.TypeStatefulSet:
		_, upgradeError = s.taskManager.appm.RollingUpgradeStatefulSet(s.modelTask.ServiceID, s.logger)
		if upgradeError != nil && upgradeError.Error() != appm.ErrTimeOut.Error() {
			logrus.Error(upgradeError.Error())
			s.logger.Info("应用升级发生错误", map[string]string{"step": "worker-executor", "status": "failure"})
			return upgradeError
		}
		break
	case dbmodel.TypeDeployment:
		s.logger.Error("版本构建成功，当前类型不支持滚动升级，请手动重启", map[string]string{"step": "callback", "status": "failure"})
		return fmt.Errorf("dont't support")
	case dbmodel.TypeReplicationController:
		var rc *v1.ReplicationController
		rc, upgradeError = s.taskManager.appm.RollingUpgradeReplicationController(s.modelTask.ServiceID, s.stopChan, s.logger)
		if upgradeError != nil && upgradeError.Error() != appm.ErrTimeOut.Error() {
			logrus.Error(upgradeError.Error())
			s.logger.Info("应用滚动升级发生错误", map[string]string{"step": "worker-executor", "status": "failure"})
			return upgradeError
		}
		err := s.taskManager.appm.UpdateService(s.modelTask.ServiceID, s.logger, rc.Name, dbmodel.TypeReplicationController)
		if err != nil {
			logrus.Error(err.Error())
			s.logger.Info("应用Service升级发生错误", map[string]string{"step": "worker-executor", "status": "failure"})
			return err
		}
		break
	}
	return upgradeError
}
func (s *rollingUpgradeTask) TaskID() string {
	return s.taskID
}
func (s *rollingUpgradeTask) Logger() event.Logger {
	return s.logger
}
