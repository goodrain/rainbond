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

	status "github.com/goodrain/rainbond/appruntimesync/client"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/appm"
	"github.com/goodrain/rainbond/worker/discover/model"

	"github.com/jinzhu/gorm"

	"github.com/Sirupsen/logrus"
)

type startTask struct {
	modelTask   model.StartTaskBody
	taskID      string
	logger      event.Logger
	taskManager *TaskManager
	serviceType string
}

func (s *startTask) RunSuccess() {
	//设置应用状态为运行中
	s.taskManager.statusManager.SetStatus(s.modelTask.ServiceID, "running")
	s.logger.Info("应用启动任务完成", map[string]string{"step": "last", "status": "success"})
}

//RunError 如果有错误发生，回滚移除可能创建的资源
func (s *startTask) RunError(e error) {
	if e == appm.ErrTimeOut {
		//TODO:
		//应用启动超时，怎么处理？
		//是否关闭应用？
		//暂时不自动关闭
		s.logger.Error("应用启动超时，请稍等并注意应用状态", map[string]string{"step": "callback", "status": "timeout"})
		return
	}
	if e.Error() == "deploy info is exist" {
		s.taskManager.statusManager.CheckStatus(s.modelTask.ServiceID)
		return
	}
	s.logger.Error("启动应用失败,原因:"+e.Error(), map[string]string{"step": "worker-executor", "status": "error"})
	s.logger.Info("启动失败开始移除Service", map[string]string{"step": "worker-executor", "status": "starting"})
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
	s.logger.Error("启动错误，请重试", map[string]string{"step": "callback", "status": "failure"})
}

func (s *startTask) BeforeRun() {
	s.taskManager.statusManager.SetStatus(s.modelTask.ServiceID, "starting")
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

func (s *startTask) AfterRun() {
	s.logger.Info("应用启动任务工作器退出", map[string]string{"step": "worker-executor", "status": "success"})
	event.GetManager().ReleaseLogger(s.logger)
	logrus.Info("start worker run complete.")
}
func (s *startTask) Stop() {
	s.logger.Info("应用启动任务工作器退出", map[string]string{"step": "worker-executor", "status": "success"})
	//不支持
}
func (s *startTask) RollBack() {
	//不支持
}
func (s *startTask) Run() error {
	s.logger.Info("应用启动任务开始执行", map[string]string{"step": "worker-executor", "status": "starting"})
	deploys, err := db.GetManager().K8sDeployReplicationDao().GetK8sDeployReplicationByService(s.modelTask.ServiceID)
	if err != nil && err != gorm.ErrRecordNotFound {
		logrus.Error("find deploy info error from db.", err.Error())
		return err
	}
	isExist := false
	if deploys != nil {
		for _, d := range deploys {
			if !d.IsDelete {
				isExist = true
			}
		}
	}
	if isExist {
		appstatus := s.taskManager.statusManager.GetStatus(s.modelTask.ServiceID)
		if appstatus == status.CLOSED {
			//is app status is CLOSED 考虑是手动
			//db.GetManager().K8sDeployReplicationDao().DeleteK8sDeployReplicationByService(s.modelTask.ServiceID)
		} else {
			s.logger.Info("部署信息已存在，请尝试关闭应用", map[string]string{"step": "callback", "status": "failure"})
			return fmt.Errorf("deploy info is exist")
		}
	}
	switch s.serviceType {
	case dbmodel.TypeStatefulSet:
		s.logger.Info("应用部署类型为有状态应用", map[string]string{"step": "worker-executor"})
		if err := s.startStatefulSet(); err != nil {
			return err
		}
		break
	case dbmodel.TypeDeployment:
		s.logger.Info("应用部署类型为无状态应用", map[string]string{"step": "worker-executor"})
		if err := s.startDeployment(); err != nil {
			return err
		}
		break
	case dbmodel.TypeReplicationController:
		s.logger.Info("应用部署类型为无状态应用", map[string]string{"step": "worker-executor"})
		if err := s.startReplicationController(); err != nil {
			return err
		}
		break
	}
	return nil
}
func (s *startTask) startStatefulSet() error {
	s.logger.Info("有状态应用启动任务开始执行", map[string]string{"step": "worker-executor", "status": "starting"})
	_, err := s.taskManager.appm.StartStatefulSet(s.modelTask.ServiceID, s.logger)
	var timeout error
	if err != nil {
		if err != appm.ErrTimeOut {
			s.logger.Error("有状态应用启动任务执行失败。"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return err
		}
		timeout = err
	}
	//有状态服务先创建SERVICE,在StartStatefulSet方法内部
	return timeout
}

func (s *startTask) startDeployment() error {
	s.logger.Info("无状态应用启动任务开始执行", map[string]string{"step": "worker-executor", "status": "starting"})
	stateful, err := s.taskManager.appm.StartDeployment(s.modelTask.ServiceID, s.logger)
	var timeout error
	if err != nil {
		if err != appm.ErrTimeOut {
			s.logger.Error("无状态应用启动任务执行失败。"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return err
		}
		timeout = err
	}
	if stateful != nil {
		err = s.taskManager.appm.StartService(s.modelTask.ServiceID, s.logger, stateful.Name, dbmodel.TypeDeployment)
		if err != nil {
			s.logger.Error("Service创建执行失败。"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return err
		}
	}
	return timeout
}
func (s *startTask) startReplicationController() error {
	s.logger.Info("无状态应用启动任务开始执行", map[string]string{"step": "worker-executor", "status": "starting"})
	stateful, err := s.taskManager.appm.StartReplicationController(s.modelTask.ServiceID, s.logger)
	var timeout error
	if err != nil {
		//TODO:
		//如果是超时失败，应该继续创建service
		if err != appm.ErrTimeOut {
			s.logger.Error("无状态应用启动任务执行失败。"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return err
		}
		timeout = err
	}
	if stateful != nil {
		err = s.taskManager.appm.StartService(s.modelTask.ServiceID, s.logger, stateful.Name, dbmodel.TypeReplicationController)
		if err != nil {
			s.logger.Error("Service创建执行失败。"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return err
		}
	}
	return timeout
}
func (s *startTask) TaskID() string {
	return s.taskID
}
func (s *startTask) Logger() event.Logger {
	return s.logger
}
