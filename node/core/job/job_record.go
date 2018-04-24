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

package job

import (
	"time"

	"github.com/pquerna/ffjson/ffjson"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/core/store"

	"github.com/twinj/uuid"
)

//ExecutionRecord 任务执行记录
type ExecutionRecord struct {
	ID         string    `json:"id"`
	JobID      string    `json:"job_id"` // 任务 Id，索引
	TaskID     string    `json:"task_id"`
	User       string    `json:"user"`              // 执行此次任务的用户
	Name       string    `json:"name"`              // 任务名称
	Node       string    `json:"node"`              // 运行此次任务的节点 ip，索引
	Command    string    `json:"command,omitempty"` // 执行的命令，包括参数
	Output     string    `json:"output"`            // 任务输出的所有内容
	Success    bool      `json:"success"`           // 是否执行成功
	BeginTime  time.Time `json:"beginTime"`         // 任务开始执行时间，精确到毫秒，索引
	EndTime    time.Time `json:"endTime"`           // 任务执行完毕时间，精确到毫秒
	IsHandle   bool      `json:"is_handle"`         //是否已经处理
	HandleTime time.Time `json:"handle_time"`       //处理时间
}

func (e ExecutionRecord) String() string {
	body, _ := ffjson.Marshal(e)
	return string(body)
}

//GetExecutionRecordByID 获取执行记录
func GetExecutionRecordByID(id string) (l *ExecutionRecord, err error) {
	return nil, nil
}

//GetJobExecutionRecords 获取某个任务执行记录
func GetJobExecutionRecords(JobID string) ([]*ExecutionRecord, error) {
	return nil, nil
}

//IsHandleRight 是否具有处理结果权限
func (e ExecutionRecord) IsHandleRight() bool {
	if e.IsHandle {
		return false
	}
	resp, err := store.DefalutClient.Grant(5)
	if err != nil {
		logrus.Infof("execution record[%s] didn't get a lock, err: %s", e.ID, err.Error())
		return false
	}
	ok, err := store.DefalutClient.GetLock(e.ID, resp.ID)
	if err != nil {
		logrus.Infof("execution record[%s] didn't get a lock, err: %s", e.ID, err.Error())
		return false
	}
	return ok
}

//CompleteHandle 完成处理记录
//master节点处理完成后调用
func (e ExecutionRecord) CompleteHandle() {
	e.HandleTime = time.Now()
	e.IsHandle = true
	_, err := store.DefalutClient.Put(option.Config.ExecutionRecordPath+"/"+e.JobID+"/"+e.ID, e.String())
	if err != nil {
		logrus.Error("put exec record to etcd in complete handle error.", err.Error())
	}
}

//ParseExecutionRecord 解析
func ParseExecutionRecord(body []byte) (e ExecutionRecord) {
	ffjson.Unmarshal(body, &e)
	return
}

//CreateExecutionRecord 创建存储记录
func CreateExecutionRecord(j *Job, t time.Time, rs string, success bool) {
	//存储执行记录，master端获取结果，并处理结果
	//例如 检测任务，安装任务等
	record := ExecutionRecord{
		ID:        uuid.NewV4().String(),
		JobID:     j.ID,
		TaskID:    j.TaskID,
		User:      j.User,
		Name:      j.Name,
		Node:      j.NodeID,
		Command:   j.Command,
		Output:    rs,
		Success:   success,
		BeginTime: t,
		EndTime:   time.Now(),
	}
	_, err := store.DefalutClient.Put(option.Config.ExecutionRecordPath+"/"+record.JobID+"/"+record.ID, record.String())
	if err != nil {
		logrus.Error("put exec record to etcd error.", err.Error())
	}
	status := "Success"
	if !success {
		status = "Failure"
	}
	j.RunStatus = &RunStatus{
		Status:    status,
		StartTime: t,
		EndTime:   time.Now(),
		RecordID:  record.ID,
	}
	PutJob(j)
}
