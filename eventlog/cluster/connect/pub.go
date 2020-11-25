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
	"fmt"
	"github.com/goodrain/rainbond/eventlog/conf"
	"github.com/goodrain/rainbond/eventlog/db"
	"github.com/goodrain/rainbond/eventlog/store"

	"golang.org/x/net/context"

	"sync"

	"github.com/goodrain/rainbond/eventlog/cluster/discover"

	"github.com/pebbe/zmq4"
	"github.com/sirupsen/logrus"
)

type Pub struct {
	conf           conf.PubSubConf
	log            *logrus.Entry
	cancel         func()
	context        context.Context
	pubServer      *zmq4.Socket
	pubLock        sync.Mutex
	storemanager   store.Manager
	messageChan    chan [][]byte
	listenErr      chan error
	Closed         chan struct{}
	stopPubMessage bool
	discover       discover.Manager
	instance       *discover.Instance
	RadioChan      chan db.ClusterMessage
}

//NewPub 创建zmq pub服务端
func NewPub(conf conf.PubSubConf, log *logrus.Entry, storeManager store.Manager, discover discover.Manager) *Pub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Pub{
		conf:         conf,
		log:          log,
		cancel:       cancel,
		context:      ctx,
		storemanager: storeManager,
		listenErr:    make(chan error),
		Closed:       make(chan struct{}),
		discover:     discover,
		RadioChan:    make(chan db.ClusterMessage, 5),
	}
}

//Run 执行
func (s *Pub) Run() error {
	s.log.Info("message receive server start.")
	pub, err := zmq4.NewSocket(zmq4.PUB)
	if err != nil {
		s.log.Error("create pub zmq socket error.", err.Error())
		return err
	}
	address := fmt.Sprintf("tcp://%s:%d", s.conf.PubBindIP, s.conf.PubBindPort)
	pub.Bind(address)
	s.log.Infof("Message pub server listen %s", address)
	s.pubServer = pub
	s.messageChan = s.storemanager.PubMessageChan()
	if s.messageChan == nil {
		return errors.New("pub log message server can not get store message chan ")
	}
	go s.handleMessage()
	s.registInstance()
	return nil
}

//Stop 停止
func (s *Pub) Stop() {
	if s.instance != nil {
		s.discover.CancellationInstance(s.instance)
	}
	s.cancel()
	<-s.Closed
	s.log.Info("Stop pub message server")
}

func (s *Pub) handleMessage() {
	for !s.stopPubMessage {
		select {
		case msg := <-s.messageChan:
			//s.log.Debugf("Message Pub Server PUB a message %s", string(msg.Content))
			s.pubServer.SendBytes(msg[0], zmq4.SNDMORE)
			s.pubServer.SendBytes(msg[1], 0)
		case m := <-s.RadioChan:
			s.pubServer.SendBytes([]byte(m.Mode), zmq4.SNDMORE)
			s.pubServer.SendBytes(m.Data, 0)
		case <-s.context.Done():
			s.log.Debug("pub message core begin close.")
			s.stopPubMessage = true
			if err := s.pubServer.Close(); err != nil {
				s.log.Warn("Close message pub server error.", err.Error())
			}
			close(s.Closed)
		}
	}
}

func (s *Pub) registInstance() {
	s.instance = s.discover.RegisteredInstance(s.conf.PubBindIP, s.conf.PubBindPort, &s.stopPubMessage)
}
