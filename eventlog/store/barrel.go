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
	"sync"
	"time"

	"github.com/goodrain/rainbond/eventlog/db"

	"github.com/Sirupsen/logrus"
)

//EventBarrel 事件桶
//不能在此结构上起协程
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

//insert 插入消息
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

//值归零，放回对象池
func (e *EventBarrel) empty() {
	e.size = 0
	e.eventID = ""
	e.barrel = e.barrel[:0]
	e.persistenceBarrel = e.persistenceBarrel[:0]
	e.needPersistence = false
	e.isCallback = false
}

//analysis 实时分析
func (e *EventBarrel) analysis(newMessage *db.EventLogMessage) {
	if newMessage.Step == "last" || newMessage.Step == "callback" {
		e.persistence()
		e.barrelEvent <- []string{"callback", e.eventID, newMessage.Status, newMessage.Message}
	}
	if newMessage.Step == "code-version" {
		e.barrelEvent <- []string{"code-version", e.eventID, newMessage.Message}
	}
}

//persistence 持久化
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

//调用者加锁
func (e *EventBarrel) gcPersistence() {
	e.needPersistence = true
	e.persistenceBarrel = append(e.persistenceBarrel, e.barrel...) //数据转到持久化等候队列
	e.barrel = nil
}

type readEventBarrel struct {
	barrel        []*db.EventLogMessage
	subSocketChan map[string]chan *db.EventLogMessage
	subLock       sync.Mutex
	updateTime    time.Time
}

func (r *readEventBarrel) empty() {
	r.subLock.Lock()
	defer r.subLock.Unlock()
	if r.barrel != nil {
		r.barrel = r.barrel[:0]
	}
	//关闭订阅chan
	for _, ch := range r.subSocketChan {
		close(ch)
	}
	r.subSocketChan = make(map[string]chan *db.EventLogMessage)
}

func (r *readEventBarrel) insertMessage(message *db.EventLogMessage) {
	r.barrel = append(r.barrel, message)
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

func (r *readEventBarrel) pushCashMessage(ch chan *db.EventLogMessage, subID string) {
	r.subLock.Lock()
	defer r.subLock.Unlock()
	// for _, m := range r.barrel {
	// 	select {
	// 	case ch <- m:
	// 	default:
	// 	}
	// }
	r.subSocketChan[subID] = ch
}

//增加socket订阅
func (r *readEventBarrel) addSubChan(subID string) chan *db.EventLogMessage {
	r.subLock.Lock()
	defer r.subLock.Unlock()
	if sub, ok := r.subSocketChan[subID]; ok {
		return sub
	}
	ch := make(chan *db.EventLogMessage, 10)
	go r.pushCashMessage(ch, subID)
	return ch
}

//删除socket订阅
func (r *readEventBarrel) delSubChan(subID string) {
	r.subLock.Lock()
	defer r.subLock.Unlock()
	if _, ok := r.subSocketChan[subID]; ok {
		delete(r.subSocketChan, subID)
	}
}

type dockerLogEventBarrel struct {
	name              string
	barrel            []*db.EventLogMessage
	subSocketChan     map[string]chan *db.EventLogMessage
	subLock           sync.Mutex
	updateTime        time.Time
	size              int
	cacheSize         int64
	persistencelock   sync.Mutex
	persistenceTime   time.Time
	needPersistence   bool
	persistenceBarrel []*db.EventLogMessage
	barrelEvent       chan []string
}

func (r *dockerLogEventBarrel) empty() {
	r.subLock.Lock()
	defer r.subLock.Unlock()
	if r.barrel != nil {
		r.barrel = r.barrel[:0]
	}
	for _, ch := range r.subSocketChan {
		close(ch)
	}
	r.subSocketChan = make(map[string]chan *db.EventLogMessage)
	r.size = 0
	r.name = ""
	r.persistenceBarrel = r.persistenceBarrel[:0]
	r.needPersistence = false
}

func (r *dockerLogEventBarrel) insertMessage(message *db.EventLogMessage) {
	r.subLock.Lock()
	defer r.subLock.Unlock()
	r.barrel = append(r.barrel, message)
	r.updateTime = time.Now()
	for _, v := range r.subSocketChan { //向订阅的通道发送消息
		select {
		case v <- message:
		default:
		}
	}
	r.size++
	if int64(len(r.barrel)) >= r.cacheSize {
		r.persistence()
	}
}

func (r *dockerLogEventBarrel) pushCashMessage(ch chan *db.EventLogMessage, subID string) {
	r.subLock.Lock()
	defer r.subLock.Unlock()
	// send cache log will cause user illusion
	// for _, m := range r.barrel {
	// 	select {
	// 	case ch <- m:
	// 	default:
	// 	}
	// }
	r.subSocketChan[subID] = ch
}

//增加socket订阅
func (r *dockerLogEventBarrel) addSubChan(subID string) chan *db.EventLogMessage {
	r.subLock.Lock()
	defer r.subLock.Unlock()
	if sub, ok := r.subSocketChan[subID]; ok {
		return sub
	}
	ch := make(chan *db.EventLogMessage, 100)
	go r.pushCashMessage(ch, subID)
	return ch
}

//删除socket订阅
func (r *dockerLogEventBarrel) delSubChan(subID string) {
	r.subLock.Lock()
	defer r.subLock.Unlock()
	if _, ok := r.subSocketChan[subID]; ok {
		delete(r.subSocketChan, subID)
	}
}

//persistence 持久化
func (r *dockerLogEventBarrel) persistence() {
	r.persistencelock.Lock()
	defer r.persistencelock.Unlock()
	r.needPersistence = true
	r.persistenceBarrel = append(r.persistenceBarrel, r.barrel...) //数据转到持久化等候队列
	r.barrel = r.barrel[:0]                                        //缓存队列清空
	select {
	case r.barrelEvent <- []string{"persistence", r.name}: //发出持久化命令
		r.persistenceTime = time.Now()
	default:
		logrus.Debug("docker log persistence delay")
	}
}

//调用者加锁
func (r *dockerLogEventBarrel) gcPersistence() {
	r.needPersistence = true
	r.persistenceBarrel = append(r.persistenceBarrel, r.barrel...) //数据转到持久化等候队列
	r.barrel = nil
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
	//r.barrel = append(r.barrel, message)
	//logrus.Info(string(message.Content))
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
	// for _, m := range r.barrel {
	// 	select {
	// 	case ch <- m:
	// 	default:
	// 	}
	// }
	r.subSocketChan[subID] = ch
}

//增加socket订阅
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

//删除socket订阅
func (r *monitorMessageBarrel) delSubChan(subID string) {
	r.subLock.Lock()
	defer r.subLock.Unlock()
	if _, ok := r.subSocketChan[subID]; ok {
		delete(r.subSocketChan, subID)
	}
}
