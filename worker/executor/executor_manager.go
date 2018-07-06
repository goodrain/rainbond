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
	"fmt"
	"sync"

	"github.com/goodrain/rainbond/cmd/worker/option"
	status "github.com/goodrain/rainbond/appruntimesync/client"
	"github.com/goodrain/rainbond/worker/appm"
	"github.com/goodrain/rainbond/worker/executor/task"

	"github.com/Sirupsen/logrus"
)

//Manager Manager
type Manager interface {
	AddTask(task task.Task) error
	RemoveTask(task task.Task)
	// Start starts the Manager sync loops.
	Start()
	Stop()
	TaskManager() *task.TaskManager
	GetWorker(taskID string, eventID string) (Worker, bool)
	WorkerCount() int
}

type manager struct {
	// Map of active workers for probes
	workers map[workerKey]Worker
	// Lock for accessing & mutating workers
	workerLock    sync.RWMutex
	taskManager   *task.TaskManager
	statusManager *status.AppRuntimeSyncClient
	wg            sync.WaitGroup
}

//NewManager newManager
func NewManager(conf option.Config, statusManager *status.AppRuntimeSyncClient, appmm appm.Manager) (Manager, error) {
	return &manager{
		workers:       make(map[workerKey]Worker),
		taskManager:   task.NewTaskManager(appmm, statusManager),
		statusManager: statusManager,
	}, nil
}

func (m *manager) Start() {

}

func (m *manager) Stop() {
	m.workerLock.Lock()
	defer m.workerLock.Unlock()
	for _, w := range m.workers {
		w.Cancel()
	}
	logrus.Info("Waiting for all threads to complete.")
	m.wg.Wait()
	logrus.Info("All threads is exited.")
}

// Key uniquely identifying container probes
type workerKey struct {
	taskUID string
	eventID string
}

func (m *manager) TaskManager() *task.TaskManager {
	return m.taskManager
}
func (m *manager) AddTask(t task.Task) error {
	m.workerLock.Lock()
	defer m.workerLock.Unlock()
	if t == nil {
		return fmt.Errorf("task manager receive a nil task")
	}
	if _, ok := m.workers[workerKey{t.TaskID(), t.Logger().Event()}]; ok {
		logrus.Errorf("worker %s:%s is exist.", t.TaskID(), t.Logger().Event())
		return fmt.Errorf("worker %s:%s is exist ", t.TaskID(), t.Logger().Event())
	}
	worker := newWorker(m, t)
	m.wg.Add(1)
	go worker.Start(m.wg)
	m.workers[workerKey{t.TaskID(), t.Logger().Event()}] = worker
	return nil
}

func (m *manager) RemoveTask(t task.Task) {
	m.workerLock.RLock()
	defer m.workerLock.RUnlock()
	if w, ok := m.workers[workerKey{t.TaskID(), t.Logger().Event()}]; ok {
		switch w.Status() {
		case "running":
			w.RollBack()
		case "error":
			w.Cancel()
		case "success":
		}
		m.removeWorker(t.TaskID(), t.Logger().Event())
	}
}

func (m *manager) GetWorker(taskID string, eventID string) (Worker, bool) {
	m.workerLock.RLock()
	defer m.workerLock.RUnlock()
	worker, ok := m.workers[workerKey{taskID, eventID}]
	return worker, ok
}

// Called by the worker after exiting.
func (m *manager) removeWorker(taskID string, eventID string) {
	m.workerLock.Lock()
	defer m.workerLock.Unlock()
	delete(m.workers, workerKey{taskID, eventID})
}

// workerCount returns the total number of probe workers. For testing.
func (m *manager) WorkerCount() int {
	m.workerLock.RLock()
	defer m.workerLock.RUnlock()
	return len(m.workers)
}
