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

package db

//EventLogMessage 事件日志实体
type EventLogMessage struct {
	EventID string `json:"event_id"`
	Step    string `json:"step"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Level   string `json:"level"`
	Time    string `json:"time"`
	Content []byte `json:"-"`
	//monitor消息使用
	MonitorData []byte `json:"monitorData,omitempty"`
}

type MonitorData struct {
	InstanceID   string
	ServiceSize  int
	LogSizePeerM int64
}
type ClusterMessageType string

const (
	//EventMessage 操作日志共享
	EventMessage ClusterMessageType = "event_log"
	//ServiceMonitorMessage 业务监控数据消息
	ServiceMonitorMessage ClusterMessageType = "monitor_message"
	//ServiceNewMonitorMessage 新业务监控数据消息
	ServiceNewMonitorMessage ClusterMessageType = "new_monitor_message"
	//MonitorMessage 节点监控数据
	MonitorMessage ClusterMessageType = "monitor"
)

type ClusterMessage struct {
	Data []byte
	Mode ClusterMessageType
}
