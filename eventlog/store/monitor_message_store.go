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
	"sync"
	"time"

	"github.com/goodrain/rainbond/eventlog/conf"
	"github.com/goodrain/rainbond/eventlog/db"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
)

type monitorMessageStore struct {
	conf        conf.EventStoreConf
	log         *logrus.Entry
	barrels     map[string]*monitorMessageBarrel
	lock        sync.RWMutex
	cancel      func()
	ctx         context.Context
	pool        *sync.Pool
	size        int64
	allLogCount float64
}

func (h *monitorMessageStore) Scrape(ch chan<- prometheus.Metric, namespace, exporter, from string) error {
	chanDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, exporter, "monitor_store_barrel_count"),
		"the handle container log count size.",
		[]string{"from"}, nil,
	)
	ch <- prometheus.MustNewConstMetric(chanDesc, prometheus.GaugeValue, float64(len(h.barrels)), from)
	logDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, exporter, "monitor_store_log_count"),
		"the handle monitor log count size.",
		[]string{"from"}, nil,
	)
	ch <- prometheus.MustNewConstMetric(logDesc, prometheus.GaugeValue, h.allLogCount, from)

	return nil
}
func (h *monitorMessageStore) insertMessage(message *db.EventLogMessage) bool {
	h.lock.RLock()
	defer h.lock.RUnlock()
	if ba, ok := h.barrels[message.EventID]; ok {
		ba.insertMessage(message)
		return true
	}
	return false
}

func (h *monitorMessageStore) InsertMessage(message *db.EventLogMessage) {
	if message == nil || message.EventID == "" {
		return
	}
	//h.log.Debug("Receive a monitor message:" + string(message.Content))
	h.size++
	h.allLogCount++
	if h.insertMessage(message) {
		return
	}
	h.lock.Lock()
	defer h.lock.Unlock()
	ba := h.pool.Get().(*monitorMessageBarrel)
	ba.insertMessage(message)
	h.barrels[message.EventID] = ba
}
func (h *monitorMessageStore) GetMonitorData() *db.MonitorData {
	data := &db.MonitorData{
		ServiceSize:  len(h.barrels),
		LogSizePeerM: h.size,
	}
	return data
}

func (h *monitorMessageStore) SubChan(eventID, subID string) chan *db.EventLogMessage {
	h.lock.Lock()
	defer h.lock.Unlock()
	if ba, ok := h.barrels[eventID]; ok {
		return ba.addSubChan(subID)
	}
	ba := h.pool.Get().(*monitorMessageBarrel)
	h.barrels[eventID] = ba
	return ba.addSubChan(subID)
}
func (h *monitorMessageStore) RealseSubChan(eventID, subID string) {
	h.lock.RLock()
	defer h.lock.RUnlock()
	if ba, ok := h.barrels[eventID]; ok {
		ba.delSubChan(subID)
	}
}
func (h *monitorMessageStore) Run() {
	go h.Gc()
}
func (h *monitorMessageStore) Gc() {
	tiker := time.NewTicker(time.Second * 30)
	for {
		select {
		case <-tiker.C:
		case <-h.ctx.Done():
			h.log.Debug("read message store gc stop.")
			tiker.Stop()
			return
		}
		h.size = 0
		if len(h.barrels) == 0 {
			continue
		}
		var gcEvent []string
		for k, v := range h.barrels {
			if v.updateTime.Add(time.Minute * 3).Before(time.Now()) { // barrel 超时未收到消息
				gcEvent = append(gcEvent, k)
			}
		}
		if gcEvent != nil && len(gcEvent) > 0 {
			for _, id := range gcEvent {
				barrel := h.barrels[id]
				barrel.empty()
				h.pool.Put(barrel) //放回对象池
				delete(h.barrels, id)
			}
		}
	}
}
func (h *monitorMessageStore) stop() {
	h.cancel()
}
func (h *monitorMessageStore) InsertGarbageMessage(message ...*db.EventLogMessage) {}
