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

package connect

import (
	"errors"
	"strings"
	"time"

	dis "github.com/goodrain/rainbond/eventlog/cluster/discover"
	"github.com/goodrain/rainbond/eventlog/cluster/distribution"
	"github.com/goodrain/rainbond/eventlog/conf"
	"github.com/goodrain/rainbond/eventlog/store"

	"golang.org/x/net/context"

	"fmt"

	"sync"

	"github.com/goodrain/rainbond/eventlog/db"

	"strconv"

	"github.com/pebbe/zmq4"
	"github.com/sirupsen/logrus"
)

type Sub struct {
	conf           conf.PubSubConf
	log            *logrus.Entry
	cancel         func()
	context        context.Context
	storemanager   store.Manager
	subMessageChan chan [][]byte
	listenErr      chan error
	discover       dis.Manager
	instanceMap    map[string]*dis.Instance
	subClient      map[string]*SubClient
	mapLock        sync.Mutex
	subLock        sync.Mutex
	distribution   *distribution.Distribution
}
type SubClient struct {
	context *zmq4.Context
	socket  *zmq4.Socket
	lock    sync.Mutex
	quit    chan interface{}
}

//NewSub 创建zmq sub客户端
func NewSub(conf conf.PubSubConf, log *logrus.Entry, storeManager store.Manager, discover dis.Manager, distribution *distribution.Distribution) *Sub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Sub{
		conf:           conf,
		log:            log,
		cancel:         cancel,
		context:        ctx,
		storemanager:   storeManager,
		listenErr:      make(chan error),
		discover:       discover,
		instanceMap:    make(map[string]*dis.Instance),
		subClient:      make(map[string]*SubClient),
		subMessageChan: storeManager.SubMessageChan(),
		distribution:   distribution,
	}
}

//Run 执行
func (s *Sub) Run() error {
	go s.instanceListen()
	go s.checkHealth()
	s.log.Info("Message Sub manager start")
	return nil
}

//Stop 停止
func (s *Sub) Stop() {
	s.cancel()
	s.subLock.Lock()
	defer s.subLock.Unlock()
	for _, v := range s.subClient {
		close(v.quit)
	}
	s.log.Info("Message Sub manager stop")
}

func (s *Sub) checkHealth() {
	tike := time.Tick(time.Minute * 2)
	for {
		select {
		case <-s.context.Done():
			return
		case <-tike:
		}
		var unHealth []string
		s.mapLock.Lock()
		s.subLock.Lock()
		for k, ins := range s.instanceMap {
			if _, ok := s.subClient[k]; !ok {
				unHealth = append(unHealth, ins.HostName)
				go s.listen(ins)
			}
		}
		s.subLock.Unlock()
		s.mapLock.Unlock()
		if len(unHealth) == 0 {
			s.log.Debug("sub manager check listen client health: All health.")
		} else {
			s.log.Info("sub manager check listen client health: UnHealth instances:", unHealth)
		}

	}
}
func (s *Sub) instanceListen() {
	for {
		select {
		case instance := <-s.discover.MonitorAddInstances():
			key := fmt.Sprintf("%s:%d", instance.HostIP, instance.PubPort)
			s.mapLock.Lock()
			if _, ok := s.instanceMap[key]; !ok {
				go s.listen(instance)
			}
			s.instanceMap[key] = instance
			s.mapLock.Unlock()
		case instance := <-s.discover.MonitorDelInstances():
			s.log.Debugf("Sub manager receive a del instance %s", instance.HostID)
			key := fmt.Sprintf("%s:%d", instance.HostIP, instance.PubPort)
			s.mapLock.Lock()
			if _, ok := s.instanceMap[key]; ok {
				delete(s.instanceMap, key)
			}
			s.unlisten(instance)
			s.mapLock.Unlock()
		case <-s.context.Done():
			s.mapLock.Lock()
			s.instanceMap = nil
			s.mapLock.Unlock()
			s.log.Debug("Instance listen manager stop.")
			return
		}
	}
}

func (s *Sub) listen(ins *dis.Instance) {
	chQuit := make(chan interface{})
	chErr := make(chan error, 2)
	newInstanceListen := func(sock *zmq4.Socket, instance string) {
		socketHandler := func(state zmq4.State) error {
			msgs, err := sock.RecvMessageBytes(0)
			if err != nil {
				s.log.Error("sub client receive message error.", err.Error())
				return err
			}
			if len(msgs) == 2 {
				if string(msgs[0]) == string(db.EventMessage) || string(msgs[0]) == string(db.ServiceMonitorMessage) || string(msgs[0]) == string(db.ServiceNewMonitorMessage) {
					s.subMessageChan <- msgs
				} else if string(msgs[0]) == string(db.MonitorMessage) {
					//s.log.Debug("Receive a monitor message ", string(msgs[1]))
					data := strings.Split(string(msgs[1]), ",")
					if len(data) == 3 {
						serviceSize, _ := strconv.Atoi(data[1])
						logSize, _ := strconv.Atoi(data[2])
						s.distribution.Update(db.MonitorData{InstanceID: data[0], ServiceSize: serviceSize, LogSizePeerM: int64(logSize)})
					} else {
						s.log.Error("cluster sub receive a message protocol error.")
					}
				} else {
					s.log.Error("cluster sub receive a message protocol error.")
				}
			} else {
				s.log.Error("cluster sub receive a message protocol error.")
			}
			return nil
		}
		quitHandler := func(interface{}) error {
			s.log.Infof("Sub instance %s quit.", instance)
			sock.Close()
			return errors.New("Quit")
		}
		reactor := zmq4.NewReactor()
		reactor.AddSocket(sock, zmq4.POLLIN, socketHandler)
		reactor.AddChannel(chQuit, 0, quitHandler)
		err := reactor.Run(100 * time.Millisecond)
		chErr <- err
	}
	for {
		context, err := zmq4.NewContext()
		if err != nil {
			s.log.Error("create sub context error", err.Error())
			time.Sleep(time.Second * 5)
			continue
		}
		subscriber, err := context.NewSocket(zmq4.SUB)
		if err != nil {
			s.log.Errorf("create sub client to instance %s error %s", ins.HostName, err.Error())
			time.Sleep(time.Second * 5)
			continue
		}
		err = subscriber.Connect(fmt.Sprintf("tcp://%s:%d", ins.HostIP, ins.PubPort))
		if err != nil {
			s.log.Errorf("sub client connect to instance %s host %s port %d error", ins.HostName, ins.HostIP, ins.PubPort)
			time.Sleep(time.Second * 5)
			continue
		}
		subscriber.SetSubscribe("")
		go newInstanceListen(subscriber, ins.HostName)

		key := fmt.Sprintf("%s:%d", ins.HostIP, ins.PubPort)
		s.subLock.Lock()
		client := &SubClient{socket: subscriber, context: context, quit: chQuit}
		s.subClient[key] = client
		s.subLock.Unlock()
		s.log.Infof("message client sub instance %s ", ins.HostName)
		break
	}
}

func (s *Sub) unlisten(ins *dis.Instance) {
	s.subLock.Lock()
	defer s.subLock.Unlock()
	key := fmt.Sprintf("%s:%d", ins.HostIP, ins.PubPort)
	if client, ok := s.subClient[key]; ok {
		client.quit <- true
		close(client.quit)
		delete(s.subClient, key)
	}
}
