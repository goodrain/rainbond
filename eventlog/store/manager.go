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
	"strconv"

	"github.com/goodrain/rainbond/eventlog/db"
	coreutil "github.com/goodrain/rainbond/util"

	"github.com/goodrain/rainbond/eventlog/conf"

	"time"

	"strings"

	"fmt"

	"encoding/json"

	"bytes"

	"context"
	"os"
	"path/filepath"

	"github.com/pquerna/ffjson/ffjson"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

// Manager 存储管理器
type Manager interface {
	ReceiveMessageChan() chan []byte
	SubMessageChan() chan [][]byte
	PubMessageChan() chan [][]byte
	DockerLogMessageChan() chan []byte
	GetDockerLogs(serviceID string, length int) []string
	MonitorMessageChan() chan [][]byte
	WebSocketMessageChan(mode, eventID, subID string) chan *db.EventLogMessage
	NewMonitorMessageChan() chan []byte
	RealseWebSocketMessageChan(mode, EventID, subID string)
	Run() error
	Stop()
	Monitor() []db.MonitorData
	Scrape(ch chan<- prometheus.Metric, namespace, exporter, from string) error
	Error() chan error
	HealthCheck() map[string]string
}

// NewManager 存储管理器
func NewManager(conf conf.EventStoreConf, log *logrus.Entry) (Manager, error) {
	conf.DB.Type = "eventfile"
	dbPlugin, err := db.NewManager(conf.DB, log)
	if err != nil {
		return nil, err
	}
	conf.DB.Type = "file"
	filePlugin, err := db.NewManager(conf.DB, log)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	storeManager := &storeManager{
		cancel:                cancel,
		context:               ctx,
		conf:                  conf,
		log:                   log,
		receiveChan:           make(chan []byte, 300),
		subChan:               make(chan [][]byte, 300),
		pubChan:               make(chan [][]byte, 300),
		dockerLogChan:         make(chan []byte, 2048),
		monitorMessageChan:    make(chan [][]byte, 100),
		newmonitorMessageChan: make(chan []byte, 2048),
		chanCacheSize:         100,
		dbPlugin:              dbPlugin,
		filePlugin:            filePlugin,
		errChan:               make(chan error),
	}
	handle := NewStore("handle", storeManager)
	read := NewStore("read", storeManager)
	docker := NewStore("docker_log", storeManager)
	newmonitor := NewStore("newmonitor", storeManager)
	storeManager.handleMessageStore = handle
	storeManager.readMessageStore = read
	storeManager.dockerLogStore = docker
	storeManager.newmonitorMessageStore = newmonitor
	return storeManager, nil
}

type storeManager struct {
	cancel                 func()
	context                context.Context
	handleMessageStore     MessageStore
	readMessageStore       MessageStore
	dockerLogStore         MessageStore
	newmonitorMessageStore MessageStore
	receiveChan            chan []byte
	pubChan, subChan       chan [][]byte
	dockerLogChan          chan []byte
	monitorMessageChan     chan [][]byte
	newmonitorMessageChan  chan []byte
	chanCacheSize          int
	conf                   conf.EventStoreConf
	log                    *logrus.Entry
	dbPlugin               db.Manager
	filePlugin             db.Manager
	errChan                chan error
}

func (s *storeManager) HealthCheck() map[string]string {
	receiveChan := len(s.receiveChan) == 300
	pubChan := len(s.pubChan) == 300
	subChan := len(s.subChan) == 300
	dockerLogChan := len(s.dockerLogChan) == 2048
	monitorMessageChan := len(s.monitorMessageChan) == 100
	newmonitorMessageChan := len(s.newmonitorMessageChan) == 2048
	if receiveChan || pubChan || subChan || dockerLogChan || monitorMessageChan || newmonitorMessageChan {
		return map[string]string{"status": "unusual", "info": "channel blockage"}
	}
	return map[string]string{"status": "health", "info": "eventlog service health"}
}

// Scrape prometheue monitor metrics
// step1: docker log monitor
// step2: event message monitor
// step3: monitor message monitor
var healthStatus float64

