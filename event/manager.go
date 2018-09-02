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

package event

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/goodrain/rainbond/discover"
	"github.com/goodrain/rainbond/discover/config"
	"github.com/goodrain/rainbond/util"

	"github.com/pquerna/ffjson/ffjson"

	"github.com/Sirupsen/logrus"

	eventclient "github.com/goodrain/rainbond/eventlog/entry/grpc/client"
	eventpb "github.com/goodrain/rainbond/eventlog/entry/grpc/pb"

	"golang.org/x/net/context"
)

//Manager 操作日志，客户端服务
//客户端负载均衡
type Manager interface {
	GetLogger(eventID string) Logger
	Start() error
	Close() error
	ReleaseLogger(Logger)
}
type EventConfig struct {
	EventLogServers []string
	DiscoverAddress []string
}
type manager struct {
	ctx            context.Context
	cancel         context.CancelFunc
	config         EventConfig
	qos            int32
	loggers        map[string]Logger
	handles        map[string]handle
	lock           sync.Mutex
	eventServer    []string
	abnormalServer map[string]string
	dis            discover.Discover
}

var defaultManager Manager

const (
	//REQUESTTIMEOUT  time out
	REQUESTTIMEOUT = 1000 * time.Millisecond
	//MAXRETRIES 重试
	MAXRETRIES = 3 //  Before we abandon
	buffersize = 1000
)

//NewManager 创建manager
func NewManager(conf EventConfig) error {
	dis, err := discover.GetDiscover(config.DiscoverConfig{EtcdClusterEndpoints: conf.DiscoverAddress})
	if err != nil {
		logrus.Error("create discover manager error.", err.Error())
		if len(conf.EventLogServers) < 1 {
			return err
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defaultManager = &manager{
		ctx:            ctx,
		cancel:         cancel,
		config:         conf,
		loggers:        make(map[string]Logger, 1024),
		handles:        make(map[string]handle),
		eventServer:    conf.EventLogServers,
		dis:            dis,
		abnormalServer: make(map[string]string),
	}
	return defaultManager.Start()
}

//GetManager 获取日志服务
func GetManager() Manager {
	return defaultManager
}

//CloseManager 关闭日志服务
func CloseManager() {
	if defaultManager != nil {
		defaultManager.Close()
	}
}

func (m *manager) Start() error {
	m.lock.Lock()
	defer m.lock.Unlock()
	for i := 0; i < len(m.eventServer); i++ {
		h := handle{
			cacheChan: make(chan []byte, buffersize),
			stop:      make(chan struct{}),
			server:    m.eventServer[i],
			manager:   m,
			ctx:       m.ctx,
		}
		m.handles[m.eventServer[i]] = h
		go h.HandleLog()
	}
	if m.dis != nil {
		m.dis.AddProject("event_log_event_grpc", m)
	}
	go m.GC()
	return nil
}

func (m *manager) UpdateEndpoints(endpoints ...*config.Endpoint) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if endpoints == nil || len(endpoints) < 1 {
		return
	}
	logrus.Infof("Update event server endpoint,%+v", endpoints)
	//清空不可用节点信息，以服务发现为主
	m.abnormalServer = make(map[string]string)
	//增加新节点
	var new = make(map[string]string)
	for _, end := range endpoints {
		new[end.URL] = end.URL
		if _, ok := m.handles[end.URL]; !ok {
			h := handle{
				cacheChan: make(chan []byte, buffersize),
				stop:      make(chan struct{}),
				server:    end.URL,
				manager:   m,
				ctx:       m.ctx,
			}
			m.handles[end.URL] = h
			go h.HandleLog()
		}
	}
	//删除旧节点
	for k := range m.handles {
		if _, ok := new[k]; !ok {
			delete(m.handles, k)
		}
	}
	var eventServer []string
	for k := range new {
		eventServer = append(eventServer, k)
	}
	m.eventServer = eventServer
	logrus.Infof("update event handle core success,handle core count:%d, event server count:%d", len(m.handles), len(m.eventServer))

}

func (m *manager) Error(err error) {

}
func (m *manager) Close() error {
	m.cancel()
	if m.dis != nil {
		m.dis.Stop()
	}
	return nil
}

func (m *manager) GC() {
	util.IntermittentExec(m.ctx, func() {
		m.lock.Lock()
		defer m.lock.Unlock()
		var needRelease []string
		for k, l := range m.loggers {
			//1min 未release ,自动gc
			if l.CreateTime().Add(time.Minute).Before(time.Now()) {
				needRelease = append(needRelease, k)
			}
		}
		if len(needRelease) > 0 {
			for _, event := range needRelease {
				logrus.Infof("start auto release event logger. %s", event)
				delete(m.loggers, event)
			}
		}
	}, time.Second*20)
}

//GetLogger
//使用完成后必须调用ReleaseLogger方法
func (m *manager) GetLogger(eventID string) Logger {
	m.lock.Lock()
	defer m.lock.Unlock()
	if eventID == " " || len(eventID) == 0 {
		eventID = "system"
	}
	if l, ok := m.loggers[eventID]; ok {
		return l
	}
	l := &logger{
		event:      eventID,
		sendChan:   m.getLBChan(),
		createTime: time.Now(),
	}
	m.loggers[eventID] = l
	return l
}

func (m *manager) ReleaseLogger(l Logger) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if l, ok := m.loggers[l.Event()]; ok {
		delete(m.loggers, l.Event())
	}
}

