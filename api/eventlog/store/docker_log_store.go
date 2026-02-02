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
	fileStore    FileStore
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
	ba.fileStore = h.fileStore
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
	ba.fileStore = h.fileStore
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
		barrel := h.barrels[k]

		// 简化GC策略：1分钟未活跃 且 无订阅者
		if barrel.updateTime.Add(time.Minute*1).Before(time.Now()) && barrel.GetSubChanLength() == 0 {
			gcEvent = append(gcEvent, k)
			h.log.Debugf("barrel %s need be gc", k)
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
// saveBeforeGc 废弃：现在使用流式处理，消息立即写入文件
// 保留方法以兼容现有调用
func (h *dockerLogStore) saveBeforeGc(eventID string, v *dockerLogEventBarrel) {
	// 不再需要保存，消息已经实时写入文件
}
func (h *dockerLogStore) InsertGarbageMessage(message ...*db2.EventLogMessage) {}

// TODD
// handleBarrelEvent 废弃：现在使用流式处理，消息立即写入文件
// 保留方法以兼容现有启动流程
func (h *dockerLogStore) handleBarrelEvent() {
	for {
		select {
		case <-h.barrelEvent:
			// 不再处理，消息已经在insertMessage中实时写入
		case <-h.ctx.Done():
			return
		}
	}
}

// persistence 废弃：现在使用流式处理
func (h *dockerLogStore) persistence(event []string) {
	// 不再需要批量持久化
}

func (h *dockerLogStore) GetHistoryMessage(eventID string, length int) (re []string) {
	// 从fileStore读取历史消息
	if h.fileStore != nil {
		messages, err := h.fileStore.ReadLast(eventID, length)
		if err != nil {
			logrus.Errorf("Failed to read history for %s: %v", eventID, err)
		} else {
			for _, m := range messages {
				// 将消息转换为字符串
				re = append(re, m.Message)
			}
		}
	}

	logrus.Debugf("want length: %d; the length of re: %d;", length, len(re))
	if len(re) >= length && length > 0 {
		return re[:length]
	}

	// 如果fileStore没有足够的消息，尝试从旧的filePlugin读取
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
