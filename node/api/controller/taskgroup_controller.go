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
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/goodrain/rainbond/node/api/model"
	"net/http"

	"github.com/go-chi/chi"
)

//CreateTaskGroup 创建任务
func CreateTaskGroup(w http.ResponseWriter, r *http.Request) {
	var t model.TaskGroup
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &t, nil); !ok {
		return
	}
	if err := taskGroupService.AddTaskGroup(&t); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//GetTaskGroups 获取TaskGroups
func GetTaskGroups(w http.ResponseWriter, r *http.Request) {
	taskgroups, err := taskGroupService.GetTaskGroups()
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, taskgroups)
}

//GetTaskGroup 获取某组任务
func GetTaskGroup(w http.ResponseWriter, r *http.Request) {
	taskGroupID := chi.URLParam(r, "group_id")
	if taskGroupID == "" {
		httputil.ReturnError(r, w, 400, "task group id can not be empty")
	}
	task, err := taskGroupService.GetTaskGroup(taskGroupID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, task)
}

//ExecTaskGroup 执行某组任务
func ExecTaskGroup(w http.ResponseWriter, r *http.Request) {

}

//GetTaskGroupStatus 获取某组任务 状态
func GetTaskGroupStatus(w http.ResponseWriter, r *http.Request) {

}

//DeleteTaskGroup 删除某个任务组
func DeleteTaskGroup(w http.ResponseWriter, r *http.Request) {

}
