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

package model

import (
	"time"

	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/node/core/store"
	"github.com/pquerna/ffjson/ffjson"
)

//Shell 执行脚本配置
type Shell struct {
	Cmd []string
}

//TaskTemp 任务模版
type TaskTemp struct {
	Name    string            `json:"name" validate:"name|required"`
	ID      string            `json:"id" validate:"id|uuid"`
	Shell   Shell             `json:"shell"`
	Envs    map[string]string `json:"envs"`
	Input   string            `json:"input"`
	Args    []string          `json:"args"`
	Depends []string          `json:"depends"`
	Timeout int               `json:"timeout|required|numeric"`
	//OutPutChan
	//结果输出通道，错误输出OR标准输出
	OutPutChan string    `json:"out_put_chan" validate:"out_put_chan|required|in:stdout,stderr"`
	CreateTime time.Time `json:"create_time"`
}

func (t TaskTemp) String() string {
	res, _ := ffjson.Marshal(&t)
	return string(res)
}

//Task 任务
type Task struct {
	Name   string    `json:"name" validate:"name|required"`
	ID     string    `json:"id" validate:"id|uuid"`
	TempID string    `json:"temp_id,omitempty" validate:"temp_id|uuid"`
	Temp   *TaskTemp `json:"temp,omitempty"`
	//执行的节点
	Nodes []string `json:"nodes"`
	//每个执行节点执行状态
	Status       map[string]TaskStatus `json:"status,omitempty"`
	CreateTime   time.Time             `json:"create_time"`
	StartTime    time.Time             `json:"start_time"`
	CompleteTime time.Time             `json:"complete_time"`
	ResultPath   string                `json:"result_path"`
	EventID      string                `json:"event_id"`
	OutPut       *TaskOutPut           `json:"out_put"`
}

func (t Task) String() string {
	res, _ := ffjson.Marshal(&t)
	return string(res)
}

//CanBeDelete 能否被删除
func (t Task) CanBeDelete() bool {
	if t.Status == nil || len(t.Status) == 0 {
		return true
	}
	for _, v := range t.Status {
		if v.Status == "exec" {
			return false
		}
	}
	return true
}

//SendTask 发送任务
func SendTask(tasks ...*Task) error {
	for _, task := range tasks {
		_, err := store.DefalutClient.Put(option.Config.TaskPath+"/"+task.ID, task.String())
		if err != nil {
			return err
		}
	}
	return nil
}

//TaskOutPut 任务输出
type TaskOutPut struct {
	Global map[string]string `json:"global"`
	Inner  map[string]string `json:"inner"`
	Status string            `json:"status"`
}

//TaskStatus 任务状态
type TaskStatus struct {
	Status       string    `json:"status"` //执行状态，create init exec complete timeout
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	TakeTime     int       `json:"take_time"`
	CompleStatus string    `json:"comple_status"`
	//脚本退出码
	ShellCode int `json:"shell_code"`
}

//TaskGroup 任务组
type TaskGroup struct {
	Name       string           `json:"name" validate:"name|required"`
	ID         string           `json:"id" validate:"id|uuid"`
	Tasks      []*Task          `json:"tasks"`
	CreateTime time.Time        `json:"create_time"`
	Status     *TaskGroupStatus `json:"status"`
}

func (t TaskGroup) String() string {
	res, _ := ffjson.Marshal(&t)
	return string(res)
}

//CanBeDelete 是否能被删除
func (t TaskGroup) CanBeDelete() bool {
	if t.Status == nil || len(t.Status.TaskStatus) == 0 {
		return true
	}
	for _, v := range t.Status.TaskStatus {
		if v.Status == "exec" {
			return false
		}
	}
	return true
}

//TaskGroupStatus 任务组状态
type TaskGroupStatus struct {
	TaskStatus map[string]TaskStatus `json:"task_status"`
	InitTime   time.Time             `json:"init_time"`
	StartTime  time.Time             `json:"start_time"`
	EndTime    time.Time             `json:"end_time"`
	Status     string                `json:"status"` //create init exec complete timeout
}
