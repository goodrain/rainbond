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

// 文件: server.go
// 说明: 该文件实现了服务器的核心功能。文件中定义了启动和管理服务器的相关方法，
// 用于处理客户端请求、管理连接、以及提供平台的核心服务。通过这些方法，Rainbond 平台
// 能够确保服务器的稳定运行和高效响应，支持平台的各种业务功能。

package entry

import (
	"github.com/goodrain/rainbond/eventlog/conf"
	"github.com/goodrain/rainbond/eventlog/store"

	"github.com/sirupsen/logrus"
	"github.com/thejerf/suture"
)

// Entry 数据入口
type Entry struct {
	supervisor   *suture.Supervisor
	log          *logrus.Entry
	conf         conf.EntryConf
	storeManager store.Manager
}

// NewEntry 创建
func NewEntry(conf conf.EntryConf, log *logrus.Entry, storeManager store.Manager) *Entry {
	return &Entry{
		log:          log,
		conf:         conf,
		storeManager: storeManager,
	}
}

// Start 启动
func (e *Entry) Start() error {
	supervisor := suture.New("Entry Server", suture.Spec{
		Log: func(m string) {
			e.log.Info(m)
		},
	})
	eventServer, err := NewEventLogServer(e.conf.EventLogServer, e.log.WithField("server", "EventLog"), e.storeManager)
	if err != nil {
		return err
	}
	dockerServer, err := NewDockerLogServer(e.conf.DockerLogServer, e.log.WithField("server", "DockerLog"), e.storeManager)
	if err != nil {
		return err
	}
	monitorServer, err := NewMonitorMessageServer(e.conf.MonitorMessageServer, e.log.WithField("server", "MonitorMessage"), e.storeManager)
	if err != nil {
		return err
	}
	newmonitorServer, err := NewNMonitorMessageServer(e.conf.NewMonitorMessageServerConf, e.log.WithField("server", "NewMonitorMessage"), e.storeManager)
	if err != nil {
		return err
	}

	supervisor.Add(eventServer)
	supervisor.Add(dockerServer)
	supervisor.Add(monitorServer)
	supervisor.Add(newmonitorServer)
	supervisor.ServeBackground()
	e.supervisor = supervisor
	return nil
}

// Stop 停止
func (e *Entry) Stop() {
	if e.supervisor != nil {
		e.supervisor.Stop()
	}
}
