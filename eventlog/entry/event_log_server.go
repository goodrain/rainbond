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

package entry

import (
	"errors"
	"github.com/goodrain/rainbond/eventlog/conf"
	grpcserver "github.com/goodrain/rainbond/eventlog/entry/grpc/server"
	"github.com/goodrain/rainbond/eventlog/store"
	"time"

	"golang.org/x/net/context"

	"fmt"

	"sync"

	"github.com/Sirupsen/logrus"
	zmq4 "github.com/pebbe/zmq4"
)

//EventLogServer 日志接受服务
type EventLogServer struct {
	conf               conf.EventLogServerConf
	log                *logrus.Entry
	cancel             func()
	context            context.Context
	server             *zmq4.Socket
	storemanager       store.Manager
	messageChan        chan []byte
	listenErr          chan error
	serverLock         sync.Mutex
	stopReceiveMessage bool
	eventRPCServer     *grpcserver.EventLogRPCServer
}

//NewEventLogServer 创建zmq server服务端
func NewEventLogServer(conf conf.EventLogServerConf, log *logrus.Entry, storeManager store.Manager) (*EventLogServer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &EventLogServer{
		conf:         conf,
		log:          log,
		cancel:       cancel,
		context:      ctx,
		storemanager: storeManager,
		listenErr:    make(chan error),
	}
	server, err := zmq4.NewSocket(zmq4.REP)
	if err != nil {
		s.log.Error("create rep zmq socket error.", err.Error())
		return nil, err
	}
	address := fmt.Sprintf("tcp://%s:%d", s.conf.BindIP, s.conf.BindPort)
	server.Bind(address)
	s.log.Infof("Message server listen %s", address)
	s.server = server
	s.messageChan = s.storemanager.ReceiveMessageChan()
	if s.messageChan == nil {
		return nil, errors.New("receive log message server can not get store message chan ")
	}
	//grpc服务
	eventRPCServer := grpcserver.NewServer(conf, log, storeManager, s.listenErr)
	s.eventRPCServer = eventRPCServer
	return s, nil
}

//Serve 执行
func (s *EventLogServer) Serve() {
	s.eventRPCServer.Start()
	s.handleMessage()
}

//Stop 停止
func (s *EventLogServer) Stop() {
	s.cancel()
	s.eventRPCServer.Stop()
	s.log.Info("receive event message server stop")
}

func (s *EventLogServer) handleMessage() {
	chQuit := make(chan interface{})
	chErr := make(chan error, 2)
	channel := make(chan []byte, s.conf.CacheMessageSize)
	newServerListen := func(sock *zmq4.Socket, channel chan []byte) {
		socketHandler := func(state zmq4.State) error {
			msgs, err := sock.RecvMessageBytes(0)
			if err != nil {
				s.log.Error("docker log server receive message error.", err.Error())
				return err
			}
			_, err = sock.SendMessage("OK") //回复ok
			if err != nil {
				s.log.Error("server reback message error.", err.Error())
				return err
			}
			for _, msg := range msgs {
				channel <- msg
			}
			return nil
		}
		quitHandler := func(interface{}) error {
			close(channel)
			s.log.Infof("Docker container message receive Server quit.")
			return nil
		}
		reactor := zmq4.NewReactor()
		reactor.AddSocket(sock, zmq4.POLLIN, socketHandler)
		reactor.AddChannel(chQuit, 1, quitHandler)
		err := reactor.Run(100 * time.Millisecond)
		chErr <- err
	}
	go newServerListen(s.server, channel)

	func() {
		for !s.stopReceiveMessage {
			select {
			case msg := <-channel:
				s.messageChan <- msg
			case <-s.context.Done():
				s.log.Debug("handle message core begin close.")
				close(chQuit)
				s.stopReceiveMessage = true
				// close(s.messageChan)
			}
		}
	}()
	s.log.Info("Handle message core stop.")
}

//ListenError listen error chan
func (s *EventLogServer) ListenError() chan error {
	return s.listenErr
}
