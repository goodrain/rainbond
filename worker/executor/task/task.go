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
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/appm"
	"github.com/goodrain/rainbond/worker/discover/model"
)

//Task 任务接口
type Task interface {
	RunSuccess()
	RunError(err error)
	BeforeRun()
	AfterRun()
	Stop()
	RollBack()
	Run() error
	TaskID() string
	Logger() event.Logger
}
type TaskManager struct {
	appm          appm.Manager
	statusManager *status.AppRuntimeSyncClient
}

func NewTaskManager(appm appm.Manager, statusManager *status.AppRuntimeSyncClient) *TaskManager {
	return &TaskManager{
		appm:          appm,
		statusManager: statusManager,
	}
}

//NewStartTask 启动应用任务
func (t *TaskManager) NewStartTask(modelTask *model.Task, logger event.Logger) Task {
	return &startTask{
		modelTask:   modelTask.Body.(model.StartTaskBody),
		logger:      logger,
		taskID:      util.NewUUID(),
		taskManager: t,
		serviceType: dbmodel.TypeReplicationController,
	}
}

//NewStopTask 停止应用任务
func (t *TaskManager) NewStopTask(modelTask *model.Task, logger event.Logger) Task {
	return &stopTask{
		modelTask:   modelTask.Body.(model.StopTaskBody),
		logger:      logger,
		taskID:      util.NewUUID(),
		taskManager: t,
		serviceType: dbmodel.TypeReplicationController,
	}
}

//NewRestartTask 重启应用任务
func (t *TaskManager) NewRestartTask(modelTask *model.Task, logger event.Logger) Task {
	return &restartTask{
		modelTask:   modelTask.Body.(model.RestartTaskBody),
		logger:      logger,
		taskID:      util.NewUUID(),
		taskManager: t,
		serviceType: dbmodel.TypeReplicationController,
	}
}

//NewHorizontalScalingTask 应用水平伸缩
func (t *TaskManager) NewHorizontalScalingTask(modelTask *model.Task, oldReplicas int32, logger event.Logger) Task {
	return &horizontalScalingTask{
		modelTask:   modelTask.Body.(model.HorizontalScalingTaskBody),
		logger:      logger,
		taskID:      util.NewUUID(),
		taskManager: t,
		serviceType: dbmodel.TypeReplicationController,
		oldReplicas: oldReplicas,
	}
}

//TODO:
//NewRollingUpgradeTask 滚动升级
func (t *TaskManager) NewRollingUpgradeTask(modelTask *model.Task, logger event.Logger) Task {
	//滚动升级task维护事务性
	return &rollingUpgradeTask{
		modelTask:   modelTask.Body.(model.RollingUpgradeTaskBody),
		logger:      logger,
		taskID:      util.NewUUID(),
		taskManager: t,
		serviceType: dbmodel.TypeReplicationController,
		oldVersion:  modelTask.Body.(model.RollingUpgradeTaskBody).CurrentDeployVersion,
	}
}
