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

package model

import (
	eventdb "github.com/goodrain/rainbond/eventlog/db"
)

//LogData log data
type LogData struct {
	num int
	msg string
}

//MessageData message data 获取指定操作的操作日志
type MessageData struct {
	Message  string `json:"message"`
	Time     string `json:"time"`
	Unixtime int64  `json:"utime"`
}

//DataLog 获取指定操作的操作日志
type DataLog struct {
	Status string
	Data   eventdb.MessageDataList
}

//LogByLevelStruct GetLogByLevelStruct
//swagger:parameters logByAction
type LogByLevelStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name" validate:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias" validate:"service_alias"`
	// in: body
	Body struct {
		// 日志级别info/debug/error
		// in: body
		// required: true
		Level string `json:"level" validate:"level|required"`
		// eventID
		// in: body
		// required: true
		EventID string `json:"event_id" validate:"event_id|required"`
	}
}

//TenantLogByLevelStruct GetTenantLogByLevelStruct
//swagger:parameters tenantLogByAction
type TenantLogByLevelStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name" validate:"tenant_name"`
	// in: body
	Body struct {
		// 日志级别info/debug/error
		// in: body
		// required: true
		Level string `json:"level" validate:"level|required"`
		// eventID
		// in: body
		// required: true
		EventID string `json:"event_id" validate:"event_id|required"`
	}
}

//LogSocketStruct LogSocketStruct
//swagger:parameters logSocket logList
type LogSocketStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name" validate:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias" validate:"service_alias"`
}

//LogFileStruct LogFileStruct
//swagger:parameters logFile
type LogFileStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name" validate:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias" validate:"service_alias"`
	// in: path
	// required: true
	FileName string `json:"file_name" validate:"file_name"`
}

//MsgStruct msg struct in eventlog_message
type MsgStruct struct {
	EventID string `json:"event_id"`
	Step    string `json:"step"`
	Message string `json:"message"`
	Level   string `json:"level"`
	Time    string `json:"time"`
}

//LastLinesStruct LastLinesStruct
//swagger:parameters lastLinesLogs
type LastLinesStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name" validate:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias" validate:"service_alias"`
	// in: body
	Body struct {
		// 行数
		// in: body
		// required: true
		Lines int `json:"lines" validate:"lines"`
	}
}
