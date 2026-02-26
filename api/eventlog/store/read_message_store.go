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
	"github.com/goodrain/rainbond/api/eventlog/conf"
	"github.com/goodrain/rainbond/api/eventlog/db"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

type readMessageStore struct {
	conf      conf.EventStoreConf
	log       *logrus.Entry
	barrels   map[string]*readEventBarrel
	lock      sync.Mutex
	cancel    func()
	ctx       context.Context
	pool      *sync.Pool
	fileStore FileStore
}

func (h *readMessageStore) Scrape(ch chan<- prometheus.Metric, namespace, exporter, from string) error {
	return nil
}
func (h *readMessageStore) InsertMessage(message *db.EventLogMessage) {
	if message == nil || message.EventID == "" {
		return
	}
	h.lock.Lock()
	defer h.lock.Unlock()
	if ba, ok := h.barrels[message.EventID]; ok {
		ba.insertMessage(message)
	} else {
		ba := h.pool.Get().(*readEventBarrel)
		// 注入eventID和fileStore
		ba.eventID = message.EventID
		ba.fileStore = h.fileStore
		ba.insertMessage(message)
		h.barrels[message.EventID] = ba
	}
}
func (h *readMessageStore) GetMonitorData() *db.MonitorData {
	return nil
}

func (h *readMessageStore) SubChan(eventID, subID string) chan *db.EventLogMessage {
	h.lock.Lock()
	defer h.lock.Unlock()
	if ba, ok := h.barrels[eventID]; ok {
		return ba.addSubChan(subID)
	}
	ba := h.pool.Get().(*readEventBarrel)
	// 注入eventID和fileStore
	ba.eventID = eventID
	ba.fileStore = h.fileStore
	ba.updateTime = time.Now()
	h.barrels[eventID] = ba
	return ba.addSubChan(subID)
}
func (h *readMessageStore) RealseSubChan(eventID, subID string) {
	h.lock.Lock()
	defer h.lock.Unlock()
	if ba, ok := h.barrels[eventID]; ok {
		ba.delSubChan(subID)
	}
}
func (h *readMessageStore) Run() {
	go h.Gc()
}
func (h *readMessageStore) Gc() {
	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
		case <-h.ctx.Done():
			h.log.Debug("read message store gc stop.")
			return
		}

		if len(h.barrels) == 0 {
			continue
		}

		var gcEvent []string
		h.lock.Lock()
		for k, v := range h.barrels {
			// 改进GC策略：
			// 1. 超时未活跃的barrel（1分钟）且无订阅者
			if v.updateTime.Add(time.Minute * 1).Before(time.Now()) {
				v.subLock.Lock()
				hasSubscribers := len(v.subSocketChan) > 0
				v.subLock.Unlock()

				// 只有没有订阅者时才清理
				if !hasSubscribers {
					gcEvent = append(gcEvent, k)
				}
			}
		}

		// 执行清理
		if len(gcEvent) > 0 {
			for _, id := range gcEvent {
				barrel := h.barrels[id]
				barrel.empty()
				h.pool.Put(barrel)
				delete(h.barrels, id)
			}
			h.log.Debugf("GC cleaned %d event barrels", len(gcEvent))
		}
		h.lock.Unlock()
	}
}
func (h *readMessageStore) stop() {
	h.cancel()
}
func (h *readMessageStore) InsertGarbageMessage(message ...*db.EventLogMessage) {}

func (h *readMessageStore) GetHistoryMessage(eventID string, length int) (re []string) {
	return nil
}
