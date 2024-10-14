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
	db2 "github.com/goodrain/rainbond/api/eventlog/db"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

type dockerLogStore struct {
	conf         conf.EventStoreConf
	log          *logrus.Entry
	barrels      map[string]*dockerLogEventBarrel
	rwLock       sync.RWMutex
	cancel       func()
	ctx          context.Context
	pool         *sync.Pool
	filePlugin   db2.Manager
	LogSizePeerM int64
	LogSize      int64
	barrelSize   int
	barrelEvent  chan []string
	allLogCount  float64 //ues to pometheus monitor
}

func (h *dockerLogStore) Scrape(ch chan<- prometheus.Metric, namespace, exporter, from string) error {
	chanDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, exporter, "container_log_store_cache_barrel_count"),
		"the cache container log barrel size.",
		[]string{"from"}, nil,
	)
	ch <- prometheus.MustNewConstMetric(chanDesc, prometheus.GaugeValue, float64(len(h.barrels)), from)
	logDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, exporter, "container_log_store_log_count"),
		"the handle container log count size.",
		[]string{"from"}, nil,
	)
	ch <- prometheus.MustNewConstMetric(logDesc, prometheus.GaugeValue, h.allLogCount, from)

	return nil
}
func (h *dockerLogStore) insertMessage(message *db2.EventLogMessage) bool {
	h.rwLock.RLock() //读锁
	defer h.rwLock.RUnlock()
	if ba, ok := h.barrels[message.EventID]; ok {
		ba.insertMessage(message)
		return true
	}
	return false
}
func (h *dockerLogStore) InsertMessage(message *db2.EventLogMessage) {
	if message == nil || message.EventID == "" {
		return
	}
	h.LogSize++
	h.allLogCount++
	if ok := h.insertMessage(message); ok {
		return
	}
	h.rwLock.Lock()
	defer h.rwLock.Unlock()
	ba := h.pool.Get().(*dockerLogEventBarrel)
	ba.name = message.EventID
	ba.persistenceTime = time.Now()
	ba.insertMessage(message)
	h.barrels[message.EventID] = ba
	h.barrelSize++
}
func (h *dockerLogStore) subChan(eventID, subID string) chan *db2.EventLogMessage {
	h.rwLock.RLock() //读锁
	defer h.rwLock.RUnlock()
	if ba, ok := h.barrels[eventID]; ok {
		ch := ba.addSubChan(subID)
		return ch
	}
	return nil
}
func (h *dockerLogStore) SubChan(eventID, subID string) chan *db2.EventLogMessage {
	if ch := h.subChan(eventID, subID); ch != nil {
		return ch
	}
	h.rwLock.Lock()
	defer h.rwLock.Unlock()
	ba := h.pool.Get().(*dockerLogEventBarrel)
	ba.updateTime = time.Now()
	ba.name = eventID
	h.barrels[eventID] = ba
	return ba.addSubChan(subID)
}
func (h *dockerLogStore) RealseSubChan(eventID, subID string) {
	h.rwLock.RLock()
	defer h.rwLock.RUnlock()
	if ba, ok := h.barrels[eventID]; ok {
		ba.delSubChan(subID)
	}
}
func (h *dockerLogStore) Run() {
	go h.Gc()
	go h.handleBarrelEvent()
}

