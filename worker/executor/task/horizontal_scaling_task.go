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
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/appm"
	"github.com/goodrain/rainbond/worker/discover/model"

	"github.com/Sirupsen/logrus"
)

type horizontalScalingTask struct {
	modelTask   model.HorizontalScalingTaskBody
	taskID      string
	logger      event.Logger
	taskManager *TaskManager
	serviceType string
	oldReplicas int32
}

func (s *horizontalScalingTask) RunSuccess() {
	//设置应用状态为运行中
	s.taskManager.statusManager.SetStatus(s.modelTask.ServiceID, "running")
	s.logger.Info("应用水平伸缩任务完成", map[string]string{"step": "last", "status": "success"})
}

//RunError 如果有错误发生，回滚移除可能创建的资源
func (s *horizontalScalingTask) RunError(e error) {
	if e == appm.ErrNotDeploy {
		s.logger.Info("应用水平伸缩任务完成", map[string]string{"step": "last", "status": "success"})
		return
	}
	if e == appm.ErrTimeOut {
		//TODO:
		//应用启动超时，怎么处理？
		//是否关闭应用？
		//暂时不自动关闭
		return
	}
	//TODO:
	//是否还原到历史实例数
}

func (s *horizontalScalingTask) BeforeRun() {
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

func (s *horizontalScalingTask) AfterRun() {
	s.logger.Info("应用水平伸缩任务工作器退出", map[string]string{"step": "worker-executor", "status": "success"})
	event.GetManager().ReleaseLogger(s.logger)
	logrus.Info("horizontal scaling worker run complete.")
}
func (s *horizontalScalingTask) Stop() {
	s.logger.Info("应用水平伸缩任务工作器退出", map[string]string{"step": "worker-executor", "status": "success"})
	//不支持
}
func (s *horizontalScalingTask) RollBack() {
	//不支持
}

func (s *horizontalScalingTask) Run() error {
	s.logger.Info("应用水平伸缩任务开始执行", map[string]string{"step": "worker-executor", "status": "starting"})
	err := s.taskManager.appm.HorizontalScaling(s.modelTask.ServiceID, s.oldReplicas, s.logger)
	if err != nil {
		if err != appm.ErrNotDeploy && err.Error() != appm.ErrTimeOut.Error() {
			s.logger.Error("水平伸缩失败", map[string]string{"step": "callback", "status": "failure"})
		}
		return err
	}
	return nil
}

func (s *horizontalScalingTask) TaskID() string {
	return s.taskID
}
func (s *horizontalScalingTask) Logger() event.Logger {
	return s.logger
}
