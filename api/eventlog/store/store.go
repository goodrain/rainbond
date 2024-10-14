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

package store

import (
	"github.com/goodrain/rainbond/api/eventlog/db"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"golang.org/x/net/context"
)

// MessageStore store
type MessageStore interface {
	InsertMessage(*db.EventLogMessage)
	InsertGarbageMessage(...*db.EventLogMessage)
	GetHistoryMessage(eventID string, length int) []string
	SubChan(eventID, subID string) chan *db.EventLogMessage
	RealseSubChan(eventID, subID string)
	GetMonitorData() *db.MonitorData
	Run()
	Gc()
	stop()
	Scrape(ch chan<- prometheus.Metric, namespace, exporter, from string) error
}

// NewStore 创建
func NewStore(storeType string, manager *storeManager) MessageStore {
	ctx, cancel := context.WithCancel(context.Background())
	if storeType == "handle" {
		handle := &handleMessageStore{
			barrels:   make(map[string]*EventBarrel, 100),
			conf:      manager.conf,
			log:       manager.log.WithField("module", "HandleMessageStore"),
			garbageGC: make(chan int),
			ctx:       ctx,
			cancel:    cancel,
			//TODO:
			//此通道过小会阻塞接收消息的插入，造成死锁
			//更改持久化事件为无阻塞插入
			barrelEvent:         make(chan []string, 100),
			dbPlugin:            manager.dbPlugin,
			handleEventCoreSize: 2,
			stopGarbage:         make(chan struct{}),
			manager:             manager,
		}
		handle.pool = &sync.Pool{
			New: func() interface{} {
				barrel := &EventBarrel{
					barrel:            make([]*db.EventLogMessage, 0),
					persistenceBarrel: make([]*db.EventLogMessage, 0),
					barrelEvent:       handle.barrelEvent,
					cacheNumber:       manager.conf.PeerEventMaxCacheLogNumber,
					maxNumber:         manager.conf.PeerEventMaxLogNumber,
				}
				return barrel
			},
		}
		go handle.handleGarbageMessage()
		for i := 0; i < handle.handleEventCoreSize; i++ {
			go handle.handleBarrelEvent()
		}
		return handle
	}
	if storeType == "read" {
		read := &readMessageStore{
			barrels: make(map[string]*readEventBarrel, 100),
			conf:    manager.conf,
			log:     manager.log.WithField("module", "SubMessageStore"),
			ctx:     ctx,
			cancel:  cancel,
		}
		read.pool = &sync.Pool{
			New: func() interface{} {
				reb := &readEventBarrel{
					subSocketChan: make(map[string]chan *db.EventLogMessage, 0),
				}
				return reb
			},
		}
		return read
	}
	if storeType == "docker_log" {
		docker := &dockerLogStore{
			barrels:    make(map[string]*dockerLogEventBarrel, 100),
			conf:       manager.conf,
			log:        manager.log.WithField("module", "DockerLogStore"),
			ctx:        ctx,
			cancel:     cancel,
			filePlugin: manager.filePlugin,
			//TODO:
			//此通道过小会阻塞接收消息的插入，造成死锁
			//更改持久化事件为无阻塞插入
			barrelEvent: make(chan []string, 100),
		}
		docker.pool = &sync.Pool{
			New: func() interface{} {
				reb := &dockerLogEventBarrel{
					subSocketChan:   make(map[string]chan *db.EventLogMessage, 0),
					cacheSize:       manager.conf.PeerDockerMaxCacheLogNumber,
					barrelEvent:     docker.barrelEvent,
					persistenceTime: time.Now(),
				}
				return reb
			},
		}
		return docker
	}
	if storeType == "newmonitor" {
		monitor := &newMonitorMessageStore{
			barrels: make(map[string]*CacheMonitorMessageList, 100),
			conf:    manager.conf,
			log:     manager.log.WithField("module", "NewMonitorMessageStore"),
			ctx:     ctx,
			cancel:  cancel,
		}
		return monitor
	}

	return nil
}