func (s *storeManager) Scrape(ch chan<- prometheus.Metric, namespace, exporter, from string) error {

	s.dockerLogStore.Scrape(ch, namespace, exporter, from)
	s.handleMessageStore.Scrape(ch, namespace, exporter, from)
	s.newmonitorMessageStore.Scrape(ch, namespace, exporter, from)
	chanDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, exporter, "chan_cache_size"),
		"the handle chan cache size.",
		[]string{"from", "chan"}, nil,
	)
	var healthDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, exporter, "health_status"),
		"health status.",
		[]string{"service_name"}, nil,
	)
	healthInfo := s.HealthCheck()
	if healthInfo["status"] == "health" {
		healthStatus = 1
	} else {
		healthStatus = 0
	}
	ch <- prometheus.MustNewConstMetric(chanDesc, prometheus.GaugeValue, float64(len(s.dockerLogChan)), from, "container_log")
	ch <- prometheus.MustNewConstMetric(chanDesc, prometheus.GaugeValue, float64(len(s.monitorMessageChan)), from, "monitor_message")
	ch <- prometheus.MustNewConstMetric(chanDesc, prometheus.GaugeValue, float64(len(s.receiveChan)), from, "event_message")
	ch <- prometheus.MustNewConstMetric(healthDesc, prometheus.GaugeValue, healthStatus, "eventlog")
	return nil
}

func (s *storeManager) Monitor() []db.MonitorData {
	data := s.dockerLogStore.GetMonitorData()
	moData := s.newmonitorMessageStore.GetMonitorData()
	if moData != nil {
		data.LogSizePeerM += moData.LogSizePeerM
		data.ServiceSize += moData.ServiceSize
	}
	re := []db.MonitorData{*data}
	dataHand := s.handleMessageStore.GetMonitorData()
	if dataHand != nil {
		re = append(re, *dataHand)
		data.LogSizePeerM += dataHand.LogSizePeerM
		data.ServiceSize += dataHand.ServiceSize
	}
	return re
}

func (s *storeManager) ReceiveMessageChan() chan []byte {
	if s.receiveChan == nil {
		s.receiveChan = make(chan []byte, 300)
	}
	return s.receiveChan
}

func (s *storeManager) SubMessageChan() chan [][]byte {
	if s.subChan == nil {
		s.subChan = make(chan [][]byte, 300)
	}
	return s.subChan
}

func (s *storeManager) PubMessageChan() chan [][]byte {
	if s.pubChan == nil {
		s.pubChan = make(chan [][]byte, 300)
	}
	return s.pubChan
}

func (s *storeManager) DockerLogMessageChan() chan []byte {
	if s.dockerLogChan == nil {
		s.dockerLogChan = make(chan []byte, 2048)
	}
	return s.dockerLogChan
}

func (s *storeManager) MonitorMessageChan() chan [][]byte {
	if s.monitorMessageChan == nil {
		s.monitorMessageChan = make(chan [][]byte, 100)
	}
	return s.monitorMessageChan
}
func (s *storeManager) NewMonitorMessageChan() chan []byte {
	if s.newmonitorMessageChan == nil {
		s.newmonitorMessageChan = make(chan []byte, 2048)
	}
	return s.newmonitorMessageChan
}

func (s *storeManager) WebSocketMessageChan(mode, eventID, subID string) chan *db.EventLogMessage {
	if mode == "event" {
		ch := s.readMessageStore.SubChan(eventID, subID)
		return ch
	}
	if mode == "docker" {
		ch := s.dockerLogStore.SubChan(eventID, subID)
		return ch
	}
	if mode == "newmonitor" {
		ch := s.newmonitorMessageStore.SubChan(eventID, subID)
		return ch
	}
	return nil
}

func (s *storeManager) Run() error {
	s.log.Info("event message store manager start")
	s.handleMessageStore.Run()
	s.readMessageStore.Run()
	s.dockerLogStore.Run()
	s.newmonitorMessageStore.Run()
	for i := 0; i < s.conf.HandleMessageCoreNumber; i++ {
		go s.handleReceiveMessage()
	}
	for i := 0; i < s.conf.HandleSubMessageCoreNumber; i++ {
		go s.handleSubMessage()
	}
	for i := 0; i < s.conf.HandleDockerLogCoreNumber; i++ {
		go s.handleDockerLog()
	}
	go s.handleNewMonitorMessage()
	go s.cleanLog()
	return nil
}

