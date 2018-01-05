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

import (
	"context"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/goodrain/rainbond/pkg/eventlog/conf"
	"github.com/goodrain/rainbond/pkg/eventlog/db"
)

type newMonitorMessageStore struct {
	conf        conf.EventStoreConf
	log         *logrus.Entry
	barrels     map[string]*CacheMonitorMessageList
	lock        sync.RWMutex
	cancel      func()
	ctx         context.Context
	size        int64
	allLogCount float64
}

func (h *newMonitorMessageStore) Scrape(ch chan<- prometheus.Metric, namespace, exporter, from string) error {
	chanDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, exporter, "new_monitor_store_barrel_count"),
		"the handle container log count size.",
		[]string{"from"}, nil,
	)
	ch <- prometheus.MustNewConstMetric(chanDesc, prometheus.GaugeValue, float64(len(h.barrels)), from)
	logDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, exporter, "new_monitor_store_log_count"),
		"the handle monitor log count size.",
		[]string{"from"}, nil,
	)
	ch <- prometheus.MustNewConstMetric(logDesc, prometheus.GaugeValue, h.allLogCount, from)

	return nil
}
func (h *newMonitorMessageStore) insertMessage(message *db.EventLogMessage) ([]MonitorMessage, bool) {
	h.lock.RLock()
	defer h.lock.RUnlock()
	mm := fromByte(message.MonitorData)
	if len(mm) < 1 {
		return mm, true
	}
	if mm[0].ServiceID == "" || mm[0].Port == "" {
		return mm, true
	}
	if ba, ok := h.barrels[mm[0].ServiceID+mm[0].Port]; ok {
		ba.Insert(mm...)
		return mm, true
	}
	return mm, false
}

func (h *newMonitorMessageStore) InsertMessage(message *db.EventLogMessage) {
	if message == nil || message.EventID == "" {
		return
	}
	//h.log.Debug("Receive a monitor message:" + string(message.Content))
	h.size++
	h.allLogCount++
	mm, ok := h.insertMessage(message)
	if ok {
		return
	}
	h.lock.Lock()
	defer h.lock.Unlock()
	ba := CreateCacheMonitorMessageList(mm[0].ServiceID + mm[0].Port)
	ba.Insert(mm...)
	h.barrels[message.EventID] = ba
}
func (h *newMonitorMessageStore) GetMonitorData() *db.MonitorData {
	data := &db.MonitorData{
		ServiceSize:  len(h.barrels),
		LogSizePeerM: h.size,
	}
	return data
}

func (h *newMonitorMessageStore) SubChan(eventID, subID string) chan *db.EventLogMessage {
	h.lock.Lock()
	defer h.lock.Unlock()
	if ba, ok := h.barrels[eventID]; ok {
		return ba.addSubChan(subID)
	}
	ba := CreateCacheMonitorMessageList(eventID)
	h.barrels[eventID] = ba
	return ba.addSubChan(subID)
}
func (h *newMonitorMessageStore) RealseSubChan(eventID, subID string) {
	h.lock.RLock()
	defer h.lock.RUnlock()
	if ba, ok := h.barrels[eventID]; ok {
		ba.delSubChan(subID)
	}
}
func (h *newMonitorMessageStore) Run() {
	go h.Gc()
}
func (h *newMonitorMessageStore) Gc() {
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
			if v.UpdateTime.Add(time.Minute * 3).Before(time.Now()) { // barrel 超时未收到消息
				gcEvent = append(gcEvent, k)
			}
		}
		if gcEvent != nil && len(gcEvent) > 0 {
			for _, id := range gcEvent {
				barrel := h.barrels[id]
				barrel.empty()
				delete(h.barrels, id)
			}
		}
	}
}
func (h *newMonitorMessageStore) stop() {
	h.cancel()
}
func (h *newMonitorMessageStore) InsertGarbageMessage(message ...*db.EventLogMessage) {}

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

//cacheMonitorMessage 每个实例的数据缓存
type cacheMonitorMessage struct {
	updateTime time.Time
	hostName   string
	mms        []MonitorMessage
}

