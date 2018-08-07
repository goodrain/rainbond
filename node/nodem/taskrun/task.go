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

package taskrun

import (
	"time"

	"github.com/pquerna/ffjson/ffjson"
)

//Shell 执行脚本配置
type Shell struct {
	Cmd []string `json:"cmd"`
}

//TaskTemp 任务模版
type TaskTemp struct {
	Name       string            `json:"name" validate:"name|required"`
	ID         string            `json:"id" validate:"id|uuid"`
	Shell      Shell             `json:"shell"`
	Envs       map[string]string `json:"envs,omitempty"`
	Input      string            `json:"input,omitempty"`
	Args       []string          `json:"args,omitempty"`
	Depends    []DependStrategy  `json:"depends,omitempty"`
	Timeout    int               `json:"timeout" validate:"timeout|required|numeric"`
	CreateTime time.Time         `json:"create_time"`
	Labels     map[string]string `json:"labels,omitempty"`
}

//DependStrategy 依赖策略
type DependStrategy struct {
	DependTaskID      string `json:"depend_task_id"`
	DetermineStrategy string `json:"strategy"`
}

//AtLeastOnceStrategy 至少已执行一次
var AtLeastOnceStrategy = "AtLeastOnce"

//SameNodeStrategy 相同节点已执行
var SameNodeStrategy = "SameNode"

func (t TaskTemp) String() string {
	res, _ := ffjson.Marshal(&t)
	return string(res)
}

//Task 任务
type Task struct {
	Name    string    `json:"name" validate:"name|required"`
	ID      string    `json:"id" validate:"id|uuid"`
	TempID  string    `json:"temp_id,omitempty" validate:"temp_id|uuid"`
	Temp    *TaskTemp `json:"temp,omitempty"`
	GroupID string    `json:"group_id,omitempty"`
	//执行的节点
	Nodes []string `json:"nodes"`
	//执行时间定义
	//例如每30分钟执行一次:@every 30m
	Timer   string `json:"timer"`
	TimeOut int64  `json:"time_out"`
	// 执行任务失败重试次数
	// 默认为 0，不重试
	Retry int `json:"retry"`
	// 执行任务失败重试时间间隔
	// 单位秒，如果不大于 0 则马上重试
	Interval int `json:"interval"`
	//ExecCount 执行次数
	ExecCount int `json:"exec_count"`
	//每个执行节点执行状态
	Status       map[string]TaskStatus `json:"status,omitempty"`
	Scheduler    Scheduler             `json:"scheduler"`
	CreateTime   time.Time             `json:"create_time"`
	StartTime    time.Time             `json:"start_time"`
	CompleteTime time.Time             `json:"complete_time"`
	ResultPath   string                `json:"result_path"`
	EventID      string                `json:"event_id"`
	RunMode      string                `json:"run_mode"`
	OutPut       []*TaskOutPut         `json:"out_put"`
}

func (t Task) String() string {
	res, _ := ffjson.Marshal(&t)
	return string(res)
}

//Decode Decode
func (t *Task) Decode(data []byte) error {
	return ffjson.Unmarshal(data, t)
}

//UpdataOutPut 更新状态
func (t *Task) UpdataOutPut(output TaskOutPut) {
	updateIndex := -1
	for i, oldOut := range t.OutPut {
		if oldOut.NodeID == output.NodeID {
			updateIndex = i
			break
		}
	}
	if updateIndex != -1 {
		t.OutPut[updateIndex] = &output
		return
	}
	t.OutPut = append(t.OutPut, &output)
}

//CanBeDelete 能否被删除
func (t Task) CanBeDelete() bool {
	if t.Status == nil || len(t.Status) == 0 {
		return true
	}
	for _, v := range t.Status {
		if v.Status != "create" {
			return false
		}
	}
	return true
}

//Scheduler 调度状态
type Scheduler struct {
	Mode   string                     `json:"mode"` //立即调度（Intime），触发调度（Passive）
	Status map[string]SchedulerStatus `json:"status"`
}

//SchedulerStatus 调度状态
type SchedulerStatus struct {
	Status          string    `json:"status"`
	Message         string    `json:"message"`
	SchedulerTime   time.Time `json:"scheduler_time"`   //调度时间
	SchedulerMaster string    `json:"scheduler_master"` //调度的管理节点
}

//TaskOutPut 任务输出
type TaskOutPut struct {
	NodeID string            `json:"node_id"`
	JobID  string            `json:"job_id"`
	Global map[string]string `json:"global"`
	Inner  map[string]string `json:"inner"`
	//返回数据类型，检测结果类(check) 执行安装类 (install) 普通类 (common)
	Type       string             `json:"type"`
	Status     []TaskOutPutStatus `json:"status"`
	ExecStatus string             `json:"exec_status"`
	Body       string             `json:"body"`
}

//ParseTaskOutPut json parse
func ParseTaskOutPut(body string) (t TaskOutPut, err error) {
	t.Body = body
	err = ffjson.Unmarshal([]byte(body), &t)
	return
}

//TaskOutPutStatus 输出数据
type TaskOutPutStatus struct {
	Name string `json:"name"`
	//节点属性
	ConditionType string `json:"condition_type"`
	//节点属性值
	ConditionStatus string   `json:"condition_status"`
	NextTask        []string `json:"next_tasks,omitempty"`
	NextGroups      []string `json:"next_groups,omitempty"`
}

//TaskStatus 任务状态
type TaskStatus struct {
	JobID        string    `json:"job_id"`
	Status       string    `json:"status"` //执行状态，create init exec complete timeout
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	TakeTime     int       `json:"take_time"`
	CompleStatus string    `json:"comple_status"`
	//脚本退出码
	ShellCode int    `json:"shell_code"`
	Message   string `json:"message,omitempty"`
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
		if v.Status != "create" {
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