// cleanLog
// clean service log that before 7 days in every 24h
// clean event log that before 30 days message in every 24h
func (s *storeManager) cleanLog() {
	coreutil.Exec(s.context, func() error {
		//do something
		pathname := s.conf.DB.HomePath
		logrus.Infof("start clean history service log %s", pathname)
		files, err := coreutil.GetFileList(pathname, 2)
		if err != nil {
			logrus.Error("list log dir error, ", err.Error())
		} else {
			for _, fi := range files {
				if !strings.Contains(fi, "eventlog") {
					if err := s.deleteFile(fi); err != nil {
						logrus.Errorf("delete log file %s error. %s", fi, err.Error())
					}
				}
			}
		}
		return nil
	}, time.Hour*24)
}

func (s *storeManager) deleteFile(filename string) error {
	now := time.Now()
	if strings.HasSuffix(filename, "stdout.log") || strings.HasSuffix(filename, "stdout-legacy.log") {
		return nil
	}
	name := filepath.Base(filename)
	lis := strings.Split(name, ".")
	if len(lis) < 1 {
		return errors.New("file name format error")
	}
	date := lis[0]
	loc, _ := time.LoadLocation("Local")
	theTime, err := time.ParseInLocation("2006-1-2", date, loc)
	if err != nil {
		return err
	}
	saveDay, _ := strconv.Atoi(os.Getenv("DOCKER_LOG_SAVE_DAY"))
	if saveDay == 0 {
		saveDay = 7
	}
	if now.After(theTime.Add(time.Duration(saveDay) * time.Hour * 24)) {
		if err := os.Remove(filename); err != nil {
			if !strings.Contains(err.Error(), "No such file or directory") {
				return err
			}
		}
		logrus.Debugf("clean service log %s", filename)
	}
	return nil
}

func (s *storeManager) checkHealth() {

}

func (s *storeManager) parsingMessage(msg []byte, messageType string) (*db.EventLogMessage, error) {
	if msg == nil {
		return nil, errors.New("unable parsing nil message")
	}
	//message := s.pool.Get().(*db.EventLogMessage)不能使用对象池，会阻塞进程
	var message db.EventLogMessage
	message.Content = msg
	if messageType == "json" {
		err := ffjson.Unmarshal(msg, &message)
		if err != nil {
			return &message, err
		}
		if message.EventID == "" {
			return &message, errors.New("are not present in the message event_id")
		}
		return &message, nil
	}
	return nil, errors.New("unable to process configuration of message format type")
}

// handleNewMonitorMessage 处理新监控数据
func (s *storeManager) handleNewMonitorMessage() {
loop:
	for {
		select {
		case <-s.context.Done():
			return
		case msg, ok := <-s.newmonitorMessageChan:
			if !ok {
				s.log.Error("handle new monitor message core stop.monitor message log chan closed")
				break loop
			}
			if msg == nil {
				continue
			}
			//s.log.Debugf("receive message %s", string(message.Content))
			if s.conf.ClusterMode {
				//消息直接集群共享
				s.pubChan <- [][]byte{[]byte(db.ServiceNewMonitorMessage), msg}
			}
			s.newmonitorMessageStore.InsertMessage(&db.EventLogMessage{MonitorData: msg})
		}
	}
	s.errChan <- fmt.Errorf("handle monitor log core exist")
}

func (s *storeManager) handleReceiveMessage() {
	s.log.Debug("event message store manager start handle receive message")
loop:
	for {
		select {
		case <-s.context.Done():
			return
		case msg, ok := <-s.receiveChan:
			if !ok {
				s.log.Error("handle receive message core stop. receive chan closed")
				break loop
			}
			if msg == nil {
				s.log.Debug("handle receive message core stop.")
				continue
			}
			//s.log.Debugf("receive message %s", string(message.Content))
			if s.conf.ClusterMode {
				//消息直接集群共享
				s.pubChan <- [][]byte{[]byte(db.EventMessage), msg}
			}
			message, err := s.parsingMessage(msg, s.conf.MessageType)
			if err != nil {
				s.log.Error("parsing the message before insert message error.", err.Error())
				if message != nil {
					s.handleMessageStore.InsertGarbageMessage(message)
				}
				continue
			}
			//s.log.Debug("Receive Message:", string(message.Content))
			s.handleMessageStore.InsertMessage(message)
			s.readMessageStore.InsertMessage(message)
		}
	}
	s.errChan <- fmt.Errorf("handle monitor log core exist")
}

