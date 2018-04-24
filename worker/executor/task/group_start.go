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
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/worker/discover/model"
)

type groupStartTask struct {
	modelTask   model.GroupStartTaskBody
	taskID      string
	taskManager *TaskManager
	serviceType string
	isOrder     bool
}

func (s *groupStartTask) RunSuccess() {
}

//RunError 如果有错误发生，回滚移除可能创建的资源
func (s *groupStartTask) RunError(e error) {

}

func (s *groupStartTask) BeforeRun() {
	for _, str := range s.modelTask.Strategy {
		if str == "order" {
			s.isOrder = true
			break
		}
	}

}

func (s *groupStartTask) AfterRun() {

}
func (s *groupStartTask) Stop() {

}
func (s *groupStartTask) RollBack() {

}
func (s *groupStartTask) Run() error {
	return nil
}
func (s *groupStartTask) startStatefulSet(serviceID string, logger event.Logger) error {
	logger.Info("有状态应用启动任务开始执行", map[string]string{"step": "worker-executor", "status": "starting"})
	stateful, err := s.taskManager.appm.StartStatefulSet(serviceID, logger)
	if err != nil {
		logger.Error("有状态应用启动任务执行失败。"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
		return err
	}
	err = s.taskManager.appm.StartService(serviceID, logger, stateful.Name, dbmodel.TypeStatefulSet)
	if err != nil {
		logger.Error("Service创建执行失败。"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
		return err
	}
	return nil
}
func (s *groupStartTask) startDeployment(serviceID string, logger event.Logger) error {
	logger.Info("无状态应用启动任务开始执行", map[string]string{"step": "worker-executor", "status": "starting"})
	stateful, err := s.taskManager.appm.StartDeployment(serviceID, logger)
	if err != nil {
		logger.Error("无状态应用启动任务执行失败。"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
		return err
	}
	err = s.taskManager.appm.StartService(serviceID, logger, stateful.Name, dbmodel.TypeStatefulSet)
	if err != nil {
		logger.Error("Service创建执行失败。"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
		return err
	}
	return nil
}
func (s *groupStartTask) startReplicationController(serviceID string, logger event.Logger) error {
	logger.Info("无状态应用启动任务开始执行", map[string]string{"step": "worker-executor", "status": "starting"})
	stateful, err := s.taskManager.appm.StartReplicationController(serviceID, logger)
	if err != nil {
		logger.Error("无状态应用启动任务执行失败。"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
		return err
	}
	err = s.taskManager.appm.StartService(serviceID, logger, stateful.Name, dbmodel.TypeStatefulSet)
	if err != nil {
		logger.Error("Service创建执行失败。"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
		return err
	}
	return nil
}

func (s *groupStartTask) TaskID() string {
	return s.taskID
}

//TODO:
func (s *groupStartTask) Logger() event.Logger {
	return nil
}