func (h *dockerLogStore) GetMonitorData() *db2.MonitorData {
	data := &db2.MonitorData{
		ServiceSize:  len(h.barrels),
		LogSizePeerM: h.LogSizePeerM,
	}
	if h.LogSizePeerM == 0 {
		data.LogSizePeerM = h.LogSize
	}
	return data
}
func (h *dockerLogStore) Gc() {
	tiker := time.NewTicker(time.Second * 30)
	for {
		select {
		case <-tiker.C:
			h.gcRun()
		case <-h.ctx.Done():
			h.log.Debug("docker log store gc stop.")
			tiker.Stop()
			return
		}
	}
}
func (h *dockerLogStore) handle() []string {
	h.rwLock.RLock()
	defer h.rwLock.RUnlock()
	if len(h.barrels) == 0 {
		return nil
	}
	var gcEvent []string
	for k := range h.barrels {
		if h.barrels[k].updateTime.Add(time.Minute*1).Before(time.Now()) && h.barrels[k].GetSubChanLength() == 0 {
			h.saveBeforeGc(k, h.barrels[k])
			gcEvent = append(gcEvent, k)
			h.log.Debugf("barrel %s need be gc", k)
		} else if h.barrels[k].persistenceTime.Add(time.Minute * 1).Before(time.Now()) {
			//The interval not persisted for more than 1 minute should be more than 30 seconds
			if len(h.barrels[k].barrel) > 0 {
				h.log.Debugf("barrel %s need persistence", k)
				h.barrels[k].persistence()
			}
		}
	}
	return gcEvent
}
func (h *dockerLogStore) gcRun() {
	t := time.Now()
	//每分钟进行数据重置，获得每分钟日志量数据
	h.LogSizePeerM = h.LogSize
	h.LogSize = 0
	gcEvent := h.handle()
	if gcEvent != nil && len(gcEvent) > 0 {
		h.rwLock.Lock()
		defer h.rwLock.Unlock()
		for _, id := range gcEvent {
			barrel := h.barrels[id]
			barrel.empty()
			h.pool.Put(barrel)
			delete(h.barrels, id)
			h.barrelSize--
			h.log.Debugf("docker log barrel(%s) gc complete", id)
		}
	}
	useTime := time.Now().UnixNano() - t.UnixNano()
	h.log.Debugf("Docker log message store complete gc in %d ns", useTime)
}
func (h *dockerLogStore) stop() {
	h.cancel()
	h.rwLock.RLock()
	defer h.rwLock.RUnlock()
	for k, v := range h.barrels {
		h.saveBeforeGc(k, v)
	}
}

// gc删除前持久化数据
func (h *dockerLogStore) saveBeforeGc(eventID string, v *dockerLogEventBarrel) {
	v.persistencelock.Lock()
	v.gcPersistence()
	if len(v.persistenceBarrel) > 0 {
		if err := h.filePlugin.SaveMessage(v.persistenceBarrel); err != nil {
			h.log.Error("persistence barrel message error.", err.Error())
			h.InsertGarbageMessage(v.persistenceBarrel...)
		}
		h.log.Infof("persistence barrel(%s) %d log message to file.", eventID, len(v.persistenceBarrel))
	}
	v.persistenceBarrel = nil
	v.persistencelock.Unlock()
}
func (h *dockerLogStore) InsertGarbageMessage(message ...*db2.EventLogMessage) {}

// TODD
func (h *dockerLogStore) handleBarrelEvent() {
	for {
		select {
		case event := <-h.barrelEvent:
			if len(event) < 1 {
				return
			}
			h.log.Debug("Handle message store do event.", event)
			if event[0] == "persistence" { //持久化命令
				h.persistence(event)
			}
		case <-h.ctx.Done():
			return
		}
	}
}

func (h *dockerLogStore) persistence(event []string) {
	if len(event) == 2 {
		eventID := event[1]
		h.rwLock.RLock()
		defer h.rwLock.RUnlock()
		if ba, ok := h.barrels[eventID]; ok {
			if ba.needPersistence { //取消异步持久化
				if err := h.filePlugin.SaveMessage(ba.persistenceBarrel); err != nil {
					h.log.Error("persistence barrel message error.", err.Error())
					h.InsertGarbageMessage(ba.persistenceBarrel...)
				}
				h.log.Infof("persistence barrel(%s) %d log message to file.", eventID, len(ba.persistenceBarrel))
				ba.persistenceBarrel = ba.persistenceBarrel[:0]
				ba.needPersistence = false
			}
		}
	}
}

func (h *dockerLogStore) GetHistoryMessage(eventID string, length int) (re []string) {
	h.rwLock.RLock()
	defer h.rwLock.RUnlock()
	if ba, ok := h.barrels[eventID]; ok {
		for _, m := range ba.barrel {
			if len(m.Content) > 0 {
				re = append(re, string(m.Content))
			}
		}
	}
	logrus.Debugf("want length: %d; the length of re: %d;", length, len(re))
	if len(re) >= length && length > 0 {
		return re[:length-1]
	}
	filelength := func() int {
		if length-len(re) > 0 {
			return length - len(re)
		}
		return 0
	}()
	result, err := h.filePlugin.GetMessages(eventID, "", filelength)
	if result == nil || err != nil {
		return re
	}
	re = append(result.([]string), re...)
	return re
}