func (s *storeManager) handleSubMessage() {
	s.log.Debug("event message store manager start handle sub message")
	for {
		select {
		case <-s.context.Done():
			return
		case msg, ok := <-s.subChan:
			if !ok {
				s.log.Debug("handle sub message core stop.receive chan closed")
				return
			}
			if msg == nil {
				continue
			}
			if len(msg) == 2 {
				if string(msg[0]) == string(db.ServiceNewMonitorMessage) {
					s.newmonitorMessageStore.InsertMessage(&db.EventLogMessage{MonitorData: msg[1]})
					continue
				}
				//s.log.Debugf("receive sub message %s", string(msg))
				message, err := s.parsingMessage(msg[1], s.conf.MessageType)
				if err != nil {
					s.log.Error("parsing the message before insert message error.", err.Error())
					continue
				}
				if string(msg[0]) == string(db.EventMessage) {
					s.readMessageStore.InsertMessage(message)
				}
			}
		}
	}
}

type containerLog struct {
	ContainerID string          `json:"container_id"`
	ServiceID   string          `json:"service_id"`
	Msg         string          `json:"msg"`
	Time        json.RawMessage `json:"time"`
}

const (
	// RBDSERVICEIDAPI rbd service id api
	RBDSERVICEIDAPI = "rbd-api"
	// RBDSERVICEIDWORKER rbd service id worker
	RBDSERVICEIDWORKER = "rbd-worker"
	// RBDSERVICEIDGATEWAY rbd service id gateway
	RBDSERVICEIDGATEWAY = "rbd-gateway"
	// RBDSERVICEIDCHAOS rbd service id chaos
	RBDSERVICEIDCHAOS = "rbd-chaos"
)

func (s *storeManager) handleDockerLog() {
	s.log.Debug("event message store manager start handle docker container log message")
loop:
	for {
		select {
		case <-s.context.Done():
			return
		case m, ok := <-s.dockerLogChan:
			if !ok {
				s.log.Error("handle docker log message core stop.docker log chan closed")
				break loop
			}
			if m == nil {
				continue
			}
			if len(m) < 47 {
				continue
			}
			containerID := m[0:12]        //0-12
			serviceID := string(m[13:45]) //13-45
			// rbd logs
			switch {
			case strings.Contains(serviceID, RBDSERVICEIDAPI):
				serviceID = RBDSERVICEIDAPI
			case strings.Contains(serviceID, RBDSERVICEIDWORKER):
				serviceID = RBDSERVICEIDWORKER
			case strings.Contains(serviceID, RBDSERVICEIDGATEWAY):
				serviceID = RBDSERVICEIDGATEWAY
			case strings.Contains(serviceID, RBDSERVICEIDCHAOS):
				serviceID = RBDSERVICEIDCHAOS
			}
			log := m[13+len(serviceID):]
			logrus.Debugf("containerID [%s] serviceID [%s] log [%s]", containerID, serviceID, string(log))
			buffer := bytes.NewBuffer(containerID)
			buffer.WriteString(":")
			buffer.Write(log)
			message := db.EventLogMessage{
				Message: buffer.String(),
				Content: buffer.Bytes(),
				EventID: serviceID,
			}
			s.dockerLogStore.InsertMessage(&message)
			buffer.Reset()
		}
	}
	s.errChan <- fmt.Errorf("handle docker log core exist")
}

type event struct {
	Name   string        `json:"name"`
	Data   []interface{} `json:"data"`
	Update string        `json:"update_time"`
}

func (s *storeManager) RealseWebSocketMessageChan(mode string, eventID, subID string) {
	if mode == "event" {
		s.readMessageStore.RealseSubChan(eventID, subID)
	}
	if mode == "docker" {
		s.dockerLogStore.RealseSubChan(eventID, subID)
	}
	if mode == "newmonitor" {
		s.newmonitorMessageStore.RealseSubChan(eventID, subID)
	}
}

func (s *storeManager) Stop() {
	s.handleMessageStore.stop()
	s.readMessageStore.stop()
	s.dockerLogStore.stop()
	s.newmonitorMessageStore.stop()
	s.cancel()
	if s.filePlugin != nil {
		s.filePlugin.Close()
	}
	if s.dbPlugin != nil {
		s.dbPlugin.Close()
	}
	s.log.Info("Stop the store manager.")
}
func (s *storeManager) Error() chan error {
	return s.errChan
}

// GetDockerLogs get history docker log
func (s *storeManager) GetDockerLogs(serviceID string, length int) []string {
	return s.dockerLogStore.GetHistoryMessage(serviceID, length)
}
