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

package controller

import (
	"net/http"

	"github.com/goodrain/rainbond/node/api/model"
	httputil "github.com/goodrain/rainbond/util/http"

	"github.com/go-chi/chi"
)

//CreateTask 创建任务
func CreateTask(w http.ResponseWriter, r *http.Request) {
	var t model.Task
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &t, nil); !ok {
		return
	}
	if err := taskService.AddTask(&t); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//GetTasks 获取tasks
func GetTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := taskService.GetTasks()
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, tasks)
}

//GetTask 获取某个任务
func GetTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "task_id")
	if taskID == "" {
		httputil.ReturnError(r, w, 400, "task id can not be empty")
		return
	}
	task, err := taskService.GetTask(taskID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, task)
}

//ExecTask 执行某个任务
func ExecTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "task_id")
	if taskID == "" {
		httputil.ReturnError(r, w, 400, "task id can not be empty")
		return
	}
	var nodes struct {
		Nodes []string `json:"nodes"`
	}
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &nodes, nil); !ok {
		return
	}
	err := taskService.ExecTask(taskID, nodes.Nodes)
	if err != nil {
		err.Handle(r, w)
		return
	}
	for _,v:=range nodes.Nodes {
		node, err := nodeService.GetNode(v)
		if err != nil {
			err.Handle(r, w)
			return
		}
		node.Status="install"
	}

	httputil.ReturnSuccess(r, w, nil)
}

//GetTaskStatus 获取某个任务状态
func GetTaskStatus(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "task_id")
	if taskID == "" {
		httputil.ReturnError(r, w, 400, "task id can not be empty")
		return
	}
	task, err := taskService.GetTask(taskID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, task.Status)
}

//DeleteTask 删除某个任务
func DeleteTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "task_id")
	if taskID == "" {
		httputil.ReturnError(r, w, 400, "task id can not be empty")
		return
	}
	err := taskService.DeleteTask(taskID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//ReloadStaticTasks 重新加载静态文件
func ReloadStaticTasks(w http.ResponseWriter, r *http.Request) {
	taskService.ReloadStaticTasks()
	httputil.ReturnSuccess(r, w, nil)
}
