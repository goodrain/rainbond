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
	"github.com/goodrain/rainbond/api/eventlog/conf"
	"github.com/goodrain/rainbond/api/eventlog/store"
	util2 "github.com/goodrain/rainbond/api/eventlog/util"
	"net"
	"time"

	"golang.org/x/net/context"

	"fmt"

	"sync"

	zmq4 "github.com/pebbe/zmq4"
	"github.com/sirupsen/logrus"
)

// DockerLogServer 日志接受服务
type DockerLogServer struct {
	conf               conf.DockerLogServerConf
	log                *logrus.Entry
	cancel             func()
	context            context.Context
	server             *zmq4.Socket
	storemanager       store.Manager
	messageChan        chan []byte
	listenErr          chan error
	serverLock         sync.Mutex
	stopReceiveMessage bool
	bufferServer       *util2.Server
	listen             *net.TCPListener
}

// NewDockerLogServer 创建zmq server服务端
func NewDockerLogServer(conf conf.DockerLogServerConf, log *logrus.Entry, storeManager store.Manager) (*DockerLogServer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &DockerLogServer{
		conf:         conf,
		log:          log,
		cancel:       cancel,
		context:      ctx,
		storemanager: storeManager,
		listenErr:    make(chan error),
	}
	s.log.Info("receive docker container log server start.")
	if conf.Mode == "zmq" {
		server, err := zmq4.NewSocket(zmq4.SUB)
		server.SetSubscribe("")
		if err != nil {
			s.log.Error("create rep zmq socket error.", err.Error())
			return nil, err
		}
		address := fmt.Sprintf("tcp://%s:%d", s.conf.BindIP, s.conf.BindPort)
		server.Bind(address)
		s.log.Infof("Docker container log server listen %s", address)
		s.server = server
	} else {
		// creates a tcp listener
		tcpAddr, err := net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:%d", s.conf.BindIP, s.conf.BindPort))
		if err != nil {
			s.log.Error("create stream log server address error.", err.Error())
			return nil, err
		}
		listener, err := net.ListenTCP("tcp", tcpAddr)
		if err != nil {
			s.log.Error("create stream log server listener error.", err.Error())
			return nil, err
		}
		s.listen = listener
		// creates a server
		config := &util2.Config{
			PacketSendChanLimit:    10,
			PacketReceiveChanLimit: 5000,
		}
		s.bufferServer = util2.NewServer(config, s, s.context)
		s.log.Infof("Docker container log server listen %s", tcpAddr)
	}
	s.messageChan = s.storemanager.DockerLogMessageChan()
	if s.messageChan == nil {
		return nil, errors.New("receive log message server can not get store message chan ")
	}
	return s, nil
}

// Serve 执行
func (s *DockerLogServer) Serve() {
	if s.conf.Mode == "zmq" {
		s.handleMessage()
	} else {
		s.bufferServer.Start(s.listen, 3*time.Second)
	}
}

// OnConnect is called when the connection was accepted,
// If the return value of false is closed
func (s *DockerLogServer) OnConnect(c *util2.Conn) bool {
	s.log.Debugf("receive a log client connect.")
	return true
}

// OnMessage is called when the connection receives a packet,
// If the return value of false is closed
func (s *DockerLogServer) OnMessage(p util2.Packet) bool {
	if len(p.Serialize()) > 0 {
		select {
		case s.messageChan <- p.Serialize():
			return true
		default:
			//TODO: return false and receive exist
			return true
		}
	} else {
		logrus.Error("receive a null message")
	}
	return true
}

// OnClose is called when the connection closed
func (s *DockerLogServer) OnClose(*util2.Conn) {
	s.log.Debugf("a log client closed.")
}

// Stop 停止
func (s *DockerLogServer) Stop() {
	s.cancel()
	if s.bufferServer != nil {
		s.bufferServer.Stop()
	}
	s.log.Info("receive event message server stop")
}

func (s *DockerLogServer) handleMessage() {
	chQuit := make(chan interface{})
	chErr := make(chan error, 2)
	channel := make(chan []byte, s.conf.CacheMessageSize)
	newServerListen := func(sock *zmq4.Socket, channel chan []byte) {
		socketHandler := func(state zmq4.State) error {
			msg, err := sock.RecvBytes(0)
			if err != nil {
				s.log.Error("server receive message error.", err.Error())
				return err
			}
			channel <- msg
			return nil
		}
		quitHandler := func(interface{}) error {
			close(channel)
			s.log.Infof("Event message receive Server quit.")
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

// ListenError listen error chan
func (s *DockerLogServer) ListenError() chan error {
	return s.listenErr
}
