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

package job

import (
	"fmt"
	"strings"

	"github.com/goodrain/rainbond/pkg/node/api/model"
	"github.com/goodrain/rainbond/pkg/node/core/config"
	"github.com/twinj/uuid"
)

//CreateJobFromTask 从task创建job
func CreateJobFromTask(task *model.Task, groupCtx *config.GroupContext) (*Job, error) {
	if task.Temp == nil {
		return nil, fmt.Errorf("task temp is nil, can not build job")
	}
	command, err := config.ResettingArray(groupCtx, task.Temp.Shell.Cmd)
	if err != nil {
		return nil, err
	}
	stdin, err := config.ResettingString(groupCtx, task.Temp.Input)
	if err != nil {
		return nil, err
	}
	envMaps, err := config.ResettingMap(groupCtx, task.Temp.Envs)
	if err != nil {
		return nil, err
	}
	var envs []string
	for k, v := range envMaps {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}
	var rules []*JobRule
	//如果任务不是一次任务
	if !task.IsOnce {
		if task.Timer == "" {
			return nil, fmt.Errorf("timer can not be empty")
		}
		rule := &JobRule{
			Labels:  task.Temp.Labels,
			NodeIDs: task.Nodes,
			ID:      uuid.NewV4().String(),
			Timer:   task.Timer,
		}
		rules = append(rules, rule)
	} else {
		rule := &JobRule{
			Labels:  task.Temp.Labels,
			NodeIDs: task.Nodes,
			ID:      uuid.NewV4().String(),
		}
		rules = append(rules, rule)
	}
	job := &Job{
		ID:       uuid.NewV4().String(),
		TaskID:   task.ID,
		EventID:  task.EventID,
		IsOnce:   task.IsOnce,
		Name:     task.Name,
		Command:  strings.Join(command, " "),
		Stdin:    stdin,
		Envs:     envs,
		Rules:    rules,
		Timeout:  task.TimeOut,
		Retry:    task.Retry,
		Interval: task.Interval,
	}
	return job, nil
}