//CacheMonitorMessageList 某个应用性能分析数据
type CacheMonitorMessageList struct {
	list          []*cacheMonitorMessage
	subSocketChan map[string]chan *db.EventLogMessage
	subLock       sync.Mutex
	message       db.EventLogMessage
	UpdateTime    time.Time
}

//CreateCacheMonitorMessageList 创建应用监控信息缓存器
func CreateCacheMonitorMessageList(eventID string) *CacheMonitorMessageList {
	return &CacheMonitorMessageList{
		subSocketChan: make(map[string]chan *db.EventLogMessage),
		message: db.EventLogMessage{
			EventID: eventID,
		},
	}
}

//Insert 认为mms的hostname一致
//每次收到消息进行gc
func (c *CacheMonitorMessageList) Insert(mms ...MonitorMessage) {
	if mms == nil || len(mms) < 1 {
		return
	}
	c.UpdateTime = time.Now()
	hostname := mms[0].HostName
	for i := range c.list {
		cm := c.list[i]
		if cm.hostName == hostname {
			cm.updateTime = time.Now()
			cm.mms = mms
			break
		}
	}
	c.Gc()
	c.pushMessage()
}

//Gc 清理数据
func (c *CacheMonitorMessageList) Gc() {
	var list []*cacheMonitorMessage
	for i := range c.list {
		cmm := c.list[i]
		if !cmm.updateTime.Add(time.Minute * 5).Before(time.Now()) {
			list = append(list, cmm)
		}
	}
	c.list = list
}

func (c *CacheMonitorMessageList) pushMessage() {
	if len(c.list) == 0 {
		return
	}
	var mdata []byte
	if len(c.list) == 1 {
		mdata = getByte(c.list[0].mms)
	}
	source := c.list[0].mms
	for i := 1; i < len(c.list); i++ {
		addSource := c.list[i].mms
		source = merge(source, addSource)
	}
	mdata = getByte(source)
	c.message.MonitorData = mdata
	for _, ch := range c.subSocketChan {
		select {
		case ch <- &c.message:
		default:
		}
	}
}

// 增加socket订阅
func (c *CacheMonitorMessageList) addSubChan(subID string) chan *db.EventLogMessage {
	c.subLock.Lock()
	defer c.subLock.Unlock()
	if sub, ok := c.subSocketChan[subID]; ok {
		return sub
	}
	ch := make(chan *db.EventLogMessage, 10)
	c.pushMessage()
	return ch
}

//删除socket订阅
func (c *CacheMonitorMessageList) delSubChan(subID string) {
	c.subLock.Lock()
	defer c.subLock.Unlock()
	if _, ok := c.subSocketChan[subID]; ok {
		delete(c.subSocketChan, subID)
	}
}
func (c *CacheMonitorMessageList) empty() {
	c.subLock.Lock()
	defer c.subLock.Unlock()
	for _, v := range c.subSocketChan {
		close(v)
	}
}
func getByte(source []MonitorMessage) []byte {
	b, _ := ffjson.Marshal(source)
	return b
}
func fromByte(source []byte) []MonitorMessage {
	var mm []MonitorMessage
	ffjson.Unmarshal(source, &mm)
	return mm
}

func merge(source, addsource []MonitorMessage) (result []MonitorMessage) {
	var cache = make(map[string]*MonitorMessage)
	for _, mm := range source {
		cache[mm.Key] = &mm
	}
	for _, mm := range addsource {
		if oldmm, ok := cache[mm.Key]; ok {
			oldmm.Count += mm.Count
			oldmm.AbnormalCount += mm.AbnormalCount
			//平均时间
			oldmm.AverageTime = (oldmm.AverageTime + mm.AverageTime) / 2
			//累积时间
			oldmm.CumulativeTime = oldmm.CumulativeTime + mm.CumulativeTime
			//最大时间
			if mm.MaxTime > oldmm.MaxTime {
				oldmm.MaxTime = mm.MaxTime
			}
			continue
		}
		cache[mm.Key] = &mm
	}
	for _, c := range cache {
		result = append(result, *c)
	}
	return
}
