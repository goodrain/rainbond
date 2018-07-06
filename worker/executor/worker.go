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

package executor

import (
	"github.com/goodrain/rainbond/worker/executor/task"
	"fmt"
	"runtime/debug"
	"sync"
)

//Worker 工作器
type Worker interface {
	Start(wg sync.WaitGroup)
	Cancel() error
	RollBack() error
	Status() string
}

type woker struct {
	status  string
	task    task.Task
	manager *manager
}

//newWorker new worker
func newWorker(manager *manager, task task.Task) Worker {
	w := &woker{
		status:  "create",
		task:    task,
		manager: manager,
	}
	return w
}

func (w *woker) Start(wg sync.WaitGroup) {
	defer wg.Done()
	defer func() {
		if err := recover(); err != nil {
			w.task.RunError(fmt.Errorf("worker recover %v", err))
			debug.PrintStack()
		}
	}()
	w.task.BeforeRun()
	defer func() {
		if w.task != nil {
			w.task.AfterRun()
		}
		if w.manager != nil {
			//移除worker
			w.manager.removeWorker(w.task.TaskID(), w.task.Logger().Event())
		}
	}()
	w.status = "running"
	if err := w.task.Run(); err != nil {
		w.status = "error"
		w.task.RunError(err)
	} else {
		w.status = "success"
		w.task.RunSuccess()
	}
}

//Cancel 任务取消
func (w *woker) Cancel() error {
	w.task.Stop()
	return nil
}

//RollBack 任务回滚
//多阻塞步骤，长时间任务支持回滚，例如大量实例应用的升级操作
//回滚粒度为一个实例的操作
func (w *woker) RollBack() error {
	w.task.RollBack()
	return nil
}

func (w *woker) Status() string {
	return w.status
}
