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
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/node/api/model"
)

//ValidationCriteria 在某个节点执行任务，行不行
type ValidationCriteria func(string, *model.Task) (bool, error)

//AllCouldRun 可以执行
var AllCouldRun ValidationCriteria = func(string, *model.Task) (bool, error) {
	return true, nil
}

//ModeRun 验证任务执行策略
var ModeRun ValidationCriteria = func(node string, task *model.Task) (bool, error) {
	if task.RunMode == "OnlyOnce" {
		if status, ok := task.Status[node]; ok {
			if status.CompleStatus == "Success" {
				return false, fmt.Errorf("this job In violation of the task runmode")
			}
		}
	}
	return true, nil
}

//DependRun 验证依赖任务执行情况
func (t *TaskEngine) DependRun(node string, task, depTask *model.Task, Strategy string) (bool, error) {
	if depTask != nil {
		//判断依赖任务调度情况
		if depTask.Scheduler.Mode == "Passive" {
			var needScheduler bool
			//当前节点未调度且依赖策略为当前节点必须执行，则调度
			if Strategy == model.SameNodeStrategy {
				if job := t.GetJob(getHash(depTask.ID, node)); job == nil {
					needScheduler = true
				}
			}
			if needScheduler {
				//发出依赖任务的调度请求
				t.PutSchedul(depTask.ID, node)
				return false, nil
			}
		}
		//判断依赖任务的执行情况
		//依赖策略为任务全局只要执行一次
		if Strategy == model.AtLeastOnceStrategy {
			if depTask.Status == nil || len(depTask.Status) < 1 {
				return false, nil
			}
			var faiiureSize int
			if len(depTask.Status) > 0 {
				for _, status := range depTask.Status {
					if status.CompleStatus == "Success" {
						logrus.Debugf("dep task %s ready", depTask.ID)
						return true, nil
					}
					faiiureSize++
				}
			}
			// if faiiureSize > 0 {
			// 	return false, fmt.Errorf("dep task run error count %d", faiiureSize)
			// }
			return false, nil
		}
		//依赖任务相同节点执行成功
		if Strategy == model.SameNodeStrategy {
			if depTask.Status == nil || len(depTask.Status) < 1 {
				return false, nil
			}
			if nodestatus, ok := depTask.Status[node]; ok && nodestatus.CompleStatus == "Success" {
				return true, nil
			} else if ok && nodestatus.CompleStatus != "" {
				return false, fmt.Errorf("depend task %s(%s) NameCondition cannot be satisfied", depTask.ID, nodestatus.CompleStatus)
			} else {
				return false, nil
			}
		}
	} else {
		return false, fmt.Errorf("task (%s) dep task is nil", task.ID)
	}
	return false, nil
}

//GetValidationCriteria 获取调度必要条件
func (t *TaskEngine) GetValidationCriteria(task *model.Task) (vas []ValidationCriteria) {
	vas = append(vas, ModeRun)
	vas = append(vas, t.DependsRun)
	return
}

//DependsRun DependRun
func (t *TaskEngine) DependsRun(node string, task *model.Task) (bool, error) {
	for _, dep := range task.Temp.Depends {
		depTask := t.GetTask(dep.DependTaskID)
		if depTask != nil {
			ok, err := t.DependRun(node, task, depTask, dep.DetermineStrategy)
			if !ok {
				return ok, err
			}
		}
	}
	return true, nil
}
