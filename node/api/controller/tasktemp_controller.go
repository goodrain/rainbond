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

//CreateTaskTemp 创建任务模版
func CreateTaskTemp(w http.ResponseWriter, r *http.Request) {
	var t model.TaskTemp
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &t, nil); !ok {
		return
	}
	if err := taskTempService.SaveTaskTemp(&t); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//UpdateTaskTemp 更新任务模版
func UpdateTaskTemp(w http.ResponseWriter, r *http.Request) {
	var t model.TaskTemp
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &t, nil); !ok {
		return
	}
	if t.ID == "" {
		tempID := chi.URLParam(r, "temp_id")
		t.ID = tempID
	}
	if err := taskTempService.SaveTaskTemp(&t); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//DeleteTaskTemp 删除任务模版
func DeleteTaskTemp(w http.ResponseWriter, r *http.Request) {
	tempID := chi.URLParam(r, "temp_id")
	if err := taskTempService.DeleteTaskTemp(tempID); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}
