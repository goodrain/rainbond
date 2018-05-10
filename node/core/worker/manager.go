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

package worker

import (
	"github.com/goodrain/rainbond/node/api/model"
	"sync"

	"golang.org/x/net/context"
)

//Worker 工作器
type Worker interface {
	Start()
	Stop() error
	Result()
}

//Manager 工作器管理
type Manager struct {
	workers map[string]Worker
	lock    sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	closed  chan struct{}
}

//NewManager 新建Manager
func NewManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := Manager{
		ctx:    ctx,
		cancel: cancel,
	}
	return &m
}

//Start 启动
func (m *Manager) Start() error {
	return nil
}

//Stop 关闭
func (m *Manager) Stop() error {
	return nil
}

//AddWorker 添加worker
func (m *Manager) AddWorker(worker Worker) error {
	return nil
}

//RemoveWorker 移除worker
func (m *Manager) RemoveWorker(worker Worker) error {
	return nil
}

//NewTaskWorker 创建worker
func (m *Manager) NewTaskWorker(task *model.Task) Worker {
	return &taskWorker{task}
}

//NewTaskGroupWorker 创建worker
func (m *Manager) NewTaskGroupWorker(taskgroup *model.TaskGroup) Worker {
	return &taskGroupWorker{taskgroup}
}
