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

import "time"

const (
	Coll_JobLog       = "job_log"
	BuildIn_JobLog    = "buildIn_log"
	Coll_JobLatestLog = "job_latest_log"
	Coll_Stat         = "stat"
)

//ExecutionRecord 任务执行记录
type ExecutionRecord struct {
	ID        string    `json:"id"`
	JobID     string    `json:"job_id"`            // 任务 Id，索引
	User      string    `json:"user"`              // 执行此次任务的用户
	Name      string    `json:"name"`              // 任务名称
	Node      string    `json:"node"`              // 运行此次任务的节点 ip，索引
	Command   string    `json:"command,omitempty"` // 执行的命令，包括参数
	Output    string    `json:"output,omitempty"`  // 任务输出的所有内容
	Success   bool      `json:"success"`           // 是否执行成功
	BeginTime time.Time `json:"beginTime"`         // 任务开始执行时间，精确到毫秒，索引
	EndTime   time.Time `json:"endTime"`           // 任务执行完毕时间，精确到毫秒
}

//GetExecutionRecordByID 获取执行记录
func GetExecutionRecordByID(id string) (l *ExecutionRecord, err error) {
	return nil, nil
}

//GetJobExecutionRecords 获取某个任务执行记录
func GetJobExecutionRecords(JobID string) ([]*ExecutionRecord, error) {
	return nil, nil
}

//CreateExecutionRecord 创建存储记录
func CreateExecutionRecord(j *Job, t time.Time, rs string, success bool) {

}
