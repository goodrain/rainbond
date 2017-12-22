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

package store

import "time"

//MonitorMessage 性能监控消息系统模型
type MonitorMessage struct {
	ServiceID   string
	Port        string
	HostName    string
	MessageType string //mysql，http ...
	Key         string
	//总时间
	CumulativeTime float64
	AverageTime    float64
	MaxTime        float64
	Count          uint64
	//异常请求次数
	AbnormalCount uint64
}

type cacheMonitorMessage struct {
	updateTime time.Time
	mm         MonitorMessage
}

type cacheMonitorMessageList struct {
	list []*cacheMonitorMessage
}

func (c *cacheMonitorMessageList) Insert(mms ...*MonitorMessage) {

}