type handle struct {
	server    string
	stop      chan struct{}
	cacheChan chan []byte
	ctx       context.Context
	manager   *manager
}

func (m *manager) DiscardedLoggerChan(cacheChan chan []byte) {
	m.lock.Lock()
	defer m.lock.Unlock()
	for k, v := range m.handles {
		if v.cacheChan == cacheChan {
			logrus.Warnf("event server %s can not link, will ignore it.", k)
			m.abnormalServer[k] = k
		}
	}
	for _, v := range m.loggers {
		if v.GetChan() == cacheChan {
			v.SetChan(m.getLBChan())
		}
	}
}

func (m *manager) getLBChan() chan []byte {
	for i := 0; i < len(m.eventServer); i++ {
		index := m.qos % int32(len(m.eventServer))
		m.qos = atomic.AddInt32(&(m.qos), 1)
		server := m.eventServer[index]
		if _, ok := m.abnormalServer[server]; ok {
			continue
		}
		if h, ok := m.handles[server]; ok {
			return h.cacheChan
		}
		h := handle{
			cacheChan: make(chan []byte, buffersize),
			stop:      make(chan struct{}),
			server:    server,
			manager:   m,
			ctx:       m.ctx,
		}
		m.handles[server] = h
		go h.HandleLog()
		return h.cacheChan
	}
	//not select, return first handle chan
	for _, v := range m.handles {
		return v.cacheChan
	}
	return nil
}
func (m *manager) RemoveHandle(server string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if _, ok := m.handles[server]; ok {
		delete(m.handles, server)
	}
}
func (m *handle) HandleLog() error {
	defer m.manager.RemoveHandle(m.server)
	return util.Exec(m.ctx, func() error {
		ctx, cancel := context.WithCancel(m.ctx)
		defer cancel()
		client, err := eventclient.NewEventClient(ctx, m.server)
		if err != nil {
			logrus.Error("create event client error.", err.Error())
			return err
		}
		logrus.Infof("start a event log handle core. connect server %s", m.server)
		logClient, err := client.Log(ctx)
		if err != nil {
			logrus.Error("create event log client error.", err.Error())
			//切换使用此chan的logger到其他chan
			m.manager.DiscardedLoggerChan(m.cacheChan)
			return err
		}
		for {
			select {
			case <-m.ctx.Done():
				logClient.CloseSend()
				return nil
			case <-m.stop:
				logClient.CloseSend()
				return nil
			case me := <-m.cacheChan:
				err := logClient.Send(&eventpb.LogMessage{Log: me})
				if err != nil {
					logrus.Error("send event log error.", err.Error())
					logClient.CloseSend()
					//切换使用此chan的logger到其他chan
					m.manager.DiscardedLoggerChan(m.cacheChan)
					return nil
				}
			}
		}
	}, time.Second*3)
}

