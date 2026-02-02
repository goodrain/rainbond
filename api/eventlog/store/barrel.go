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
	"errors"
	"github.com/goodrain/rainbond/api/eventlog/db"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// EventBarrel 事件桶
// 不能在此结构上起协程
type EventBarrel struct {
	eventID           string
	barrel            []*db.EventLogMessage
	persistenceBarrel []*db.EventLogMessage
	needPersistence   bool
	persistencelock   sync.Mutex
	barrelEvent       chan []string
	maxNumber         int64
	cacheNumber       int
	size              int64
	isCallback        bool
	updateTime        time.Time
}

// insert 插入消息
func (e *EventBarrel) insert(m *db.EventLogMessage) error {
	if e.size > e.maxNumber {
		return errors.New("received message number more than peer event max message number")
	}
	if m.Step == "progress" { //进度日志不存储
		return nil
	}
	e.barrel = append(e.barrel, m)
	e.size++      //消息数据总数
	e.analysis(m) //同步分析
	if len(e.barrel) >= e.cacheNumber {
		e.persistence()
	}
	e.updateTime = time.Now()
	return nil
}

// 值归零，放回对象池
func (e *EventBarrel) empty() {
	e.size = 0
	e.eventID = ""
	e.barrel = e.barrel[:0]
	e.persistenceBarrel = e.persistenceBarrel[:0]
	e.needPersistence = false
	e.isCallback = false
}

// analysis 实时分析
func (e *EventBarrel) analysis(newMessage *db.EventLogMessage) {
	if newMessage.Step == "last" || newMessage.Step == "callback" {
		e.persistence()
		e.barrelEvent <- []string{"callback", e.eventID, newMessage.Status, newMessage.Message}
	}
	if newMessage.Step == "code-version" {
		e.barrelEvent <- []string{"code-version", e.eventID, newMessage.Message}
	}
}

// persistence 持久化
func (e *EventBarrel) persistence() {
	e.persistencelock.Lock()
	defer e.persistencelock.Unlock()
	e.needPersistence = true
	e.persistenceBarrel = append(e.persistenceBarrel, e.barrel...) //数据转到持久化等候队列
	e.barrel = e.barrel[:0]                                        //缓存队列清空
	select {
	case e.barrelEvent <- []string{"persistence", e.eventID}: //发出持久化命令
	default:
		logrus.Debug("event message log persistence delay")
	}
}

// 调用者加锁
func (e *EventBarrel) gcPersistence() {
	e.needPersistence = true
	e.persistenceBarrel = append(e.persistenceBarrel, e.barrel...) //数据转到持久化等候队列
	e.barrel = nil
}

type readEventBarrel struct {
	// 移除barrel数组，不再在内存中缓存消息
	// barrel        []*db.EventLogMessage
	subSocketChan map[string]chan *db.EventLogMessage
	subLock       sync.Mutex
	updateTime    time.Time
	// 新增字段
	eventID   string     // 事件ID
	fileStore FileStore  // 文件存储
}

func (r *readEventBarrel) empty() {
	r.subLock.Lock()
	defer r.subLock.Unlock()

	// 关闭所有订阅通道
	for _, ch := range r.subSocketChan {
		close(ch)
	}
	r.subSocketChan = make(map[string]chan *db.EventLogMessage)

	// 注意：不删除文件，保留供后续查询
	// 如果需要删除文件，可以在这里调用：
	// if r.fileStore != nil && r.eventID != "" {
	//     r.fileStore.Delete(r.eventID)
	// }
}

func (r *readEventBarrel) insertMessage(message *db.EventLogMessage) {
	r.updateTime = time.Now()

	// 1. 立即持久化到文件（不在内存累积）
	if r.fileStore != nil && message != nil {
		if err := r.fileStore.Append(r.eventID, message); err != nil {
			logrus.Errorf("Failed to append message to file for event %s: %v", r.eventID, err)
		}
	}

	// 2. 只转发给当前活跃的订阅者（不缓存）
	r.subLock.Lock()
	defer r.subLock.Unlock()

	for _, v := range r.subSocketChan {
		select {
		case v <- message:
		default:
			// 通道满则丢弃，避免阻塞
		}
	}
}

func (r *readEventBarrel) pushCashMessage(ch chan *db.EventLogMessage, subID string) {
	// 从文件读取历史消息并推送
	if r.fileStore != nil && r.eventID != "" {
		// 只推送最近1000条，避免推送过多
		messages, err := r.fileStore.ReadLast(r.eventID, 1000)
		if err != nil {
			logrus.Errorf("Failed to read history for event %s: %v", r.eventID, err)
		} else {
			// 推送历史消息
			for _, m := range messages {
				select {
				case ch <- m:
				case <-time.After(5 * time.Second):
					// 超时保护，避免阻塞
					logrus.Warnf("Timeout pushing history for event %s", r.eventID)
					goto done
				}
			}
		}
	}

done:
	// 注册订阅通道
	r.subLock.Lock()
	defer r.subLock.Unlock()
	r.subSocketChan[subID] = ch
}

// 增加socket订阅
func (r *readEventBarrel) addSubChan(subID string) chan *db.EventLogMessage {
	r.subLock.Lock()
	if sub, ok := r.subSocketChan[subID]; ok {
		r.subLock.Unlock()
		return sub
	}
	r.subLock.Unlock()

	ch := make(chan *db.EventLogMessage, 10)
	// 异步推送历史消息（从文件读取）
	go r.pushCashMessage(ch, subID)
	return ch
}

// 删除socket订阅
func (r *readEventBarrel) delSubChan(subID string) {
	r.subLock.Lock()
	defer r.subLock.Unlock()
	if ch, ok := r.subSocketChan[subID]; ok {
		close(ch)
		delete(r.subSocketChan, subID)
	}
}

