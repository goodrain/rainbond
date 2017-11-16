// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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

package masterserver

import (
	"testing"

	"github.com/goodrain/rainbond/pkg/node/api/model"
)

func TestGroupWorker(t *testing.T) {
	taskEngine := CreateTaskEngine(nil)
	group := &model.TaskGroup{
		Tasks: []*model.Task{
			&model.Task{ID: "1", Temp: &model.TaskTemp{Depends: []model.DependStrategy{model.DependStrategy{DependTaskID: "5"}}}},
			&model.Task{ID: "2", Temp: &model.TaskTemp{Depends: []model.DependStrategy{model.DependStrategy{DependTaskID: "5"}}}},
			&model.Task{ID: "3", Temp: &model.TaskTemp{Depends: []model.DependStrategy{model.DependStrategy{DependTaskID: "5"}}}},
			&model.Task{ID: "4", Temp: &model.TaskTemp{Depends: []model.DependStrategy{}}},
			&model.Task{ID: "5", Temp: &model.TaskTemp{}},
			&model.Task{ID: "6", Temp: &model.TaskTemp{}},
			&model.Task{ID: "7", Temp: &model.TaskTemp{}},
		},
	}
	taskEngine.ScheduleGroup(nil, group)
}