func (m *handle) Stop() {
	close(m.stop)
}

//Logger 日志发送器
type Logger interface {
	Info(string, map[string]string)
	Error(string, map[string]string)
	Debug(string, map[string]string)
	Event() string
	CreateTime() time.Time
	GetChan() chan []byte
	SetChan(chan []byte)
	GetWriter(step, level string) LoggerWriter
}

type logger struct {
	event      string
	sendChan   chan []byte
	createTime time.Time
}

func (l *logger) GetChan() chan []byte {
	return l.sendChan
}
func (l *logger) SetChan(ch chan []byte) {
	l.sendChan = ch
}
func (l *logger) Event() string {
	return l.event
}
func (l *logger) CreateTime() time.Time {
	return l.createTime
}
func (l *logger) Info(message string, info map[string]string) {
	if info == nil {
		info = make(map[string]string)
	}
	info["level"] = "info"
	l.send(message, info)
}
func (l *logger) Error(message string, info map[string]string) {
	if info == nil {
		info = make(map[string]string)
	}
	info["level"] = "error"
	l.send(message, info)
}
func (l *logger) Debug(message string, info map[string]string) {
	if info == nil {
		info = make(map[string]string)
	}
	info["level"] = "debug"
	l.send(message, info)
}
func (l *logger) send(message string, info map[string]string) {
	info["event_id"] = l.event
	info["message"] = message
	info["time"] = time.Now().Format(time.RFC3339)
	log, err := ffjson.Marshal(info)
	if err == nil && l.sendChan != nil {
		util.SendNoBlocking(log, l.sendChan)
	}
}

//LoggerWriter logger writer
type LoggerWriter interface {
	io.Writer
	SetFormat(string)
}

func (l *logger) GetWriter(step, level string) LoggerWriter {
	return &loggerWriter{
		l:     l,
		step:  step,
		level: level,
	}
}

type loggerWriter struct {
	l     *logger
	step  string
	level string
	fmt   string
}

func (l *loggerWriter) SetFormat(f string) {
	l.fmt = f
}
func (l *loggerWriter) Write(b []byte) (n int, err error) {
	if b != nil && len(b) > 0 {
		message := string(b)
		message = strings.Replace(message, "\r", "", -1)
		message = strings.Replace(message, "\n", "", -1)
		message = strings.Replace(message, "\u0000", "", -1)
		message = strings.Replace(message, "\"", "", -1)
		if l.fmt != "" {
			message = fmt.Sprintf(l.fmt, message)
		}
		l.l.send(message, map[string]string{"step": l.step, "level": l.level})
	}
	return len(b), nil
}

//GetTestLogger GetTestLogger
func GetTestLogger() Logger {
	return &testLogger{}
}

type testLogger struct {
}

func (l *testLogger) GetChan() chan []byte {
	return nil
}
func (l *testLogger) SetChan(ch chan []byte) {

}
func (l *testLogger) Event() string {
	return "test"
}
func (l *testLogger) CreateTime() time.Time {
	return time.Now()
}
func (l *testLogger) Info(message string, info map[string]string) {
	fmt.Println("info:", message)
}
func (l *testLogger) Error(message string, info map[string]string) {
	fmt.Println("error:", message)
}
func (l *testLogger) Debug(message string, info map[string]string) {
	fmt.Println("debug:", message)
}

type testLoggerWriter struct {
}

func (l *testLoggerWriter) SetFormat(f string) {

}
func (l *testLoggerWriter) Write(b []byte) (n int, err error) {
	return os.Stdout.Write(b)
}

func (l *testLogger) GetWriter(step, level string) LoggerWriter {
	return &testLoggerWriter{}
}