type dockerLogEventBarrel struct {
	name            string
	// 移除barrel数组，使用流式处理
	// barrel          []*db.EventLogMessage
	subSocketChan   map[string]chan *db.EventLogMessage
	subLock         sync.Mutex
	updateTime      time.Time
	size            int
	cacheSize       int64
	persistencelock sync.Mutex
	persistenceTime time.Time
	// 移除持久化相关字段，改用直接写文件
	// needPersistence   bool
	// persistenceBarrel []*db.EventLogMessage
	// barrelEvent       chan []string
	// 新增文件存储
	fileStore FileStore
}

func (r *dockerLogEventBarrel) empty() {
	r.subLock.Lock()
	defer r.subLock.Unlock()

	// 关闭所有订阅通道
	for _, ch := range r.subSocketChan {
		close(ch)
	}
	r.subSocketChan = make(map[string]chan *db.EventLogMessage)
	r.size = 0
	r.name = ""
	// 不删除文件，保留供查询
}

func (r *dockerLogEventBarrel) insertMessage(message *db.EventLogMessage) {
	if r.name == "" {
		r.name = message.EventID
	}
	r.updateTime = time.Now()
	r.size++

	// 1. 立即持久化到文件（流式处理）
	if r.fileStore != nil && message != nil {
		if err := r.fileStore.Append(r.name, message); err != nil {
			logrus.Errorf("Failed to append docker log for %s: %v", r.name, err)
		}
	}

	// 2. 转发给订阅者（不缓存）
	r.subLock.Lock()
	defer r.subLock.Unlock()

	for _, v := range r.subSocketChan {
		select {
		case v <- message:
		default:
			// 通道满则丢弃
		}
	}
}

func (r *dockerLogEventBarrel) pushCashMessage(ch chan *db.EventLogMessage, subID string) {
	// 从文件读取历史消息并推送
	if r.fileStore != nil && r.name != "" {
		// Docker日志可能很多，只推送最近1000条
		messages, err := r.fileStore.ReadLast(r.name, 1000)
		if err != nil {
			logrus.Errorf("Failed to read docker log history for %s: %v", r.name, err)
		} else {
			for _, m := range messages {
				select {
				case ch <- m:
				case <-time.After(5 * time.Second):
					logrus.Warnf("Timeout pushing docker log history for %s", r.name)
					goto done
				}
			}
		}
	}

done:
	// 注册订阅
	r.subLock.Lock()
	defer r.subLock.Unlock()
	r.subSocketChan[subID] = ch
}

// 增加socket订阅
func (r *dockerLogEventBarrel) addSubChan(subID string) chan *db.EventLogMessage {
	r.subLock.Lock()
	if sub, ok := r.subSocketChan[subID]; ok {
		r.subLock.Unlock()
		return sub
	}
	r.subLock.Unlock()

	ch := make(chan *db.EventLogMessage, 100)
	// 异步推送历史
	go r.pushCashMessage(ch, subID)
	return ch
}

// 删除socket订阅
func (r *dockerLogEventBarrel) delSubChan(subID string) {
	r.subLock.Lock()
	defer r.subLock.Unlock()
	if ch, ok := r.subSocketChan[subID]; ok {
		close(ch)
		delete(r.subSocketChan, subID)
	}
}

// persistence 废弃：现在使用流式处理，消息立即写入文件
// 保留方法以避免编译错误，但不再使用
func (r *dockerLogEventBarrel) persistence() {
	r.persistencelock.Lock()
	defer r.persistencelock.Unlock()
	r.persistenceTime = time.Now()
	// 不再需要批量持久化，insertMessage已经实时写文件
}

// gcPersistence 废弃：保留以兼容现有调用
func (r *dockerLogEventBarrel) gcPersistence() {
	// 不再需要
}
func (r *dockerLogEventBarrel) GetSubChanLength() int {
	r.subLock.Lock()
	defer r.subLock.Unlock()
	return len(r.subSocketChan)
}

type monitorMessageBarrel struct {
	barrel        []*db.EventLogMessage
	subSocketChan map[string]chan *db.EventLogMessage
	subLock       sync.Mutex
	updateTime    time.Time
}

func (r *monitorMessageBarrel) empty() {
	r.subLock.Lock()
	defer r.subLock.Unlock()
	if r.barrel != nil {
		r.barrel = r.barrel[:0]
	}
	for _, ch := range r.subSocketChan {
		close(ch)
	}
	r.subSocketChan = make(map[string]chan *db.EventLogMessage)
}

func (r *monitorMessageBarrel) insertMessage(message *db.EventLogMessage) {
	r.updateTime = time.Now()
	r.subLock.Lock()
	defer r.subLock.Unlock()
	for _, v := range r.subSocketChan { //向订阅的通道发送消息
		select {
		case v <- message:
		default:
		}
	}
}

func (r *monitorMessageBarrel) pushCashMessage(ch chan *db.EventLogMessage, subID string) {
	r.subLock.Lock()
	defer r.subLock.Unlock()
	r.subSocketChan[subID] = ch
}

// 增加socket订阅
func (r *monitorMessageBarrel) addSubChan(subID string) chan *db.EventLogMessage {
	r.subLock.Lock()
	defer r.subLock.Unlock()
	if sub, ok := r.subSocketChan[subID]; ok {
		return sub
	}
	ch := make(chan *db.EventLogMessage, 10)
	go r.pushCashMessage(ch, subID)
	return ch
}

// 删除socket订阅
func (r *monitorMessageBarrel) delSubChan(subID string) {
	r.subLock.Lock()
	defer r.subLock.Unlock()
	if ch, ok := r.subSocketChan[subID]; ok {
		close(ch)
		delete(r.subSocketChan, subID)
	}
}
